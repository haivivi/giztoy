package jsontime

import (
	"encoding/json"
	"time"
)

// Duration is a time.Duration that serializes to/from a string or int64 in JSON.
// When marshaling, it outputs the duration string (e.g., "1h30m").
// When unmarshaling, it accepts either a string (e.g., "1h30m") or an int64 (nanoseconds).
type Duration time.Duration

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		dur, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(dur)
		return nil
	}
	var t int64
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*d = Duration(time.Duration(t))
	return nil
}

// Duration returns the underlying time.Duration value.
// Returns 0 if d is nil.
func (d *Duration) Duration() time.Duration {
	if d == nil {
		return 0
	}
	return time.Duration(*d)
}

// String returns the duration formatted as a string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// FromDuration creates a Duration pointer from a time.Duration.
func FromDuration(d time.Duration) *Duration {
	v := Duration(d)
	return &v
}

// Seconds returns the duration as a floating point number of seconds.
func (d Duration) Seconds() float64 {
	return time.Duration(d).Seconds()
}

// Milliseconds returns the duration as an integer number of milliseconds.
func (d Duration) Milliseconds() int64 {
	return time.Duration(d).Milliseconds()
}
