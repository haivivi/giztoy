package memory

import (
	"context"
	"errors"
	"strconv"

	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/recall"
	"github.com/vmihailenco/msgpack/v5"
)

// Conversation is an active dialogue tied to a device or session.
// It stores messages in KV (short-term memory) and provides access to
// recent memory segments via the underlying recall index.
//
// Messages are keyed by nanosecond timestamp for chronological ordering.
// Revert removes the most recent model response and the user message that
// triggered it, enabling a "regenerate" flow.
type Conversation struct {
	store  kv.Store
	index  *recall.Index
	mid    string // memory ID
	convID string
	labels []string
}

func newConversation(store kv.Store, index *recall.Index, mid, convID string, labels []string) *Conversation {
	return &Conversation{
		store:  store,
		index:  index,
		mid:    mid,
		convID: convID,
		labels: labels,
	}
}

// ID returns the conversation identifier (typically a device or session ID).
func (c *Conversation) ID() string { return c.convID }

// Labels returns the entity labels associated with this conversation
// (e.g., ["person:Alice"]).
func (c *Conversation) Labels() []string { return c.labels }

// Append stores a message in the conversation.
// If msg.Timestamp is zero, it is set to the current time.
//
// For user messages, a revert point is saved so that [Revert] can undo
// back to this point.
func (c *Conversation) Append(ctx context.Context, msg Message) error {
	if msg.Timestamp == 0 {
		msg.Timestamp = nowNano()
	}

	data, err := msgpack.Marshal(msg)
	if err != nil {
		return err
	}

	key := convMsgKey(c.mid, c.convID, msg.Timestamp)
	if err := c.store.Set(ctx, key, data); err != nil {
		return err
	}

	// Save revert point on user messages.
	if msg.Role == RoleUser {
		rk := convRevertKey(c.mid, c.convID)
		ts := strconv.FormatInt(msg.Timestamp, 10)
		if err := c.store.Set(ctx, rk, []byte(ts)); err != nil {
			return err
		}
	}

	return nil
}

// Recent returns the n most recent messages in chronological order
// (oldest first). If fewer than n messages exist, all are returned.
func (c *Conversation) Recent(ctx context.Context, n int) ([]Message, error) {
	if n <= 0 {
		return nil, nil
	}

	prefix := convMsgPrefix(c.mid, c.convID)
	var all []Message

	for entry, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		var msg Message
		if err := msgpack.Unmarshal(entry.Value, &msg); err != nil {
			continue
		}
		all = append(all, msg)
	}

	// KV list is ascending by key (chronological). Take the last n.
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// Count returns the total number of messages in the conversation.
func (c *Conversation) Count(ctx context.Context) (int, error) {
	prefix := convMsgPrefix(c.mid, c.convID)
	count := 0
	for _, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// Revert removes the most recent model response(s) and the user message
// that triggered them. This enables a "regenerate last response" flow.
//
// The revert point is the timestamp of the last user message. All messages
// at or after this timestamp are deleted. Returns nil if no revert point
// exists (no user messages have been sent).
func (c *Conversation) Revert(ctx context.Context) error {
	rk := convRevertKey(c.mid, c.convID)
	data, err := c.store.Get(ctx, rk)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil // nothing to revert
		}
		return err
	}

	revertTS, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}

	// Collect keys to delete: all messages with timestamp >= revertTS.
	prefix := convMsgPrefix(c.mid, c.convID)
	var toDelete []kv.Key

	for entry, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return err
		}
		// Extract timestamp from the last key segment.
		tsStr := entry.Key[len(entry.Key)-1]
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		if ts >= revertTS {
			toDelete = append(toDelete, entry.Key)
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	// Delete the messages.
	if err := c.store.BatchDelete(ctx, toDelete); err != nil {
		return err
	}

	// Update the revert point to the previous user message.
	// Scan remaining messages to find the latest user message.
	var latestUserTS int64
	for entry, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return err
		}
		var msg Message
		if err := msgpack.Unmarshal(entry.Value, &msg); err != nil {
			continue
		}
		if msg.Role == RoleUser && msg.Timestamp > latestUserTS {
			latestUserTS = msg.Timestamp
		}
	}

	if latestUserTS > 0 {
		ts := strconv.FormatInt(latestUserTS, 10)
		return c.store.Set(ctx, rk, []byte(ts))
	}
	// No user messages remain; delete the revert key.
	return c.store.Delete(ctx, rk)
}

// RecentSegments returns the n most recent memory segments from the
// underlying recall index. These complement [Recent] messages to give
// the LLM additional historical context beyond the conversation window.
func (c *Conversation) RecentSegments(ctx context.Context, n int) ([]recall.Segment, error) {
	return c.index.RecentSegments(ctx, n)
}

// All returns all messages in chronological order. This is used by the
// compression pipeline to read all messages before compressing them
// into segments.
func (c *Conversation) All(ctx context.Context) ([]Message, error) {
	prefix := convMsgPrefix(c.mid, c.convID)
	var msgs []Message

	for entry, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		var msg Message
		if err := msgpack.Unmarshal(entry.Value, &msg); err != nil {
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// Clear removes all messages and the revert point for this conversation.
func (c *Conversation) Clear(ctx context.Context) error {
	prefix := convMsgPrefix(c.mid, c.convID)
	var keys []kv.Key

	for entry, err := range c.store.List(ctx, prefix) {
		if err != nil {
			return err
		}
		keys = append(keys, entry.Key)
	}

	// Also delete the revert key.
	keys = append(keys, convRevertKey(c.mid, c.convID))

	if len(keys) > 0 {
		return c.store.BatchDelete(ctx, keys)
	}
	return nil
}
