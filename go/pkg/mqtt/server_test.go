package mqtt_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/mqtt"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func findAvailablePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestServer_Serve(t *testing.T) {
	addr := findAvailablePort(t)

	mux := mqtt.NewServeMux()
	srv := &mqtt.Server{
		Handler: mux,
	}
	defer srv.Close()

	// Create TCP listener
	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(tcp)
	}()
	_ = errCh // used for error handling if needed

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect a client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://%s", addr))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Close server
	if err := srv.Close(); err != nil {
		t.Errorf("close error: %v", err)
	}
}

func TestServer_ServeMultipleListeners(t *testing.T) {
	addr1 := findAvailablePort(t)
	addr2 := findAvailablePort(t)

	mux := mqtt.NewServeMux()
	srv := &mqtt.Server{
		Handler: mux,
	}
	defer srv.Close()

	// Create multiple listeners
	tcp1 := listeners.NewTCP(listeners.Config{
		ID:      "tcp1",
		Address: addr1,
	})
	tcp2 := listeners.NewTCP(listeners.Config{
		ID:      "tcp2",
		Address: addr2,
	})

	// Start server with multiple listeners
	go func() {
		srv.Serve(tcp1, tcp2)
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to first listener
	conn1, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://%s", addr1))
	if err != nil {
		t.Fatalf("failed to connect to addr1: %v", err)
	}
	defer conn1.Close()

	// Connect to second listener
	conn2, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://%s", addr2))
	if err != nil {
		t.Fatalf("failed to connect to addr2: %v", err)
	}
	defer conn2.Close()
}

func TestServer_ServeAlreadyRunning(t *testing.T) {
	addr := findAvailablePort(t)

	srv := &mqtt.Server{
		Handler: mqtt.NewServeMux(),
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	// Second Serve call should return ErrServerRunning
	tcp2 := listeners.NewTCP(listeners.Config{
		ID:      "tcp2",
		Address: findAvailablePort(t),
	})

	err := srv.Serve(tcp2)
	if err != mqtt.ErrServerRunning {
		t.Errorf("expected ErrServerRunning, got %v", err)
	}
}

func TestServer_OnConnect_OnDisconnect(t *testing.T) {
	addr := findAvailablePort(t)

	var connected atomic.Int32
	var disconnected atomic.Int32

	srv := &mqtt.Server{
		Handler: mqtt.NewServeMux(),
		OnConnect: func(clientID string) {
			connected.Add(1)
			t.Logf("client connected: %s", clientID)
		},
		OnDisconnect: func(clientID string) {
			disconnected.Add(1)
			t.Logf("client disconnected: %s", clientID)
		},
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	// Connect client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://%s", addr))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if connected.Load() != 1 {
		t.Errorf("expected 1 connection, got %d", connected.Load())
	}

	// Disconnect
	conn.Close()
	time.Sleep(100 * time.Millisecond)

	if disconnected.Load() != 1 {
		t.Errorf("expected 1 disconnection, got %d", disconnected.Load())
	}
}

func TestServer_WriteToTopic(t *testing.T) {
	addr := findAvailablePort(t)

	mux := mqtt.NewServeMux()
	received := make(chan []byte, 1)

	mux.HandleFunc("test/topic", func(msg mqtt.Message) error {
		received <- msg.Packet.Payload
		return nil
	})

	srv := &mqtt.Server{
		Handler: mux,
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect client and subscribe
	clientMux := mqtt.NewServeMux()
	clientReceived := make(chan []byte, 1)
	clientMux.HandleFunc("response/#", func(msg mqtt.Message) error {
		clientReceived <- msg.Packet.Payload
		return nil
	})

	dialer := &mqtt.Dialer{ServeMux: clientMux}
	conn, err := dialer.Dial(ctx, fmt.Sprintf("tcp://%s", addr))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Subscribe to response topic
	if err := conn.Subscribe(ctx, "response/#"); err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Client publishes to server
	if err := conn.WriteToTopic(ctx, []byte("hello server"), "test/topic"); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// Wait for server to receive
	select {
	case payload := <-received:
		if string(payload) != "hello server" {
			t.Errorf("expected 'hello server', got '%s'", payload)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message")
	}

	// Server publishes to client
	if err := srv.WriteToTopic(ctx, []byte("hello client"), "response/test"); err != nil {
		t.Fatalf("failed to publish from server: %v", err)
	}

	// Wait for client to receive
	select {
	case payload := <-clientReceived:
		if string(payload) != "hello client" {
			t.Errorf("expected 'hello client', got '%s'", payload)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for server message")
	}
}

func TestServer_Authenticator(t *testing.T) {
	addr := findAvailablePort(t)

	auth := &testAuthenticator{
		validUsers: map[string]string{
			"user1": "pass1",
		},
	}

	srv := &mqtt.Server{
		Handler:       mqtt.NewServeMux(),
		Authenticator: auth,
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Valid credentials should succeed
	conn, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://user1:pass1@%s", addr))
	if err != nil {
		t.Fatalf("valid credentials should connect: %v", err)
	}
	conn.Close()

	// Invalid credentials should fail
	_, err = mqtt.Dial(ctx, fmt.Sprintf("tcp://user1:wrong@%s", addr))
	if err == nil {
		t.Error("invalid credentials should fail")
	}
}

type testAuthenticator struct {
	validUsers map[string]string
}

func (a *testAuthenticator) Authenticate(clientID, username string, password []byte) bool {
	if expected, ok := a.validUsers[username]; ok {
		return expected == string(password)
	}
	return false
}

func (a *testAuthenticator) ACL(clientID, topic string, write bool) bool {
	return true // Allow all for this test
}

func TestServer_MultipleClients(t *testing.T) {
	addr := findAvailablePort(t)

	var connCount atomic.Int32

	srv := &mqtt.Server{
		Handler: mqtt.NewServeMux(),
		OnConnect: func(clientID string) {
			connCount.Add(1)
		},
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect multiple clients
	numClients := 10
	var wg sync.WaitGroup
	conns := make([]*mqtt.Conn, numClients)
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := mqtt.Dial(ctx, fmt.Sprintf("tcp://%s", addr))
			if err != nil {
				errors <- fmt.Errorf("client %d failed to connect: %v", idx, err)
				return
			}
			conns[idx] = conn
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	time.Sleep(200 * time.Millisecond)

	if int(connCount.Load()) != numClients {
		t.Errorf("expected %d connections, got %d", numClients, connCount.Load())
	}

	// Cleanup
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestServer_Broadcast(t *testing.T) {
	addr := findAvailablePort(t)

	srv := &mqtt.Server{
		Handler: mqtt.NewServeMux(),
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect multiple clients and subscribe to the same topic
	numClients := 5
	clients := make([]*mqtt.Conn, numClients)
	received := make([]chan []byte, numClients)

	for i := 0; i < numClients; i++ {
		mux := mqtt.NewServeMux()
		ch := make(chan []byte, 1)
		received[i] = ch

		mux.HandleFunc("broadcast/topic", func(msg mqtt.Message) error {
			ch <- msg.Packet.Payload
			return nil
		})

		dialer := &mqtt.Dialer{ServeMux: mux}
		conn, err := dialer.Dial(ctx, fmt.Sprintf("tcp://%s", addr))
		if err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
		clients[i] = conn

		if err := conn.Subscribe(ctx, "broadcast/topic"); err != nil {
			t.Fatalf("client %d failed to subscribe: %v", i, err)
		}
	}
	defer func() {
		for _, c := range clients {
			if c != nil {
				c.Close()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Server broadcasts a message
	payload := []byte("hello all clients")
	if err := srv.WriteToTopic(ctx, payload, "broadcast/topic"); err != nil {
		t.Fatalf("failed to broadcast: %v", err)
	}

	// All clients should receive the message
	for i, ch := range received {
		select {
		case msg := <-ch:
			if string(msg) != string(payload) {
				t.Errorf("client %d: expected %q, got %q", i, payload, msg)
			}
			t.Logf("client %d received: %s", i, msg)
		case <-time.After(2 * time.Second):
			t.Errorf("client %d: timeout waiting for message", i)
		}
	}
}
