package mqtt0

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrokerBasic(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	var connectCount, disconnectCount atomic.Int32

	broker := &Broker{
		OnConnect: func(clientID string) {
			connectCount.Add(1)
		},
		OnDisconnect: func(clientID string) {
			disconnectCount.Add(1)
		},
	}

	go broker.Serve(ln)

	// Connect client
	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-broker-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if connectCount.Load() != 1 {
		t.Errorf("OnConnect called %d times, want 1", connectCount.Load())
	}

	client.Close()
	time.Sleep(100 * time.Millisecond)

	if disconnectCount.Load() != 1 {
		t.Errorf("OnDisconnect called %d times, want 1", disconnectCount.Load())
	}

	broker.Close()
}

func TestBrokerMessageRouting(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// Client 1: subscriber
	client1, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "subscriber",
	})
	if err != nil {
		t.Fatalf("connect client1 failed: %v", err)
	}
	defer client1.Close()

	if err := client1.Subscribe(ctx, "chat/room1"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Client 2: publisher
	client2, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "publisher",
	})
	if err != nil {
		t.Fatalf("connect client2 failed: %v", err)
	}
	defer client2.Close()

	// Publish
	if err := client2.Publish(ctx, "chat/room1", []byte("hello everyone")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Client 1 should receive
	msg, err := client1.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}

	if msg.Topic != "chat/room1" {
		t.Errorf("Topic = %q, want %q", msg.Topic, "chat/room1")
	}
	if string(msg.Payload) != "hello everyone" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "hello everyone")
	}

	broker.Close()
}

func TestBrokerWildcardRouting(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// Client with wildcard subscription
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "wildcard-sub",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe to sensor/+/temperature
	if err := client.Subscribe(ctx, "sensor/+/temperature"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publisher client
	pub, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "publisher",
	})
	if err != nil {
		t.Fatalf("connect publisher failed: %v", err)
	}
	defer pub.Close()

	// Publish to various topics
	topics := []string{
		"sensor/room1/temperature",
		"sensor/room2/temperature",
		"sensor/outdoor/temperature",
	}

	for _, topic := range topics {
		if err := pub.Publish(ctx, topic, []byte("25.5")); err != nil {
			t.Fatalf("publish to %s failed: %v", topic, err)
		}
	}

	// Should receive all 3 messages
	for i := 0; i < 3; i++ {
		msg, err := client.RecvTimeout(2 * time.Second)
		if err != nil {
			t.Fatalf("recv %d failed: %v", i, err)
		}
		if msg == nil {
			t.Fatalf("expected message %d, got nil", i)
		}
	}

	broker.Close()
}

func TestBrokerTLS(t *testing.T) {
	// Generate test certificates
	cert, key := generateTestCert(t)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  key,
		}},
	}

	addr := getTestAddr()
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// Client with TLS
	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	client, err := Connect(ctx, ClientConfig{
		Addr:      "tls://" + addr,
		ClientID:  "tls-client",
		TLSConfig: clientTLSConfig,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Test basic pub/sub over TLS
	if err := client.Subscribe(ctx, "secure/topic"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := client.Publish(ctx, "secure/topic", []byte("secure message")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	msg, err := client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	if string(msg.Payload) != "secure message" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "secure message")
	}

	broker.Close()
}

func TestBrokerMultipleProtocolVersions(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// V4 client
	v4Client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "v4-client",
		ProtocolVersion: ProtocolV4,
	})
	if err != nil {
		t.Fatalf("connect v4 failed: %v", err)
	}
	defer v4Client.Close()

	// V5 client
	v5Client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "v5-client",
		ProtocolVersion: ProtocolV5,
	})
	if err != nil {
		t.Fatalf("connect v5 failed: %v", err)
	}
	defer v5Client.Close()

	// V4 subscribes
	if err := v4Client.Subscribe(ctx, "mixed/topic"); err != nil {
		t.Fatalf("v4 subscribe failed: %v", err)
	}

	// V5 publishes
	if err := v5Client.Publish(ctx, "mixed/topic", []byte("from v5")); err != nil {
		t.Fatalf("v5 publish failed: %v", err)
	}

	// V4 should receive
	msg, err := v4Client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("v4 recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message in v4 client")
	}
	if string(msg.Payload) != "from v5" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "from v5")
	}

	// V5 subscribes
	if err := v5Client.Subscribe(ctx, "mixed/topic"); err != nil {
		t.Fatalf("v5 subscribe failed: %v", err)
	}

	// V4 publishes
	if err := v4Client.Publish(ctx, "mixed/topic", []byte("from v4")); err != nil {
		t.Fatalf("v4 publish failed: %v", err)
	}

	// V5 should receive
	msg, err = v5Client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("v5 recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message in v5 client")
	}
	if string(msg.Payload) != "from v4" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "from v4")
	}

	broker.Close()
}

