// Package jsontime provides JSON-serializable time types.
package jsontime

import (
	"encoding/json"
	"time"
)

// Milli is a time.Time that serializes to/from Unix milliseconds in JSON.
type Milli time.Time

// NowEpochMilli returns the current time as Milli.
func NowEpochMilli() Milli {
	return Milli(time.Now())
}

// Time returns the underlying time.Time value.
func (ep Milli) Time() time.Time {
	return time.Time(ep)
}

// Before reports whether ep is before t.
func (ep Milli) Before(t Milli) bool {
	return time.Time(ep).Before(time.Time(t))
}

// After reports whether ep is after t.
func (ep Milli) After(t Milli) bool {
	return time.Time(ep).After(time.Time(t))
}

// Equal reports whether ep and t represent the same time instant.
func (ep Milli) Equal(t Milli) bool {
	return time.Time(ep).Equal(time.Time(t))
}

// String returns the time formatted as a string.
func (ep Milli) String() string {
	return time.Time(ep).String()
}

// UnmarshalJSON implements json.Unmarshaler.
func (ep *Milli) UnmarshalJSON(b []byte) error {
	var t int64
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*ep = Milli(time.UnixMilli(t))
	return nil
}

// MarshalJSON implements json.Marshaler.
func (ep Milli) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ep).UnixMilli())
}

// IsZero reports whether ep represents the zero time instant.
func (ep Milli) IsZero() bool {
	return time.Time(ep).IsZero()
}

// Sub returns the duration ep-t.
func (ep Milli) Sub(t Milli) time.Duration {
	return time.Time(ep).Sub(time.Time(t))
}

// Add returns the time ep+d.
func (ep Milli) Add(d time.Duration) Milli {
	return Milli(time.Time(ep).Add(d))
}
