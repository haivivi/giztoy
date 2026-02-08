package recall

import (
	"fmt"
	"strconv"
	"time"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// Key layout (relative to the Index prefix):
//
//	{prefix}:seg:{YYYYMMDD}:{ts_ns}  → msgpack-encoded Segment
//	{prefix}:sid:{id}                → timestamp string (reverse index)
//
// The date partition enables efficient time-range scans while the
// nanosecond timestamp ensures uniqueness and total ordering within
// a day. Lexicographic ordering of keys matches chronological order.
//
// The sid (segment-ID) reverse index maps segment ID → timestamp,
// enabling O(1) lookups by ID without scanning all segments.

// segmentKey builds the KV key for a segment.
// Format: {prefix} + "seg" + "YYYYMMDD" + "{ts_ns}"
func segmentKey(prefix kv.Key, ts int64) kv.Key {
	t := time.Unix(0, ts)
	date := t.UTC().Format("20060102")
	tsStr := strconv.FormatInt(ts, 10)

	k := make(kv.Key, len(prefix)+3)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	k[len(prefix)+1] = date
	k[len(prefix)+2] = tsStr
	return k
}

// segmentPrefix returns the KV prefix for listing all segments.
// Format: {prefix} + "seg"
func segmentPrefix(prefix kv.Key) kv.Key {
	k := make(kv.Key, len(prefix)+1)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	return k
}

// SegmentDatePrefix returns the KV prefix for listing segments on a given date.
// This can be used to efficiently scan segments within a specific day without
// loading all segments.
// Format: {prefix} + "seg" + "YYYYMMDD"
func SegmentDatePrefix(prefix kv.Key, date time.Time) kv.Key {
	k := make(kv.Key, len(prefix)+2)
	copy(k, prefix)
	k[len(prefix)] = "seg"
	k[len(prefix)+1] = date.UTC().Format("20060102")
	return k
}

// ParseSegmentKey extracts the nanosecond timestamp from a segment KV key.
// The key must have the form {prefix}:seg:{YYYYMMDD}:{ts_ns} where prefixLen
// is the number of segments in the prefix portion.
// Returns 0 and an error if the key is malformed.
func ParseSegmentKey(key kv.Key, prefixLen int) (int64, error) {
	// Expected: prefixLen segments + "seg" + date + ts_ns = prefixLen + 3
	if len(key) != prefixLen+3 {
		return 0, fmt.Errorf("recall: malformed segment key: expected %d segments, got %d", prefixLen+3, len(key))
	}
	if key[prefixLen] != "seg" {
		return 0, fmt.Errorf("recall: malformed segment key: expected 'seg', got %q", key[prefixLen])
	}
	ts, err := strconv.ParseInt(key[prefixLen+2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("recall: malformed segment key timestamp: %w", err)
	}
	return ts, nil
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

// graphPrefix returns the KV prefix for the graph sub-store.
// Format: {prefix} + "g"
func graphPrefix(prefix kv.Key) kv.Key {
	k := make(kv.Key, len(prefix)+1)
	copy(k, prefix)
	k[len(prefix)] = "g"
	return k
}
