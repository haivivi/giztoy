package mqtt0

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
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
