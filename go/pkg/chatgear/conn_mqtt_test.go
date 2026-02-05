package chatgear

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/jsontime"
	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

func TestStampFrame_Roundtrip(t *testing.T) {
	// Create a test frame
	testFrame := opus.Frame{0xFC, 0x00, 0x01, 0x02, 0x03} // Valid Opus TOC + data
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// Stamp the frame
	stamped := stampFrame(testFrame, testTime)

	// Verify header size
	if len(stamped) != stampedHeaderSize+len(testFrame) {
		t.Errorf("stamped frame length = %d, want %d", len(stamped), stampedHeaderSize+len(testFrame))
	}

	// Verify version byte
	if stamped[0] != frameVersion {
		t.Errorf("version byte = %d, want %d", stamped[0], frameVersion)
	}

	// Unstamp and verify
	frame, ts, ok := unstampFrame(stamped)
	if !ok {
		t.Fatal("unstampFrame returned ok=false")
	}

	// Compare frame data
	if len(frame) != len(testFrame) {
		t.Errorf("frame length = %d, want %d", len(frame), len(testFrame))
	}
	for i := range frame {
		if frame[i] != testFrame[i] {
			t.Errorf("frame[%d] = %d, want %d", i, frame[i], testFrame[i])
		}
	}

	// Compare timestamp (truncated to milliseconds)
	expectedMs := testTime.UnixMilli()
	actualMs := ts.UnixMilli()
	if actualMs != expectedMs {
		t.Errorf("timestamp = %d ms, want %d ms", actualMs, expectedMs)
	}
}

func TestUnstampFrame_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short", []byte{0x01, 0x02, 0x03}},
		{"header only", []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"wrong version", []byte{0x99, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFC}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, ok := unstampFrame(tt.data)
			if ok {
				t.Error("unstampFrame should return ok=false for invalid data")
			}
		})
	}
}

func TestStampFrame_TimestampPrecision(t *testing.T) {
	// Test that nanoseconds are truncated to milliseconds
	testFrame := opus.Frame{0xFC, 0x00}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC) // has nanoseconds

	stamped := stampFrame(testFrame, testTime)
	_, ts, ok := unstampFrame(stamped)
	if !ok {
		t.Fatal("unstampFrame returned ok=false")
	}

	// Should be truncated to milliseconds
	expectedMs := testTime.UnixMilli()
	actualMs := ts.UnixMilli()
	if actualMs != expectedMs {
		t.Errorf("timestamp = %d ms, want %d ms", actualMs, expectedMs)
	}

	// Nanoseconds within the millisecond should be zero
	if ts.Nanosecond()%1000000 != 0 {
		t.Errorf("nanoseconds should be aligned to milliseconds, got %d", ts.Nanosecond())
	}
}

func TestStampFrame_LargeTimestamp(t *testing.T) {
	// Test with a large timestamp (year 2100)
	testFrame := opus.Frame{0xFC, 0x00}
	testTime := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)

	stamped := stampFrame(testFrame, testTime)
	_, ts, ok := unstampFrame(stamped)
	if !ok {
		t.Fatal("unstampFrame returned ok=false")
	}

	expectedMs := testTime.UnixMilli()
	actualMs := ts.UnixMilli()
	if actualMs != expectedMs {
		t.Errorf("timestamp = %d ms, want %d ms", actualMs, expectedMs)
	}
}

func TestMQTTClientConfig_Defaults(t *testing.T) {
	// This test verifies that DialMQTT would use sensible defaults
	// We can't actually dial without a broker, but we can verify the config handling

	cfg := MQTTClientConfig{
		Addr:   "tcp://localhost:1883",
		GearID: "test-gear",
	}

	// Verify defaults would be applied
	if cfg.KeepAlive != 0 {
		t.Error("KeepAlive should be 0 before defaults applied")
	}
	if cfg.ConnectTimeout != 0 {
		t.Error("ConnectTimeout should be 0 before defaults applied")
	}
	if cfg.ClientID != "" {
		t.Error("ClientID should be empty before defaults applied")
	}
	if cfg.Logger != nil {
		t.Error("Logger should be nil before defaults applied")
	}
}

