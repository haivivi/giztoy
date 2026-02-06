package chatgear

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"iter"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

// =============================================================================
// Wire format for stamped Opus frames
// =============================================================================
//
// StampedFrame format:
//
//	+--------+------------------+------------------+
//	| Version| Timestamp (7B)   | Opus Frame Data  |
//	| (1B)   | Big-endian ms    |                  |
//	+--------+------------------+------------------+
//
// Total header: 8 bytes

const (
	frameVersion      = 1
	stampedHeaderSize = 8
)

// stampFrame creates a stamped frame from a frame and timestamp.
func stampFrame(frame opus.Frame, t time.Time) []byte {
	stamp := t.UnixMilli()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(stamp))
	buf[0] = frameVersion
	return append(buf[:], frame...)
}

// unstampFrame extracts the frame and timestamp from stamped data.
// Returns ok=false if the data is invalid.
func unstampFrame(b []byte) (frame opus.Frame, t time.Time, ok bool) {
	if len(b) < stampedHeaderSize {
		return nil, time.Time{}, false
	}
	if b[0] != frameVersion {
		return nil, time.Time{}, false
	}
	var buf [8]byte
	copy(buf[1:], b[1:8])
	stamp := int64(binary.BigEndian.Uint64(buf[:]))
	t = time.UnixMilli(stamp)
	frame = opus.Frame(b[stampedHeaderSize:])
	if len(frame) < 1 {
		return nil, time.Time{}, false
	}
	return frame, t, true
}

// =============================================================================
// MQTT Client Connection
// =============================================================================

// MQTTClientConfig contains configuration for dialing an MQTT connection.
type MQTTClientConfig struct {
	// Addr is the MQTT broker address (e.g., "tcp://localhost:1883").
	Addr string

	// Scope is the topic prefix (e.g., "palr/cn").
	Scope string

	// GearID is the device identifier.
	GearID string

	// Logger is used for logging warnings and errors. If nil, DefaultLogger() is used.
	Logger Logger

	// ClientID is the MQTT client identifier. If empty, a default is generated.
	ClientID string

	// KeepAlive is the keep-alive interval in seconds. Default is 60.
	KeepAlive uint16

	// ConnectTimeout is the timeout for establishing a connection. Default is 30s.
	ConnectTimeout time.Duration
}

// DialMQTT connects to an MQTT broker and returns a client connection.
func DialMQTT(ctx context.Context, cfg MQTTClientConfig) (*MQTTClientConn, error) {
	// Normalize scope
	scope := cfg.Scope
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope += "/"
	}

	// Set defaults
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("chatgear-%s-%d", cfg.GearID, time.Now().UnixNano()%10000)
	}
	keepAlive := cfg.KeepAlive
	if keepAlive == 0 {
		keepAlive = 60
	}
	connectTimeout := cfg.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 30 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = DefaultLogger()
	}

	// Parse URL to extract username/password if present
	var username string
	var password []byte
	addr := cfg.Addr
	if u, err := url.Parse(cfg.Addr); err == nil && u.User != nil {
		username = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = []byte(p)
		}
		// Reconstruct URL without userinfo for dialer
		u.User = nil
		addr = u.String()
	}

	// Connect to MQTT broker
	client, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:           addr,
		ClientID:       clientID,
		Username:       username,
		Password:       password,
		KeepAlive:      keepAlive,
		ConnectTimeout: connectTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("chatgear/mqtt: connect: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	conn := &MQTTClientConn{
		client:     client,
		ctx:        childCtx,
		cancel:     cancel,
		gearID:     cfg.GearID,
		scope:      scope,
		logger:     logger,
		opusFrames: make(chan StampedOpusFrame, 1024),
		commands:   make(chan *CommandEvent, 32),
	}

	// Subscribe to downlink topics
	audioTopic := fmt.Sprintf("%sdevice/%s/output_audio_stream", scope, cfg.GearID)
	cmdTopic := fmt.Sprintf("%sdevice/%s/command", scope, cfg.GearID)

	if err := client.Subscribe(ctx, audioTopic, cmdTopic); err != nil {
		client.Close()
		cancel()
		return nil, fmt.Errorf("chatgear/mqtt: subscribe: %w", err)
	}

	logger.InfoPrintf("subscribed to MQTT topics: audio=%s, command=%s", audioTopic, cmdTopic)

	// Start receive loop
	go conn.receiveLoop()

	return conn, nil
}