func TestBrokerHandler(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	var msgCount atomic.Int32

	broker := &Broker{
		Handler: HandlerFunc(func(clientID string, msg *Message) {
			msgCount.Add(1)
		}),
	}
	go broker.Serve(ln)

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "handler-test",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Publish some messages
	for i := 0; i < 5; i++ {
		if err := client.Publish(ctx, "handler/topic", []byte("message")); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	// Wait for handler to process
	time.Sleep(200 * time.Millisecond)

	if msgCount.Load() != 5 {
		t.Errorf("handler received %d messages, want 5", msgCount.Load())
	}

	broker.Close()
}

func TestBrokerPublish(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "broker-pub-test",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe
	if err := client.Subscribe(ctx, "system/broadcast"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Broker publishes
	if err := broker.Publish(ctx, "system/broadcast", []byte("server message")); err != nil {
		t.Fatalf("broker publish failed: %v", err)
	}

	// Client should receive
	msg, err := client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	if string(msg.Payload) != "server message" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "server message")
	}

	broker.Close()
}

// generateTestCert generates a self-signed certificate for testing.
func generateTestCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate failed: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate failed: %v", err)
	}

	return cert, key
}

// BenchmarkBrokerPubSub benchmarks the broker's publish/subscribe performance.
func BenchmarkBrokerPubSub(b *testing.B) {
	addr := net.JoinHostPort("127.0.0.1", "0")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		b.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()
	autoKeepalive := false
	client, err := Connect(ctx, ClientConfig{
		Addr:          "tcp://" + ln.Addr().String(),
		ClientID:      "bench-client",
		AutoKeepalive: &autoKeepalive,
	})
	if err != nil {
		b.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	payload := []byte("benchmark payload data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := client.Publish(ctx, "bench/topic", payload); err != nil {
				b.Errorf("publish failed: %v", err)
			}
		}
	})

	broker.Close()
}

// BenchmarkBrokerMessageRouting benchmarks message routing through the broker.
func BenchmarkBrokerMessageRouting(b *testing.B) {
	addr := net.JoinHostPort("127.0.0.1", "0")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		b.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// Create subscriber
	autoKeepaliveSub := false
	sub, err := Connect(ctx, ClientConfig{
		Addr:          "tcp://" + ln.Addr().String(),
		ClientID:      "bench-subscriber",
		AutoKeepalive: &autoKeepaliveSub,
	})
	if err != nil {
		b.Fatalf("connect subscriber failed: %v", err)
	}
	defer sub.Close()

	if err := sub.Subscribe(ctx, "bench/+/data"); err != nil {
		b.Fatalf("subscribe failed: %v", err)
	}

	// Create publisher
	autoKeepalivePub := false
	pub, err := Connect(ctx, ClientConfig{
		Addr:          "tcp://" + ln.Addr().String(),
		ClientID:      "bench-publisher",
		AutoKeepalive: &autoKeepalivePub,
	})
	if err != nil {
		b.Fatalf("connect publisher failed: %v", err)
	}
	defer pub.Close()

	payload := []byte("benchmark message")

	// Start receiver goroutine
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				sub.RecvTimeout(100 * time.Millisecond)
			}
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pub.Publish(ctx, "bench/room1/data", payload); err != nil {
			b.Errorf("publish failed: %v", err)
		}
	}

	close(done)
	broker.Close()
}

// generateTestCertPEM generates PEM-encoded certificate and key for testing.
func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	cert, key := generateTestCert(t)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certPEM, keyPEM
}

