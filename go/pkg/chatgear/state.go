// Package chatgear provides core types for device state, statistics, and commands.
package chatgear

import (
	"encoding/json"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

// GearState represents the state of a device.
type GearState int

const (
	GearUnknown GearState = iota
	GearShuttingDown
	GearSleeping
	GearResetting
	GearReady
	GearRecording
	GearWaitingForResponse
	GearStreaming
	GearCalling
	GearInterrupted
)

// String returns the string representation of the state.
func (gs GearState) String() string {
	switch gs {
	case GearShuttingDown:
		return "shutting_down"
	case GearSleeping:
		return "sleeping"
	case GearResetting:
		return "resetting"
	case GearReady:
		return "ready"
	case GearRecording:
		return "recording"
	case GearWaitingForResponse:
		return "waiting_for_response"
	case GearStreaming:
		return "streaming"
	case GearCalling:
		return "calling"
	case GearInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (gs *GearState) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	switch name {
	case "shutting_down":
		*gs = GearShuttingDown
	case "sleeping":
		*gs = GearSleeping
	case "resetting":
		*gs = GearResetting
	case "ready":
		*gs = GearReady
	case "recording":
		*gs = GearRecording
	case "waiting_for_response":
		*gs = GearWaitingForResponse
	case "streaming":
		*gs = GearStreaming
	case "calling":
		*gs = GearCalling
	case "interrupted":
		*gs = GearInterrupted
	default:
		*gs = GearUnknown
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (gs GearState) MarshalJSON() ([]byte, error) {
	return json.Marshal(gs.String())
}

// GearStateEvent represents a state change event from the device.
type GearStateEvent struct {
	Version  int                   `json:"v"`
	Time     jsontime.Milli        `json:"t"`
	State    GearState             `json:"s"`
	Cause    *GearStateChangeCause `json:"c,omitempty"`
	UpdateAt jsontime.Milli        `json:"ut"`
}

// GearStateChangeCause provides additional context for why a state changed.
type GearStateChangeCause struct {
	CallingInitiated bool `json:"calling_initiated,omitempty"`
	CallingResume    bool `json:"calling_resume,omitempty"`
}

// NewGearStateEvent creates a new GearStateEvent.
func NewGearStateEvent(state GearState, updateAt time.Time) *GearStateEvent {
	return &GearStateEvent{
		Version:  1,
		Time:     jsontime.NowEpochMilli(),
		State:    state,
		UpdateAt: jsontime.Milli(updateAt),
	}
}

// Clone returns a deep copy of the event.
func (gse *GearStateEvent) Clone() *GearStateEvent {
	if gse == nil {
		return nil
	}
	v := *gse
	if gse.Cause != nil {
		cause := *gse.Cause
		v.Cause = &cause
	}
	return &v
}

// MergeWith merges another event into this one.
// Returns true if the state changed.
func (gse *GearStateEvent) MergeWith(other *GearStateEvent) bool {
	if other.Version != 1 {
		return false
	}
	if other.Time.Before(gse.Time) {
		return false
	}
	gse.Time = other.Time
	gse.UpdateAt = other.UpdateAt
	gse.Cause = other.Cause
	if gse.State != other.State {
		gse.State = other.State
		return true
	}
	return false
}

// IsActive returns true if the device is in an active (non-idle) state.
func (gs GearState) IsActive() bool {
	switch gs {
	case GearRecording, GearWaitingForResponse, GearStreaming, GearCalling:
		return true
	default:
		return false
	}
}

// CanRecord returns true if the device can start recording in this state.
func (gs GearState) CanRecord() bool {
	return gs == GearReady || gs == GearStreaming
}
