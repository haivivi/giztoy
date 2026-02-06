package chatgear

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateUnknown, "unknown"},
		{StateShuttingDown, "shutting_down"},
		{StateSleeping, "sleeping"},
		{StateResetting, "resetting"},
		{StateReady, "ready"},
		{StateRecording, "recording"},
		{StateWaitingForResponse, "waiting_for_response"},
		{StateStreaming, "streaming"},
		{StateCalling, "calling"},
		{StateInterrupted, "interrupted"},
	}

	for _, tc := range tests {
		if tc.state.String() != tc.want {
			t.Errorf("State(%d).String() = %q; want %q", tc.state, tc.state.String(), tc.want)
		}
	}
}

func TestState_JSON(t *testing.T) {
	tests := []State{
		StateReady,
		StateRecording,
		StateStreaming,
		StateCalling,
	}

	for _, state := range tests {
		data, err := json.Marshal(state)
		if err != nil {
			t.Errorf("Marshal State(%d) error: %v", state, err)
			continue
		}

		var restored State
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Errorf("Unmarshal State error: %v", err)
			continue
		}

		if restored != state {
			t.Errorf("State JSON roundtrip: got %v, want %v", restored, state)
		}
	}
}

func TestState_IsActive(t *testing.T) {
	activeStates := []State{StateRecording, StateWaitingForResponse, StateStreaming, StateCalling}
	inactiveStates := []State{StateUnknown, StateShuttingDown, StateSleeping, StateResetting, StateReady, StateInterrupted}

	for _, s := range activeStates {
		if !s.IsActive() {
			t.Errorf("State(%v).IsActive() = false; want true", s)
		}
	}

	for _, s := range inactiveStates {
		if s.IsActive() {
			t.Errorf("State(%v).IsActive() = true; want false", s)
		}
	}
}

func TestState_CanRecord(t *testing.T) {
	canRecord := []State{StateReady, StateStreaming}
	cannotRecord := []State{StateUnknown, StateShuttingDown, StateSleeping, StateRecording, StateCalling}

	for _, s := range canRecord {
		if !s.CanRecord() {
			t.Errorf("State(%v).CanRecord() = false; want true", s)
		}
	}

	for _, s := range cannotRecord {
		if s.CanRecord() {
			t.Errorf("State(%v).CanRecord() = true; want false", s)
		}
	}
}

func TestStateEvent_Clone(t *testing.T) {
	event := &StateEvent{
		Version:  1,
		Time:     jsontime.NowEpochMilli(),
		State:    StateReady,
		Cause:    &StateChangeCause{CallingInitiated: true},
		UpdateAt: jsontime.NowEpochMilli(),
	}

	clone := event.Clone()

	// Modify original
	event.State = StateRecording
	event.Cause.CallingInitiated = false

	// Clone should be unchanged
	if clone.State != StateReady {
		t.Error("Clone was modified when original changed")
	}
	if clone.Cause.CallingInitiated != true {
		t.Error("Clone's Cause was modified when original changed")
	}
}

func TestStateEvent_MergeWith(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	event := NewStateEvent(StateReady, t1)
	event.Time = jsontime.Milli(t1)

	other := NewStateEvent(StateRecording, t2)
	other.Time = jsontime.Milli(t2)

	changed := event.MergeWith(other)

	if !changed {
		t.Error("MergeWith should return true when state changes")
	}
	if event.State != StateRecording {
		t.Errorf("State = %v; want StateRecording", event.State)
	}
}

func TestStateEvent_MergeWith_OlderEvent(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	event := NewStateEvent(StateReady, t1)
	event.Time = jsontime.Milli(t1)

	other := NewStateEvent(StateRecording, t2)
	other.Time = jsontime.Milli(t2)

	changed := event.MergeWith(other)

	if changed {
		t.Error("MergeWith should return false for older event")
	}
	if event.State != StateReady {
		t.Error("State should not change for older event")
	}
}