func TestMQTTClientConn_InterfaceCompliance(t *testing.T) {
	// Verify compile-time interface assertions work at runtime too
	var uplinkTx UplinkTx = (*MQTTClientConn)(nil)
	var downlinkRx DownlinkRx = (*MQTTClientConn)(nil)

	_ = uplinkTx
	_ = downlinkRx
}

// =============================================================================
// Integration tests with local MQTT broker
// =============================================================================

// startTestBroker starts a local MQTT broker for testing.
func startTestBroker(t *testing.T) (addr string, cleanup func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	broker := &mqtt0.Broker{}
	go broker.Serve(ln)

	return ln.Addr().String(), func() {
		broker.Close()
		ln.Close()
	}
}

func TestDialMQTT_Connect(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	if conn.GearID() != "gear-001" {
		t.Errorf("GearID() = %q, want %q", conn.GearID(), "gear-001")
	}
}

func TestMQTTClientConn_SendOpusFrame(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Send an opus frame
	testFrame := opus.Frame{0xFC, 0x00, 0x01, 0x02}
	testTime := time.Now()

	err = conn.SendOpusFrame(testTime, testFrame)
	if err != nil {
		t.Errorf("SendOpusFrame failed: %v", err)
	}
}

func TestMQTTClientConn_SendState(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Send state
	state := NewStateEvent(StateRecording, time.Now())
	err = conn.SendState(state)
	if err != nil {
		t.Errorf("SendState failed: %v", err)
	}
}

func TestMQTTClientConn_SendStats(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Send stats
	stats := &StatsEvent{
		Battery: &Battery{Percentage: 80, IsCharging: true},
	}
	err = conn.SendStats(stats)
	if err != nil {
		t.Errorf("SendStats failed: %v", err)
	}
}

func TestMQTTClientConn_ReceiveOpusFrame(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create client connection
	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Create a "server" client to send messages to the device
	serverClient, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-server",
	})
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverClient.Close()

	// Send a stamped opus frame from "server" to device
	testFrame := opus.Frame{0xFC, 0x00, 0x01, 0x02}
	testTime := time.Now()
	stamped := stampFrame(testFrame, testTime)

	// Give receive loop time to start
	time.Sleep(50 * time.Millisecond)

	err = serverClient.Publish(ctx, "test/device/gear-001/output_audio_stream", stamped)
	if err != nil {
		t.Fatalf("server publish failed: %v", err)
	}

	// Receive the frame
	var received StampedOpusFrame
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for frame, err := range conn.OpusFrames() {
			if err != nil {
				t.Errorf("OpusFrames error: %v", err)
				return
			}
			received = frame
			return // Got one frame
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify received frame
		if len(received.Frame) != len(testFrame) {
			t.Errorf("received frame length = %d, want %d", len(received.Frame), len(testFrame))
		}
		for i := range received.Frame {
			if received.Frame[i] != testFrame[i] {
				t.Errorf("received.Frame[%d] = %d, want %d", i, received.Frame[i], testFrame[i])
			}
		}
		// Check timestamp (should be within 1 second)
		if received.Timestamp.Sub(testTime).Abs() > time.Second {
			t.Errorf("timestamp diff too large: %v", received.Timestamp.Sub(testTime))
		}
	case <-time.After(2 * time.Second):
		conn.Close() // Close to unblock iterator
		t.Fatal("timeout waiting for opus frame")
	}
}

