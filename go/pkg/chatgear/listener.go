package chatgear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

// ErrListenerClosed is returned when Accept is called on a closed Listener.
var ErrListenerClosed = errors.New("chatgear: listener closed")

// =============================================================================
// Listener - Multi-device MQTT server
// =============================================================================

// Listener listens for device connections via MQTT.
// It follows the net.Listener pattern with Accept() blocking until a new device connects.
type Listener struct {
	broker   *mqtt0.Broker
	listener net.Listener
	acceptCh chan *AcceptedPort

	scope   string
	logger  Logger
	timeout time.Duration

	mu     sync.RWMutex
	ports  map[string]*managedPort
	closed bool

	ctx    context.Context
	cancel context.CancelFunc
}

// AcceptedPort represents a newly connected device.
type AcceptedPort struct {
	Port   *ServerPort
	GearID string
}

// managedPort tracks a ServerPort with its activity time.
type managedPort struct {
	port       *ServerPort
	gearID     string
	downlink   *gearDownlink
	lastActive time.Time
}

// gearDownlink implements DownlinkTx for a specific gearID.
type gearDownlink struct {
	listener *Listener
	gearID   string
	scope    string
}

// ListenerConfig contains configuration for the Listener.
type ListenerConfig struct {
	// Addr is the address to listen on (e.g., ":1883").
	Addr string

	// Scope is the topic prefix (e.g., "palr/cn/").
	Scope string

	// Timeout is the inactivity timeout for device connections.
	// Default is 30 seconds.
	Timeout time.Duration

	// Logger is used for logging. If nil, DefaultLogger() is used.
	Logger Logger
}

// ListenMQTT0 creates a new Listener that accepts device connections via MQTT.
//
// Usage:
//
//	ln, err := chatgear.ListenMQTT0(ctx, chatgear.ListenerConfig{Addr: ":1883"})
//	if err != nil {
//	    return err
//	}
//	defer ln.Close()
//
//	for {
//	    accepted, err := ln.Accept()
//	    if err != nil {
//	        break
//	    }
//	    go handleDevice(accepted.Port, accepted.GearID)
//	}
func ListenMQTT0(ctx context.Context, cfg ListenerConfig) (*Listener, error) {
	// Normalize scope
	scope := cfg.Scope
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope += "/"
	}

	// Set defaults
	logger := cfg.Logger
	if logger == nil {
		logger = DefaultLogger()
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	addr := cfg.Addr
	if addr == "" {
		addr = ":1883"
	}

	childCtx, cancel := context.WithCancel(ctx)

	l := &Listener{
		acceptCh: make(chan *AcceptedPort, 32),
		scope:    scope,
		logger:   logger,
		timeout:  timeout,
		ports:    make(map[string]*managedPort),
		ctx:      childCtx,
		cancel:   cancel,
	}

	// Create broker with wildcard handler
	l.broker = &mqtt0.Broker{
		Handler: mqtt0.HandlerFunc(func(clientID string, msg *mqtt0.Message) {
			l.handleMessage(msg.Topic, msg.Payload)
		}),
	}

	// Start listener
	ln, err := mqtt0.Listen("tcp", addr, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("chatgear/listener: listen: %w", err)
	}
	l.listener = ln

	// Start broker serve loop
	go func() {
		if err := l.broker.Serve(ln); err != nil {
			logger.ErrorPrintf("broker serve error: %v", err)
		}
	}()

	// Start timeout checker
	go l.timeoutChecker()

	// Handle context cancellation
	go func() {
		<-childCtx.Done()
		l.closeAll()
	}()

	logger.InfoPrintf("listener started on %s", addr)

	return l, nil
}

// Accept blocks until a new device connects.
// Returns the ServerPort and GearID for the new device.
func (l *Listener) Accept() (*AcceptedPort, error) {
	select {
	case accepted, ok := <-l.acceptCh:
		if !ok {
			return nil, ErrListenerClosed
		}
		return accepted, nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

// Close closes the Listener and all managed ServerPorts.
func (l *Listener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	l.cancel()
	return nil
}

// Addr returns the listener address.
func (l *Listener) Addr() string {
	if l.listener != nil {
		return l.listener.Addr().String()
	}
	return ""
}

// =============================================================================
// Internal methods
// =============================================================================

// Topic patterns for parsing gearID
var (
	topicAudioPattern = regexp.MustCompile(`^(.*)device/([^/]+)/input_audio_stream$`)
	topicStatePattern = regexp.MustCompile(`^(.*)device/([^/]+)/state$`)
	topicStatsPattern = regexp.MustCompile(`^(.*)device/([^/]+)/stats$`)
)

// handleMessage routes incoming MQTT messages to appropriate ServerPorts.
func (l *Listener) handleMessage(topic string, payload []byte) {
	l.logger.DebugPrintf("handleMessage: topic=%s len=%d", topic, len(payload))

	// Parse topic to extract gearID and message type
	var gearID string
	var msgType string

	if matches := topicAudioPattern.FindStringSubmatch(topic); matches != nil {
		gearID = matches[2]
		msgType = "audio"
	} else if matches := topicStatePattern.FindStringSubmatch(topic); matches != nil {
		gearID = matches[2]
		msgType = "state"
	} else if matches := topicStatsPattern.FindStringSubmatch(topic); matches != nil {
		gearID = matches[2]
		msgType = "stats"
	} else {
		// Unknown topic - log for debugging
		l.logger.DebugPrintf("unknown topic: %s", topic)
		return
	}

	// Get or create port
	mp := l.getOrCreatePort(gearID)
	if mp == nil {
		return // listener closed
	}

	// Update last active time
	l.mu.Lock()
	mp.lastActive = time.Now()
	l.mu.Unlock()

	// Route message to port
	switch msgType {
	case "audio":
		frame, t, ok := unstampFrame(payload)
		if !ok {
			l.logger.WarnPrintf("invalid stamped frame from %s", gearID)
			return
		}
		l.logger.DebugPrintf("RX audio from %s: len=%d ts=%v", gearID, len(frame), t.Format("15:04:05.000"))
		mp.port.HandleAudio(&StampedOpusFrame{Timestamp: t, Frame: frame})

	case "state":
		l.logger.InfoPrintf("RX state from %s: %s", gearID, string(payload))
		var evt StateEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			l.logger.WarnPrintf("failed to unmarshal state from %s: %v", gearID, err)
			return
		}
		mp.port.HandleState(&evt)

		// Check for shutdown/sleep state
		if evt.State == StateShuttingDown || evt.State == StateSleeping {
			l.releasePort(gearID)
		}

	case "stats":
		l.logger.InfoPrintf("RX stats from %s: %s", gearID, string(payload))
		var evt StatsEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			l.logger.WarnPrintf("failed to unmarshal stats from %s: %v", gearID, err)
			return
		}
		mp.port.HandleStats(&evt)
	}
}

