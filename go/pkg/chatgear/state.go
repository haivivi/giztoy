// Package chatgear provides core types for device state, statistics, and commands.
package chatgear

import (
	"encoding/json"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

// State represents the state of a device.
type State int

const (
	StateUnknown State = iota
	StateShuttingDown
	StateSleeping
	StateResetting
	StateReady
	StateRecording
	StateWaitingForResponse
	StateStreaming
	StateCalling
	StateInterrupted
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateShuttingDown:
		return "shutting_down"
	case StateSleeping:
		return "sleeping"
	case StateResetting:
		return "resetting"
	case StateReady:
		return "ready"
	case StateRecording:
		return "recording"
	case StateWaitingForResponse:
		return "waiting_for_response"
	case StateStreaming:
		return "streaming"
	case StateCalling:
		return "calling"
	case StateInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *State) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	switch name {
	case "shutting_down":
		*s = StateShuttingDown
	case "sleeping":
		*s = StateSleeping
	case "resetting":
		*s = StateResetting
	case "ready":
		*s = StateReady
	case "recording":
		*s = StateRecording
	case "waiting_for_response":
		*s = StateWaitingForResponse
	case "streaming":
		*s = StateStreaming
	case "calling":
		*s = StateCalling
	case "interrupted":
		*s = StateInterrupted
	default:
		*s = StateUnknown
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (s State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// StateEvent represents a state change event from the device.
type StateEvent struct {
	Version  int               `json:"v"`
	Time     jsontime.Milli    `json:"t"`
	State    State             `json:"s"`
	Cause    *StateChangeCause `json:"c,omitempty"`
	UpdateAt jsontime.Milli    `json:"ut"`
}

// StateChangeCause provides additional context for why a state changed.
type StateChangeCause struct {
	CallingInitiated bool `json:"calling_initiated,omitempty"`
	CallingResume    bool `json:"calling_resume,omitempty"`
}

// NewStateEvent creates a new StateEvent.
func NewStateEvent(state State, updateAt time.Time) *StateEvent {
	return &StateEvent{
		Version:  1,
		Time:     jsontime.NowEpochMilli(),
		State:    state,
		UpdateAt: jsontime.Milli(updateAt),
	}
}

// Clone returns a deep copy of the event.
func (e *StateEvent) Clone() *StateEvent {
	if e == nil {
		return nil
	}
	v := *e
	if e.Cause != nil {
		cause := *e.Cause
		v.Cause = &cause
	}
	return &v
}

// MergeWith merges another event into this one.
// Returns true if the state changed.
func (e *StateEvent) MergeWith(other *StateEvent) bool {
	if other.Version != 1 {
		return false
	}
	if other.Time.Before(e.Time) {
		return false
	}
	e.Time = other.Time
	e.UpdateAt = other.UpdateAt
	e.Cause = other.Cause
	if e.State != other.State {
		e.State = other.State
		return true
	}
	return false
}

// IsActive returns true if the device is in an active (non-idle) state.
func (s State) IsActive() bool {
	switch s {
	case StateRecording, StateWaitingForResponse, StateStreaming, StateCalling:
		return true
	default:
		return false
	}
}

// CanRecord returns true if the device can start recording in this state.
func (s State) CanRecord() bool {
	return s == StateReady || s == StateStreaming
}