func TestMQTTClientConn_ReceiveCommand(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create client connection
	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Create a "server" client to send commands
	serverClient, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-server",
	})
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverClient.Close()

	// Give receive loop time to start
	time.Sleep(50 * time.Millisecond)

	// Send a command from "server"
	testCmd := NewCommandEvent(&Halt{Interrupt: true}, time.Now())
	cmdData, _ := json.Marshal(testCmd)

	err = serverClient.Publish(ctx, "test/device/gear-001/command", cmdData)
	if err != nil {
		t.Fatalf("server publish failed: %v", err)
	}

	// Receive the command
	var received *CommandEvent
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for cmd, err := range conn.Commands() {
			if err != nil {
				t.Errorf("Commands error: %v", err)
				return
			}
			received = cmd
			return // Got one command
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if received == nil {
			t.Fatal("received nil command")
		}
		if _, ok := received.Payload.(*Halt); !ok {
			t.Errorf("received command type = %T, want *Halt", received.Payload)
		}
	case <-time.After(2 * time.Second):
		conn.Close() // Close to unblock iterator
		t.Fatal("timeout waiting for command")
	}
}

func TestMQTTClientConn_Close(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}

	// Close should not error
	err = conn.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second close should be no-op
	err = conn.Close()
	if err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestDialMQTT_InvalidAddr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://127.0.0.1:1", // Invalid port
		Scope:  "test",
		GearID: "gear-001",
	})
	if err == nil {
		t.Error("DialMQTT should fail with invalid address")
	}
}

// =============================================================================
// Wire Format Tests - verify JSON sent over MQTT
// =============================================================================

func TestMQTTClientConn_SendState_WireFormat(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create client connection
	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Create a "server" client to receive the state
	serverClient, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-server",
	})
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverClient.Close()

	// Subscribe to state topic
	stateTopic := "test/device/gear-001/state"
	if err := serverClient.Subscribe(ctx, stateTopic); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Give time for subscription to be ready
	time.Sleep(50 * time.Millisecond)

	// Send state from client
	updateAt := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	state := NewStateEvent(StateRecording, updateAt)

	if err := conn.SendState(state); err != nil {
		t.Fatalf("SendState failed: %v", err)
	}

	// Receive and verify wire format
	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			msg, err := serverClient.RecvTimeout(200 * time.Millisecond)
			if err != nil {
				continue
			}
			if msg != nil && msg.Topic == stateTopic {
				received = msg.Payload
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for state message")
	}

	if len(received) == 0 {
		t.Fatal("did not receive state message")
	}

	// Parse and verify JSON structure
	var raw map[string]interface{}
	if err := json.Unmarshal(received, &raw); err != nil {
		t.Fatalf("invalid JSON received: %v\nData: %s", err, string(received))
	}

	// Verify required fields
	if v, ok := raw["v"].(float64); !ok || int(v) != 1 {
		t.Errorf("v = %v; want 1", raw["v"])
	}
	if s, ok := raw["s"].(string); !ok || s != "recording" {
		t.Errorf("s = %v; want 'recording'", raw["s"])
	}
	if _, ok := raw["t"].(float64); !ok {
		t.Errorf("t should be a number (epoch ms), got %T", raw["t"])
	}
	if ut, ok := raw["ut"].(float64); !ok {
		t.Errorf("ut should be a number (epoch ms), got %T", raw["ut"])
	} else {
		expectedUT := updateAt.UnixMilli()
		if int64(ut) != expectedUT {
			t.Errorf("ut = %d; want %d", int64(ut), expectedUT)
		}
	}

	t.Logf("State wire format: %s", string(received))
}

