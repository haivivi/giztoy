package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/chatgear"
	"github.com/haivivi/giztoy/pkg/jsontime"
	"github.com/haivivi/giztoy/pkg/mqtt0"
)

// mqttConn implements chatgear.UplinkTx and chatgear.DownlinkRx using mqtt0.
type mqttConn struct {
	client *mqtt0.Client
	ctx    context.Context
	cancel context.CancelFunc
	gearID string
	scope  string // topic prefix (e.g., "namespace/")

	// Downlink channels
	opusFrames chan []byte
	commands   chan *chatgear.SessionCommandEvent

	mu     sync.Mutex
	closed bool
}

// mqttDial connects to an MQTT broker and creates a chatgear connection.
func mqttDial(ctx context.Context, mqttURL, scope, gearID string) (*mqttConn, error) {
	// Normalize scope
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope += "/"
	}

	// Connect to MQTT broker
	client, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:           mqttURL,
		ClientID:       fmt.Sprintf("geartest-%s-%d", gearID, time.Now().UnixNano()%10000),
		KeepAlive:      60,
		ConnectTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	conn := &mqttConn{
		client:     client,
		ctx:        childCtx,
		cancel:     cancel,
		gearID:     gearID,
		scope:      scope,
		opusFrames: make(chan []byte, 1024),
		commands:   make(chan *chatgear.SessionCommandEvent, 32),
	}

	// Subscribe to downlink topics
	audioTopic := fmt.Sprintf("%sdevice/%s/output_audio_stream", scope, gearID)
	cmdTopic := fmt.Sprintf("%sdevice/%s/command", scope, gearID)

	if err := client.Subscribe(ctx, audioTopic, cmdTopic); err != nil {
		client.Close()
		cancel()
		return nil, fmt.Errorf("mqtt subscribe: %w", err)
	}

	slog.Info("subscribed to MQTT topics",
		"audio", audioTopic,
		"command", cmdTopic)

	// Start receive loop
	go conn.receiveLoop()

	return conn, nil
}

func (c *mqttConn) receiveLoop() {
	audioTopic := fmt.Sprintf("%sdevice/%s/output_audio_stream", c.scope, c.gearID)
	cmdTopic := fmt.Sprintf("%sdevice/%s/command", c.scope, c.gearID)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		msg, err := c.client.RecvTimeout(100 * time.Millisecond)
		if err != nil {
			if c.client.IsRunning() {
				slog.Error("mqtt recv error", "error", err)
			}
			return
		}
		if msg == nil {
			continue // timeout, no message
		}

		switch msg.Topic {
		case audioTopic:
			select {
			case c.opusFrames <- msg.Payload:
			default:
				// Drop frame if buffer full
			}
		case cmdTopic:
			var cmd chatgear.SessionCommandEvent
			if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
				slog.Warn("failed to unmarshal command", "error", err)
				continue
			}
			select {
			case c.commands <- &cmd:
			default:
				slog.Warn("commands channel full, dropping command")
			}
		}
	}
}

// --- UplinkTx implementation ---

func (c *mqttConn) SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error {
	topic := fmt.Sprintf("%sdevice/%s/input_audio_stream", c.scope, c.gearID)
	for _, frame := range frames {
		stamped := opusrt.Stamp(frame, stamp)
		if err := c.client.Publish(ctx, topic, stamped); err != nil {
			return err
		}
		stamp += opusrt.EpochMillis(opusrt.Frame(frame).Duration().Milliseconds())
	}
	return nil
}

func (c *mqttConn) SendState(ctx context.Context, state *chatgear.GearStateEvent) error {
	topic := fmt.Sprintf("%sdevice/%s/state", c.scope, c.gearID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, topic, data)
}

func (c *mqttConn) SendStats(ctx context.Context, stats *chatgear.GearStatsEvent) error {
	topic := fmt.Sprintf("%sdevice/%s/stats", c.scope, c.gearID)

	// Build stats message matching C implementation format
	statsMsg := struct {
		Type    string                   `json:"type"`
		Time    jsontime.Milli           `json:"time"`
		GearID  string                   `json:"gear_id"`
		Payload *chatgear.GearStatsEvent `json:"payload"`
	}{
		Type:    "stats",
		Time:    jsontime.Milli(time.Now()),
		GearID:  c.gearID,
		Payload: stats,
	}
	data, err := json.Marshal(statsMsg)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, topic, data)
}

func (c *mqttConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()
	close(c.opusFrames)
	close(c.commands)
	return c.client.Close()
}

// --- DownlinkRx implementation ---

func (c *mqttConn) OpusFrames() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		for frame := range c.opusFrames {
			if !yield(frame, nil) {
				return
			}
		}
	}
}

func (c *mqttConn) Commands() iter.Seq2[*chatgear.SessionCommandEvent, error] {
	return func(yield func(*chatgear.SessionCommandEvent, error) bool) {
		for cmd := range c.commands {
			if !yield(cmd, nil) {
				return
			}
		}
	}
}

// Compile-time interface assertions
var (
	_ chatgear.UplinkTx   = (*mqttConn)(nil)
	_ chatgear.DownlinkRx = (*mqttConn)(nil)
)
