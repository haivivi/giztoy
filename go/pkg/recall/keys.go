package recall

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// Key layout (relative to the Index prefix):
//
//	{prefix}:seg:{bucket}:{ts_ns}    → msgpack-encoded Segment
//	{prefix}:sid:{id}                → "{bucket}:{ts_ns}" (reverse index)
//
// Segments are partitioned by bucket (time granularity). Within each
// bucket, segments are ordered by nanosecond timestamp. The sid reverse
// index maps segment ID → "bucket:ts" for O(1) lookups by ID.

// segmentKey builds the KV key for a segment.
// Format: {prefix} + "seg" + {bucket} + "{ts_ns}"
func segmentKey(prefix kv.Key, bucket Bucket, ts int64) kv.Key {
	tsStr := strconv.FormatInt(ts, 10)

	k := make(kv.Key, len(prefix)+3)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	k[len(prefix)+1] = string(bucket)
	k[len(prefix)+2] = tsStr
	return k
}

// segmentPrefix returns the KV prefix for listing all segments across
// all buckets. Format: {prefix} + "seg"
func segmentPrefix(prefix kv.Key) kv.Key {
	k := make(kv.Key, len(prefix)+1)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	return k
}

// bucketPrefix returns the KV prefix for listing segments in a specific
// bucket. Format: {prefix} + "seg" + {bucket}
func bucketPrefix(prefix kv.Key, bucket Bucket) kv.Key {
	k := make(kv.Key, len(prefix)+2)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	k[len(prefix)+1] = string(bucket)
	return k
}

// sidKey returns the KV key for the segment-ID reverse index.
// Format: {prefix} + "sid" + {id}
func sidKey(prefix kv.Key, id string) kv.Key {
	k := make(kv.Key, len(prefix)+2)
	copy(k, prefix)
	k[len(prefix)] = "sid"
	k[len(prefix)+1] = id
	return k
}

// sidValue encodes bucket and timestamp into the sid reverse index value.
// Format: "{bucket}:{ts_ns}"
func sidValue(bucket Bucket, ts int64) []byte {
	return []byte(string(bucket) + ":" + strconv.FormatInt(ts, 10))
}

// parseSidValue decodes a sid reverse index value into bucket and timestamp.
// Handles legacy format (just "{ts_ns}") by defaulting to Bucket1H.
func parseSidValue(data []byte) (Bucket, int64, error) {
	s := string(data)
	if idx := strings.IndexByte(s, ':'); idx >= 0 {
		bucket := Bucket(s[:idx])
		ts, err := strconv.ParseInt(s[idx+1:], 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("recall: malformed sid value timestamp: %w", err)
		}
		return bucket, ts, nil
	}
	// Legacy format: just the timestamp, default to Bucket1H.
	ts, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("recall: malformed sid value: %w", err)
	}
	return Bucket1H, ts, nil
}

// graphPrefix returns the KV prefix for the graph sub-store.
// Format: {prefix} + "g"
func graphPrefix(prefix kv.Key) kv.Key {
	k := make(kv.Key, len(prefix)+1)
	copy(k, prefix)
	k[len(prefix)] = "g"
	return k
}
