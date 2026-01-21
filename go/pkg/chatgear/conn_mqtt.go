package chatgear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net"
	"path"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/mqtt"
)

var (
	errListenerClosed = fmt.Errorf("chatgear/mqtt: %w", net.ErrClosed)

	stateTopicRegexp      = regexp.MustCompile(`(?:^|/)device/([^/]+)/state$`)
	statsTopicRegexp      = regexp.MustCompile(`(?:^|/)device/([^/]+)/stats$`)
	inputAudioTopicRegexp = regexp.MustCompile(`(?:^|/)device/([^/]+)/input_audio_stream$`)
)

// Topic path helpers
func inputAudioTopic(scope, gearID string) string {
	return path.Join(scope, "device", gearID, "input_audio_stream")
}

func stateTopic(scope, gearID string) string {
	return path.Join(scope, "device", gearID, "state")
}

func statsTopic(scope, gearID string) string {
	return path.Join(scope, "device", gearID, "stats")
}

func outputAudioTopic(scope, gearID string) string {
	return path.Join(scope, "device", gearID, "output_audio_stream")
}

func commandTopic(scope, gearID string) string {
	return path.Join(scope, "device", gearID, "command")
}

// MQTTListener listens for incoming device connections via an embedded MQTT broker.
type MQTTListener struct {
	server *mqtt.Server
	scope  string
	logger Logger

	ctx    context.Context
	cancel context.CancelFunc

	closed    atomic.Bool
	connQueue chan *MQTTServerConn
	conns     sync.Map // gearID -> *MQTTServerConn
}

// MQTTListenerConfig contains the configuration for creating a MQTTListener.
type MQTTListenerConfig struct {
	// Server is the embedded MQTT broker. Required.
	Server *mqtt.Server
	// Mux is the ServeMux to register device handlers on. Required.
	Mux *mqtt.ServeMux
	// Scope is the topic prefix (e.g., "palr/cn").
	Scope string
	// QueueSize is the size of the connection accept queue. Default is 64.
	QueueSize int
	// Logger is used for logging warnings and errors. If nil, DefaultLogger() is used.
	Logger Logger
}

// MQTTListen creates a new MQTT listener that registers device handlers on the given ServeMux.
// The caller is responsible for creating and managing the mqtt.Server lifecycle.
func MQTTListen(ctx context.Context, cfg MQTTListenerConfig) (*MQTTListener, error) {
	if cfg.Server == nil {
		return nil, errors.New("chatgear/mqtt: Server is required")
	}
	if cfg.Mux == nil {
		return nil, errors.New("chatgear/mqtt: Mux is required")
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 64
	}
	if cfg.Logger == nil {
		cfg.Logger = DefaultLogger()
	}

	ctx, cancel := context.WithCancel(ctx)
	lis := &MQTTListener{
		server:    cfg.Server,
		scope:     cfg.Scope,
		logger:    cfg.Logger,
		ctx:       ctx,
		cancel:    cancel,
		connQueue: make(chan *MQTTServerConn, cfg.QueueSize),
	}

	// Register MQTT handlers for device topics
	if err := lis.registerHandlers(cfg.Mux); err != nil {
		cancel()
		return nil, err
	}

	return lis, nil
}

func (lis *MQTTListener) registerHandlers(mux *mqtt.ServeMux) error {
	if err := mux.HandleFunc(path.Join(lis.scope, "device/+/state"), lis.handleStateEvent); err != nil {
		return err
	}
	if err := mux.HandleFunc(path.Join(lis.scope, "device/+/stats"), lis.handleStatsEvent); err != nil {
		return err
	}
	if err := mux.HandleFunc(path.Join(lis.scope, "device/+/input_audio_stream"), lis.handleInputAudio); err != nil {
		return err
	}
	return nil
}

// Server returns the underlying MQTT server.
func (lis *MQTTListener) Server() *mqtt.Server {
	return lis.server
}

// Accept waits for and returns the next device connection.
func (lis *MQTTListener) Accept() (*MQTTServerConn, error) {
	select {
	case <-lis.ctx.Done():
		return nil, errListenerClosed
	case conn, ok := <-lis.connQueue:
		if !ok {
			return nil, errListenerClosed
		}
		return conn, nil
	}
}

// Close closes the listener.
func (lis *MQTTListener) Close() error {
	if !lis.closed.CompareAndSwap(false, true) {
		return nil
	}
	lis.cancel()
	close(lis.connQueue)

	// Close all active connections
	lis.conns.Range(func(key, value any) bool {
		if conn, ok := value.(*MQTTServerConn); ok {
			conn.CloseWithError(errListenerClosed)
		}
		return true
	})

	return nil
}