// getOrCreatePort returns an existing port or creates a new one.
func (l *Listener) getOrCreatePort(gearID string) *managedPort {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	// Return existing port
	if mp, exists := l.ports[gearID]; exists {
		return mp
	}

	// Create new port
	port := NewServerPort()

	// Create downlink for this gearID
	downlink := &gearDownlink{
		listener: l,
		gearID:   gearID,
		scope:    l.scope,
	}

	// Start WriteTo goroutine
	go port.WriteTo(downlink)

	mp := &managedPort{
		port:       port,
		gearID:     gearID,
		downlink:   downlink,
		lastActive: time.Now(),
	}
	l.ports[gearID] = mp

	l.logger.InfoPrintf("new device connected: %s", gearID)

	// Send to accept channel (non-blocking)
	select {
	case l.acceptCh <- &AcceptedPort{Port: port, GearID: gearID}:
	default:
		l.logger.WarnPrintf("accept channel full, dropping new device %s", gearID)
	}

	return mp
}

// releasePort closes and removes a port.
func (l *Listener) releasePort(gearID string) {
	l.mu.Lock()
	mp, exists := l.ports[gearID]
	if exists {
		delete(l.ports, gearID)
	}
	l.mu.Unlock()

	if exists && mp.port != nil {
		l.logger.InfoPrintf("releasing device: %s", gearID)
		mp.port.Close()
	}
}

// timeoutChecker periodically checks for inactive ports.
func (l *Listener) timeoutChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.checkTimeouts()
		}
	}
}

// checkTimeouts checks for and releases inactive ports.
func (l *Listener) checkTimeouts() {
	l.mu.Lock()
	now := time.Now()
	var toRelease []string
	for gearID, mp := range l.ports {
		if now.Sub(mp.lastActive) > l.timeout {
			toRelease = append(toRelease, gearID)
		}
	}
	l.mu.Unlock()

	for _, gearID := range toRelease {
		l.logger.InfoPrintf("device timeout: %s", gearID)
		l.releasePort(gearID)
	}
}

// closeAll closes the listener and all ports.
func (l *Listener) closeAll() {
	l.mu.Lock()
	ports := make([]*managedPort, 0, len(l.ports))
	for _, mp := range l.ports {
		ports = append(ports, mp)
	}
	l.ports = nil
	l.mu.Unlock()

	// Close all ports
	for _, mp := range ports {
		if mp.port != nil {
			mp.port.Close()
		}
	}

	// Close accept channel
	close(l.acceptCh)

	// Close broker and listener
	if l.listener != nil {
		l.listener.Close()
	}
	if l.broker != nil {
		l.broker.Close()
	}
}

// =============================================================================
// gearDownlink - DownlinkTx implementation for a specific gearID
// =============================================================================

func (d *gearDownlink) SendOpusFrame(timestamp time.Time, frame opus.Frame) error {
	topic := fmt.Sprintf("%sdevice/%s/output_audio_stream", d.scope, d.gearID)
	stamped := stampFrame(frame, timestamp)
	d.listener.logger.DebugPrintf("TX audio to %s: len=%d ts=%v", d.gearID, len(frame), timestamp.Format("15:04:05.000"))
	return d.listener.broker.Publish(d.listener.ctx, topic, stamped)
}

func (d *gearDownlink) IssueCommand(cmd Command, t time.Time) error {
	topic := fmt.Sprintf("%sdevice/%s/command", d.scope, d.gearID)
	evt := NewCommandEvent(cmd, t)
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	d.listener.logger.InfoPrintf("TX command to %s: %s", d.gearID, string(data))
	return d.listener.broker.Publish(d.listener.ctx, topic, data)
}

func (d *gearDownlink) Close() error {
	// No-op: the listener manages the lifecycle
	return nil
}

// Compile-time interface assertion
var _ DownlinkTx = (*gearDownlink)(nil)
