// Package main demonstrates multi-client MQTT communication using mqtt0.
//
// This example demonstrates:
// - Starting an embedded MQTT broker
// - Multiple clients connecting to the broker
// - Clients subscribing and publishing messages
// - Verifying all clients receive the expected messages
//
// Run with: go run ./examples/go/mqtt/multi_client/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

// MessageTracker tracks received messages for verification.
type MessageTracker struct {
	mu             sync.RWMutex
	clientMessages map[string][]MessageRecord
	brokerMessages map[string][]string
	totalCount     atomic.Int32
}

type MessageRecord struct {
	Topic   string
	Payload string
}

func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		clientMessages: make(map[string][]MessageRecord),
		brokerMessages: make(map[string][]string),
	}
}

func (t *MessageTracker) RecordClientMessage(clientID, topic, payload string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clientMessages[clientID] = append(t.clientMessages[clientID], MessageRecord{
		Topic:   topic,
		Payload: payload,
	})
	t.totalCount.Add(1)
}

func (t *MessageTracker) RecordBrokerMessage(topic, payload string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.brokerMessages[topic] = append(t.brokerMessages[topic], payload)
}

func (t *MessageTracker) GetClientMessages(clientID string) []MessageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]MessageRecord{}, t.clientMessages[clientID]...)
}

func (t *MessageTracker) GetBrokerMessages(topic string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]string{}, t.brokerMessages[topic]...)
}

func (t *MessageTracker) Total() int32 {
	return t.totalCount.Load()
}

func findAvailablePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := l.Addr().String()
	l.Close()
	return addr, nil
}

func main() {
	log.Println("Starting multi-client MQTT test (mqtt0)")

	addr, err := findAvailablePort()
	if err != nil {
		log.Fatalf("Failed to find available port: %v", err)
	}
	log.Printf("Using address: %s", addr)

	// Create message tracker
	tracker := NewMessageTracker()

	// Start broker with handler
	broker := &mqtt0.Broker{
		Handler: mqtt0.HandlerFunc(func(clientID string, msg *mqtt0.Message) {
			tracker.RecordBrokerMessage(msg.Topic, string(msg.Payload))
		}),
	}
	defer broker.Close()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		if err := broker.Serve(ln); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	log.Printf("Broker started on %s", addr)

	// Create clients
	numClients := 3
	clients := make([]*mqtt0.Client, numClients)
	clientIDs := make([]string, numClients)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < numClients; i++ {
		clientID := fmt.Sprintf("client-%d", i)
		clientIDs[i] = clientID

		client, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
			Addr:     "tcp://" + addr,
			ClientID: clientID,
		})
		if err != nil {
			log.Fatalf("Client %s failed to connect: %v", clientID, err)
		}

		// Subscribe to chat topic
		if err := client.Subscribe(ctx, "chat/#"); err != nil {
			log.Fatalf("Client %s failed to subscribe: %v", clientID, err)
		}

		log.Printf("Client %s connected and subscribed", clientID)
		clients[i] = client

		// Start message receiver goroutine for this client
		go func(cid string, c *mqtt0.Client) {
			for {
				msg, err := c.RecvTimeout(5 * time.Second)
				if err != nil {
					return
				}
				if msg == nil {
					continue
				}
				tracker.RecordClientMessage(cid, msg.Topic, string(msg.Payload))
			}
		}(clientID, client)
	}

	// Wait for subscriptions to complete
	time.Sleep(300 * time.Millisecond)

	// Each client publishes a message
	for i, client := range clients {
		topic := "chat/room1"
		payload := fmt.Sprintf("Hello from %s", clientIDs[i])

		if err := client.Publish(ctx, topic, []byte(payload)); err != nil {
			log.Fatalf("Client %s failed to publish: %v", clientIDs[i], err)
		}
		log.Printf("Client %s published: %s", clientIDs[i], payload)

		// Small delay between publishes
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for messages to propagate
	time.Sleep(time.Second)

	// Verify results
	log.Println("\n=== Verification ===")

	// Each client should have received messages from all clients
	expectedMessages := numClients

	for _, clientID := range clientIDs {
		messages := tracker.GetClientMessages(clientID)
		log.Printf("Client %s received %d messages: %v", clientID, len(messages), messages)

		if len(messages) >= expectedMessages {
			log.Printf("✓ %s received expected %d messages", clientID, expectedMessages)
		} else {
			log.Printf("✗ %s received %d messages, expected %d", clientID, len(messages), expectedMessages)
		}
	}

	// Check broker received all messages
	brokerMsgs := tracker.GetBrokerMessages("chat/room1")
	log.Printf("\nBroker received %d messages on chat/room1: %v", len(brokerMsgs), brokerMsgs)

	if len(brokerMsgs) >= numClients {
		log.Printf("✓ Broker received all %d messages", numClients)
	} else {
		log.Printf("✗ Broker received %d messages, expected %d", len(brokerMsgs), numClients)
	}

	// Total messages tracked
	log.Printf("\nTotal client messages tracked: %d", tracker.Total())

	// Cleanup
	log.Println("\n=== Cleanup ===")
	for i, client := range clients {
		client.Close()
		log.Printf("Client %s disconnected", clientIDs[i])
	}

	broker.Close()
	log.Println("Broker stopped")

	log.Println("\n=== Test Complete ===")
}