// Context returns the listener's context.
func (lis *MQTTListener) Context() context.Context {
	return lis.ctx
}

// Conn returns an existing connection by gearID if it exists.
func (lis *MQTTListener) Conn(gearID string) (*MQTTServerConn, bool) {
	v, ok := lis.conns.Load(gearID)
	if !ok {
		return nil, false
	}
	return v.(*MQTTServerConn), true
}

func (lis *MQTTListener) getOrCreateConn(gearID string) *MQTTServerConn {
	if v, ok := lis.conns.Load(gearID); ok {
		return v.(*MQTTServerConn)
	}

	conn := newMQTTServerConn(lis, gearID)
	actual, loaded := lis.conns.LoadOrStore(gearID, conn)
	if loaded {
		// Another goroutine created the conn first
		return actual.(*MQTTServerConn)
	}

	// Queue the new connection for Accept()
	select {
	case lis.connQueue <- conn:
	default:
		// Queue is full, close and remove
		lis.logger.WarnPrintf("connection queue full, dropping connection for gear %s", gearID)
		conn.CloseWithError(errors.New("connection queue full"))
		lis.conns.Delete(gearID)
		return nil
	}

	return conn
}

func (lis *MQTTListener) deleteConn(gearID string) {
	lis.conns.Delete(gearID)
}

func (lis *MQTTListener) handleStateEvent(msg mqtt.Message) error {
	m := stateTopicRegexp.FindStringSubmatch(msg.Packet.Topic)
	if len(m) != 2 {
		return nil
	}
	gearID := m[1]

	var evt GearStateEvent
	if err := json.Unmarshal(msg.Packet.Payload, &evt); err != nil {
		lis.logger.WarnPrintf("failed to unmarshal state event for gear %s: %v", gearID, err)
		return nil
	}

	conn := lis.getOrCreateConn(gearID)
	if conn != nil {
		conn.pushState(&evt)
	}
	return nil
}

func (lis *MQTTListener) handleStatsEvent(msg mqtt.Message) error {
	m := statsTopicRegexp.FindStringSubmatch(msg.Packet.Topic)
	if len(m) != 2 {
		return nil
	}
	gearID := m[1]

	var evt GearStatsEvent
	if err := json.Unmarshal(msg.Packet.Payload, &evt); err != nil {
		lis.logger.WarnPrintf("failed to unmarshal stats event for gear %s: %v", gearID, err)
		return nil
	}

	if conn, ok := lis.Conn(gearID); ok {
		conn.pushStats(&evt)
	}
	return nil
}

func (lis *MQTTListener) handleInputAudio(msg mqtt.Message) error {
	m := inputAudioTopicRegexp.FindStringSubmatch(msg.Packet.Topic)
	if len(m) != 2 {
		return nil
	}
	gearID := m[1]

	if conn, ok := lis.Conn(gearID); ok {
		// Copy the payload since MQTT may reuse the buffer
		frame := make([]byte, len(msg.Packet.Payload))
		copy(frame, msg.Packet.Payload)
		conn.pushOpusFrame(frame)
	}
	return nil
}

// MQTTServerConn represents a server-side connection to a device via MQTT.
// It implements both UplinkRx (receive from device) and DownlinkTx (send to device).
type MQTTServerConn struct {
	listener *MQTTListener
	gearID   string
	logger   Logger

	// Receiving channels (UplinkRx)
	opusFrames chan []byte
	states     chan *GearStateEvent
	stats      chan *GearStatsEvent

	// Topic names for sending (DownlinkTx)
	outputAudioTopic string
	commandTopic     string

	mu             sync.Mutex
	gearStats      *GearStatsEvent
	opusEncodeOpts []opusrt.EncodePCMOption
	closed         bool
	err            error
}

func newMQTTServerConn(lis *MQTTListener, gearID string) *MQTTServerConn {
	return &MQTTServerConn{
		listener:         lis,
		gearID:           gearID,
		logger:           lis.logger,
		opusFrames:       make(chan []byte, 1024),
		states:           make(chan *GearStateEvent, 32),
		stats:            make(chan *GearStatsEvent, 32),
		outputAudioTopic: outputAudioTopic(lis.scope, gearID),
		commandTopic:     commandTopic(lis.scope, gearID),
	}
}

// GearID returns the gear ID for this connection.
func (c *MQTTServerConn) GearID() string {
	return c.gearID
}

// SetOpusEncodeOptions sets the opus encoding options for this connection.
func (c *MQTTServerConn) SetOpusEncodeOptions(opts ...opusrt.EncodePCMOption) {
	c.mu.Lock()
	c.opusEncodeOpts = opts
	c.mu.Unlock()
}

