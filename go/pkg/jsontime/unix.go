package jsontime

import (
	"encoding/json"
	"time"
)

// Unix is a time.Time that serializes to/from Unix seconds in JSON.
type Unix time.Time

// NowEpoch returns the current time as Unix.
func NowEpoch() Unix {
	return Unix(time.Now())
}

// Time returns the underlying time.Time value.
func (ep Unix) Time() time.Time {
	return time.Time(ep)
}

// Before reports whether ep is before t.
func (ep Unix) Before(t Unix) bool {
	return time.Time(ep).Before(time.Time(t))
}

// After reports whether ep is after t.
func (ep Unix) After(t Unix) bool {
	return time.Time(ep).After(time.Time(t))
}

// Equal reports whether ep and t represent the same time instant.
func (ep Unix) Equal(t Unix) bool {
	return time.Time(ep).Equal(time.Time(t))
}

// UnmarshalJSON implements json.Unmarshaler.
func (ep *Unix) UnmarshalJSON(b []byte) error {
	var t int64
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*ep = Unix(time.Unix(t, 0))
	return nil
}

// MarshalJSON implements json.Marshaler.
func (ep Unix) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ep).Unix())
}

// String returns the time formatted as a string.
func (ep Unix) String() string {
	return time.Time(ep).String()
}

// IsZero reports whether ep represents the zero time instant.
func (ep Unix) IsZero() bool {
	return time.Time(ep).IsZero()
}

// Sub returns the duration ep-t.
func (ep Unix) Sub(t Unix) time.Duration {
	return time.Time(ep).Sub(time.Time(t))
}

// Add returns the time ep+d.
func (ep Unix) Add(d time.Duration) Unix {
	return Unix(time.Time(ep).Add(d))
}
