package chatgear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

// Ensure all command types implement SessionCommand.
var (
	_ SessionCommand = (*Streaming)(nil)
	_ SessionCommand = (*Reset)(nil)
	_ SessionCommand = (*SetVolume)(nil)
	_ SessionCommand = (*SetBrightness)(nil)
	_ SessionCommand = (*SetLightMode)(nil)
	_ SessionCommand = (*SetWifi)(nil)
	_ SessionCommand = (*DeleteWifi)(nil)
	_ SessionCommand = (*OTA)(nil)
	_ SessionCommand = (*Raise)(nil)
	_ SessionCommand = (*Halt)(nil)
)

// SessionCommand is the interface for device commands.
type SessionCommand interface {
	isSessionCommand()
	commandType() string
}

// SessionCommandEvent wraps a command with metadata.
type SessionCommandEvent struct {
	Type    string         `json:"type"`
	Time    jsontime.Milli `json:"time"`
	Payload SessionCommand `json:"pld"`
	IssueAt jsontime.Milli `json:"issue_at"`
}

// NewSessionCommandEvent creates a new command event.
func NewSessionCommandEvent(cmd SessionCommand, issueAt time.Time) *SessionCommandEvent {
	return &SessionCommandEvent{
		Type:    cmd.commandType(),
		Time:    jsontime.NowEpochMilli(),
		Payload: cmd,
		IssueAt: jsontime.Milli(issueAt),
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (sce *SessionCommandEvent) UnmarshalJSON(b []byte) error {
	var v struct {
		Type    string          `json:"type"`
		Time    jsontime.Milli  `json:"time"`
		Payload json.RawMessage `json:"pld"`
		IssueAt jsontime.Milli  `json:"issue_at"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var cmd SessionCommand
	switch v.Type {
	case "streaming":
		cmd = new(Streaming)
	case "reset":
		cmd = new(Reset)
	case "set_volume":
		cmd = new(SetVolume)
	case "set_brightness":
		cmd = new(SetBrightness)
	case "set_light_mode":
		cmd = new(SetLightMode)
	case "set_wifi":
		cmd = new(SetWifi)
	case "delete_wifi":
		cmd = new(DeleteWifi)
	case "ota_upgrade":
		cmd = new(OTA)
	case "raise":
		cmd = new(Raise)
	case "halt":
		cmd = new(Halt)
	default:
		return fmt.Errorf("unknown command type: %s", v.Type)
	}

	if err := json.Unmarshal(v.Payload, cmd); err != nil {
		return err
	}

	*sce = SessionCommandEvent{
		Type:    v.Type,
		Time:    v.Time,
		Payload: cmd,
		IssueAt: v.IssueAt,
	}
	return nil
}

// Streaming is a command to start/stop audio streaming.
type Streaming bool

// NewStreaming creates a new Streaming command.
func NewStreaming(enabled bool) *Streaming {
	s := Streaming(enabled)
	return &s
}

func (*Streaming) isSessionCommand()    {}
func (*Streaming) commandType() string  { return "streaming" }

func (s Streaming) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(s))
}

func (s *Streaming) UnmarshalJSON(b []byte) error {
	var v bool
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = Streaming(v)
	return nil
}

// Reset is a command to reset the device.
type Reset struct {
	Unpair bool `json:"unpair,omitempty"`
}

func (*Reset) isSessionCommand()    {}
func (*Reset) commandType() string  { return "reset" }

func (r Reset) MarshalJSON() ([]byte, error) {
	if r == (Reset{}) {
		return json.Marshal(nil)
	}
	v := struct {
		Unpair bool `json:"unpair"`
	}{
		Unpair: r.Unpair,
	}
	return json.Marshal(v)
}

func (r *Reset) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		*r = Reset{}
		return nil
	}
	var v struct {
		Unpair bool `json:"unpair"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*r = Reset{Unpair: v.Unpair}
	return nil
}

// Raise is a command to raise an event (e.g., start a call).
type Raise struct {
	Call bool `json:"call,omitempty"`
}

func (*Raise) isSessionCommand()    {}
func (*Raise) commandType() string  { return "raise" }

func (r Raise) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Call bool `json:"call"`
	}{
		Call: r.Call,
	})
}

