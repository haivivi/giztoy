package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/haivivi/giztoy/go/pkg/recall"
)

// Compact checks all compactable buckets and compacts any that exceed
// the configured [CompressPolicy] thresholds. Compaction cascades:
// compacting the 1h bucket may push segments into 1d, which may then
// also need compaction, and so on up to 1y → lt.
//
// Compact is safe to call frequently — it is a no-op for buckets that
// are within their thresholds. Returns nil if no compaction was needed.
//
// If no compressor is configured, Compact returns nil (no-op).
func (m *Memory) Compact(ctx context.Context) error {
	if m.compressor == nil {
		return nil
	}

	for _, bucket := range recall.CompactableBuckets {
		if err := m.CompactBucket(ctx, bucket); err != nil {
			return fmt.Errorf("memory: compact bucket %s: %w", bucket, err)
		}
	}
	return nil
}

// CompactBucket compacts a single bucket if it exceeds the configured
// thresholds. It reads the oldest segments that push the bucket over
// the threshold, feeds their summaries to [Compressor.CompactSegments],
// stores the result in the appropriate target bucket (determined by time
// span), and deletes the compacted source segments.
//
// The target bucket is chosen by [recall.BucketForSpan]: the smallest
// bucket whose duration covers the time span of the compacted segments.
// This means segments may skip levels (e.g., 1h → 1m) if the source
// segments span a long time range.
func (m *Memory) CompactBucket(ctx context.Context, bucket recall.Bucket) error {
	if m.compressor == nil {
		return nil
	}

	count, chars, err := m.index.BucketStats(ctx, bucket)
	if err != nil {
		return err
	}

	if !m.policy.shouldCompress(chars, count) {
		return nil
	}

	// Read all segments in this bucket (oldest first).
	segments, err := m.index.BucketSegments(ctx, bucket)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return nil
	}

	// Determine how many segments to compact: take the oldest ones
	// that bring the bucket back under the threshold.
	// We compact at least half the segments to avoid thrashing.
	compactCount := len(segments) / 2
	if compactCount < 1 {
		compactCount = 1
	}
	// Ensure we leave the bucket under threshold after compaction.
	// If count is over threshold, compact enough to get under.
	if m.policy.MaxMessages > 0 && count > m.policy.MaxMessages {
		need := count - m.policy.MaxMessages + 1
		if need > compactCount {
			compactCount = need
		}
	}
	if compactCount > len(segments) {
		compactCount = len(segments)
	}

	toCompact := segments[:compactCount]

	// Collect summaries for the compressor.
	summaries := make([]string, len(toCompact))
	for i, seg := range toCompact {
		summaries[i] = seg.Summary
	}

	// Run the compactor.
	result, err := m.compressor.CompactSegments(ctx, summaries)
	if err != nil {
		return fmt.Errorf("compact segments: %w", err)
	}

	// Determine target bucket from time span.
	firstTS := toCompact[0].Timestamp
	lastTS := toCompact[len(toCompact)-1].Timestamp
	span := time.Duration(lastTS-firstTS) * time.Nanosecond
	targetBucket := recall.BucketForSpan(span)

	// Ensure the target bucket is coarser than the source.
	// If BucketForSpan returns the same or finer bucket, move to the
	// next coarser one. This prevents segments from staying in the
	// same bucket forever.
	targetBucket = ensureCoarser(bucket, targetBucket)

	// Store the new compacted segments.
	for _, seg := range result.Segments {
		ts := nowNano()
		newSeg := recall.Segment{
			ID:        fmt.Sprintf("%s-%d", m.id, ts),
			Summary:   seg.Summary,
			Keywords:  seg.Keywords,
			Labels:    seg.Labels,
			Timestamp: lastTS, // use the latest timestamp from source segments
			Bucket:    targetBucket,
		}
		if err := m.index.StoreSegment(ctx, newSeg); err != nil {
			return fmt.Errorf("store compacted segment: %w", err)
		}
	}

	// Delete the source segments.
	for _, seg := range toCompact {
		if err := m.index.DeleteSegment(ctx, seg.ID); err != nil {
			return fmt.Errorf("delete source segment %s: %w", seg.ID, err)
		}
	}

	return nil
}

// ensureCoarser ensures the target bucket is strictly coarser than the
// source bucket. If target is the same as or finer than source, it
// returns the next coarser bucket.
func ensureCoarser(source, target recall.Bucket) recall.Bucket {
	buckets := recall.AllBuckets
	sourceIdx := -1
	targetIdx := -1
	for i, b := range buckets {
		if b == source {
			sourceIdx = i
		}
		if b == target {
			targetIdx = i
		}
	}

	if targetIdx <= sourceIdx {
		// Target is same or finer — move to next coarser.
		next := sourceIdx + 1
		if next >= len(buckets) {
			return recall.BucketLT
		}
		return buckets[next]
	}
	return target
}
