package mqtt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// TopicSubscriber is a MQTT topic subscriber.
type TopicSubscriber struct {
	Name    string
	Options []SubscribeOption
	Conn    *Conn
}

// Subscribe subscribes to the topic.
func (ts *TopicSubscriber) Subscribe(ctx context.Context) error {
	return ts.Conn.Subscribe(ctx, ts.Name, ts.Options...)
}

// Unsubscribe unsubscribes from the topic.
func (ts *TopicSubscriber) Unsubscribe(ctx context.Context) error {
	return ts.Conn.Unsubscribe(ctx, ts.Name)
}

var _ io.Writer = (*TopicWriter)(nil)

// TopicWriter is a MQTT topic writer.
type TopicWriter struct {
	Name    string
	Options []WriteOption
	Conn    *Conn

	mu            sync.RWMutex
	writeDeadline time.Time
}

// SetWriteDeadline sets the write deadline.
func (tp *TopicWriter) SetWriteDeadline(t time.Time) error {
	if tp == nil {
		return errors.New("mqtt: set write deadline to nil topic writer")
	}
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.writeDeadline = t
	return nil
}

func (tp *TopicWriter) writeContext() (context.Context, context.CancelFunc) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	if tp.writeDeadline.IsZero() {
		return context.Background(), nil
	}
	if tp.writeDeadline.Before(time.Now()) {
		return nil, nil
	}
	return context.WithDeadline(context.Background(), tp.writeDeadline)
}

// Write writes a message to the topic.
func (tp *TopicWriter) Write(b []byte) (int, error) {
	if tp == nil {
		return 0, errors.New("mqtt: write to nil topic writer")
	}
	ctx, cancel := tp.writeContext()
	if ctx == nil {
		return 0, fmt.Errorf("mqtt: write deadline exceeded: %w", context.Canceled)
	}
	if cancel != nil {
		defer cancel()
	}
	if err := tp.Conn.WriteToTopic(ctx, b, tp.Name, tp.Options...); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Publish publishes a message to the topic.
func (tp *TopicWriter) Publish(ctx context.Context, b []byte) error {
	if tp == nil {
		return errors.New("mqtt: publish to nil topic writer")
	}
	return tp.Conn.WriteToTopic(ctx, b, tp.Name, tp.Options...)
}
