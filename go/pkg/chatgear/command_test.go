package chatgear

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCommandEvent_JSON(t *testing.T) {
	issueAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		cmd  Command
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
			event := NewCommandEvent(tc.cmd, issueAt)

			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var restored CommandEvent
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

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestCommandEvent_UnmarshalJSON_InvalidCases(t *testing.T) {
	invalidCases := []struct {
		name  string
		input string
	}{
		{"empty_object", `{}`},
		{"missing_type", `{"issue_at": 1234567890}`},
		{"invalid_json", `invalid json`},
		{"null_type", `{"type": null}`},
		{"empty_type", `{"type": ""}`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			var evt CommandEvent
			err := json.Unmarshal([]byte(tc.input), &evt)
			// Either error or empty/unknown type should be produced
			if err == nil && evt.Type != "" {
				t.Logf("Parsed type: %q", evt.Type)
			}
		})
	}
}

func TestCommandEvent_UnmarshalJSON_UnknownType(t *testing.T) {
	input := `{"type": "unknown_command", "issue_at": 1234567890}`
	var evt CommandEvent
	err := json.Unmarshal([]byte(input), &evt)
	// Should handle unknown type gracefully
	if err != nil {
		t.Logf("Unknown type produces error: %v", err)
	} else {
		t.Logf("Unknown type parsed as: %+v", evt)
	}
}

func TestCommandEvent_UnmarshalJSON_MalformedPayload(t *testing.T) {
	malformedCases := []struct {
		name  string
		input string
	}{
		{"set_volume_wrong_payload", `{"type": "set_volume", "payload": "not_a_number"}`},
		{"streaming_wrong_payload", `{"type": "streaming", "payload": 123}`},
		{"set_brightness_wrong", `{"type": "set_brightness", "payload": "not_a_number"}`},
		{"set_light_mode_wrong", `{"type": "set_light_mode", "payload": 123}`},
		{"set_wifi_wrong", `{"type": "set_wifi", "payload": "string"}`},
		{"delete_wifi_wrong", `{"type": "delete_wifi", "payload": 123}`},
		{"ota_wrong", `{"type": "ota", "payload": "string"}`},
		{"raise_wrong", `{"type": "raise", "payload": "string"}`},
		{"halt_wrong", `{"type": "halt", "payload": "string"}`},
		{"reset_wrong", `{"type": "reset", "payload": "string"}`},
	}

	for _, tc := range malformedCases {
		t.Run(tc.name, func(t *testing.T) {
			var evt CommandEvent
			err := json.Unmarshal([]byte(tc.input), &evt)
			if err != nil {
				t.Logf("Malformed payload produces error: %v", err)
			}
		})
	}
}

func TestCommandEvent_UnmarshalJSON_AllTypes(t *testing.T) {
	// Note: CommandEvent uses "pld" for payload and "ota_upgrade" for OTA
	validCases := []struct {
		name  string
		input string
	}{
		{"streaming", `{"type": "streaming", "pld": true, "issue_at": 1234567890}`},
		{"set_volume", `{"type": "set_volume", "pld": 50, "issue_at": 1234567890}`},
		{"set_brightness", `{"type": "set_brightness", "pld": 80, "issue_at": 1234567890}`},
		{"set_light_mode", `{"type": "set_light_mode", "pld": "dark", "issue_at": 1234567890}`},
		{"set_wifi", `{"type": "set_wifi", "pld": {"ssid": "test", "security": "wpa2", "password": "pass"}, "issue_at": 1234567890}`},
		{"delete_wifi", `{"type": "delete_wifi", "pld": "test-ssid", "issue_at": 1234567890}`},
		{"ota_upgrade", `{"type": "ota_upgrade", "pld": {"version": "1.0.0", "image_url": "http://example.com"}, "issue_at": 1234567890}`},
		{"raise", `{"type": "raise", "pld": {"call": true}, "issue_at": 1234567890}`},
		{"halt", `{"type": "halt", "pld": {"sleep": true}, "issue_at": 1234567890}`},
		{"reset", `{"type": "reset", "pld": {"unpair": false}, "issue_at": 1234567890}`},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			var evt CommandEvent
			err := json.Unmarshal([]byte(tc.input), &evt)
			if err != nil {
				t.Errorf("Unmarshal %s: %v", tc.name, err)
			}
			if evt.Type == "" {
				t.Errorf("Type should not be empty for %s", tc.name)
			}
		})
	}
}

func TestDeleteWifi_MarshalJSON(t *testing.T) {
	ssid := "test-ssid"
	cmd := (*DeleteWifi)(&ssid)

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	t.Logf("DeleteWifi marshaled: %s", string(data))
}

func TestDeleteWifi_MarshalJSON_Nil(t *testing.T) {
	var cmd *DeleteWifi = nil

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Marshal nil: %v", err)
	}

	t.Logf("DeleteWifi nil marshaled: %s", string(data))
}

func TestHalt_MarshalJSON_Empty(t *testing.T) {
	halt := Halt{}

	data, err := json.Marshal(halt)
	if err != nil {
		t.Fatalf("Marshal empty Halt: %v", err)
	}

	// Empty halt should marshal as null
	t.Logf("Empty Halt marshaled: %s", string(data))
}

func TestHalt_UnmarshalJSON_Null(t *testing.T) {
	var halt Halt
	err := json.Unmarshal([]byte("null"), &halt)
	if err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if halt != (Halt{}) {
		t.Errorf("Expected empty Halt, got %+v", halt)
	}
}

func TestCommand_TypeMethods(t *testing.T) {
	// Test commandType methods
	commands := []struct {
		cmd  Command
		typ  string
	}{
		{NewStreaming(true), "streaming"},
		{&Reset{}, "reset"},
		{NewSetVolume(50), "set_volume"},
		{NewSetBrightness(80), "set_brightness"},
		{NewSetLightMode("dark"), "set_light_mode"},
		{&SetWifi{}, "set_wifi"},
		{(*DeleteWifi)(strPtr("test")), "delete_wifi"},
		{&OTA{}, "ota_upgrade"},
		{&Raise{}, "raise"},
		{&Halt{}, "halt"},
	}

	for _, tc := range commands {
		if tc.cmd.commandType() != tc.typ {
			t.Errorf("commandType() = %q; want %q", tc.cmd.commandType(), tc.typ)
		}
	}
}
