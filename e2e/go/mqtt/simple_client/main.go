// Package main demonstrates a simple MQTT client using mqtt0.
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
// You can start one with: go run ./examples/go/mqtt/echo_server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

func main() {
	log.Println("Starting Simple MQTT Client (mqtt0)")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to broker
	log.Println("Connecting to tcp://127.0.0.1:1883...")
	client, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://127.0.0.1:1883",
		ClientID: "simple-client",
	})
	if err != nil {
		log.Printf("Failed to connect: %v", err)
		log.Println("Make sure an MQTT broker is running on localhost:1883")
		log.Println("You can start one with: go run ./examples/go/mqtt/echo_server/main.go")
		os.Exit(1)
	}
	defer client.Close()

	log.Println("Connected!")

	// Subscribe to topics
	if err := client.Subscribe(ctx, "response/#"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	if err := client.Subscribe(ctx, "broadcast/#"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	log.Println("Subscribed to response/# and broadcast/#")

	// Start message receiver goroutine
	go func() {
		for {
			msg, err := client.RecvTimeout(5 * time.Second)
			if err != nil {
				if err.Error() != "timeout" {
					log.Printf("Receive error: %v", err)
				}
				return
			}
			if msg != nil {
				log.Printf("Received on '%s': %s", msg.Topic, string(msg.Payload))
			}
		}
	}()

	// Publish some messages
	for i := 1; i <= 5; i++ {
		topic := fmt.Sprintf("request/test/%d", i)
		payload := fmt.Sprintf("Message %d", i)

		if err := client.Publish(ctx, topic, []byte(payload)); err != nil {
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
	client.Close()
	log.Println("Disconnected")
}
