// Package main demonstrates a simple MQTT client.
//
// This example shows basic MQTT client operations:
// - Connecting to a broker
// - Subscribing to topics
// - Publishing messages
// - Handling received messages
//
// Run with: go run ./examples/go/mqtt/simple_client/main.go
//
// Note: Requires an MQTT broker running on localhost:1883
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/pkg/mqtt"
)

func main() {
	log.Println("Starting Simple MQTT Client")

	// Create message handler
	mux := mqtt.NewServeMux()

	// Handle messages on subscribed topics
	mux.HandleFunc("response/#", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		log.Printf("Received on '%s': %s", msg.Packet.Topic, payload)
		return nil
	})

	mux.HandleFunc("broadcast/#", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		log.Printf("[BROADCAST] %s: %s", msg.Packet.Topic, payload)
		return nil
	})

	// Create dialer
	dialer := &mqtt.Dialer{
		ID:        "simple-client",
		KeepAlive: 30,
		ServeMux:  mux,
		OnConnectionUp: func() {
			log.Println("Connected to broker!")
		},
		OnConnectError: func(err error) {
			log.Printf("Connection error: %v", err)
		},
	}

	log.Println("Connecting to mqtt://127.0.0.1:1883...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, "tcp://127.0.0.1:1883")
	if err != nil {
		log.Printf("Failed to connect: %v", err)
		log.Println("Make sure an MQTT broker is running on localhost:1883")
		log.Println("You can start one with: go run ./examples/go/mqtt/echo_server/main.go")
		os.Exit(1)
	}
	defer conn.Close()

	log.Println("Connected!")

	// Subscribe to topics
	if err := conn.Subscribe(ctx, "response/#"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	if err := conn.Subscribe(ctx, "broadcast/#"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	log.Println("Subscribed to response/# and broadcast/#")

	// Publish some messages
	for i := 1; i <= 5; i++ {
		topic := fmt.Sprintf("request/test/%d", i)
		payload := fmt.Sprintf("Message %d", i)

		if err := conn.WriteToTopic(ctx, []byte(payload), topic); err != nil {
			log.Printf("Failed to publish: %v", err)
			continue
		}
		log.Printf("Published to '%s': %s", topic, payload)

		time.Sleep(500 * time.Millisecond)
	}

	// Keep running to receive messages
	log.Println("Waiting for messages... Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Disconnecting...")
	conn.Close()
	log.Println("Disconnected")
}
