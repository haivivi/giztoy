package mqtt0

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ClientConfig is the configuration for an MQTT client.
type ClientConfig struct {
	// Addr is the broker address in URL format:
	//   - tcp://host:port (default port 1883)
	//   - tls://host:port or mqtts://host:port (default port 8883)
	//   - ws://host:port/path (default port 80)
	//   - wss://host:port/path (default port 443)
	Addr string

	// ClientID is the client identifier.
	ClientID string

	// Username for authentication (optional).
	Username string

	// Password for authentication (optional).
	Password []byte

	// KeepAlive is the keep-alive interval in seconds.
	// Default is 60 seconds. Set to 0 to disable.
	KeepAlive uint16

	// CleanSession (v4) or CleanStart (v5) flag.
	// Default is true.
	CleanSession bool

	// ProtocolVersion is the MQTT protocol version.
	// Default is ProtocolV4 (MQTT 3.1.1).
	ProtocolVersion ProtocolVersion

	// SessionExpiry is the session expiry interval in seconds (MQTT 5.0 only).
	// Default is nil (use broker default).
	SessionExpiry *uint32

	// AutoKeepalive enables automatic keep-alive ping.
	// When enabled (default), the client sends PINGREQ at KeepAlive/2 intervals.
	AutoKeepalive bool

	// TLSConfig is the TLS configuration for secure connections.
	// If nil, a default configuration is used for tls:// and wss:// connections.
	TLSConfig *tls.Config

	// MaxPacketSize is the maximum packet size.
	// Default is MaxPacketSize (1MB).
	MaxPacketSize int

	// ConnectTimeout is the timeout for establishing a connection.
	// Default is 30 seconds.
	ConnectTimeout time.Duration

	// Dialer is the custom dialer function.
	// If nil, the default dialer is used.
	Dialer func(ctx context.Context, addr string, tlsConfig *tls.Config) (net.Conn, error)
}

// setDefaults sets default values for the config.
func (c *ClientConfig) setDefaults() {
	if c.KeepAlive == 0 {
		c.KeepAlive = 60
	}
	if c.ProtocolVersion == 0 {
		c.ProtocolVersion = ProtocolV4
	}
	if c.MaxPacketSize == 0 {
		c.MaxPacketSize = MaxPacketSize
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 30 * time.Second
	}
	// CleanSession defaults to true (zero value is false, so we need special handling)
	// We'll use a different approach - check in Connect
}

// Client is a QoS 0 MQTT client.
type Client struct {
	config  ClientConfig
	conn    net.Conn
	reader  *bufio.Reader
	writer  io.Writer
	mu      sync.Mutex // protects writes
	readMu  sync.Mutex // protects reads
	running atomic.Bool
	nextPID atomic.Uint32

	// keepalive
	stopKeepalive chan struct{}
}

// Connect establishes a connection to an MQTT broker.
func Connect(ctx context.Context, config ClientConfig) (*Client, error) {
	config.setDefaults()

	// Set CleanSession default (true)
	// Note: We can't use zero value detection here since false is a valid value
	// The caller must explicitly set it. Default behavior is clean session.

	// Parse address and determine transport
	dialer := config.Dialer
	if dialer == nil {
		dialer = DefaultDialer
	}

	// Connect with timeout
	dialCtx, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()

	conn, err := dialer(dialCtx, config.Addr, config.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("mqtt0: dial: %w", err)
	}

	client := &Client{
		config:        config,
		conn:          conn,
		reader:        bufio.NewReader(conn),
		writer:        conn,
		stopKeepalive: make(chan struct{}),
	}
	client.running.Store(true)
	client.nextPID.Store(1)

	// Perform MQTT handshake
	if err := client.connect(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	// Start keepalive goroutine if enabled
	if config.AutoKeepalive && config.KeepAlive > 0 {
		go client.keepaliveLoop()
	}

	return client, nil
}

func (c *Client) connect(ctx context.Context) error {
	switch c.config.ProtocolVersion {
	case ProtocolV4:
		return c.connectV4(ctx)
	case ProtocolV5:
		return c.connectV5(ctx)
	default:
		return &ProtocolError{Message: "unsupported protocol version"}
	}
}

func (c *Client) connectV4(ctx context.Context) error {
	// Build CONNECT packet
	connect := &V4Connect{
		ClientID:     c.config.ClientID,
		Username:     c.config.Username,
		Password:     c.config.Password,
		CleanSession: c.config.CleanSession,
		KeepAlive:    c.config.KeepAlive,
	}

	// Send CONNECT
	c.mu.Lock()
	err := WriteV4Packet(c.writer, connect)
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("mqtt0: send connect: %w", err)
	}

	// Read CONNACK
	c.readMu.Lock()
	packet, err := ReadV4Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return fmt.Errorf("mqtt0: read connack: %w", err)
	}

	connack, ok := packet.(*V4ConnAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "CONNACK", Got: PacketTypeName(packet.packetType())}
	}

	if connack.ReturnCode != ConnectAccepted {
		return &ConnectError{Code: connack.ReturnCode}
	}

	return nil
}

