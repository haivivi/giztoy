package mqtt0

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Broker is a QoS 0 MQTT broker.
type Broker struct {
	// Authenticator provides authentication and ACL.
	// If nil, all connections are allowed (AllowAll).
	Authenticator Authenticator

	// Handler is called for each message received by the broker.
	Handler Handler

	// OnConnect is called when a client connects.
	OnConnect func(clientID string)

	// OnDisconnect is called when a client disconnects.
	OnDisconnect func(clientID string)

	// MaxPacketSize is the maximum packet size.
	// Default is MaxPacketSize (1MB).
	MaxPacketSize int

	// internal state
	mu                  sync.Mutex
	running             atomic.Bool
	subscriptions       *Trie[*clientHandle]
	clients             map[string]*clientHandle
	clientSubscriptions map[string][]string // track subscriptions per client for cleanup
}

// clientHandle represents a connected client.
type clientHandle struct {
	clientID string
	msgCh    chan *Message
}

// Serve starts the broker and accepts connections from the listener.
// It blocks until the listener is closed or an error occurs.
func (b *Broker) Serve(ln net.Listener) error {
	if b.running.Swap(true) {
		return ErrAlreadyRunning
	}

	b.init()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if b.running.Load() {
				return err
			}
			return nil // Closed normally
		}

		go b.handleConnection(conn)
	}
}

// ServeConn handles a single connection.
// This is useful for custom listeners or testing.
func (b *Broker) ServeConn(conn net.Conn) {
	b.init()
	b.handleConnection(conn)
}

func (b *Broker) init() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscriptions == nil {
		b.subscriptions = NewTrie[*clientHandle]()
	}
	if b.clients == nil {
		b.clients = make(map[string]*clientHandle)
	}
	if b.clientSubscriptions == nil {
		b.clientSubscriptions = make(map[string][]string)
	}
	if b.MaxPacketSize == 0 {
		b.MaxPacketSize = MaxPacketSize
	}
}

func (b *Broker) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Peek to detect protocol version
	peek, err := reader.Peek(16)
	if err != nil {
		slog.Debug("mqtt0: peek failed", "error", err)
		return
	}

	version, err := b.detectProtocolVersion(peek)
	if err != nil {
		slog.Debug("mqtt0: protocol detection failed", "error", err)
		return
	}

	switch version {
	case ProtocolV4:
		b.handleConnectionV4(conn, reader)
	case ProtocolV5:
		b.handleConnectionV5(conn, reader)
	default:
		slog.Debug("mqtt0: unsupported protocol version", "version", version)
	}
}

// detectProtocolVersion detects the MQTT protocol version from the CONNECT packet.
func (b *Broker) detectProtocolVersion(peek []byte) (ProtocolVersion, error) {
	if len(peek) < 2 {
		return 0, &ProtocolError{Message: "insufficient data"}
	}

	// Check if it's a CONNECT packet (0x10)
	if peek[0] != 0x10 {
		return 0, &ProtocolError{Message: "expected CONNECT packet"}
	}

	// Parse remaining length (variable length encoding)
	headerLen := 1
	for i := 1; i < len(peek) && i < 5; i++ {
		headerLen++
		if peek[i]&0x80 == 0 {
			break
		}
	}

	// Protocol level offset: headerLen + 2 (name length) + 4 (name "MQTT")
	protocolLevelOffset := headerLen + 2 + 4
	if len(peek) <= protocolLevelOffset {
		// Not enough data, assume v4
		return ProtocolV4, nil
	}

	protocolLevel := peek[protocolLevelOffset]
	switch protocolLevel {
	case 4:
		return ProtocolV4, nil
	case 5:
		return ProtocolV5, nil
	default:
		return 0, &ProtocolError{Message: fmt.Sprintf("unsupported protocol level: %d", protocolLevel)}
	}
}

