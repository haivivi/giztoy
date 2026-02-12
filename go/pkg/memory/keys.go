package memory

import (
	"fmt"
	"strconv"
	"time"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// nanoToTime converts a Unix nanosecond timestamp to time.Time.
func nanoToTime(ns int64) time.Time {
	return time.Unix(0, ns)
}

// KV key layout for the memory package.
//
// All keys are scoped under a memory prefix ("mem:{mid}") which is managed
// by the recall.Index for segments and graph data. The memory package adds
// keys for conversations and long-term summaries.
//
//	{mid}:conv:{convID}:msg:{ts_ns}   → msgpack Message
//	{mid}:conv:{convID}:revert        → timestamp string (revert point)
//	{mid}:long:{grain}:{timeKey}      → summary text
//	{mid}:long:life                    → life summary text
//
// The {mid} prefix is set per-Memory instance and ensures complete isolation
// between personas.

// memPrefix returns the base KV prefix for a memory ID.
// Format: "mem" + {mid}
func memPrefix(mid string) kv.Key {
	return kv.Key{"mem", mid}
}

// convMsgKey builds the KV key for a conversation message.
// Format: "mem" + {mid} + "conv" + {convID} + "msg" + {ts_ns}
func convMsgKey(mid, convID string, ts int64) kv.Key {
	return kv.Key{"mem", mid, "conv", convID, "msg", strconv.FormatInt(ts, 10)}
}

// convMsgPrefix returns the prefix for listing all messages in a conversation.
// Format: "mem" + {mid} + "conv" + {convID} + "msg"
func convMsgPrefix(mid, convID string) kv.Key {
	return kv.Key{"mem", mid, "conv", convID, "msg"}
}

// convRevertKey builds the KV key for a conversation revert point.
// Format: "mem" + {mid} + "conv" + {convID} + "revert"
func convRevertKey(mid, convID string) kv.Key {
	return kv.Key{"mem", mid, "conv", convID, "revert"}
}

// longTermKey builds the KV key for a long-term summary.
// Format: "mem" + {mid} + "long" + {grain} + {timeKey}
//
// The timeKey format varies by grain:
//
//	GrainHour:  "2006010215"     (YYYYMMDDHH)
//	GrainDay:   "20060102"       (YYYYMMDD)
//	GrainWeek:  "2006W01"        (YYYY"W"WW, ISO week)
//	GrainMonth: "200601"         (YYYYMM)
//	GrainYear:  "2006"           (YYYY)
//	GrainLife:  key is different, see longTermLifeKey
func longTermKey(mid string, grain Grain, timeKey string) kv.Key {
	return kv.Key{"mem", mid, "long", grain.String(), timeKey}
}

// longTermPrefix returns the prefix for listing all summaries at a grain.
// Format: "mem" + {mid} + "long" + {grain}
func longTermPrefix(mid string, grain Grain) kv.Key {
	return kv.Key{"mem", mid, "long", grain.String()}
}

// longTermLifeKey builds the KV key for the life summary.
// Format: "mem" + {mid} + "long" + "life"
func longTermLifeKey(mid string) kv.Key {
	return kv.Key{"mem", mid, "long", "life"}
}

// grainTimeKey formats a timestamp into the appropriate time key for a grain.
func grainTimeKey(grain Grain, ts int64) (string, error) {
	t := nanoToTime(ts)
	switch grain {
	case GrainHour:
		return t.UTC().Format("2006010215"), nil
	case GrainDay:
		return t.UTC().Format("20060102"), nil
	case GrainWeek:
		year, week := t.UTC().ISOWeek()
		return fmt.Sprintf("%04dW%02d", year, week), nil
	case GrainMonth:
		return t.UTC().Format("200601"), nil
	case GrainYear:
		return t.UTC().Format("2006"), nil
	default:
		return "", fmt.Errorf("memory: unsupported grain %v for time key", grain)
	}
}