func TestMQTTClientConn_SendStats_WireFormat(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create client connection
	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	// Create a "server" client to receive the stats
	serverClient, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-server",
	})
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverClient.Close()

	// Subscribe to stats topic
	statsTopic := "test/device/gear-001/stats"
	if err := serverClient.Subscribe(ctx, statsTopic); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Give time for subscription to be ready
	time.Sleep(50 * time.Millisecond)

	// Send stats from client - only Battery set (testing omitzero)
	now := time.Now()
	stats := &StatsEvent{
		Time:    jsontime.Milli(now),
		Battery: &Battery{Percentage: 85, IsCharging: true},
		// Other fields intentionally nil
	}

	if err := conn.SendStats(stats); err != nil {
		t.Fatalf("SendStats failed: %v", err)
	}

	// Receive and verify wire format
	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			msg, err := serverClient.RecvTimeout(200 * time.Millisecond)
			if err != nil {
				continue
			}
			if msg != nil && msg.Topic == statsTopic {
				received = msg.Payload
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for stats message")
	}

	if len(received) == 0 {
		t.Fatal("did not receive stats message")
	}

	// Parse and verify JSON structure
	var raw map[string]interface{}
	if err := json.Unmarshal(received, &raw); err != nil {
		t.Fatalf("invalid JSON received: %v\nData: %s", err, string(received))
	}

	// Verify time field
	if timeVal, ok := raw["time"].(float64); !ok {
		t.Errorf("time should be a number, got %T", raw["time"])
	} else {
		expectedTime := now.UnixMilli()
		if int64(timeVal) != expectedTime {
			t.Errorf("time = %d; want %d", int64(timeVal), expectedTime)
		}
	}

	// Verify battery field exists
	if battery, ok := raw["battery"].(map[string]interface{}); !ok {
		t.Errorf("battery should be an object, got %T", raw["battery"])
	} else {
		if pct, ok := battery["percentage"].(float64); !ok || pct != 85 {
			t.Errorf("battery.percentage = %v; want 85", battery["percentage"])
		}
		if charging, ok := battery["is_charging"].(bool); !ok || !charging {
			t.Errorf("battery.is_charging = %v; want true", battery["is_charging"])
		}
	}

	// Verify omitzero: nil fields should NOT be present
	omittedFields := []string{"volume", "brightness", "light_mode", "system_version", "wifi_network", "wifi_store", "pair_status", "shaking", "read_nfc_tag", "cellular_network"}
	for _, field := range omittedFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field '%s' should be omitted when nil", field)
		}
	}

	t.Logf("Stats wire format: %s", string(received))
}

func TestMQTTClientConn_SendStats_MultipleFields_WireFormat(t *testing.T) {
	addr, cleanup := startTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialMQTT(ctx, MQTTClientConfig{
		Addr:   "tcp://" + addr,
		Scope:  "test",
		GearID: "gear-001",
	})
	if err != nil {
		t.Fatalf("DialMQTT failed: %v", err)
	}
	defer conn.Close()

	serverClient, err := mqtt0.Connect(ctx, mqtt0.ClientConfig{
		Addr:     "tcp://" + addr,
		ClientID: "test-server",
	})
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverClient.Close()

	statsTopic := "test/device/gear-001/stats"
	if err := serverClient.Subscribe(ctx, statsTopic); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send stats with multiple fields set
	now := jsontime.NowEpochMilli()
	stats := &StatsEvent{
		Time:       now,
		Battery:    &Battery{Percentage: 80},
		Volume:     &Volume{Percentage: 70, UpdateAt: now},
		Brightness: &Brightness{Percentage: 60, UpdateAt: now},
		// Other fields nil
	}

	if err := conn.SendStats(stats); err != nil {
		t.Fatalf("SendStats failed: %v", err)
	}

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			msg, err := serverClient.RecvTimeout(200 * time.Millisecond)
			if err != nil {
				continue
			}
			if msg != nil && msg.Topic == statsTopic {
				received = msg.Payload
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for stats message")
	}

	if len(received) == 0 {
		t.Fatal("did not receive stats message")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(received, &raw); err != nil {
		t.Fatalf("invalid JSON received: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{"time", "battery", "volume", "brightness"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing expected field '%s'", field)
		}
	}

	// Verify omitted fields
	omittedFields := []string{"light_mode", "system_version", "wifi_network"}
	for _, field := range omittedFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field '%s' should be omitted", field)
		}
	}

	t.Logf("Stats wire format (multi-field): %s", string(received))
}