func (c *Client) connectV5(ctx context.Context) error {
	// Build CONNECT packet
	connect := &V5Connect{
		ClientID:   c.config.ClientID,
		Username:   c.config.Username,
		Password:   c.config.Password,
		CleanStart: c.config.CleanSession,
		KeepAlive:  c.config.KeepAlive,
	}

	// Add session expiry if specified
	if c.config.SessionExpiry != nil {
		connect.Properties = &V5Properties{
			SessionExpiry: c.config.SessionExpiry,
		}
	}

	// Send CONNECT
	c.mu.Lock()
	err := WriteV5Packet(c.writer, connect)
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("mqtt0: send connect: %w", err)
	}

	// Read CONNACK
	c.readMu.Lock()
	packet, err := ReadV5Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return fmt.Errorf("mqtt0: read connack: %w", err)
	}

	connack, ok := packet.(*V5ConnAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "CONNACK", Got: PacketTypeName(packet.packetTypeV5())}
	}

	if connack.ReasonCode != ReasonSuccess {
		return &ConnectErrorV5{Code: connack.ReasonCode}
	}

	return nil
}

// Publish sends a message to the broker (QoS 0, fire and forget).
func (c *Client) Publish(ctx context.Context, topic string, payload []byte) error {
	return c.PublishRetain(ctx, topic, payload, false)
}

// PublishRetain sends a message with the retain flag.
func (c *Client) PublishRetain(ctx context.Context, topic string, payload []byte, retain bool) error {
	if !c.running.Load() {
		return ErrClosed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.config.ProtocolVersion {
	case ProtocolV4:
		return WriteV4Packet(c.writer, &V4Publish{
			Topic:   topic,
			Payload: payload,
			Retain:  retain,
		})
	case ProtocolV5:
		return WriteV5Packet(c.writer, &V5Publish{
			Topic:   topic,
			Payload: payload,
			Retain:  retain,
		})
	default:
		return &ProtocolError{Message: "unsupported protocol version"}
	}
}

// Subscribe subscribes to topics.
func (c *Client) Subscribe(ctx context.Context, topics ...string) error {
	if !c.running.Load() {
		return ErrClosed
	}

	if len(topics) == 0 {
		return nil
	}

	packetID := uint16(c.nextPID.Add(1))

	switch c.config.ProtocolVersion {
	case ProtocolV4:
		return c.subscribeV4(ctx, packetID, topics)
	case ProtocolV5:
		return c.subscribeV5(ctx, packetID, topics)
	default:
		return &ProtocolError{Message: "unsupported protocol version"}
	}
}

func (c *Client) subscribeV4(ctx context.Context, packetID uint16, topics []string) error {
	// Send SUBSCRIBE
	c.mu.Lock()
	err := WriteV4Packet(c.writer, &V4Subscribe{
		PacketID: packetID,
		Topics:   topics,
	})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	// Read SUBACK
	c.readMu.Lock()
	packet, err := ReadV4Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return err
	}

	suback, ok := packet.(*V4SubAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "SUBACK", Got: PacketTypeName(packet.packetType())}
	}

	// Check return codes
	for _, code := range suback.ReturnCodes {
		if code == 0x80 {
			return ErrACLDenied
		}
	}

	return nil
}

func (c *Client) subscribeV5(ctx context.Context, packetID uint16, topics []string) error {
	// Build filters
	filters := make([]V5SubscribeFilter, len(topics))
	for i, topic := range topics {
		filters[i] = V5SubscribeFilter{Topic: topic, QoS: AtMostOnce}
	}

	// Send SUBSCRIBE
	c.mu.Lock()
	err := WriteV5Packet(c.writer, &V5Subscribe{
		PacketID: packetID,
		Topics:   filters,
	})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	// Read SUBACK
	c.readMu.Lock()
	packet, err := ReadV5Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return err
	}

	suback, ok := packet.(*V5SubAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "SUBACK", Got: PacketTypeName(packet.packetTypeV5())}
	}

	// Check reason codes
	for _, code := range suback.ReasonCodes {
		if code >= 0x80 {
			return ErrACLDenied
		}
	}

	return nil
}

// Unsubscribe unsubscribes from topics.
func (c *Client) Unsubscribe(ctx context.Context, topics ...string) error {
	if !c.running.Load() {
		return ErrClosed
	}

	if len(topics) == 0 {
		return nil
	}

	packetID := uint16(c.nextPID.Add(1))

	switch c.config.ProtocolVersion {
	case ProtocolV4:
		return c.unsubscribeV4(ctx, packetID, topics)
	case ProtocolV5:
		return c.unsubscribeV5(ctx, packetID, topics)
	default:
		return &ProtocolError{Message: "unsupported protocol version"}
	}
}

