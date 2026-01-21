// Package main demonstrates an MQTT echo server.
//
// This example runs an MQTT broker that logs received messages
// and can publish responses back to clients.
//
// Run with: go run ./examples/go/mqtt/echo_server/main.go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/haivivi/giztoy/pkg/mqtt"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	log.Println("Starting MQTT Echo Server")

	// Create handler that logs messages
	mux := mqtt.NewServeMux()

	mux.HandleFunc("request/#", func(msg mqtt.Message) error {
		payload := string(msg.Packet.Payload)
		log.Printf("Received on '%s': %s", msg.Packet.Topic, payload)

		// In a real application, you'd use the server's WriteToTopic
		// to echo back. For this demo, we just log the message.
		responseTopic := "response/" + msg.Packet.Topic[8:] // Strip "request/"
		log.Printf("Would echo to: %s", responseTopic)

		return nil
	})

	// Create server with callbacks
	srv := &mqtt.Server{
		Handler: mux,
		OnConnect: func(clientID string) {
			log.Printf("Client connected: %s", clientID)
		},
		OnDisconnect: func(clientID string) {
			log.Printf("Client disconnected: %s", clientID)
		},
	}

	// Create TCP listener
	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: "127.0.0.1:1883",
	})

	log.Println("Echo server listening on 127.0.0.1:1883")
	log.Println("Subscribe to 'response/#' to receive echoed messages")
	log.Println("Publish to 'request/...' to send messages")
	log.Println("Press Ctrl+C to stop")

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		srv.Close()
	}()

	// Run server
	if err := srv.Serve(tcp); err != nil {
		log.Printf("Server stopped: %v", err)
	}
}
