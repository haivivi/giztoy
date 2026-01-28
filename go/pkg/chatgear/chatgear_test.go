package chatgear

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

func TestGearState_String(t *testing.T) {
	tests := []struct {
		state GearState
		want  string
	}{
		{GearUnknown, "unknown"},
		{GearShuttingDown, "shutting_down"},
		{GearSleeping, "sleeping"},
		{GearResetting, "resetting"},
		{GearReady, "ready"},
		{GearRecording, "recording"},
		{GearWaitingForResponse, "waiting_for_response"},
		{GearStreaming, "streaming"},
		{GearCalling, "calling"},
		{GearInterrupted, "interrupted"},
	}

	for _, tc := range tests {
		if tc.state.String() != tc.want {
			t.Errorf("GearState(%d).String() = %q; want %q", tc.state, tc.state.String(), tc.want)
		}
	}
}

func TestGearState_JSON(t *testing.T) {
	tests := []GearState{
		GearReady,
		GearRecording,
		GearStreaming,
		GearCalling,
	}

	for _, state := range tests {
		data, err := json.Marshal(state)
		if err != nil {
			t.Errorf("Marshal GearState(%d) error: %v", state, err)
			continue
		}

		var restored GearState
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Errorf("Unmarshal GearState error: %v", err)
			continue
		}

		if restored != state {
			t.Errorf("GearState JSON roundtrip: got %v, want %v", restored, state)
		}
	}
}

func TestGearState_IsActive(t *testing.T) {
	activeStates := []GearState{GearRecording, GearWaitingForResponse, GearStreaming, GearCalling}
	inactiveStates := []GearState{GearUnknown, GearShuttingDown, GearSleeping, GearResetting, GearReady, GearInterrupted}

	for _, s := range activeStates {
		if !s.IsActive() {
			t.Errorf("GearState(%v).IsActive() = false; want true", s)
		}
	}

	for _, s := range inactiveStates {
		if s.IsActive() {
			t.Errorf("GearState(%v).IsActive() = true; want false", s)
		}
	}
}

func TestGearState_CanRecord(t *testing.T) {
	canRecord := []GearState{GearReady, GearStreaming}
	cannotRecord := []GearState{GearUnknown, GearShuttingDown, GearSleeping, GearRecording, GearCalling}

	for _, s := range canRecord {
		if !s.CanRecord() {
			t.Errorf("GearState(%v).CanRecord() = false; want true", s)
		}
	}

	for _, s := range cannotRecord {
		if s.CanRecord() {
			t.Errorf("GearState(%v).CanRecord() = true; want false", s)
		}
	}
}

func TestGearStateEvent_Clone(t *testing.T) {
	event := &GearStateEvent{
		Version:  1,
		Time:     jsontime.NowEpochMilli(),
		State:    GearReady,
		Cause:    &GearStateChangeCause{CallingInitiated: true},
		UpdateAt: jsontime.NowEpochMilli(),
	}

	clone := event.Clone()

	// Modify original
	event.State = GearRecording
	event.Cause.CallingInitiated = false

	// Clone should be unchanged
	if clone.State != GearReady {
		t.Error("Clone was modified when original changed")
	}
	if clone.Cause.CallingInitiated != true {
		t.Error("Clone's Cause was modified when original changed")
	}
}

func TestGearStateEvent_MergeWith(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	event := NewGearStateEvent(GearReady, t1)
	event.Time = jsontime.Milli(t1)

	other := NewGearStateEvent(GearRecording, t2)
	other.Time = jsontime.Milli(t2)

	changed := event.MergeWith(other)

	if !changed {
		t.Error("MergeWith should return true when state changes")
	}
	if event.State != GearRecording {
		t.Errorf("State = %v; want GearRecording", event.State)
	}
}

func TestGearStateEvent_MergeWith_OlderEvent(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	event := NewGearStateEvent(GearReady, t1)
	event.Time = jsontime.Milli(t1)

	other := NewGearStateEvent(GearRecording, t2)
	other.Time = jsontime.Milli(t2)

	changed := event.MergeWith(other)

	if changed {
		t.Error("MergeWith should return false for older event")
	}
	if event.State != GearReady {
		t.Error("State should not change for older event")
	}
}

