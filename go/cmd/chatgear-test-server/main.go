// Chatgear Integration Test Server
//
// A simple MQTT server that collects and verifies chatgear protocol messages.
//
// Usage:
//   go run . [options]
//
// Options:
//   -port=1883         MQTT broker port
//   -scope=test/       Topic scope prefix
//   -gear-id=zig-test  Expected gear ID

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

// ReceivedMessage represents a message received from the client
type ReceivedMessage struct {
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"-"`
	PayloadB64 string   `json:"payload_b64,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// StateEvent matches the chatgear state event format
type StateEvent struct {
	Version  int    `json:"version"`
	Time     int64  `json:"time"`
	State    string `json:"state"`
	UpdateAt int64  `json:"update_at"`
}

// TestServer is the chatgear test server
type TestServer struct {
	broker *mqtt0.Broker
	
	scope   string
	gearID  string
	
	mu              sync.Mutex
	stateMessages   []StateEvent
	statsMessages   []json.RawMessage
	audioFrames     [][]byte
	clientConnected bool
	
	// For sending commands
	commandTopic string
	audioTopic   string
}

// NewTestServer creates a new test server
func NewTestServer(scope, gearID string) *TestServer {
	ts := &TestServer{
		scope:  scope,
		gearID: gearID,
		commandTopic: fmt.Sprintf("%sdevice/%s/command", scope, gearID),
		audioTopic:   fmt.Sprintf("%sdevice/%s/output_audio_stream", scope, gearID),
	}
	
	ts.broker = &mqtt0.Broker{
		Handler: ts,
		OnConnect: func(clientID string) {
			log.Printf("[SERVER] Client connected: %s", clientID)
			ts.mu.Lock()
			ts.clientConnected = true
			ts.mu.Unlock()
		},
		OnDisconnect: func(clientID string) {
			log.Printf("[SERVER] Client disconnected: %s", clientID)
			ts.mu.Lock()
			ts.clientConnected = false
			ts.mu.Unlock()
		},
	}
	
	return ts
}

// HandleMessage implements mqtt0.Handler
func (ts *TestServer) HandleMessage(clientID string, msg *mqtt0.Message) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	log.Printf("[SERVER] Received on %s: %d bytes", msg.Topic, len(msg.Payload))
	
	// Parse topic to determine message type
	expectedStateTop := fmt.Sprintf("%sdevice/%s/state", ts.scope, ts.gearID)
	expectedStatsTop := fmt.Sprintf("%sdevice/%s/stats", ts.scope, ts.gearID)
	expectedAudioTop := fmt.Sprintf("%sdevice/%s/input_audio_stream", ts.scope, ts.gearID)
	
	switch msg.Topic {
	case expectedStateTop:
		var evt StateEvent
		if err := json.Unmarshal(msg.Payload, &evt); err != nil {
			log.Printf("[SERVER] Failed to parse state: %v", err)
			return
		}
		ts.stateMessages = append(ts.stateMessages, evt)
		log.Printf("[SERVER] State received: %s (version=%d, time=%d)", evt.State, evt.Version, evt.Time)
		
	case expectedStatsTop:
		ts.statsMessages = append(ts.statsMessages, json.RawMessage(msg.Payload))
		log.Printf("[SERVER] Stats received: %s", string(msg.Payload))
		
	case expectedAudioTop:
		ts.audioFrames = append(ts.audioFrames, msg.Payload)
		log.Printf("[SERVER] Audio frame received: %d bytes", len(msg.Payload))
		
	default:
		log.Printf("[SERVER] Unknown topic: %s", msg.Topic)
	}
}

// Serve starts the server
func (ts *TestServer) Serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("[SERVER] Listening on %s", addr)
	log.Printf("[SERVER] Scope: %q, GearID: %q", ts.scope, ts.gearID)
	log.Printf("[SERVER] Expected topics:")
	log.Printf("[SERVER]   State: %sdevice/%s/state", ts.scope, ts.gearID)
	log.Printf("[SERVER]   Stats: %sdevice/%s/stats", ts.scope, ts.gearID)
	log.Printf("[SERVER]   Audio: %sdevice/%s/input_audio_stream", ts.scope, ts.gearID)
	return ts.broker.Serve(ln)
}

// SendCommand sends a command to the connected client
func (ts *TestServer) SendCommand(cmdType string, payload interface{}) error {
	cmd := map[string]interface{}{
		"cmd_type": cmdType,
		"time":     time.Now().UnixMilli(),
		"payload":  payload,
		"issue_at": time.Now().UnixMilli(),
	}
	
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	
	log.Printf("[SERVER] Sending command: %s", string(data))
	return ts.broker.Publish(nil, ts.commandTopic, data)
}

// SendAudioFrame sends an audio frame to the client
func (ts *TestServer) SendAudioFrame(timestamp int64, frame []byte) error {
	// Stamp frame: 1 byte version + 7 bytes timestamp + data
	stamped := make([]byte, 8+len(frame))
	stamped[0] = 1 // Version
	
	// 7 bytes big-endian timestamp
	stamped[1] = byte(timestamp >> 48)
	stamped[2] = byte(timestamp >> 40)
	stamped[3] = byte(timestamp >> 32)
	stamped[4] = byte(timestamp >> 24)
	stamped[5] = byte(timestamp >> 16)
	stamped[6] = byte(timestamp >> 8)
	stamped[7] = byte(timestamp)
	
	copy(stamped[8:], frame)
	
	return ts.broker.Publish(nil, ts.audioTopic, stamped)
}

// GetStats returns collected statistics
func (ts *TestServer) GetStats() (stateCount, statsCount, audioCount int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.stateMessages), len(ts.statsMessages), len(ts.audioFrames)
}

// GetStateMessages returns all received state messages
func (ts *TestServer) GetStateMessages() []StateEvent {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	result := make([]StateEvent, len(ts.stateMessages))
	copy(result, ts.stateMessages)
	return result
}

// PrintSummary prints a summary of received messages
func (ts *TestServer) PrintSummary() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	fmt.Println("\n=== Test Summary ===")
	fmt.Printf("State messages: %d\n", len(ts.stateMessages))
	for i, s := range ts.stateMessages {
		fmt.Printf("  [%d] state=%s time=%d\n", i, s.State, s.Time)
	}
	
	fmt.Printf("Stats messages: %d\n", len(ts.statsMessages))
	for i, s := range ts.statsMessages {
		fmt.Printf("  [%d] %s\n", i, string(s))
	}
	
	fmt.Printf("Audio frames: %d\n", len(ts.audioFrames))
	if len(ts.audioFrames) > 0 {
		totalBytes := 0
		for _, f := range ts.audioFrames {
			totalBytes += len(f)
		}
		fmt.Printf("  Total bytes: %d\n", totalBytes)
	}
}

func main() {
	port := flag.Int("port", 1883, "MQTT broker port")
	scope := flag.String("scope", "test/", "Topic scope prefix")
	gearID := flag.String("gear-id", "zig-test", "Expected gear ID")
	flag.Parse()
	
	server := NewTestServer(*scope, *gearID)
	
	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.PrintSummary()
		os.Exit(0)
	}()
	
	addr := fmt.Sprintf(":%d", *port)
	if err := server.Serve(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