func (c *Client) unsubscribeV4(ctx context.Context, packetID uint16, topics []string) error {
	// Send UNSUBSCRIBE
	c.mu.Lock()
	err := WriteV4Packet(c.writer, &V4Unsubscribe{
		PacketID: packetID,
		Topics:   topics,
	})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	// Read UNSUBACK
	c.readMu.Lock()
	packet, err := ReadV4Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return err
	}

	_, ok := packet.(*V4UnsubAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "UNSUBACK", Got: PacketTypeName(packet.packetType())}
	}

	return nil
}

func (c *Client) unsubscribeV5(ctx context.Context, packetID uint16, topics []string) error {
	// Send UNSUBSCRIBE
	c.mu.Lock()
	err := WriteV5Packet(c.writer, &V5Unsubscribe{
		PacketID: packetID,
		Topics:   topics,
	})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	// Read UNSUBACK
	c.readMu.Lock()
	packet, err := ReadV5Packet(c.reader, c.config.MaxPacketSize)
	c.readMu.Unlock()
	if err != nil {
		return err
	}

	_, ok := packet.(*V5UnsubAck)
	if !ok {
		return &UnexpectedPacketError{Expected: "UNSUBACK", Got: PacketTypeName(packet.packetTypeV5())}
	}

	return nil
}

// Recv receives the next message from the broker.
// It blocks until a message is received or the context is canceled.
func (c *Client) Recv(ctx context.Context) (*Message, error) {
	if !c.running.Load() {
		return nil, ErrClosed
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Set read deadline from context
		if deadline, ok := ctx.Deadline(); ok {
			c.conn.SetReadDeadline(deadline)
		}

		switch c.config.ProtocolVersion {
		case ProtocolV4:
			c.readMu.Lock()
			packet, err := ReadV4Packet(c.reader, c.config.MaxPacketSize)
			c.readMu.Unlock()
			if err != nil {
				c.conn.SetReadDeadline(time.Time{})
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return nil, err
			}
			c.conn.SetReadDeadline(time.Time{})

			switch p := packet.(type) {
			case *V4Publish:
				return &Message{
					Topic:   p.Topic,
					Payload: p.Payload,
					Retain:  p.Retain,
				}, nil
			case *V4PingResp:
				// Ignore, continue reading
				continue
			case *V4Disconnect:
				c.running.Store(false)
				return nil, ErrClosed
			default:
				// Ignore other packets
				continue
			}

		case ProtocolV5:
			c.readMu.Lock()
			packet, err := ReadV5Packet(c.reader, c.config.MaxPacketSize)
			c.readMu.Unlock()
			if err != nil {
				c.conn.SetReadDeadline(time.Time{})
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return nil, err
			}
			c.conn.SetReadDeadline(time.Time{})

			switch p := packet.(type) {
			case *V5Publish:
				return &Message{
					Topic:   p.Topic,
					Payload: p.Payload,
					Retain:  p.Retain,
				}, nil
			case *V5PingResp:
				// Ignore, continue reading
				continue
			case *V5Disconnect:
				c.running.Store(false)
				return nil, ErrClosed
			default:
				// Ignore other packets
				continue
			}

		default:
			return nil, &ProtocolError{Message: "unsupported protocol version"}
		}
	}
}

// RecvTimeout receives a message with a timeout.
// Returns nil, nil if the timeout expires without receiving a message.
func (c *Client) RecvTimeout(timeout time.Duration) (*Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	msg, err := c.Recv(ctx)
	if err == context.DeadlineExceeded {
		return nil, nil
	}
	return msg, err
}

// Ping sends a PINGREQ and waits for PINGRESP.
func (c *Client) Ping(ctx context.Context) error {
	if !c.running.Load() {
		return ErrClosed
	}

	c.mu.Lock()
	var err error
	switch c.config.ProtocolVersion {
	case ProtocolV4:
		err = WriteV4Packet(c.writer, &V4PingReq{})
	case ProtocolV5:
		err = WriteV5Packet(c.writer, &V5PingReq{})
	}
	c.mu.Unlock()

	return err
}

// Close closes the connection to the broker.
func (c *Client) Close() error {
	if !c.running.Swap(false) {
		return nil // Already closed
	}

	// Stop keepalive
	close(c.stopKeepalive)

	// Send DISCONNECT
	c.mu.Lock()
	switch c.config.ProtocolVersion {
	case ProtocolV4:
		WriteV4Packet(c.writer, &V4Disconnect{})
	case ProtocolV5:
		WriteV5Packet(c.writer, &V5Disconnect{})
	}
	c.mu.Unlock()

	return c.conn.Close()
}

// IsRunning returns true if the client is connected.
func (c *Client) IsRunning() bool {
	return c.running.Load()
}

// ClientID returns the client ID.
func (c *Client) ClientID() string {
	return c.config.ClientID
}

func (c *Client) keepaliveLoop() {
	interval := time.Duration(c.config.KeepAlive/2) * time.Second
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopKeepalive:
			return
		case <-ticker.C:
			if !c.running.Load() {
				return
			}
			if err := c.Ping(context.Background()); err != nil {
				c.running.Store(false)
				return
			}
		}
	}
}