func TestSessionCommandEvent_JSON(t *testing.T) {
	issueAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		cmd  SessionCommand
	}{
		{"streaming_true", NewStreaming(true)},
		{"streaming_false", NewStreaming(false)},
		{"reset", &Reset{}},
		{"reset_unpair", &Reset{Unpair: true}},
		{"set_volume", NewSetVolume(50)},
		{"set_brightness", NewSetBrightness(80)},
		{"set_light_mode", NewSetLightMode("dark")},
		{"set_wifi", &SetWifi{SSID: "test", Security: "wpa2", Password: "pass"}},
		{"delete_wifi", (*DeleteWifi)(strPtr("test-ssid"))},
		{"ota", &OTA{Version: "1.0.0", ImageURL: "http://example.com/image"}},
		{"raise", &Raise{Call: true}},
		{"halt_sleep", &Halt{Sleep: true}},
		{"halt_shutdown", &Halt{Shutdown: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := NewSessionCommandEvent(tc.cmd, issueAt)

			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var restored SessionCommandEvent
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if restored.Type != event.Type {
				t.Errorf("Type = %q; want %q", restored.Type, event.Type)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestBattery_Equal(t *testing.T) {
	b1 := &Battery{Percentage: 50, IsCharging: true}
	b2 := &Battery{Percentage: 50, IsCharging: true}
	b3 := &Battery{Percentage: 60, IsCharging: true}

	if !b1.Equal(b2) {
		t.Error("Equal batteries should be equal")
	}
	if b1.Equal(b3) {
		t.Error("Different batteries should not be equal")
	}
	if b1.Equal(nil) {
		t.Error("Non-nil battery should not equal nil")
	}
	if (*Battery)(nil).Equal(b1) {
		t.Error("Nil battery should not equal non-nil")
	}
}

func TestConnectedWifi_Equal(t *testing.T) {
	w1 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w2 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w3 := &ConnectedWifi{SSID: "other", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w4 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.4.4"}}

	if !w1.Equal(w2) {
		t.Error("Equal WiFi should be equal")
	}
	if w1.Equal(w3) {
		t.Error("Different SSID should not be equal")
	}
	if w1.Equal(w4) {
		t.Error("Different DNS should not be equal")
	}
}

func TestGearStatsEvent_Clone(t *testing.T) {
	event := &GearStatsEvent{
		Time:    jsontime.NowEpochMilli(),
		Battery: &Battery{Percentage: 80},
		Volume:  &Volume{Percentage: 50},
		WifiNetwork: &ConnectedWifi{
			SSID: "test",
			DNS:  []string{"8.8.8.8"},
		},
	}

	clone := event.Clone()

	// Modify original
	event.Battery.Percentage = 20
	event.WifiNetwork.SSID = "changed"
	event.WifiNetwork.DNS[0] = "1.1.1.1"

	// Clone should be unchanged
	if clone.Battery.Percentage != 80 {
		t.Error("Clone's Battery was modified")
	}
	if clone.WifiNetwork.SSID != "test" {
		t.Error("Clone's WiFi SSID was modified")
	}
	if clone.WifiNetwork.DNS[0] != "8.8.8.8" {
		t.Error("Clone's WiFi DNS was modified")
	}
}

func TestGearStatsEvent_MergeWith(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC))

	event := &GearStatsEvent{
		Time:    t1,
		Battery: &Battery{Percentage: 80},
	}

	other := &GearStatsEvent{
		Time:    t2,
		Battery: &Battery{Percentage: 70},
		Volume:  &Volume{Percentage: 50, UpdateAt: t2},
	}

	changes := event.MergeWith(other)

	if changes == nil {
		t.Fatal("MergeWith should return changes")
	}
	if changes.Battery == nil || changes.Battery.Percentage != 70 {
		t.Error("Battery change not detected")
	}
	if changes.Volume == nil || changes.Volume.Percentage != 50 {
		t.Error("Volume change not detected")
	}
}

func TestReadNFCTag_Equal(t *testing.T) {
	tag1 := &NFCTag{UID: "abc123"}
	tag2 := &NFCTag{UID: "def456"}

	nfc1 := &ReadNFCTag{Tags: []*NFCTag{tag1, tag2}}
	nfc2 := &ReadNFCTag{Tags: []*NFCTag{tag2, tag1}} // Same UIDs, different order
	nfc3 := &ReadNFCTag{Tags: []*NFCTag{tag1}}       // Missing one

	if !nfc1.Equal(nfc2) {
		t.Error("Same UIDs in different order should be equal")
	}
	if nfc1.Equal(nfc3) {
		t.Error("Different number of tags should not be equal")
	}
}

func TestReadNFCTag_JSON(t *testing.T) {
	nfc := &ReadNFCTag{
		Tags: []*NFCTag{
			{UID: "abc", Type: "NDEF"},
			{UID: "def", Type: "MIFARE"},
		},
	}

	data, err := json.Marshal(nfc)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored ReadNFCTag
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(restored.Tags) != 2 {
		t.Errorf("Tags length = %d; want 2", len(restored.Tags))
	}
}

func TestShaking_Equal(t *testing.T) {
	s1 := &Shaking{Level: 0.5}
	s2 := &Shaking{Level: 0.5}
	s3 := &Shaking{Level: 0.8}

	if !s1.Equal(s2) {
		t.Error("Equal Shaking should be equal")
	}
	if s1.Equal(s3) {
		t.Error("Different Shaking should not be equal")
	}
}
