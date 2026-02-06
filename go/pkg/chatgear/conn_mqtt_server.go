package chatgear

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

// =============================================================================
// Server Mux - shared message routing logic
// =============================================================================

// serverMux handles message routing for both Dial and Listen modes.
type serverMux struct {
	scope  string
	gearID string
	logger Logger

	// Uplink channels (from client)
	opusFrames chan StampedOpusFrame
	states     chan *StateEvent
	stats      chan *StatsEvent

	mu          sync.Mutex
	latestStats *StatsEvent
}

// newServerMux creates a new server mux with the given configuration.
func newServerMux(scope, gearID string, logger Logger) *serverMux {
	return &serverMux{
		scope:      scope,
		gearID:     gearID,
		logger:     logger,
		opusFrames: make(chan StampedOpusFrame, 1024),
		states:     make(chan *StateEvent, 32),
		stats:      make(chan *StatsEvent, 32),
	}
}

// topics returns the uplink topics for this gear.
func (m *serverMux) topics() (audio, state, stats string) {
	audio = fmt.Sprintf("%sdevice/%s/input_audio_stream", m.scope, m.gearID)
	state = fmt.Sprintf("%sdevice/%s/state", m.scope, m.gearID)
	stats = fmt.Sprintf("%sdevice/%s/stats", m.scope, m.gearID)
	return
}

// downlinkTopics returns the downlink topics for this gear.
func (m *serverMux) downlinkTopics() (audio, command string) {
	audio = fmt.Sprintf("%sdevice/%s/output_audio_stream", m.scope, m.gearID)
	command = fmt.Sprintf("%sdevice/%s/command", m.scope, m.gearID)
	return
}

// handleMessage routes incoming MQTT messages to appropriate channels.
func (m *serverMux) handleMessage(topic string, payload []byte) {
	audioTopic, stateTopic, statsTopic := m.topics()

	switch topic {
	case audioTopic:
		frame, t, ok := unstampFrame(payload)
		if !ok {
			m.logger.WarnPrintf("invalid stamped frame received")
			return
		}
		m.logger.DebugPrintf("MQTT RX audio: len=%d ts=%v", len(frame), t.Format("15:04:05.000"))
		select {
		case m.opusFrames <- StampedOpusFrame{Timestamp: t, Frame: frame}:
		default:
			m.logger.DebugPrintf("opusFrames channel full, dropping frame")
		}

	case stateTopic:
		m.logger.InfoPrintf("MQTT RX state: %s", string(payload))
		var evt StateEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			m.logger.WarnPrintf("failed to unmarshal state: %v", err)
			return
		}
		select {
		case m.states <- &evt:
		default:
			m.logger.WarnPrintf("states channel full, dropping state")
		}

	case statsTopic:
		m.logger.InfoPrintf("MQTT RX stats: %s", string(payload))
		var evt StatsEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			m.logger.WarnPrintf("failed to unmarshal stats: %v", err)
			return
		}
		m.mu.Lock()
		m.latestStats = &evt
		m.mu.Unlock()
		select {
		case m.stats <- &evt:
		default:
			m.logger.WarnPrintf("stats channel full, dropping stats")
		}
	}
}

// close closes all channels.
func (m *serverMux) close() {
	close(m.opusFrames)
	close(m.states)
	close(m.stats)
}

// =============================================================================
// MQTT Server Connection
// =============================================================================

// MQTTServerConfig contains configuration for an MQTT server connection.
type MQTTServerConfig struct {
	// Addr is the MQTT broker address for DialMQTTServer (e.g., "tcp://localhost:1883").
	// For ListenMQTTServer, this is the address to listen on (e.g., ":1883").
	Addr string

	// Scope is the topic prefix (e.g., "palr/cn").
	Scope string

	// GearID is the device identifier to listen for.
	GearID string

	// Logger is used for logging warnings and errors. If nil, DefaultLogger() is used.
	Logger Logger

	// ClientID is the MQTT client identifier (for DialMQTTServer only).
	// If empty, a default is generated.
	ClientID string

	// KeepAlive is the keep-alive interval in seconds (for DialMQTTServer only).
	// Default is 60.
	KeepAlive uint16

	// ConnectTimeout is the timeout for establishing a connection (for DialMQTTServer only).
	// Default is 30s.
	ConnectTimeout time.Duration
}

