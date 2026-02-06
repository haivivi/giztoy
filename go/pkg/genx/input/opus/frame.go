package opus

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// OpusFrame is a raw Opus frame.
type OpusFrame []byte

// Duration returns the duration of this Opus frame based on its TOC byte.
func (f OpusFrame) Duration() time.Duration {
	if len(f) == 0 {
		return 0
	}
	toc := f.TOC()
	fd := toc.Configuration().FrameDuration()
	switch toc.FrameCode() {
	case opus.OneFrame:
		return fd.Duration()
	case opus.TwoEqualFrames:
		return fd.Duration() * 2
	case opus.TwoDifferentFrames:
		return fd.Duration() * 2
	case opus.ArbitraryFrames:
		if len(f) < 2 {
			return 0
		}
		frameCount := f[1] & 0b00111111
		return fd.Duration() * time.Duration(frameCount)
	}
	return 0
}

// TOC returns the TOC byte of this frame.
func (f OpusFrame) TOC() opus.TOC {
	if len(f) == 0 {
		return 0
	}
	return opus.TOC(f[0])
}

// Clone returns a copy of this frame.
func (f OpusFrame) Clone() OpusFrame {
	return slices.Clone(f)
}

// IsStereo returns true if this frame is stereo.
func (f OpusFrame) IsStereo() bool {
	return f.TOC().IsStereo()
}

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

// StampedFrame holds an Opus frame with its timestamp.
// Implements input.Timestamped[EpochMillis] for use with JitterBuffer.
type StampedFrame struct {
	Frame OpusFrame
	Stamp EpochMillis
}

// Timestamp returns the frame's timestamp for JitterBuffer ordering.
func (sf StampedFrame) Timestamp() EpochMillis {
	return sf.Stamp
}

// Wire format constants
const (
	// FrameVersion is the current stamped frame format version.
	FrameVersion = 1

	// StampedHeaderSize is the size of the stamped frame header (8 bytes).
	StampedHeaderSize = 8
)

// ParseStamped extracts the frame and timestamp from stamped wire data.
// Returns ok=false if the data is invalid.
//
// Wire format:
//
//	[Version(1B) | Timestamp(7B big-endian ms) | OpusFrameData(N)]
func ParseStamped(b []byte) (frame OpusFrame, ts EpochMillis, ok bool) {
	if len(b) < StampedHeaderSize {
		return nil, 0, false
	}
	if b[0] != FrameVersion {
		return nil, 0, false
	}
	var buf [8]byte
	copy(buf[1:], b[1:8])
	ts = EpochMillis(binary.BigEndian.Uint64(buf[:]))
	frame = OpusFrame(b[StampedHeaderSize:])
	if len(frame) < 1 {
		return nil, 0, false
	}
	return frame, ts, true
}

// MakeStamped creates stamped wire data from a frame and timestamp.
// Returns a new byte slice containing the stamped frame.
func MakeStamped(frame OpusFrame, stamp EpochMillis) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(stamp))
	buf[0] = FrameVersion
	return append(buf[:], frame...)
}