func TestStateEvent_Clone_WithCause(t *testing.T) {
	event := &StateEvent{
		Version: 1,
		State:   StateReady,
		Cause: &StateChangeCause{
			CallingInitiated: true,
			CallingResume:    true,
		},
	}

	clone := event.Clone()

	// Verify clone is independent
	event.Cause.CallingInitiated = false
	if !clone.Cause.CallingInitiated {
		t.Error("Clone's Cause was modified when original changed")
	}
}

func TestState_UnmarshalJSON_Unknown(t *testing.T) {
	var state State
	err := json.Unmarshal([]byte(`"invalid_state"`), &state)
	// Invalid state may return error or default to unknown
	if err == nil && state != StateUnknown {
		t.Errorf("Invalid state should either error or default to unknown, got %v", state)
	}
}

func TestState_UnmarshalJSON_AllStates(t *testing.T) {
	states := []string{
		"unknown", "shutting_down", "sleeping", "resetting", "ready",
		"recording", "waiting_for_response", "streaming", "calling", "interrupted",
	}

	for _, s := range states {
		var state State
		err := json.Unmarshal([]byte(`"`+s+`"`), &state)
		if err != nil {
			t.Errorf("Unmarshal %q error: %v", s, err)
		}
	}
}

func TestStateEvent_MergeWith_WithCause(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	event := NewStateEvent(StateReady, t1)
	event.Time = jsontime.Milli(t1)

	other := &StateEvent{
		Version: 1,
		Time:    jsontime.Milli(t2),
		State:   StateRecording,
		Cause: &StateChangeCause{
			CallingInitiated: true,
		},
	}

	changed := event.MergeWith(other)

	if !changed {
		t.Error("MergeWith should return true when state changes")
	}
	if event.Cause == nil || !event.Cause.CallingInitiated {
		t.Error("Cause should be copied")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestState_UnmarshalJSON_InvalidTypes(t *testing.T) {
	invalidCases := []struct {
		name  string
		input string
	}{
		{"number", `123`},
		{"object", `{"state": "ready"}`},
		{"array", `["ready"]`},
		{"null", `null`},
		{"empty_string", `""`},
		{"bool", `true`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			var state State
			err := json.Unmarshal([]byte(tc.input), &state)
			if err != nil {
				t.Logf("Invalid input %s produces error: %v", tc.name, err)
			} else {
				t.Logf("Invalid input %s parsed as: %v", tc.name, state)
			}
		})
	}
}

func TestStateEvent_JSON_RoundTrip(t *testing.T) {
	now := time.Now()
	event := NewStateEvent(StateRecording, now)
	event.Cause = &StateChangeCause{
		CallingResume:    true,
		CallingInitiated: true,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored StateEvent
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.State != event.State {
		t.Errorf("State = %v; want %v", restored.State, event.State)
	}
	if restored.Cause == nil {
		t.Error("Cause should be restored")
	}
}

func TestStateEvent_UnmarshalJSON_Invalid(t *testing.T) {
	invalidCases := []struct {
		name  string
		input string
	}{
		{"invalid_json", `invalid`},
		{"empty_object", `{}`},
		{"null", `null`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			var event StateEvent
			err := json.Unmarshal([]byte(tc.input), &event)
			if err != nil {
				t.Logf("Invalid %s produces error: %v", tc.name, err)
			} else {
				t.Logf("Invalid %s parsed successfully: %+v", tc.name, event)
			}
		})
	}
}

func TestStateEvent_Clone_Nil(t *testing.T) {
	var event *StateEvent = nil
	clone := event.Clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestStateEvent_MergeWith_WrongVersion(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	event := NewStateEvent(StateReady, t1)
	other := &StateEvent{
		Version: 2, // Wrong version
		Time:    jsontime.Milli(t1.Add(time.Hour)),
		State:   StateRecording,
	}

	changed := event.MergeWith(other)
	if changed {
		t.Error("MergeWith should return false for wrong version")
	}
	if event.State != StateReady {
		t.Error("State should not change for wrong version")
	}
}

// =============================================================================
// JSON Format and Timestamp Tests
// =============================================================================

func TestStateEvent_JSONFormat_TimestampsAreEpochMillis(t *testing.T) {
	// Create event with known time
	updateAt := time.Date(2024, 6, 15, 10, 30, 45, 123000000, time.UTC)
	event := NewStateEvent(StateRecording, updateAt)

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Parse as raw JSON to inspect field values
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map error: %v", err)
	}

	// Verify required fields exist
	if _, ok := raw["v"]; !ok {
		t.Error("Missing 'v' (version) field")
	}
	if _, ok := raw["t"]; !ok {
		t.Error("Missing 't' (time) field")
	}
	if _, ok := raw["s"]; !ok {
		t.Error("Missing 's' (state) field")
	}
	if _, ok := raw["ut"]; !ok {
		t.Error("Missing 'ut' (update_at) field")
	}

	// Verify version
	if v, ok := raw["v"].(float64); !ok || int(v) != 1 {
		t.Errorf("'v' = %v; want 1", raw["v"])
	}

	// Verify state is string
	if s, ok := raw["s"].(string); !ok || s != "recording" {
		t.Errorf("'s' = %v; want 'recording'", raw["s"])
	}

	// Verify 't' is epoch milliseconds (a large number)
	if tVal, ok := raw["t"].(float64); ok {
		// Should be around 2024 epoch millis (> 1700000000000)
		if tVal < 1700000000000 {
			t.Errorf("'t' = %v; should be epoch milliseconds (> 1.7e12)", tVal)
		}
	} else {
		t.Errorf("'t' is not a number: %T", raw["t"])
	}

	// Verify 'ut' is the expected epoch milliseconds
	expectedUT := updateAt.UnixMilli()
	if utVal, ok := raw["ut"].(float64); ok {
		if int64(utVal) != expectedUT {
			t.Errorf("'ut' = %v; want %v (epoch ms for %v)", int64(utVal), expectedUT, updateAt)
		}
	} else {
		t.Errorf("'ut' is not a number: %T", raw["ut"])
	}

	t.Logf("StateEvent JSON: %s", string(data))
}

func TestStateEvent_JSONFormat_AllFields(t *testing.T) {
	updateAt := time.Now()
	event := NewStateEvent(StateCalling, updateAt)
	event.Cause = &StateChangeCause{
		CallingInitiated: true,
		CallingResume:    false,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Parse as raw JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Verify cause is included
	if _, ok := raw["c"]; !ok {
		t.Error("Missing 'c' (cause) field when Cause is set")
	}

	// Verify cause structure
	if cause, ok := raw["c"].(map[string]interface{}); ok {
		if ci, ok := cause["calling_initiated"].(bool); !ok || !ci {
			t.Errorf("cause.calling_initiated = %v; want true", cause["calling_initiated"])
		}
	} else {
		t.Errorf("'c' is not an object: %T", raw["c"])
	}

	t.Logf("StateEvent with Cause JSON: %s", string(data))
}

func TestNewStateEvent_SetsTimeAndUpdateAt(t *testing.T) {
	before := time.Now()
	updateAt := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	event := NewStateEvent(StateReady, updateAt)
	after := time.Now()

	// Time should be "now" (between before and after)
	eventTime := time.Time(event.Time)
	if eventTime.Before(before) || eventTime.After(after) {
		t.Errorf("Time = %v; should be between %v and %v", eventTime, before, after)
	}

	// UpdateAt should match the passed value
	eventUpdateAt := time.Time(event.UpdateAt)
	if eventUpdateAt.UnixMilli() != updateAt.UnixMilli() {
		t.Errorf("UpdateAt = %v; want %v", eventUpdateAt, updateAt)
	}
}