func (b *Broker) handleConnectionV4(conn net.Conn, reader *bufio.Reader) {
	// Read CONNECT packet
	packet, err := ReadV4Packet(reader, b.MaxPacketSize)
	if err != nil {
		slog.Debug("mqtt0: read connect failed", "error", err)
		return
	}

	connect, ok := packet.(*V4Connect)
	if !ok {
		slog.Debug("mqtt0: expected CONNECT packet", "got", PacketTypeName(packet.packetType()))
		return
	}

	// Authenticate
	auth := b.Authenticator
	if auth == nil {
		auth = AllowAll{}
	}

	if !auth.Authenticate(connect.ClientID, connect.Username, connect.Password) {
		slog.Debug("mqtt0: authentication failed", "clientID", connect.ClientID)
		WriteV4Packet(conn, &V4ConnAck{ReturnCode: ConnectNotAuthorized})
		return
	}

	// Send CONNACK
	if err := WriteV4Packet(conn, &V4ConnAck{ReturnCode: ConnectAccepted}); err != nil {
		slog.Debug("mqtt0: write connack failed", "error", err)
		return
	}

	// Register client
	handle := &clientHandle{
		clientID: connect.ClientID,
		msgCh:    make(chan *Message, 100),
	}

	b.mu.Lock()
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v4")

	// Run client loop
	b.clientLoopV4(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup
	b.cleanupClient(connect.ClientID)

	if b.OnDisconnect != nil {
		b.OnDisconnect(connect.ClientID)
	}

	slog.Info("mqtt0: client disconnected", "clientID", connect.ClientID)
}

func (b *Broker) handleConnectionV5(conn net.Conn, reader *bufio.Reader) {
	// Read CONNECT packet
	packet, err := ReadV5Packet(reader, b.MaxPacketSize)
	if err != nil {
		slog.Debug("mqtt0: read connect failed", "error", err)
		return
	}

	connect, ok := packet.(*V5Connect)
	if !ok {
		slog.Debug("mqtt0: expected CONNECT packet", "got", PacketTypeName(packet.packetTypeV5()))
		return
	}

	// Authenticate
	auth := b.Authenticator
	if auth == nil {
		auth = AllowAll{}
	}

	if !auth.Authenticate(connect.ClientID, connect.Username, connect.Password) {
		slog.Debug("mqtt0: authentication failed", "clientID", connect.ClientID)
		WriteV5Packet(conn, &V5ConnAck{ReasonCode: ReasonNotAuthorized})
		return
	}

	// Send CONNACK
	if err := WriteV5Packet(conn, &V5ConnAck{ReasonCode: ReasonSuccess}); err != nil {
		slog.Debug("mqtt0: write connack failed", "error", err)
		return
	}

	// Register client
	handle := &clientHandle{
		clientID: connect.ClientID,
		msgCh:    make(chan *Message, 100),
	}

	b.mu.Lock()
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v5")

	// Run client loop
	b.clientLoopV5(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup
	b.cleanupClient(connect.ClientID)

	if b.OnDisconnect != nil {
		b.OnDisconnect(connect.ClientID)
	}

	slog.Info("mqtt0: client disconnected", "clientID", connect.ClientID)
}

func (b *Broker) clientLoopV4(conn net.Conn, reader *bufio.Reader, clientID string, keepAlive uint16, handle *clientHandle, auth Authenticator) {
	// Set up keepalive timeout
	var timeout time.Duration
	if keepAlive > 0 {
		timeout = time.Duration(keepAlive*3/2) * time.Second
	}

	readCh := make(chan V4Packet, 1)
	errCh := make(chan error, 1)

	// Start read goroutine
	go func() {
		for {
			packet, err := ReadV4Packet(reader, b.MaxPacketSize)
			if err != nil {
				errCh <- err
				return
			}
			readCh <- packet
		}
	}()

	for {
		var timeoutCh <-chan time.Time
		if timeout > 0 {
			timeoutCh = time.After(timeout)
		}

		select {
		case msg := <-handle.msgCh:
			// Send message to client
			err := WriteV4Packet(conn, &V4Publish{
				Topic:   msg.Topic,
				Payload: msg.Payload,
				Retain:  msg.Retain,
			})
			if err != nil {
				slog.Debug("mqtt0: write publish failed", "error", err)
				return
			}

		case packet := <-readCh:
			switch p := packet.(type) {
			case *V4Publish:
				b.handlePublishV4(clientID, p, auth)
			case *V4Subscribe:
				codes := b.handleSubscribeV4(clientID, handle, p.Topics, auth)
				WriteV4Packet(conn, &V4SubAck{PacketID: p.PacketID, ReturnCodes: codes})
			case *V4Unsubscribe:
				b.handleUnsubscribe(clientID, p.Topics)
				WriteV4Packet(conn, &V4UnsubAck{PacketID: p.PacketID})
			case *V4PingReq:
				WriteV4Packet(conn, &V4PingResp{})
			case *V4Disconnect:
				return
			}

		case err := <-errCh:
			if err != io.EOF {
				slog.Debug("mqtt0: read error", "error", err)
			}
			return

		case <-timeoutCh:
			slog.Debug("mqtt0: keepalive timeout", "clientID", clientID)
			return
		}
	}
}

func (b *Broker) clientLoopV5(conn net.Conn, reader *bufio.Reader, clientID string, keepAlive uint16, handle *clientHandle, auth Authenticator) {
	// Set up keepalive timeout
	var timeout time.Duration
	if keepAlive > 0 {
		timeout = time.Duration(keepAlive*3/2) * time.Second
	}

	readCh := make(chan V5Packet, 1)
	errCh := make(chan error, 1)

	// Start read goroutine
	go func() {
		for {
			packet, err := ReadV5Packet(reader, b.MaxPacketSize)
			if err != nil {
				errCh <- err
				return
			}
			readCh <- packet
		}
	}()

	for {
		var timeoutCh <-chan time.Time
		if timeout > 0 {
			timeoutCh = time.After(timeout)
		}

		select {
		case msg := <-handle.msgCh:
			// Send message to client
			err := WriteV5Packet(conn, &V5Publish{
				Topic:   msg.Topic,
				Payload: msg.Payload,
				Retain:  msg.Retain,
			})
			if err != nil {
				slog.Debug("mqtt0: write publish failed", "error", err)
				return
			}

		case packet := <-readCh:
			switch p := packet.(type) {
			case *V5Publish:
				b.handlePublishV5(clientID, p, auth)
			case *V5Subscribe:
				codes := b.handleSubscribeV5(clientID, handle, p.Topics, auth)
				WriteV5Packet(conn, &V5SubAck{PacketID: p.PacketID, ReasonCodes: codes})
			case *V5Unsubscribe:
				b.handleUnsubscribeV5(clientID, p.Topics)
				WriteV5Packet(conn, &V5UnsubAck{PacketID: p.PacketID, ReasonCodes: make([]ReasonCode, len(p.Topics))})
			case *V5PingReq:
				WriteV5Packet(conn, &V5PingResp{})
			case *V5Disconnect:
				return
			}

		case err := <-errCh:
			if err != io.EOF {
				slog.Debug("mqtt0: read error", "error", err)
			}
			return

		case <-timeoutCh:
			slog.Debug("mqtt0: keepalive timeout", "clientID", clientID)
			return
		}
	}
}

func (b *Broker) handlePublishV4(clientID string, p *V4Publish, auth Authenticator) {
	if !auth.ACL(clientID, p.Topic, true) {
		slog.Debug("mqtt0: acl denied publish", "clientID", clientID, "topic", p.Topic)
		return
	}

	msg := &Message{
		Topic:   p.Topic,
		Payload: p.Payload,
		Retain:  p.Retain,
	}

	if b.Handler != nil {
		b.Handler.HandleMessage(clientID, msg)
	}

	b.routeMessage(msg)
}

func (b *Broker) handlePublishV5(clientID string, p *V5Publish, auth Authenticator) {
	if !auth.ACL(clientID, p.Topic, true) {
		slog.Debug("mqtt0: acl denied publish", "clientID", clientID, "topic", p.Topic)
		return
	}

	msg := &Message{
		Topic:   p.Topic,
		Payload: p.Payload,
		Retain:  p.Retain,
	}

	if b.Handler != nil {
		b.Handler.HandleMessage(clientID, msg)
	}

	b.routeMessage(msg)
}

func (b *Broker) handleSubscribeV4(clientID string, handle *clientHandle, topics []string, auth Authenticator) []byte {
	codes := make([]byte, len(topics))

	for i, topic := range topics {
		if !auth.ACL(clientID, topic, false) {
			slog.Debug("mqtt0: acl denied subscribe", "clientID", clientID, "topic", topic)
			codes[i] = 0x80 // Failure
			continue
		}

		if err := b.subscriptions.Insert(topic, handle); err != nil {
			slog.Debug("mqtt0: subscribe failed", "error", err)
			codes[i] = 0x80
			continue
		}

		// Track subscription for cleanup
		b.mu.Lock()
		b.clientSubscriptions[clientID] = append(b.clientSubscriptions[clientID], topic)
		b.mu.Unlock()

		codes[i] = 0x00 // Success QoS 0
		slog.Debug("mqtt0: subscribed", "clientID", clientID, "topic", topic)
	}

	return codes
}

func (b *Broker) handleSubscribeV5(clientID string, handle *clientHandle, filters []V5SubscribeFilter, auth Authenticator) []ReasonCode {
	codes := make([]ReasonCode, len(filters))

	for i, filter := range filters {
		if !auth.ACL(clientID, filter.Topic, false) {
			slog.Debug("mqtt0: acl denied subscribe", "clientID", clientID, "topic", filter.Topic)
			codes[i] = ReasonNotAuthorized
			continue
		}

		if err := b.subscriptions.Insert(filter.Topic, handle); err != nil {
			slog.Debug("mqtt0: subscribe failed", "error", err)
			codes[i] = ReasonUnspecifiedError
			continue
		}

		// Track subscription for cleanup
		b.mu.Lock()
		b.clientSubscriptions[clientID] = append(b.clientSubscriptions[clientID], filter.Topic)
		b.mu.Unlock()

		codes[i] = ReasonGrantedQoS0
		slog.Debug("mqtt0: subscribed", "clientID", clientID, "topic", filter.Topic)
	}

	return codes
}

func (b *Broker) handleUnsubscribe(clientID string, topics []string) {
	for _, topic := range topics {
		b.subscriptions.Remove(topic, func(h *clientHandle) bool {
			return h.clientID == clientID
		})
		slog.Debug("mqtt0: unsubscribed", "clientID", clientID, "topic", topic)
	}

	// Remove from tracking
	b.mu.Lock()
	if subs, ok := b.clientSubscriptions[clientID]; ok {
		newSubs := make([]string, 0, len(subs))
		for _, s := range subs {
			found := false
			for _, t := range topics {
				if s == t {
					found = true
					break
				}
			}
			if !found {
				newSubs = append(newSubs, s)
			}
		}
		b.clientSubscriptions[clientID] = newSubs
	}
	b.mu.Unlock()
}

func (b *Broker) handleUnsubscribeV5(clientID string, topics []string) {
	b.handleUnsubscribe(clientID, topics)
}

func (b *Broker) routeMessage(msg *Message) {
	handles := b.subscriptions.Get(msg.Topic)
	for _, handle := range handles {
		select {
		case handle.msgCh <- msg:
		default:
			slog.Debug("mqtt0: message dropped (channel full)", "clientID", handle.clientID)
		}
	}
}

func (b *Broker) cleanupClient(clientID string) {
	b.mu.Lock()
	delete(b.clients, clientID)
	topics := b.clientSubscriptions[clientID]
	delete(b.clientSubscriptions, clientID)
	b.mu.Unlock()

	// Remove all subscriptions
	for _, topic := range topics {
		b.subscriptions.Remove(topic, func(h *clientHandle) bool {
			return h.clientID == clientID
		})
	}
}

// Publish sends a message from the broker to all matching subscribers.
func (b *Broker) Publish(ctx context.Context, topic string, payload []byte) error {
	msg := &Message{
		Topic:   topic,
		Payload: payload,
	}
	b.routeMessage(msg)
	return nil
}

// Close stops the broker.
func (b *Broker) Close() error {
	b.running.Store(false)
	return nil
}