// TestParseSharedTopic tests shared subscription topic parsing.
func TestParseSharedTopic(t *testing.T) {
	tests := []struct {
		topic    string
		group    string
		actual   string
		wantOk   bool
	}{
		{"$share/group1/sensor/data", "group1", "sensor/data", true},
		{"$share/g/a/b/c", "g", "a/b/c", true},
		{"sensor/data", "", "", false},
		{"$share/", "", "", false},
		{"$share/group/", "", "", false},
		{"$share//topic", "", "", false},
		{"$share/group/sensor/+/data", "group", "sensor/+/data", true},
		{"$share/group/sensor/#", "group", "sensor/#", true},
	}

	for _, tt := range tests {
		group, actual, ok := ParseSharedTopic(tt.topic)
		if ok != tt.wantOk {
			t.Errorf("ParseSharedTopic(%q) ok=%v, want %v", tt.topic, ok, tt.wantOk)
			continue
		}
		if ok {
			if group != tt.group {
				t.Errorf("ParseSharedTopic(%q) group=%q, want %q", tt.topic, group, tt.group)
			}
			if actual != tt.actual {
				t.Errorf("ParseSharedTopic(%q) actual=%q, want %q", tt.topic, actual, tt.actual)
			}
		}
	}
}

// TestTopicMatches tests topic matching with wildcards.
func TestTopicMatches(t *testing.T) {
	tests := []struct {
		pattern string
		topic   string
		want    bool
	}{
		// Exact match
		{"sensor/data", "sensor/data", true},
		{"sensor/data", "sensor/info", false},
		
		// Single-level wildcard
		{"sensor/+/data", "sensor/1/data", true},
		{"sensor/+/data", "sensor/room1/data", true},
		{"sensor/+/data", "sensor/data", false},
		{"+/data", "sensor/data", true},
		{"+", "sensor", true},
		{"+", "sensor/data", false},
		
		// Multi-level wildcard
		{"sensor/#", "sensor/data", true},
		{"sensor/#", "sensor/a/b/c", true},
		{"#", "sensor/data", true},
		{"#", "a/b/c/d", true},
		
		// $ topics should NOT match wildcards at root level
		{"#", "$SYS/broker/stats", false},
		{"+/stats", "$SYS/stats", false},
		{"+", "$SYS", false},
		
		// BUT explicit $ patterns SHOULD match $ topics
		{"$SYS/#", "$SYS/broker/stats", true},
		{"$SYS/+/stats", "$SYS/broker/stats", true},
		{"$SYS/broker/stats", "$SYS/broker/stats", true},
	}

	for _, tt := range tests {
		got := TopicMatches(tt.pattern, tt.topic)
		if got != tt.want {
			t.Errorf("TopicMatches(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
		}
	}
}

// TestSharedSubscriptions tests shared subscription round-robin delivery.
func TestSharedSubscriptions(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{SysEventsEnabled: false} // Disable $SYS events for this test
	go broker.Serve(ln)

	ctx := context.Background()

	// Create two subscribers in the same shared group
	sub1, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "shared-sub-1",
	})
	if err != nil {
		t.Fatalf("connect sub1 failed: %v", err)
	}
	defer sub1.Close()

	sub2, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "shared-sub-2",
	})
	if err != nil {
		t.Fatalf("connect sub2 failed: %v", err)
	}
	defer sub2.Close()

	// Subscribe both to the same shared subscription
	if err := sub1.Subscribe(ctx, "$share/mygroup/test/topic"); err != nil {
		t.Fatalf("subscribe sub1 failed: %v", err)
	}
	if err := sub2.Subscribe(ctx, "$share/mygroup/test/topic"); err != nil {
		t.Fatalf("subscribe sub2 failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create publisher
	pub, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "shared-publisher",
	})
	if err != nil {
		t.Fatalf("connect publisher failed: %v", err)
	}
	defer pub.Close()

	// Publish 4 messages
	for i := 0; i < 4; i++ {
		if err := pub.Publish(ctx, "test/topic", []byte("msg")); err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Count received messages
	var sub1Count, sub2Count int
	for {
		msg, err := sub1.RecvTimeout(50 * time.Millisecond)
		if err != nil || msg == nil {
			break
		}
		sub1Count++
	}
	for {
		msg, err := sub2.RecvTimeout(50 * time.Millisecond)
		if err != nil || msg == nil {
			break
		}
		sub2Count++
	}

	total := sub1Count + sub2Count
	if total != 4 {
		t.Errorf("Expected exactly 4 messages total, got %d (sub1=%d, sub2=%d)", total, sub1Count, sub2Count)
	}
	if sub1Count == 0 || sub2Count == 0 {
		t.Errorf("Expected messages distributed between both subscribers, got sub1=%d, sub2=%d", sub1Count, sub2Count)
	}

	broker.Close()
}