// MQTTClientConn represents a client-side connection to the server via MQTT.
// It implements both UplinkTx (send to server) and DownlinkRx (receive from server).
type MQTTClientConn struct {
	client *mqtt0.Client
	ctx    context.Context
	cancel context.CancelFunc
	gearID string
	scope  string
	logger Logger

	// Downlink channels
	opusFrames chan StampedOpusFrame
	commands   chan *CommandEvent

	mu     sync.Mutex
	closed bool
}

func (c *MQTTClientConn) receiveLoop() {
	c.logger.InfoPrintf("receiveLoop started")
	audioTopic := fmt.Sprintf("%sdevice/%s/output_audio_stream", c.scope, c.gearID)
	cmdTopic := fmt.Sprintf("%sdevice/%s/command", c.scope, c.gearID)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.InfoPrintf("receiveLoop: context done")
			return
		default:
		}

		msg, err := c.client.RecvTimeout(100 * time.Millisecond)
		if err != nil {
			if c.client.IsRunning() {
				c.logger.ErrorPrintf("mqtt recv error: %v", err)
			} else {
				c.logger.InfoPrintf("receiveLoop: client stopped")
			}
			return
		}
		if msg == nil {
			continue // timeout, no message
		}

		switch msg.Topic {
		case audioTopic:
			frame, t, ok := unstampFrame(msg.Payload)
			if !ok {
				c.logger.WarnPrintf("invalid stamped frame received")
				continue
			}
			c.logger.DebugPrintf("MQTT RX audio: len=%d ts=%v", len(frame), t.Format("15:04:05.000"))
			select {
			case c.opusFrames <- StampedOpusFrame{Timestamp: t, Frame: frame}:
			default:
				// Drop frame if buffer full
			}
		case cmdTopic:
			c.logger.InfoPrintf("MQTT RX command: %s", string(msg.Payload))
			var evt CommandEvent
			if err := json.Unmarshal(msg.Payload, &evt); err != nil {
				c.logger.WarnPrintf("failed to unmarshal command: %v", err)
				continue
			}
			select {
			case c.commands <- &evt:
			default:
				c.logger.WarnPrintf("commands channel full, dropping command")
			}
		}
	}
}

// --- UplinkTx implementation ---

func (c *MQTTClientConn) SendOpusFrame(timestamp time.Time, frame opus.Frame) error {
	topic := fmt.Sprintf("%sdevice/%s/input_audio_stream", c.scope, c.gearID)
	stamped := stampFrame(frame, timestamp)
	c.logger.DebugPrintf("MQTT TX audio: len=%d ts=%v", len(frame), timestamp.Format("15:04:05.000"))
	return c.client.Publish(c.ctx, topic, stamped)
}

func (c *MQTTClientConn) SendState(state *StateEvent) error {
	topic := fmt.Sprintf("%sdevice/%s/state", c.scope, c.gearID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	c.logger.InfoPrintf("MQTT TX state: %s", string(data))
	return c.client.Publish(c.ctx, topic, data)
}

func (c *MQTTClientConn) SendStats(stats *StatsEvent) error {
	topic := fmt.Sprintf("%sdevice/%s/stats", c.scope, c.gearID)
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	c.logger.InfoPrintf("MQTT TX stats: %s", string(data))
	return c.client.Publish(c.ctx, topic, data)
}

// --- DownlinkRx implementation ---

func (c *MQTTClientConn) OpusFrames() iter.Seq2[StampedOpusFrame, error] {
	return func(yield func(StampedOpusFrame, error) bool) {
		for {
			select {
			case <-c.ctx.Done():
				return
			case frame, ok := <-c.opusFrames:
				if !ok {
					return
				}
				if !yield(frame, nil) {
					return
				}
			}
		}
	}
}

func (c *MQTTClientConn) Commands() iter.Seq2[*CommandEvent, error] {
	return func(yield func(*CommandEvent, error) bool) {
		for {
			select {
			case <-c.ctx.Done():
				return
			case cmd, ok := <-c.commands:
				if !ok {
					return
				}
				if !yield(cmd, nil) {
					return
				}
			}
		}
	}
}

// --- Lifecycle ---

func (c *MQTTClientConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()
	return c.client.Close()
}

// GearID returns the gear ID for this connection.
func (c *MQTTClientConn) GearID() string {
	return c.gearID
}

// Compile-time interface assertions
var (
	_ UplinkTx   = (*MQTTClientConn)(nil)
	_ DownlinkRx = (*MQTTClientConn)(nil)
)
