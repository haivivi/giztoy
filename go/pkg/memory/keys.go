package memory

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// KV key layout for the memory package.
//
// All keys are scoped under a memory prefix ("mem:{mid}") which is managed
// by the recall.Index for segments and graph data. The memory package adds
// keys for conversations.
//
//	{mid}:conv:{convID}:msg:{ts_ns}   → msgpack Message
//	{mid}:conv:{convID}:revert        → timestamp string (revert point)
//
// The {mid} prefix is set per-Memory instance and ensures complete isolation
// between personas. Segment and graph keys are managed by the recall layer:
//
//	{mid}:seg:{bucket}:{ts_ns}        → msgpack Segment (recall manages)
//	{mid}:sid:{id}                    → "{bucket}:{ts}" (recall manages)
//	{mid}:g:e:{label}                 → Entity (recall/graph manages)
//	{mid}:g:r:{from}:{type}:{to}     → Relation (recall/graph manages)

// memPrefix returns the base KV prefix for a memory ID.
// Format: "mem" + {mid}
func memPrefix(mid string) kv.Key {
	return kv.Key{"mem", mid}
}

// hostMetaKey returns the KV key for host-level metadata.
// Format: "mem" + "__meta" + {name}
func hostMetaKey(name string) kv.Key {
	return kv.Key{"mem", "__meta", name}
}

// convMsgKey builds the KV key for a conversation message.
// Format: "mem" + {mid} + "conv" + {convID} + "msg" + {ts_ns}
//
// The timestamp is zero-padded to 20 digits so that lexicographic KV ordering
// matches numeric ordering. (Without padding, "10000" sorts before "9000".)
func convMsgKey(mid, convID string, ts int64) kv.Key {
	return kv.Key{"mem", mid, "conv", convID, "msg", fmt.Sprintf("%020d", ts)}
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
