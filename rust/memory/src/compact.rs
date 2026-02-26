use std::time::Duration;

use giztoy_recall::{
    Segment, all_buckets, bucket_for_span, bucket_lt, compactable_buckets, Bucket,
};

use crate::error::MemoryError;
use crate::memory::Memory;
use crate::types::now_nano;

impl Memory {
    /// Check all compactable buckets and compact any exceeding thresholds.
    /// Cascades: compacting 1h may push into 1d, which may also need compaction.
    /// No-op if no compressor is configured.
    pub async fn compact(&self) -> Result<(), MemoryError> {
        if !self.has_compressor() {
            return Ok(());
        }

        for bucket in compactable_buckets() {
            self.compact_bucket(&bucket).await?;
        }
        Ok(())
    }

    /// Compact a single bucket if it exceeds thresholds.
    async fn compact_bucket(&self, bucket: &Bucket) -> Result<(), MemoryError> {
        let compressor = match self.compressor_ref() {
            Some(c) => c,
            None => return Ok(()),
        };

        let (count, chars) = self.index().bucket_stats(bucket)?;
        if !self.policy().should_compress(chars, count) {
            return Ok(());
        }

        let segments = self.index().bucket_segments(bucket)?;
        if segments.is_empty() {
            return Ok(());
        }

        // Compact at least half to avoid thrashing.
        let mut compact_count = segments.len() / 2;
        if compact_count < 1 {
            compact_count = 1;
        }
        if self.policy().max_messages > 0 && count > self.policy().max_messages {
            let need = count - self.policy().max_messages + 1;
            if need > compact_count {
                compact_count = need;
            }
        }
        if compact_count > segments.len() {
            compact_count = segments.len();
        }

        let to_compact = &segments[..compact_count];

        let summaries: Vec<String> = to_compact.iter().map(|s| s.summary.clone()).collect();
        let result = compressor.compact_segments(&summaries).await?;

        let first_ts = to_compact[0].timestamp;
        let last_ts = to_compact[to_compact.len() - 1].timestamp;
        let span = Duration::from_nanos((last_ts - first_ts) as u64);
        let target_bucket = ensure_coarser(bucket, &bucket_for_span(span));

        let mut next_ts = last_ts;
        for (idx, seg_input) in result.segments.into_iter().enumerate() {
            // Segment storage key is keyed by bucket+timestamp.
            // Ensure each compacted segment has a distinct timestamp to avoid
            // key collisions when a compressor returns multiple segments.
            let seg_ts = if idx == 0 {
                last_ts
            } else {
                next_ts
                    .checked_add(1)
                    .unwrap_or_else(now_nano)
            };
            next_ts = seg_ts;

            let id_ts = now_nano();
            let new_seg = Segment {
                id: format!("{}-{}", self.id(), id_ts),
                summary: seg_input.summary,
                keywords: seg_input.keywords,
                labels: seg_input.labels,
                timestamp: seg_ts,
                bucket: target_bucket.as_str().to_string(),
            };
            self.index().store_segment(&new_seg).await?;
        }

        for seg in to_compact {
            self.index().delete_segment(&seg.id)?;
        }

        Ok(())
    }
}

/// Ensure target bucket is strictly coarser than source.
fn ensure_coarser(source: &Bucket, target: &Bucket) -> Bucket {
    let buckets = all_buckets();
    let source_idx = buckets.iter().position(|b| b == source);
    let target_idx = buckets.iter().position(|b| b == target);

    match (source_idx, target_idx) {
        (Some(si), Some(ti)) if ti > si => target.clone(),
        (Some(si), _) => {
            let next = si + 1;
            if next >= buckets.len() {
                bucket_lt()
            } else {
                buckets[next].clone()
            }
        }
        _ => target.clone(),
    }
}
