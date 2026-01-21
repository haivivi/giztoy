package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/haivivi/giztoy/pkg/mqtt"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func TestMultiClientCommunication(t *testing.T) {
	addr, err := findAvailablePort()
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}

	tracker := NewMessageTracker()

	// Create broker handler
	brokerMux := mqtt.NewServeMux()
	brokerMux.HandleFunc("test/#", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		tracker.RecordBrokerMessage(msg.Packet.Topic, payload)
		return nil
	})

	// Start broker
	srv := &mqtt.Server{
		Handler: brokerMux,
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mqttAddr := fmt.Sprintf("tcp://%s", addr)

	// Create client 1
	mux1 := mqtt.NewServeMux()
	mux1.HandleFunc("test/+", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		tracker.RecordClientMessage("client1", msg.Packet.Topic, payload)
		return nil
	})

	conn1, err := (&mqtt.Dialer{ID: "client1", ServeMux: mux1}).Dial(ctx, mqttAddr)
	if err != nil {
		t.Fatalf("Client1 failed to connect: %v", err)
	}
	defer conn1.Close()

	if err := conn1.Subscribe(ctx, "test/+"); err != nil {
		t.Fatalf("Client1 failed to subscribe: %v", err)
	}

	// Create client 2
	mux2 := mqtt.NewServeMux()
	mux2.HandleFunc("test/+", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		tracker.RecordClientMessage("client2", msg.Packet.Topic, payload)
		return nil
	})

	conn2, err := (&mqtt.Dialer{ID: "client2", ServeMux: mux2}).Dial(ctx, mqttAddr)
	if err != nil {
		t.Fatalf("Client2 failed to connect: %v", err)
	}
	defer conn2.Close()

	if err := conn2.Subscribe(ctx, "test/+"); err != nil {
		t.Fatalf("Client2 failed to subscribe: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Client 1 sends message
	if err := conn1.WriteToTopic(ctx, []byte("hello from client1"), "test/topic1"); err != nil {
		t.Fatalf("Client1 failed to publish: %v", err)
	}

	// Client 2 sends message
	if err := conn2.WriteToTopic(ctx, []byte("hello from client2"), "test/topic2"); err != nil {
		t.Fatalf("Client2 failed to publish: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify both clients received both messages
	msgs1 := tracker.GetClientMessages("client1")
	msgs2 := tracker.GetClientMessages("client2")

	if len(msgs1) < 2 {
		t.Errorf("Client1 should receive 2 messages, got %d: %v", len(msgs1), msgs1)
	}
	if len(msgs2) < 2 {
		t.Errorf("Client2 should receive 2 messages, got %d: %v", len(msgs2), msgs2)
	}

	// Verify broker received all messages
	brokerMsgs1 := tracker.GetBrokerMessages("test/topic1")
	brokerMsgs2 := tracker.GetBrokerMessages("test/topic2")

	if len(brokerMsgs1) < 1 {
		t.Errorf("Broker should receive message on test/topic1, got: %v", brokerMsgs1)
	}
	if len(brokerMsgs2) < 1 {
		t.Errorf("Broker should receive message on test/topic2, got: %v", brokerMsgs2)
	}

	t.Logf("Test passed! Client1 msgs: %d, Client2 msgs: %d", len(msgs1), len(msgs2))
}

// MessageTracker for tests (duplicated for test independence)
type testMessageTracker struct {
	mu             sync.RWMutex
	clientMessages map[string][]testMessageRecord
	brokerMessages map[string][]string
	totalCount     atomic.Int32
}

type testMessageRecord struct {
	Topic   string
	Payload string
}

func newTestMessageTracker() *testMessageTracker {
	return &testMessageTracker{
		clientMessages: make(map[string][]testMessageRecord),
		brokerMessages: make(map[string][]string),
	}
}

func TestBroadcastToMultipleClients(t *testing.T) {
	addr, err := findAvailablePort()
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}

	srv := &mqtt.Server{
		Handler: mqtt.NewServeMux(),
	}
	defer srv.Close()

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: addr,
	})

	go srv.Serve(tcp)
	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mqttAddr := fmt.Sprintf("tcp://%s", addr)

	// Create multiple clients
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
		conn, err := dialer.Dial(ctx, mqttAddr)
		if err != nil {
			t.Fatalf("Client %d failed to connect: %v", i, err)
		}
		clients[i] = conn

		if err := conn.Subscribe(ctx, "broadcast/topic"); err != nil {
			t.Fatalf("Client %d failed to subscribe: %v", i, err)
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
		t.Fatalf("Failed to broadcast: %v", err)
	}

	// All clients should receive the message
	for i, ch := range received {
		select {
		case msg := <-ch:
			if string(msg) != string(payload) {
				t.Errorf("Client %d: expected %q, got %q", i, payload, msg)
			}
			t.Logf("Client %d received: %s", i, msg)
		case <-time.After(2 * time.Second):
			t.Errorf("Client %d: timeout waiting for message", i)
		}
	}
}
