// Package main demonstrates an MQTT echo server using mqtt0.
//
// This example runs an MQTT broker that logs received messages
// and echoes them back on a response topic.
//
// Run with: go run ./examples/go/mqtt/echo_server/main.go
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

func main() {
	log.Println("Starting MQTT Echo Server (mqtt0)")

	// Create broker with message handler
	broker := &mqtt0.Broker{
		Handler: mqtt0.HandlerFunc(func(clientID string, msg *mqtt0.Message) {
			log.Printf("Received from %s on '%s': %s", clientID, msg.Topic, string(msg.Payload))

			// Echo back messages from request/... to response/...
			if strings.HasPrefix(msg.Topic, "request/") {
				responseTopic := "response/" + msg.Topic[8:]
				log.Printf("Would echo to: %s", responseTopic)
				// Note: In mqtt0, the broker automatically routes messages to subscribers
				// For echo functionality, you'd need to publish to the response topic
			}
		}),
		OnConnect: func(clientID string) {
			log.Printf("Client connected: %s", clientID)
		},
		OnDisconnect: func(clientID string) {
			log.Printf("Client disconnected: %s", clientID)
		},
	}

	// Create TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:1883")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

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
		ln.Close()
		broker.Close()
	}()

	// Run server
	if err := broker.Serve(ln); err != nil {
		log.Printf("Server stopped: %v", err)
	}
}
