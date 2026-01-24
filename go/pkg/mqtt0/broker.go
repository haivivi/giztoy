package mqtt0

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
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

	// SysEventsEnabled enables $SYS event publishing.
	// Note: Must be explicitly set to true to enable; default is false.
	SysEventsEnabled bool

	// MaxTopicAlias is the maximum topic alias value per client (MQTT 5.0).
	// Default: 65535. Range: 1-65535 (0 is treated as default).
	MaxTopicAlias uint16

	// MaxTopicLength is the maximum topic string length in bytes.
	// Default: 256. MQTT spec allows up to 65535 (0 is treated as default).
	MaxTopicLength int

	// MaxSubscriptionsPerClient is the maximum number of subscriptions per client.
	// Default: 100. Range: 1+ (0 is treated as default).
	MaxSubscriptionsPerClient int

	// internal state
	mu                  sync.Mutex
	running             atomic.Bool
	subscriptions       *Trie[*clientHandle]
	clients             map[string]*clientHandle
	clientSubscriptions map[string][]string // track subscriptions per client for cleanup
	sharedTrie          *Trie[*sharedEntry] // shared subscriptions trie for O(topic_length) lookup
}

// sharedGroup manages subscribers for a shared subscription.
// All methods are thread-safe via internal mutex.
type sharedGroup struct {
	mu          sync.RWMutex
	subscribers []*clientHandle
	nextIndex   atomic.Uint64
}

// sharedEntry is an entry in the shared subscription trie.
type sharedEntry struct {
	groupName string
	group     *sharedGroup
}

func (g *sharedGroup) add(h *clientHandle) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, s := range g.subscribers {
		if s.clientID == h.clientID {
			return // already subscribed
		}
	}
	g.subscribers = append(g.subscribers, h)
}

func (g *sharedGroup) remove(clientID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, s := range g.subscribers {
		if s.clientID == clientID {
			g.subscribers = append(g.subscribers[:i], g.subscribers[i+1:]...)
			return
		}
	}
}

// removeByHandle removes a subscriber only if it matches the given handle pointer.
// This prevents race conditions where a new client with the same clientID replaces an old one.
func (g *sharedGroup) removeByHandle(h *clientHandle) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, s := range g.subscribers {
		if s == h { // Pointer comparison
			g.subscribers = append(g.subscribers[:i], g.subscribers[i+1:]...)
			return
		}
	}
}

func (g *sharedGroup) isEmpty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.subscribers) == 0
}

