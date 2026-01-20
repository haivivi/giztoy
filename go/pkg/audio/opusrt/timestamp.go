package opusrt

import "time"

// EpochMillis represents a timestamp in milliseconds since Unix epoch.
type EpochMillis int64

// Now returns the current time as EpochMillis.
func Now() EpochMillis {
	return EpochMillis(time.Now().UnixMilli())
}

// FromTime converts a time.Time to EpochMillis.
func FromTime(t time.Time) EpochMillis {
	return EpochMillis(t.UnixMilli())
}

// FromDuration converts a duration to EpochMillis (milliseconds).
func FromDuration(d time.Duration) EpochMillis {
	return EpochMillis(d.Milliseconds())
}

// Duration converts EpochMillis to time.Duration.
func (ms EpochMillis) Duration() time.Duration {
	return time.Duration(ms) * time.Millisecond
}

// Time converts EpochMillis to time.Time.
func (ms EpochMillis) Time() time.Time {
	return time.Unix(0, int64(ms)*int64(time.Millisecond))
}

// Add returns ms + d.
func (ms EpochMillis) Add(d time.Duration) EpochMillis {
	return ms + FromDuration(d)
}

// Sub returns the duration ms - other.
func (ms EpochMillis) Sub(other EpochMillis) time.Duration {
	return time.Duration(ms-other) * time.Millisecond
}