// --- Push methods (called by Listener) ---

func (c *MQTTServerConn) pushOpusFrame(frame []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.opusFrames <- frame:
	default:
	}
}

func (c *MQTTServerConn) pushState(evt *GearStateEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.states <- evt:
	default:
		c.logger.WarnPrintf("state channel full, dropping event for gear %s", c.gearID)
	}
}

func (c *MQTTServerConn) pushStats(evt *GearStatsEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.gearStats = evt
	select {
	case c.stats <- evt:
	default:
	}
}

// --- UplinkRx implementation ---

func (c *MQTTServerConn) OpusFrames() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		for frame := range c.opusFrames {
			if !yield(frame, nil) {
				return
			}
		}
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *MQTTServerConn) States() iter.Seq2[*GearStateEvent, error] {
	return func(yield func(*GearStateEvent, error) bool) {
		for state := range c.states {
			if !yield(state, nil) {
				return
			}
		}
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *MQTTServerConn) Stats() iter.Seq2[*GearStatsEvent, error] {
	return func(yield func(*GearStatsEvent, error) bool) {
		for stats := range c.stats {
			if !yield(stats, nil) {
				return
			}
		}
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *MQTTServerConn) GearStats() *GearStatsEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gearStats
}

// --- DownlinkTx implementation ---

func (c *MQTTServerConn) SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error {
	server := c.listener.server
	for _, frame := range frames {
		stamped := opusrt.Stamp(frame, stamp)
		if err := server.WriteToTopic(ctx, stamped, c.outputAudioTopic); err != nil {
			return err
		}
		stamp += opusrt.EpochMillis(opusrt.Frame(frame).Duration().Milliseconds())
	}
	return nil
}

func (c *MQTTServerConn) IssueCommand(ctx context.Context, cmd SessionCommand, issueAt time.Time) error {
	evt := NewSessionCommandEvent(cmd, issueAt)
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return c.listener.server.WriteToTopic(ctx, b, c.commandTopic)
}

func (c *MQTTServerConn) OpusEncodeOptions() []opusrt.EncodePCMOption {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opusEncodeOpts
}

// --- Lifecycle ---

func (c *MQTTServerConn) Close() error {
	return c.CloseWithError(nil)
}

func (c *MQTTServerConn) CloseWithError(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.err = err
	close(c.opusFrames)
	close(c.states)
	close(c.stats)

	c.listener.deleteConn(c.gearID)
	return nil
}

// MQTTDialer creates client connections to an MQTT broker.
type MQTTDialer struct {
	// URL is the MQTT broker URL (e.g., "tcp://localhost:1883").
	URL string
	// Scope is the topic prefix (e.g., "palr/cn").
	Scope string
	// Dialer is the underlying MQTT dialer. If nil, a default one is created.
	Dialer *mqtt.Dialer
	// Logger is used for logging warnings and errors. If nil, DefaultLogger() is used.
	Logger Logger
}

// Dial creates a new client connection to the server for the given gear.
func (d *MQTTDialer) Dial(ctx context.Context, gearID string) (*MQTTClientConn, error) {
	mux := mqtt.NewServeMux()

	// Create a new dialer config to avoid mutating the one from the caller.
	dialer := &mqtt.Dialer{}
	if d.Dialer != nil {
		*dialer = *d.Dialer // Copy settings from provided dialer.
	} else {
		dialer.KeepAlive = 30
	}
	dialer.ServeMux = mux // Always use the local mux.

	logger := d.Logger
	if logger == nil {
		logger = DefaultLogger()
	}

	conn, err := dialer.Dial(ctx, d.URL)
	if err != nil {
		return nil, err
	}

	c, err := newMQTTClientConn(ctx, conn, mux, d.Scope, gearID, logger)
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.ownsConn = true // Mark that we own the connection

	return c, nil
}

// MQTTClientConn represents a client-side connection to the server via MQTT.
// It implements both UplinkTx (send to server) and DownlinkRx (receive from server).
type MQTTClientConn struct {
	conn   *mqtt.Conn
	scope  string
	gearID string
	logger Logger

	// Receiving channels (DownlinkRx)
	opusFrames chan []byte
	commands   chan *SessionCommandEvent

	// Transmitting (UplinkTx)
	inputAudio *mqtt.TopicWriter
	state      *mqtt.TopicWriter
	stats      *mqtt.TopicWriter

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	ownsConn bool // Whether we own the mqtt.Conn (should close it)
	closed   bool
	err      error
}

// MQTTDial creates a new client connection using an existing MQTT connection.
// This is useful when you want to manage the MQTT connection separately.
func MQTTDial(ctx context.Context, conn *mqtt.Conn, mux *mqtt.ServeMux, scope, gearID string) (*MQTTClientConn, error) {
	return newMQTTClientConn(ctx, conn, mux, scope, gearID, DefaultLogger())
}

func newMQTTClientConn(ctx context.Context, conn *mqtt.Conn, mux *mqtt.ServeMux, scope, gearID string, logger Logger) (*MQTTClientConn, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &MQTTClientConn{
		conn:       conn,
		scope:      scope,
		gearID:     gearID,
		logger:     logger,
		opusFrames: make(chan []byte, 1024),
		commands:   make(chan *SessionCommandEvent, 32),
		inputAudio: &mqtt.TopicWriter{
			Name:    inputAudioTopic(scope, gearID),
			Options: []mqtt.WriteOption{mqtt.AtMostOnce},
			Conn:    conn,
		},
		state: &mqtt.TopicWriter{
			Name:    stateTopic(scope, gearID),
			Options: []mqtt.WriteOption{mqtt.AtMostOnce},
			Conn:    conn,
		},
		stats: &mqtt.TopicWriter{
			Name:    statsTopic(scope, gearID),
			Options: []mqtt.WriteOption{mqtt.AtMostOnce},
			Conn:    conn,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Register handlers for receiving from server
	outputTopic := outputAudioTopic(scope, gearID)
	cmdTopic := commandTopic(scope, gearID)

	if err := mux.HandleFunc(outputTopic, c.handleOutputAudio); err != nil {
		cancel()
		return nil, err
	}
	if err := mux.HandleFunc(cmdTopic, c.handleCommand); err != nil {
		cancel()
		return nil, err
	}

	// Subscribe to server topics
	opts := []mqtt.SubscribeOption{mqtt.AtMostOnce, mqtt.AutoResubscribe{}}
	if err := conn.SubscribeAll(ctx, []string{outputTopic, cmdTopic}, opts...); err != nil {
		cancel()
		return nil, err
	}

	return c, nil
}

func (c *MQTTClientConn) handleOutputAudio(msg mqtt.Message) error {
	// Copy the payload first since MQTT may reuse the buffer
	frame := make([]byte, len(msg.Packet.Payload))
	copy(frame, msg.Packet.Payload)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	select {
	case c.opusFrames <- frame:
	default:
	}
	return nil
}

func (c *MQTTClientConn) handleCommand(msg mqtt.Message) error {
	var evt SessionCommandEvent
	if err := json.Unmarshal(msg.Packet.Payload, &evt); err != nil {
		c.logger.WarnPrintf("failed to unmarshal command event for gear %s: %v", c.gearID, err)
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	select {
	case c.commands <- &evt:
	default:
		c.logger.WarnPrintf("command channel full, dropping event for gear %s", c.gearID)
	}
	return nil
}

// --- UplinkTx implementation ---

func (c *MQTTClientConn) SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error {
	for _, frame := range frames {
		stamped := opusrt.Stamp(frame, stamp)
		if err := c.inputAudio.Publish(ctx, stamped); err != nil {
			return err
		}
		stamp += opusrt.EpochMillis(opusrt.Frame(frame).Duration().Milliseconds())
	}
	return nil
}

func (c *MQTTClientConn) SendState(ctx context.Context, state *GearStateEvent) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.state.Publish(ctx, b)
}

func (c *MQTTClientConn) SendStats(ctx context.Context, stats *GearStatsEvent) error {
	b, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return c.stats.Publish(ctx, b)
}

// --- DownlinkRx implementation ---

func (c *MQTTClientConn) OpusFrames() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		for frame := range c.opusFrames {
			if !yield(frame, nil) {
				return
			}
		}
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *MQTTClientConn) Commands() iter.Seq2[*SessionCommandEvent, error] {
	return func(yield func(*SessionCommandEvent, error) bool) {
		for cmd := range c.commands {
			if !yield(cmd, nil) {
				return
			}
		}
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

// --- Lifecycle ---

func (c *MQTTClientConn) Close() error {
	return c.CloseWithError(nil)
}

func (c *MQTTClientConn) CloseWithError(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.err = err
	c.cancel()
	close(c.opusFrames)
	close(c.commands)

	// Close the MQTT connection if we own it
	if c.ownsConn && c.conn != nil {
		c.conn.Close()
	}
	return nil
}

// Compile-time interface assertions
var (
	_ UplinkRx   = (*MQTTServerConn)(nil)
	_ DownlinkTx = (*MQTTServerConn)(nil)
	_ UplinkTx   = (*MQTTClientConn)(nil)
	_ DownlinkRx = (*MQTTClientConn)(nil)
)