// MQTTServerConn represents a server-side connection to the client via MQTT.
// It implements both UplinkRx (receive from client) and DownlinkTx (send to client).
type MQTTServerConn struct {
	mux *serverMux

	// For DialMQTTServer - MQTT client mode
	client *mqtt0.Client

	// For ListenMQTTServer - embedded broker mode
	broker   *mqtt0.Broker
	listener net.Listener

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	closed bool
}

// DialMQTTServer connects to an MQTT broker and returns a server connection.
// The server connection receives uplink data (audio, state, stats) from the client
// and sends downlink data (audio, commands) to the client.
func DialMQTTServer(ctx context.Context, cfg MQTTServerConfig) (*MQTTServerConn, error) {
	// Normalize scope
	scope := cfg.Scope
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope += "/"
	}

	// Set defaults
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("chatgear-server-%s-%d", cfg.GearID, time.Now().UnixNano()%10000)
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
		return nil, fmt.Errorf("chatgear/mqtt-server: connect: %w", err)
	}

	// Create mux and connection
	mux := newServerMux(scope, cfg.GearID, logger)
	childCtx, cancel := context.WithCancel(ctx)

	conn := &MQTTServerConn{
		mux:    mux,
		client: client,
		ctx:    childCtx,
		cancel: cancel,
	}

	// Subscribe to uplink topics (from client)
	audioTopic, stateTopic, statsTopic := mux.topics()
	if err := client.Subscribe(ctx, audioTopic, stateTopic, statsTopic); err != nil {
		client.Close()
		cancel()
		return nil, fmt.Errorf("chatgear/mqtt-server: subscribe: %w", err)
	}

	logger.InfoPrintf("subscribed to MQTT topics: audio=%s, state=%s, stats=%s", audioTopic, stateTopic, statsTopic)

	// Start receive loop for client mode
	go conn.clientReceiveLoop()

	return conn, nil
}