// TestSysEvents tests $SYS client connect/disconnect events.
func TestSysEvents(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{SysEventsEnabled: true}
	go broker.Serve(ln)

	ctx := context.Background()

	// Create a subscriber for $SYS events
	sysMonitor, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "sys-monitor",
	})
	if err != nil {
		t.Fatalf("connect sys-monitor failed: %v", err)
	}
	defer sysMonitor.Close()

	if err := sysMonitor.Subscribe(ctx, "$SYS/brokers/+/connected"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if err := sysMonitor.Subscribe(ctx, "$SYS/brokers/+/disconnected"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Connect a test client
	testClient, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-client-sys",
	})
	if err != nil {
		t.Fatalf("connect test-client failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Check for connected event
	msg, err := sysMonitor.RecvTimeout(500 * time.Millisecond)
	if err != nil {
		t.Logf("recv connected event error: %v", err)
	}
	if msg != nil {
		if !strings.Contains(msg.Topic, "connected") {
			t.Errorf("Expected connected event, got topic: %s", msg.Topic)
		}
	}

	// Disconnect test client
	testClient.Close()
	time.Sleep(100 * time.Millisecond)

	// Check for disconnected event
	msg, err = sysMonitor.RecvTimeout(500 * time.Millisecond)
	if err != nil {
		t.Logf("recv disconnected event error: %v", err)
	}
	if msg != nil {
		if !strings.Contains(msg.Topic, "disconnected") {
			t.Errorf("Expected disconnected event, got topic: %s", msg.Topic)
		}
	}

	broker.Close()
}

func TestTopicAlias(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{
		MaxTopicAlias: 5, // Allow up to 5 topic aliases
	}
	go broker.Serve(ln)

	// Connect subscriber with regular client
	ctx := context.Background()
	sub, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "topic-alias-sub",
		ProtocolVersion: ProtocolV5,
	})
	if err != nil {
		t.Fatalf("connect subscriber failed: %v", err)
	}
	defer sub.Close()

	if err := sub.Subscribe(ctx, "alias/test/#"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Connect publisher with raw TCP to send custom packets with topic alias
	pubConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer pubConn.Close()

	// Send CONNECT
	connectPkt := &V5Connect{
		ClientID:   "topic-alias-pub",
		CleanStart: true,
		KeepAlive:  60,
	}
	if err := WriteV5Packet(pubConn, connectPkt); err != nil {
		t.Fatalf("write connect failed: %v", err)
	}

	// Read CONNACK
	reader := bufio.NewReader(pubConn)
	_, err = ReadV5Packet(reader, 1024*1024)
	if err != nil {
		t.Fatalf("read connack failed: %v", err)
	}

	// Test 1: Set alias 1 = "alias/test/one"
	alias1 := uint16(1)
	pub1 := &V5Publish{
		Topic:      "alias/test/one",
		Payload:    []byte("msg1"),
		Properties: &V5Properties{TopicAlias: &alias1},
	}
	if err := WriteV5Packet(pubConn, pub1); err != nil {
		t.Fatalf("write publish1 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, err := sub.RecvTimeout(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("recv msg1 failed: %v", err)
	}
	if msg.Topic != "alias/test/one" {
		t.Errorf("Expected topic alias/test/one, got %s", msg.Topic)
	}
	if string(msg.Payload) != "msg1" {
		t.Errorf("Expected payload msg1, got %s", string(msg.Payload))
	}

	// Test 2: Reuse alias 1 with empty topic
	pub2 := &V5Publish{
		Topic:      "", // Empty topic - should resolve from alias
		Payload:    []byte("msg2"),
		Properties: &V5Properties{TopicAlias: &alias1},
	}
	if err := WriteV5Packet(pubConn, pub2); err != nil {
		t.Fatalf("write publish2 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, err = sub.RecvTimeout(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("recv msg2 failed: %v", err)
	}
	if msg.Topic != "alias/test/one" {
		t.Errorf("Expected topic alias/test/one (resolved from alias), got %s", msg.Topic)
	}
	if string(msg.Payload) != "msg2" {
		t.Errorf("Expected payload msg2, got %s", string(msg.Payload))
	}

	// Small delay to ensure all previous messages are fully processed
	time.Sleep(100 * time.Millisecond)

	// Test 3: Invalid alias 0 should be ignored (no message delivered)
	alias0 := uint16(0)
	pub3 := &V5Publish{
		Topic:      "alias/test/zero",
		Payload:    []byte("msg_zero"),
		Properties: &V5Properties{TopicAlias: &alias0},
	}
	if err := WriteV5Packet(pubConn, pub3); err != nil {
		t.Fatalf("write publish3 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, _ = sub.RecvTimeout(100 * time.Millisecond)
	if msg != nil {
		t.Errorf("Expected no message for alias 0, but received: topic=%s", msg.Topic)
	}

	// Test 4: Alias exceeding limit (max=5, use alias=10) should be ignored
	alias10 := uint16(10)
	pub4 := &V5Publish{
		Topic:      "alias/test/exceed",
		Payload:    []byte("msg_exceed"),
		Properties: &V5Properties{TopicAlias: &alias10},
	}
	if err := WriteV5Packet(pubConn, pub4); err != nil {
		t.Fatalf("write publish4 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, _ = sub.RecvTimeout(100 * time.Millisecond)
	if msg != nil {
		t.Errorf("Expected no message for alias exceeding limit, but received: topic=%s", msg.Topic)
	}

	// Test 5: Unknown alias (empty topic with alias 3 that was never set)
	alias3 := uint16(3)
	pub5 := &V5Publish{
		Topic:      "", // Empty topic
		Payload:    []byte("msg_unknown"),
		Properties: &V5Properties{TopicAlias: &alias3},
	}
	if err := WriteV5Packet(pubConn, pub5); err != nil {
		t.Fatalf("write publish5 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, _ = sub.RecvTimeout(100 * time.Millisecond)
	if msg != nil {
		t.Errorf("Expected no message for unknown alias, but received: topic=%s", msg.Topic)
	}

	// Test 6: Valid message without alias should still work
	pub6 := &V5Publish{
		Topic:   "alias/test/normal",
		Payload: []byte("msg_normal"),
	}
	if err := WriteV5Packet(pubConn, pub6); err != nil {
		t.Fatalf("write publish6 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	msg, err = sub.RecvTimeout(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("recv msg_normal failed: %v", err)
	}
	if msg.Topic != "alias/test/normal" {
		t.Errorf("Expected topic alias/test/normal, got %s", msg.Topic)
	}
	if string(msg.Payload) != "msg_normal" {
		t.Errorf("Expected payload msg_normal, got %s", string(msg.Payload))
	}

	broker.Close()
}

func TestTopicLengthLimit(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{
		MaxTopicLength: 50, // Limit topic length to 50 bytes
	}
	go broker.Serve(ln)

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "topic-length-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Short topic should work
	if err := client.Publish(ctx, "short/topic", []byte("ok")); err != nil {
		t.Fatalf("publish short topic failed: %v", err)
	}

	// Long topic (>50 bytes) should be silently dropped by broker
	longTopic := "this/is/a/very/long/topic/that/exceeds/the/limit/of/fifty/bytes"
	if err := client.Publish(ctx, longTopic, []byte("dropped")); err != nil {
		t.Fatalf("publish long topic failed: %v", err)
	}

	// No error expected - broker just drops the message
	broker.Close()
}

func TestDollarTopicProtection(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{
		SysEventsEnabled: true,
	}
	go broker.Serve(ln)

	ctx := context.Background()

	// Subscribe to $SYS topics
	sub, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "dollar-sub",
	})
	if err != nil {
		t.Fatalf("connect subscriber failed: %v", err)
	}
	defer sub.Close()

	if err := sub.Subscribe(ctx, "$SYS/#"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Connect publisher that will try to publish to $SYS
	pub, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "dollar-pub",
	})
	if err != nil {
		t.Fatalf("connect publisher failed: %v", err)
	}
	defer pub.Close()

	// Try to publish to $SYS topic (should be blocked)
	if err := pub.Publish(ctx, "$SYS/fake/event", []byte("malicious")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Subscriber should NOT receive the fake event
	msg, err := sub.RecvTimeout(200 * time.Millisecond)
	if err == nil && msg != nil {
		// Check if the message is the real $SYS connected event or the fake one
		if strings.Contains(msg.Topic, "fake") {
			t.Errorf("Broker should block client publishing to $ topics, got: %s", msg.Topic)
		}
	}

	broker.Close()
}

func TestSubscriptionLimit(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{
		MaxSubscriptionsPerClient: 5, // Limit to 5 subscriptions
	}
	go broker.Serve(ln)

	ctx := context.Background()

	// Client that will hit the subscription limit
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "sub-limit-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe to 5 topics (should succeed)
	for i := 0; i < 5; i++ {
		topic := "topic/" + string(rune('a'+i))
		if err := client.Subscribe(ctx, topic); err != nil {
			t.Fatalf("subscribe %d failed: %v", i, err)
		}
	}

	// 6th subscription to "topic/f" - broker rejects but client.Subscribe doesn't check SUBACK
	if err := client.Subscribe(ctx, "topic/f"); err != nil {
		t.Logf("6th subscription error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create another client (verifier) that subscribes to "topic/f" to verify it works
	verifier, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "verifier-client",
	})
	if err != nil {
		t.Fatalf("connect verifier failed: %v", err)
	}
	defer verifier.Close()

	if err := verifier.Subscribe(ctx, "topic/f"); err != nil {
		t.Fatalf("verifier subscribe failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Create a publisher
	publisher, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "publisher-client",
	})
	if err != nil {
		t.Fatalf("connect publisher failed: %v", err)
	}
	defer publisher.Close()

	// Publish to "topic/f"
	if err := publisher.Publish(ctx, "topic/f", []byte("test-message")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Verifier should receive the message (its subscription succeeded)
	msg, err := verifier.RecvTimeout(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("verifier should receive message: %v", err)
	}
	if msg.Topic != "topic/f" {
		t.Errorf("verifier got wrong topic: %s", msg.Topic)
	}

	// Original client should NOT receive the message (6th subscription was rejected)
	msg, err = client.RecvTimeout(200 * time.Millisecond)
	if err == nil && msg != nil && msg.Topic == "topic/f" {
		t.Errorf("client should NOT receive message on topic/f (subscription was rejected)")
	}

	broker.Close()
}

// TestDuplicateClientID tests that when a new client connects with the same clientID,
// the old client is disconnected gracefully without panic.
func TestDuplicateClientID(t *testing.T) {
	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	broker := &Broker{}
	go broker.Serve(ln)

	ctx := context.Background()

	// Connect first client
	client1, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "duplicate-test",
	})
	if err != nil {
		t.Fatalf("connect client1 failed: %v", err)
	}

	// Subscribe to a topic
	if err := client1.Subscribe(ctx, "test/dup"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Connect second client with same clientID
	client2, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "duplicate-test", // Same clientID
	})
	if err != nil {
		t.Fatalf("connect client2 failed: %v", err)
	}
	defer client2.Close()

	// Subscribe with new client
	if err := client2.Subscribe(ctx, "test/dup"); err != nil {
		t.Fatalf("client2 subscribe failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	pub, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "dup-publisher",
	})
	if err != nil {
		t.Fatalf("connect publisher failed: %v", err)
	}
	defer pub.Close()

	if err := pub.Publish(ctx, "test/dup", []byte("hello")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Client2 should receive the message
	msg, err := client2.RecvTimeout(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("client2 should receive message: %v", err)
	}
	if msg == nil {
		t.Fatal("client2 received nil message")
	}
	if msg.Topic != "test/dup" {
		t.Errorf("wrong topic: got %s, want test/dup", msg.Topic)
	}
	if string(msg.Payload) != "hello" {
		t.Errorf("wrong payload: got %s, want hello", string(msg.Payload))
	}

	// Old client (client1) should be disconnected and not receive any messages
	// Note: client1 may already be closed, so we don't check error
	_ = client1.Close()

	broker.Close()
}
