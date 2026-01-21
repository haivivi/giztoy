package mqtt0

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

var testPort atomic.Uint32

func init() {
	testPort.Store(19000)
}

func getTestAddr() string {
	port := testPort.Add(1)
	return net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
}

// startTestBroker starts a test broker and returns its address.
func startTestBroker(t *testing.T, auth Authenticator) (string, func()) {
	t.Helper()

	addr := getTestAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	broker := &Broker{
		Authenticator: auth,
	}

	go broker.Serve(ln)

	// Wait for broker to start
	time.Sleep(50 * time.Millisecond)

	return addr, func() {
		ln.Close()
		broker.Close()
	}
}

func TestClientConnectV4(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "test-client",
		ProtocolVersion: ProtocolV4,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	if !client.IsRunning() {
		t.Error("client should be running")
	}

	if client.ClientID() != "test-client" {
		t.Errorf("ClientID() = %q, want %q", client.ClientID(), "test-client")
	}
}

func TestClientConnectV5(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "test-client-v5",
		ProtocolVersion: ProtocolV5,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	if !client.IsRunning() {
		t.Error("client should be running")
	}
}

func TestClientConnectV5WithSessionExpiry(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	sessionExpiry := uint32(3600)
	client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "test-client-session",
		ProtocolVersion: ProtocolV5,
		SessionExpiry:   &sessionExpiry,
		CleanSession:    false,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()
}

func TestClientPubSubV4(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "pubsub-client",
		ProtocolVersion: ProtocolV4,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe
	if err := client.Subscribe(ctx, "test/topic"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish
	if err := client.Publish(ctx, "test/topic", []byte("hello")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Receive
	msg, err := client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}

	if msg.Topic != "test/topic" {
		t.Errorf("Topic = %q, want %q", msg.Topic, "test/topic")
	}
	if string(msg.Payload) != "hello" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "hello")
	}
}

func TestClientPubSubV5(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        "pubsub-client-v5",
		ProtocolVersion: ProtocolV5,
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe
	if err := client.Subscribe(ctx, "test/v5/topic"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish
	if err := client.Publish(ctx, "test/v5/topic", []byte("hello v5")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Receive
	msg, err := client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}

	if msg.Topic != "test/v5/topic" {
		t.Errorf("Topic = %q, want %q", msg.Topic, "test/v5/topic")
	}
	if string(msg.Payload) != "hello v5" {
		t.Errorf("Payload = %q, want %q", msg.Payload, "hello v5")
	}
}

func TestClientAuth(t *testing.T) {
	auth := &testAuthenticator{
		validUser: "admin",
		validPass: []byte("secret"),
	}
	addr, cleanup := startTestBroker(t, auth)
	defer cleanup()

	ctx := context.Background()

	// Connect with correct credentials
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "auth-client",
		Username: "admin",
		Password: []byte("secret"),
	})
	if err != nil {
		t.Fatalf("connect with correct credentials failed: %v", err)
	}
	client.Close()

	// Connect with wrong credentials
	_, err = Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "auth-client-2",
		Username: "admin",
		Password: []byte("wrong"),
	})
	if err == nil {
		t.Fatal("expected error for wrong credentials")
	}
}

func TestClientACL(t *testing.T) {
	auth := &testACLAuthenticator{
		allowedTopics: []string{"public/"},
	}
	addr, cleanup := startTestBroker(t, auth)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "acl-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe to allowed topic
	if err := client.Subscribe(ctx, "public/news"); err != nil {
		t.Errorf("subscribe to allowed topic failed: %v", err)
	}

	// Subscribe to forbidden topic should fail
	err = client.Subscribe(ctx, "private/data")
	if err != ErrACLDenied {
		t.Errorf("expected ErrACLDenied, got %v", err)
	}
}

func TestClientWildcardSubscription(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "wildcard-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe with + wildcard
	if err := client.Subscribe(ctx, "sensor/+/temp"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish to matching topic
	if err := client.Publish(ctx, "sensor/room1/temp", []byte("25")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Should receive the message
	msg, err := client.RecvTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message")
	}
	if msg.Topic != "sensor/room1/temp" {
		t.Errorf("Topic = %q, want %q", msg.Topic, "sensor/room1/temp")
	}
}

func TestClientUnsubscribe(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "unsub-client",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Subscribe
	if err := client.Subscribe(ctx, "test/unsub"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Unsubscribe
	if err := client.Unsubscribe(ctx, "test/unsub"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}
}

func TestClientPing(t *testing.T) {
	addr, cleanup := startTestBroker(t, nil)
	defer cleanup()

	ctx := context.Background()
	client, err := Connect(ctx, ClientConfig{
		Addr:          "tcp://" + addr,
		ClientID:      "ping-client",
		AutoKeepalive: false, // Disable auto keepalive
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Manual ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

// Test helpers

type testAuthenticator struct {
	validUser string
	validPass []byte
}

func (a *testAuthenticator) Authenticate(clientID, username string, password []byte) bool {
	return username == a.validUser && string(password) == string(a.validPass)
}

func (a *testAuthenticator) ACL(clientID, topic string, write bool) bool {
	return true
}

type testACLAuthenticator struct {
	allowedTopics []string
}

func (a *testACLAuthenticator) Authenticate(clientID, username string, password []byte) bool {
	return true
}

func (a *testACLAuthenticator) ACL(clientID, topic string, write bool) bool {
	for _, allowed := range a.allowedTopics {
		if len(topic) >= len(allowed) && topic[:len(allowed)] == allowed {
			return true
		}
	}
	return false
}