// ListenMQTTServer starts an embedded MQTT broker and returns a server connection.
// The server handles messages internally without network overhead for the server side.
// Clients (like geartest) connect to cfg.Addr to communicate.
func ListenMQTTServer(ctx context.Context, cfg MQTTServerConfig) (*MQTTServerConn, error) {
	// Normalize scope
	scope := cfg.Scope
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope += "/"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = DefaultLogger()
	}

	// Default address
	addr := cfg.Addr
	if addr == "" {
		addr = ":1883"
	}

	// Create mux
	mux := newServerMux(scope, cfg.GearID, logger)

	// Create broker with handler
	broker := &mqtt0.Broker{
		Handler: mqtt0.HandlerFunc(func(clientID string, msg *mqtt0.Message) {
			mux.handleMessage(msg.Topic, msg.Payload)
		}),
	}

	// Start listener
	ln, err := mqtt0.Listen("tcp", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("chatgear/mqtt-server: listen: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)

	conn := &MQTTServerConn{
		mux:      mux,
		broker:   broker,
		listener: ln,
		ctx:      childCtx,
		cancel:   cancel,
	}

	// Start broker serve loop
	go func() {
		if err := broker.Serve(ln); err != nil {
			logger.ErrorPrintf("broker serve error: %v", err)
		}
	}()

	// Handle context cancellation
	go func() {
		<-childCtx.Done()
		ln.Close()
		broker.Close()
	}()

	logger.InfoPrintf("MQTT broker listening on %s for gear %s", addr, cfg.GearID)

	return conn, nil
}

// clientReceiveLoop receives messages from MQTT client (for DialMQTTServer mode).
func (c *MQTTServerConn) clientReceiveLoop() {
	c.mux.logger.InfoPrintf("receiveLoop started")

	for {
		select {
		case <-c.ctx.Done():
			c.mux.logger.InfoPrintf("receiveLoop: context done")
			return
		default:
		}

		msg, err := c.client.RecvTimeout(100 * time.Millisecond)
		if err != nil {
			if c.client.IsRunning() {
				c.mux.logger.ErrorPrintf("mqtt recv error: %v", err)
			} else {
				c.mux.logger.InfoPrintf("receiveLoop: client stopped")
			}
			return
		}
		if msg == nil {
			continue // timeout, no message
		}

		c.mux.handleMessage(msg.Topic, msg.Payload)
	}
}

// --- UplinkRx implementation (receive from client) ---

func (c *MQTTServerConn) OpusFrames() iter.Seq2[StampedOpusFrame, error] {
	return func(yield func(StampedOpusFrame, error) bool) {
		for {
			select {
			case <-c.ctx.Done():
				return
			case frame, ok := <-c.mux.opusFrames:
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

func (c *MQTTServerConn) States() iter.Seq2[*StateEvent, error] {
	return func(yield func(*StateEvent, error) bool) {
		for {
			select {
			case <-c.ctx.Done():
				return
			case state, ok := <-c.mux.states:
				if !ok {
					return
				}
				if !yield(state, nil) {
					return
				}
			}
		}
	}
}

func (c *MQTTServerConn) Stats() iter.Seq2[*StatsEvent, error] {
	return func(yield func(*StatsEvent, error) bool) {
		for {
			select {
			case <-c.ctx.Done():
				return
			case stats, ok := <-c.mux.stats:
				if !ok {
					return
				}
				if !yield(stats, nil) {
					return
				}
			}
		}
	}
}

func (c *MQTTServerConn) LatestStats() *StatsEvent {
	c.mux.mu.Lock()
	defer c.mux.mu.Unlock()
	return c.mux.latestStats
}

// --- DownlinkTx implementation (send to client) ---

func (c *MQTTServerConn) SendOpusFrame(timestamp time.Time, frame opus.Frame) error {
	audioTopic, _ := c.mux.downlinkTopics()
	stamped := stampFrame(frame, timestamp)
	c.mux.logger.DebugPrintf("MQTT TX audio: len=%d ts=%v", len(frame), timestamp.Format("15:04:05.000"))
	return c.publish(audioTopic, stamped)
}

func (c *MQTTServerConn) IssueCommand(cmd Command, t time.Time) error {
	_, cmdTopic := c.mux.downlinkTopics()
	evt := NewCommandEvent(cmd, t)
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	c.mux.logger.InfoPrintf("MQTT TX command: %s", string(data))
	return c.publish(cmdTopic, data)
}

// publish sends a message using either client or broker depending on mode.
func (c *MQTTServerConn) publish(topic string, payload []byte) error {
	if c.client != nil {
		return c.client.Publish(c.ctx, topic, payload)
	}
	if c.broker != nil {
		return c.broker.Publish(c.ctx, topic, payload)
	}
	return fmt.Errorf("no client or broker available")
}

// --- Lifecycle ---

func (c *MQTTServerConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()
	c.mux.close()

	if c.client != nil {
		return c.client.Close()
	}
	if c.listener != nil {
		c.listener.Close()
	}
	if c.broker != nil {
		c.broker.Close()
	}
	return nil
}

// GearID returns the gear ID for this connection.
func (c *MQTTServerConn) GearID() string {
	return c.mux.gearID
}

// ListenAddr returns the listener address (for ListenMQTTServer mode).
// Returns empty string for DialMQTTServer mode.
func (c *MQTTServerConn) ListenAddr() string {
	if c.listener != nil {
		return c.listener.Addr().String()
	}
	return ""
}

// Compile-time interface assertions
var (
	_ UplinkRx   = (*MQTTServerConn)(nil)
	_ DownlinkTx = (*MQTTServerConn)(nil)
)
