package mqtt0

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
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

	// SysEventsEnabled enables $SYS event publishing (default: true).
	SysEventsEnabled bool

	// MaxTopicAlias is the maximum topic alias value per client (default: 65535).
	MaxTopicAlias uint16

	// internal state
	mu                  sync.Mutex
	running             atomic.Bool
	subscriptions       *Trie[*clientHandle]
	clients             map[string]*clientHandle
	clientSubscriptions map[string][]string // track subscriptions per client for cleanup
	sharedSubscriptions map[sharedKey]*sharedGroup // $share/group/topic subscriptions
}

// sharedKey is the key for shared subscriptions.
type sharedKey struct {
	group string
	topic string
}

// sharedGroup manages subscribers for a shared subscription.
type sharedGroup struct {
	subscribers []*clientHandle
	nextIndex   atomic.Uint64
}

func (g *sharedGroup) add(h *clientHandle) {
	for _, s := range g.subscribers {
		if s.clientID == h.clientID {
			return // already subscribed
		}
	}
	g.subscribers = append(g.subscribers, h)
}

func (g *sharedGroup) remove(clientID string) {
	for i, s := range g.subscribers {
		if s.clientID == clientID {
			g.subscribers = append(g.subscribers[:i], g.subscribers[i+1:]...)
			return
		}
	}
}

func (g *sharedGroup) isEmpty() bool {
	return len(g.subscribers) == 0
}