func (r *Raise) UnmarshalJSON(b []byte) error {
	var v struct {
		Call bool `json:"call"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*r = Raise{Call: v.Call}
	return nil
}

// Halt is a command to halt device operation.
type Halt struct {
	Sleep     bool `json:"sleep,omitempty"`
	Shutdown  bool `json:"shutdown,omitempty"`
	Interrupt bool `json:"interrupt,omitempty"`
}

func (*Halt) isSessionCommand()    {}
func (*Halt) commandType() string  { return "halt" }

func (h Halt) MarshalJSON() ([]byte, error) {
	if h == (Halt{}) {
		return json.Marshal(nil)
	}
	v := struct {
		Sleep     bool `json:"sleep"`
		Shutdown  bool `json:"shutdown"`
		Interrupt bool `json:"interrupt"`
	}{
		Sleep:     h.Sleep,
		Shutdown:  h.Shutdown,
		Interrupt: h.Interrupt,
	}
	return json.Marshal(v)
}

func (h *Halt) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		*h = Halt{}
		return nil
	}
	var v struct {
		Sleep     bool `json:"sleep"`
		Shutdown  bool `json:"shutdown"`
		Interrupt bool `json:"interrupt"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*h = Halt{
		Sleep:     v.Sleep,
		Shutdown:  v.Shutdown,
		Interrupt: v.Interrupt,
	}
	return nil
}

// SetVolume is a command to set audio volume.
type SetVolume int

// NewSetVolume creates a new SetVolume command.
func NewSetVolume(volume int) *SetVolume {
	v := SetVolume(volume)
	return &v
}

func (*SetVolume) isSessionCommand()    {}
func (*SetVolume) commandType() string  { return "set_volume" }

func (s SetVolume) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(s))
}

func (s *SetVolume) UnmarshalJSON(b []byte) error {
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = SetVolume(v)
	return nil
}

// SetBrightness is a command to set display brightness.
type SetBrightness int

// NewSetBrightness creates a new SetBrightness command.
func NewSetBrightness(brightness int) *SetBrightness {
	b := SetBrightness(brightness)
	return &b
}

func (*SetBrightness) isSessionCommand()    {}
func (*SetBrightness) commandType() string  { return "set_brightness" }

func (s SetBrightness) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(s))
}

func (s *SetBrightness) UnmarshalJSON(b []byte) error {
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = SetBrightness(v)
	return nil
}

// SetLightMode is a command to set light mode.
type SetLightMode string

// NewSetLightMode creates a new SetLightMode command.
func NewSetLightMode(mode string) *SetLightMode {
	m := SetLightMode(mode)
	return &m
}

func (*SetLightMode) isSessionCommand()    {}
func (*SetLightMode) commandType() string  { return "set_light_mode" }

func (s SetLightMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s *SetLightMode) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = SetLightMode(v)
	return nil
}

// SetWifi is a command to configure WiFi.
type SetWifi struct {
	SSID     string `json:"ssid"`
	Security string `json:"security"`
	Password string `json:"password"`
}

func (SetWifi) isSessionCommand()    {}
func (SetWifi) commandType() string  { return "set_wifi" }

// DeleteWifi is a command to delete a stored WiFi network.
type DeleteWifi string

func (DeleteWifi) isSessionCommand()    {}
func (DeleteWifi) commandType() string  { return "delete_wifi" }

func (s DeleteWifi) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s *DeleteWifi) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = DeleteWifi(v)
	return nil
}

// OTA is a command to initiate firmware upgrade.
type OTA struct {
	Version     string         `json:"version,omitzero"`
	ImageURL    string         `json:"image_url,omitzero"`
	ImageMD5    string         `json:"image_md5,omitzero"`
	DataFileURL string         `json:"data_file_url,omitzero"`
	DataFileMD5 string         `json:"data_file_md5,omitzero"`
	Components  []ComponentOTA `json:"components,omitzero"`
}

// ComponentOTA contains OTA info for a component.
type ComponentOTA struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitzero"`
	ImageURL    string `json:"image_url,omitzero"`
	ImageMD5    string `json:"image_md5,omitzero"`
	DataFileURL string `json:"data_file_url,omitzero"`
	DataFileMD5 string `json:"data_file_md5,omitzero"`
}

func (*OTA) isSessionCommand()    {}
func (*OTA) commandType() string  { return "ota_upgrade" }