func (g *sharedGroup) nextSubscriber() *clientHandle {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if len(g.subscribers) == 0 {
		return nil
	}
	// Use (Add(1) - 1) to get pre-increment value, consistent with Rust's fetch_add
	idx := (g.nextIndex.Add(1) - 1) % uint64(len(g.subscribers))
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
	if b.sharedTrie == nil {
		b.sharedTrie = NewTrie[*sharedEntry]()
	}
	if b.MaxPacketSize == 0 {
		b.MaxPacketSize = MaxPacketSize
	}
	if b.MaxTopicAlias == 0 {
		b.MaxTopicAlias = 65535
	}
	if b.MaxTopicLength == 0 {
		b.MaxTopicLength = 256
	}
	if b.MaxSubscriptionsPerClient == 0 {
		b.MaxSubscriptionsPerClient = 100
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
	var oldHandle *clientHandle
	var oldTopics []string
	if old, exists := b.clients[connect.ClientID]; exists {
		close(old.msgCh) // Signal old client to disconnect
		oldHandle = old
		oldTopics = b.clientSubscriptions[connect.ClientID]
		delete(b.clientSubscriptions, connect.ClientID)
	}
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	// Clean up old client's subscriptions (both normal and shared) outside the lock
	if oldHandle != nil {
		b.removeClientSubscriptions(oldTopics, oldHandle)
	}

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	// Publish $SYS connected event
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		b.publishSysConnected(connect.ClientID, connect.Username, tcpAddr.AddrPort(), ProtocolV4, connect.KeepAlive)
	}

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v4")

	// Run client loop
	b.clientLoopV4(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup - pass handle for pointer comparison to prevent race conditions
	b.cleanupClient(connect.ClientID, connect.Username, handle)

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
	var oldHandle *clientHandle
	var oldTopics []string
	if old, exists := b.clients[connect.ClientID]; exists {
		close(old.msgCh) // Signal old client to disconnect
		oldHandle = old
		oldTopics = b.clientSubscriptions[connect.ClientID]
		delete(b.clientSubscriptions, connect.ClientID)
	}
	b.clients[connect.ClientID] = handle
	b.mu.Unlock()

	// Clean up old client's subscriptions (both normal and shared) outside the lock
	if oldHandle != nil {
		b.removeClientSubscriptions(oldTopics, oldHandle)
	}

	if b.OnConnect != nil {
		b.OnConnect(connect.ClientID)
	}

	// Publish $SYS connected event
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		b.publishSysConnected(connect.ClientID, connect.Username, tcpAddr.AddrPort(), ProtocolV5, connect.KeepAlive)
	}

	slog.Info("mqtt0: client connected", "clientID", connect.ClientID, "version", "v5")

	// Run client loop
	b.clientLoopV5(conn, reader, connect.ClientID, connect.KeepAlive, handle, auth)

	// Cleanup - pass handle for pointer comparison to prevent race conditions
	b.cleanupClient(connect.ClientID, connect.Username, handle)

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

	// Topic alias map for this client (MQTT 5.0)
	topicAliases := make(map[uint16]string)

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
				b.handlePublishV5(clientID, p, auth, topicAliases)
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
	// Enforce topic length limit
	if len(p.Topic) > b.MaxTopicLength {
		slog.Debug("mqtt0: topic too long", "clientID", clientID, "len", len(p.Topic), "max", b.MaxTopicLength)
		return
	}

	// Prevent clients from publishing to $ topics (MQTT spec 3.3.1.3)
	if len(p.Topic) > 0 && p.Topic[0] == '$' {
		slog.Debug("mqtt0: client cannot publish to $ topic", "clientID", clientID, "topic", p.Topic)
		return
	}

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

func (b *Broker) handlePublishV5(clientID string, p *V5Publish, auth Authenticator, topicAliases map[uint16]string) {
	// Resolve topic from packet or alias
	topic := p.Topic

	// Handle Topic Alias (MQTT 5.0)
	if p.Properties != nil && p.Properties.TopicAlias != nil {
		alias := *p.Properties.TopicAlias

		// Reject alias 0 as per MQTT 5.0 spec
		if alias == 0 {
			slog.Debug("mqtt0: invalid topic alias 0", "clientID", clientID)
			return
		}

		// Enforce max topic alias limit
		if alias > b.MaxTopicAlias {
			slog.Debug("mqtt0: topic alias exceeds limit", "clientID", clientID, "alias", alias, "max", b.MaxTopicAlias)
			return
		}

		if topic != "" {
			// Topic is provided with alias - update the mapping
			// Enforce topic length limit
			if len(topic) > b.MaxTopicLength {
				slog.Debug("mqtt0: topic too long for alias", "clientID", clientID, "len", len(topic), "max", b.MaxTopicLength)
				return
			}
			topicAliases[alias] = topic
			slog.Debug("mqtt0: set topic alias", "clientID", clientID, "alias", alias, "topic", topic)
		} else {
			// Topic is empty - look up from alias mapping
			resolved, ok := topicAliases[alias]
			if !ok {
				slog.Debug("mqtt0: unknown topic alias", "clientID", clientID, "alias", alias)
				return
			}
			topic = resolved
		}
	}

	// Validate topic
	if topic == "" {
		slog.Debug("mqtt0: empty topic in publish", "clientID", clientID)
		return
	}

	// Enforce topic length limit
	if len(topic) > b.MaxTopicLength {
		slog.Debug("mqtt0: topic too long", "clientID", clientID, "len", len(topic), "max", b.MaxTopicLength)
		return
	}

	// Prevent clients from publishing to $ topics (MQTT spec 3.3.1.3)
	if len(topic) > 0 && topic[0] == '$' {
		slog.Debug("mqtt0: client cannot publish to $ topic", "clientID", clientID, "topic", topic)
		return
	}

	if !auth.ACL(clientID, topic, true) {
		slog.Debug("mqtt0: acl denied publish", "clientID", clientID, "topic", topic)
		return
	}

	msg := &Message{
		Topic:   topic,
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
		// Check subscription limit
		b.mu.Lock()
		currentCount := len(b.clientSubscriptions[clientID])
		b.mu.Unlock()
		if b.MaxSubscriptionsPerClient > 0 && currentCount >= b.MaxSubscriptionsPerClient {
			slog.Debug("mqtt0: subscription limit exceeded", "clientID", clientID, "current", currentCount, "max", b.MaxSubscriptionsPerClient)
			codes[i] = 0x80 // Failure
			continue
		}

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
			// Use Trie for O(topic_length) lookup
			b.sharedTrie.Update(actualTopic, func(entries *[]*sharedEntry) {
				// Find existing group or create new one
				for _, e := range *entries {
					if e.groupName == group {
						e.group.add(handle)
						return
					}
				}
				// Create new group
				g := &sharedGroup{}
				g.add(handle)
				*entries = append(*entries, &sharedEntry{groupName: group, group: g})
			})
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
		// Check subscription limit
		b.mu.Lock()
		currentCount := len(b.clientSubscriptions[clientID])
		b.mu.Unlock()
		if b.MaxSubscriptionsPerClient > 0 && currentCount >= b.MaxSubscriptionsPerClient {
			slog.Debug("mqtt0: subscription limit exceeded", "clientID", clientID, "current", currentCount, "max", b.MaxSubscriptionsPerClient)
			codes[i] = ReasonQuotaExceeded
			continue
		}

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
			// Use Trie for O(topic_length) lookup
			b.sharedTrie.Update(actualTopic, func(entries *[]*sharedEntry) {
				// Find existing group or create new one
				for _, e := range *entries {
					if e.groupName == group {
						e.group.add(handle)
						return
					}
				}
				// Create new group
				g := &sharedGroup{}
				g.add(handle)
				*entries = append(*entries, &sharedEntry{groupName: group, group: g})
			})
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

// removeOneSubscription removes a single subscription by clientID.
// This is used by handleUnsubscribe for explicit unsubscribe requests.
func (b *Broker) removeOneSubscription(clientID, topic string) {
	if group, actualTopic, ok := ParseSharedTopic(topic); ok {
		b.sharedTrie.Update(actualTopic, func(entries *[]*sharedEntry) {
			for _, e := range *entries {
				if e.groupName == group {
					e.group.remove(clientID)
					break
				}
			}
			// Remove empty groups
			newEntries := (*entries)[:0]
			for _, e := range *entries {
				if !e.group.isEmpty() {
					newEntries = append(newEntries, e)
				}
			}
			*entries = newEntries
		})
		slog.Debug("mqtt0: unsubscribed from shared", "clientID", clientID, "group", group, "topic", actualTopic)
	} else {
		b.subscriptions.Remove(topic, func(h *clientHandle) bool {
			return h.clientID == clientID
		})
		slog.Debug("mqtt0: unsubscribed", "clientID", clientID, "topic", topic)
	}
}

func (b *Broker) handleUnsubscribe(clientID string, topics []string) {
	for _, topic := range topics {
		b.removeOneSubscription(clientID, topic)
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

	// Route to shared subscription groups (round-robin) using Trie lookup - O(topic_length)
	entries := b.sharedTrie.Get(msg.Topic)
	for _, entry := range entries {
		if handle := entry.group.nextSubscriber(); handle != nil {
			select {
			case handle.msgCh <- msg:
			default:
				slog.Debug("mqtt0: message dropped (channel full)", "clientID", handle.clientID, "group", entry.groupName)
			}
		}
	}
}

// removeClientSubscriptions removes a client's subscriptions from both
// normal subscriptions trie and shared subscriptions trie.
// Uses pointer comparison to ensure only the correct client instance is removed.
func (b *Broker) removeClientSubscriptions(topics []string, handle *clientHandle) {
	for _, topic := range topics {
		// Handle shared subscriptions
		if group, actualTopic, ok := ParseSharedTopic(topic); ok {
			// Use Trie for cleanup - use pointer comparison
			b.sharedTrie.Update(actualTopic, func(entries *[]*sharedEntry) {
				for _, e := range *entries {
					if e.groupName == group {
						e.group.removeByHandle(handle) // Pointer comparison
						break
					}
				}
				// Remove empty groups
				newEntries := (*entries)[:0]
				for _, e := range *entries {
					if !e.group.isEmpty() {
						newEntries = append(newEntries, e)
					}
				}
				*entries = newEntries
			})
		} else {
			// Use pointer comparison for normal subscriptions
			b.subscriptions.Remove(topic, func(h *clientHandle) bool {
				return h == handle // Pointer comparison
			})
		}
	}
}

// cleanupClient removes a client and its subscriptions.
// The handle parameter is used for pointer comparison to prevent race conditions
// where a new client with the same clientID replaces an old one before cleanup.
func (b *Broker) cleanupClient(clientID, username string, handle *clientHandle) {
	b.mu.Lock()
	// Only delete from clients map if the current handle matches (pointer comparison)
	// This prevents removing a new client that connected with the same clientID
	var topics []string
	if current, exists := b.clients[clientID]; exists && current == handle {
		delete(b.clients, clientID)
		// Only remove subscriptions tracking if this is the correct client instance
		// This prevents a stale cleanup from wiping a new client's subscription data
		topics = b.clientSubscriptions[clientID]
		delete(b.clientSubscriptions, clientID)
	}
	b.mu.Unlock()

	// If topics is nil, this cleanup is for a stale client - skip subscription removal
	if topics == nil {
		return
	}

	// Remove all subscriptions (both normal and shared)
	b.removeClientSubscriptions(topics, handle)

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
func (b *Broker) publishSysConnected(clientID, username string, addr netip.AddrPort, protoVer ProtocolVersion, keepAlive uint16) {
	if !b.SysEventsEnabled {
		return
	}

	topic := fmt.Sprintf("$SYS/brokers/%s/connected", clientID)

	event := sysConnectedEvent{
		ClientID:    clientID,
		Username:    username,
		IPAddress:   addr.Addr().String(),
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