func (g *sharedGroup) nextSubscriber() *clientHandle {
	if len(g.subscribers) == 0 {
		return nil
	}
	idx := g.nextIndex.Add(1) % uint64(len(g.subscribers))
	return g.subscribers[idx]
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
	if b.sharedSubscriptions == nil {
		b.sharedSubscriptions = make(map[sharedKey]*sharedGroup)
	}
	if b.MaxPacketSize == 0 {
		b.MaxPacketSize = MaxPacketSize
	}
	if b.MaxTopicAlias == 0 {
		b.MaxTopicAlias = 65535 // default max topic alias
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
		if err := WriteV4Packet(conn, &V4ConnAck{ReturnCode: ConnectNotAuthorized}); err != nil {
			slog.Debug("mqtt0: write connack failed", "error", err)
		}
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
	// Per MQTT spec: if a client connects with a clientID already in use,
	// disconnect the old client first
	if old, exists := b.clients[connect.ClientID]; exists {
		close(old.msgCh) // Signal old client to disconnect
		// Clean up old subscriptions
		if topics, ok := b.clientSubscriptions[connect.ClientID]; ok {
			for _, topic := range topics {
				b.subscriptions.Remove(topic, func(h *clientHandle) bool {
					return h == old
				})
			}
			delete(b.clientSubscriptions, connect.ClientID)
		}
	}
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	// Publish $SYS connected event
	b.publishSysConnected(connect.ClientID, connect.Username, conn.RemoteAddr(), ProtocolV4, connect.KeepAlive)

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v4")

	// Run client loop
	b.clientLoopV4(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup
	b.cleanupClient(connect.ClientID, connect.Username)

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
		if err := WriteV5Packet(conn, &V5ConnAck{ReasonCode: ReasonNotAuthorized}); err != nil {
			slog.Debug("mqtt0: write connack failed", "error", err)
		}
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
	// Per MQTT spec: if a client connects with a clientID already in use,
	// disconnect the old client first
	if old, exists := b.clients[connect.ClientID]; exists {
		close(old.msgCh) // Signal old client to disconnect
		// Clean up old subscriptions
		if topics, ok := b.clientSubscriptions[connect.ClientID]; ok {
			for _, topic := range topics {
				b.subscriptions.Remove(topic, func(h *clientHandle) bool {
					return h == old
				})
			}
			delete(b.clientSubscriptions, connect.ClientID)
		}
	}
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	// Publish $SYS connected event
	b.publishSysConnected(connect.ClientID, connect.Username, conn.RemoteAddr(), ProtocolV5, connect.KeepAlive)

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v5")

	// Run client loop
	b.clientLoopV5(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup
	b.cleanupClient(connect.ClientID, connect.Username)

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
	doneCh := make(chan struct{})

	// Start read goroutine
	go func() {
		defer close(errCh)
		for {
			packet, err := ReadV4Packet(reader, b.MaxPacketSize)
			if err != nil {
				select {
				case errCh <- err:
				case <-doneCh:
				}
				return
			}
			select {
			case readCh <- packet:
			case <-doneCh:
				return
			}
		}
	}()

	defer close(doneCh) // Signal read goroutine to exit

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
	doneCh := make(chan struct{})

	// Start read goroutine
	go func() {
		defer close(errCh)
		for {
			packet, err := ReadV5Packet(reader, b.MaxPacketSize)
			if err != nil {
				select {
				case errCh <- err:
				case <-doneCh:
				}
				return
			}
			select {
			case readCh <- packet:
			case <-doneCh:
				return
			}
		}
	}()

	defer close(doneCh) // Signal read goroutine to exit

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
		// For shared subscriptions, check ACL on the actual topic
		aclTopic := topic
		if group, actualTopic, ok := ParseSharedTopic(topic); ok {
			aclTopic = actualTopic
			_ = group // used below
		}

		if !auth.ACL(clientID, aclTopic, false) {
			slog.Debug("mqtt0: acl denied subscribe", "clientID", clientID, "topic", topic)
			codes[i] = 0x80 // Failure
			continue
		}

		// Handle shared subscriptions
		if group, actualTopic, ok := ParseSharedTopic(topic); ok {
			b.mu.Lock()
			key := sharedKey{group: group, topic: actualTopic}
			if b.sharedSubscriptions[key] == nil {
				b.sharedSubscriptions[key] = &sharedGroup{}
			}
			b.sharedSubscriptions[key].add(handle)
			b.mu.Unlock()
			slog.Debug("mqtt0: subscribed to shared", "clientID", clientID, "group", group, "topic", actualTopic)
		} else {
			if err := b.subscriptions.Insert(topic, handle); err != nil {
				slog.Debug("mqtt0: subscribe failed", "error", err)
				codes[i] = 0x80
				continue
			}
			slog.Debug("mqtt0: subscribed", "clientID", clientID, "topic", topic)
		}

		// Track subscription for cleanup
		b.mu.Lock()
		b.clientSubscriptions[clientID] = append(b.clientSubscriptions[clientID], topic)
		b.mu.Unlock()

		codes[i] = 0x00 // Success QoS 0
	}

	return codes
}

func (b *Broker) handleSubscribeV5(clientID string, handle *clientHandle, filters []V5SubscribeFilter, auth Authenticator) []ReasonCode {
	codes := make([]ReasonCode, len(filters))

	for i, filter := range filters {
		// For shared subscriptions, check ACL on the actual topic
		aclTopic := filter.Topic
		if group, actualTopic, ok := ParseSharedTopic(filter.Topic); ok {
			aclTopic = actualTopic
			_ = group // used below
		}

		if !auth.ACL(clientID, aclTopic, false) {
			slog.Debug("mqtt0: acl denied subscribe", "clientID", clientID, "topic", filter.Topic)
			codes[i] = ReasonNotAuthorized
			continue
		}

		// Handle shared subscriptions
		if group, actualTopic, ok := ParseSharedTopic(filter.Topic); ok {
			b.mu.Lock()
			key := sharedKey{group: group, topic: actualTopic}
			if b.sharedSubscriptions[key] == nil {
				b.sharedSubscriptions[key] = &sharedGroup{}
			}
			b.sharedSubscriptions[key].add(handle)
			b.mu.Unlock()
			slog.Debug("mqtt0: subscribed to shared", "clientID", clientID, "group", group, "topic", actualTopic)
		} else {
			if err := b.subscriptions.Insert(filter.Topic, handle); err != nil {
				slog.Debug("mqtt0: subscribe failed", "error", err)
				codes[i] = ReasonUnspecifiedError
				continue
			}
			slog.Debug("mqtt0: subscribed", "clientID", clientID, "topic", filter.Topic)
		}

		// Track subscription for cleanup
		b.mu.Lock()
		b.clientSubscriptions[clientID] = append(b.clientSubscriptions[clientID], filter.Topic)
		b.mu.Unlock()

		codes[i] = ReasonGrantedQoS0
	}

	return codes
}

func (b *Broker) handleUnsubscribe(clientID string, topics []string) {
	for _, topic := range topics {
		// Handle shared subscriptions
		if group, actualTopic, ok := ParseSharedTopic(topic); ok {
			b.mu.Lock()
			key := sharedKey{group: group, topic: actualTopic}
			if g := b.sharedSubscriptions[key]; g != nil {
				g.remove(clientID)
				if g.isEmpty() {
					delete(b.sharedSubscriptions, key)
				}
			}
			b.mu.Unlock()
			slog.Debug("mqtt0: unsubscribed from shared", "clientID", clientID, "group", group, "topic", actualTopic)
		} else {
			b.subscriptions.Remove(topic, func(h *clientHandle) bool {
				return h.clientID == clientID
			})
			slog.Debug("mqtt0: unsubscribed", "clientID", clientID, "topic", topic)
		}
	}

	// Remove from tracking - optimized O(N+M) instead of O(N*M)
	b.mu.Lock()
	if subs, ok := b.clientSubscriptions[clientID]; ok {
		topicsToUnsub := make(map[string]struct{}, len(topics))
		for _, t := range topics {
			topicsToUnsub[t] = struct{}{}
		}
		newSubs := make([]string, 0, len(subs))
		for _, s := range subs {
			if _, found := topicsToUnsub[s]; !found {
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
	// Route to normal subscribers
	handles := b.subscriptions.Get(msg.Topic)
	for _, handle := range handles {
		select {
		case handle.msgCh <- msg:
		default:
			slog.Debug("mqtt0: message dropped (channel full)", "clientID", handle.clientID)
		}
	}

	// Route to shared subscription groups (round-robin)
	b.mu.Lock()
	for key, group := range b.sharedSubscriptions {
		if TopicMatches(key.topic, msg.Topic) {
			if handle := group.nextSubscriber(); handle != nil {
				select {
				case handle.msgCh <- msg:
				default:
					slog.Debug("mqtt0: message dropped (channel full)", "clientID", handle.clientID, "group", key.group)
				}
			}
		}
	}
	b.mu.Unlock()
}

func (b *Broker) cleanupClient(clientID, username string) {
	b.mu.Lock()
	delete(b.clients, clientID)
	topics := b.clientSubscriptions[clientID]
	delete(b.clientSubscriptions, clientID)
	b.mu.Unlock()

	// Remove all subscriptions
	for _, topic := range topics {
		// Handle shared subscriptions
		if group, actualTopic, ok := ParseSharedTopic(topic); ok {
			b.mu.Lock()
			key := sharedKey{group: group, topic: actualTopic}
			if g := b.sharedSubscriptions[key]; g != nil {
				g.remove(clientID)
				if g.isEmpty() {
					delete(b.sharedSubscriptions, key)
				}
			}
			b.mu.Unlock()
		} else {
			b.subscriptions.Remove(topic, func(h *clientHandle) bool {
				return h.clientID == clientID
			})
		}
	}

	// Publish $SYS disconnected event
	b.publishSysDisconnected(clientID, username)
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

// ParseSharedTopic parses a shared subscription topic.
// Format: $share/{group}/{topic}
// Returns group, actual topic, and ok=true if valid.
func ParseSharedTopic(topic string) (group, actualTopic string, ok bool) {
	if !strings.HasPrefix(topic, "$share/") {
		return "", "", false
	}
	rest := topic[7:] // Skip "$share/"
	idx := strings.Index(rest, "/")
	if idx <= 0 {
		return "", "", false
	}
	group = rest[:idx]
	actualTopic = rest[idx+1:]
	if group == "" || actualTopic == "" {
		return "", "", false
	}
	return group, actualTopic, true
}

// TopicMatches checks if a subscription pattern matches a topic.
// Supports MQTT wildcards: + (single level) and # (multi level).
// MQTT spec: wildcards should not match $ topics unless pattern also starts with $.
func TopicMatches(pattern, topic string) bool {
	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	// MQTT spec: wildcards should not match $ topics unless pattern also starts with $
	if len(topicParts) > 0 && len(topicParts[0]) > 0 && topicParts[0][0] == '$' {
		if len(patternParts) == 0 {
			return false
		}
		firstPattern := patternParts[0]
		if firstPattern == "#" || firstPattern == "+" {
			return false
		}
	}

	pIdx, tIdx := 0, 0

	for pIdx < len(patternParts) {
		p := patternParts[pIdx]

		if p == "#" {
			// # matches everything remaining
			return true
		}

		if tIdx >= len(topicParts) {
			return false
		}

		if p == "+" {
			// + matches exactly one level
			pIdx++
			tIdx++
		} else if p == topicParts[tIdx] {
			// Exact match
			pIdx++
			tIdx++
		} else {
			return false
		}
	}

	// Both should be exhausted for exact match
	return pIdx == len(patternParts) && tIdx == len(topicParts)
}

// sysConnectedEvent is the JSON payload for $SYS connected events.
type sysConnectedEvent struct {
	ClientID    string `json:"clientid"`
	Username    string `json:"username"`
	IPAddress   string `json:"ipaddress"`
	ProtoVer    int    `json:"proto_ver"`
	KeepAlive   uint16 `json:"keepalive"`
	ConnectedAt int64  `json:"connected_at"`
}

// sysDisconnectedEvent is the JSON payload for $SYS disconnected events.
type sysDisconnectedEvent struct {
	ClientID       string `json:"clientid"`
	Username       string `json:"username"`
	Reason         string `json:"reason"`
	DisconnectedAt int64  `json:"disconnected_at"`
}

// publishSysConnected publishes a $SYS client connected event.
func (b *Broker) publishSysConnected(clientID, username string, addr net.Addr, protoVer ProtocolVersion, keepAlive uint16) {
	if !b.SysEventsEnabled {
		return
	}

	topic := fmt.Sprintf("$SYS/brokers/%s/connected", clientID)

	ipAddr := ""
	if addr != nil {
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			ipAddr = tcpAddr.IP.String()
		} else {
			ipAddr = addr.String()
		}
	}

	event := sysConnectedEvent{
		ClientID:    clientID,
		Username:    username,
		IPAddress:   ipAddr,
		ProtoVer:    int(protoVer),
		KeepAlive:   keepAlive,
		ConnectedAt: time.Now().Unix(),
	}

	payload, err := json.Marshal(&event)
	if err != nil {
		slog.Debug("mqtt0: failed to marshal $SYS event", "error", err)
		return
	}

	b.routeMessage(&Message{Topic: topic, Payload: payload})
}

// publishSysDisconnected publishes a $SYS client disconnected event.
func (b *Broker) publishSysDisconnected(clientID, username string) {
	if !b.SysEventsEnabled {
		return
	}

	topic := fmt.Sprintf("$SYS/brokers/%s/disconnected", clientID)

	event := sysDisconnectedEvent{
		ClientID:       clientID,
		Username:       username,
		Reason:         "normal",
		DisconnectedAt: time.Now().Unix(),
	}

	payload, err := json.Marshal(&event)
	if err != nil {
		slog.Debug("mqtt0: failed to marshal $SYS event", "error", err)
		return
	}

	b.routeMessage(&Message{Topic: topic, Payload: payload})
}
