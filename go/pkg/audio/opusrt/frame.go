package opusrt

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/codec/opus"
)

// Frame represents a raw Opus frame.
type Frame []byte

// Duration returns the duration of this Opus frame based on its TOC byte.
func (f Frame) Duration() time.Duration {
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
func (f Frame) TOC() opus.TOC {
	if len(f) == 0 {
		return 0
	}
	return opus.TOC(f[0])
}

// Clone returns a copy of this frame.
func (f Frame) Clone() Frame {
	return slices.Clone(f)
}

// IsStereo returns true if this frame is stereo.
func (f Frame) IsStereo() bool {
	return f.TOC().IsStereo()
}

// FrameReader is the interface for reading Opus frames.
type FrameReader interface {
	// Frame returns the next frame, its duration (or loss duration), and any error.
	// If loss > 0, the frame is nil and loss indicates the duration of lost data.
	Frame() (frame Frame, loss time.Duration, err error)
}

// StampedFrame format:
//
//	+--------+------------------+------------------+
//	| Version| Timestamp (7B)   | Opus Frame Data  |
//	| (1B)   | Big-endian ms    |                  |
//	+--------+------------------+------------------+
//
// Total header: 8 bytes
const (
	// FrameVersion is the current stamped frame format version.
	FrameVersion = 1

	// StampedHeaderSize is the size of the stamped frame header.
	StampedHeaderSize = 8
)

// StampedFrame is an Opus frame with an embedded timestamp.
type StampedFrame []byte

// Frame returns the Opus frame data (without the timestamp header).
func (sf StampedFrame) Frame() Frame {
	if len(sf) < StampedHeaderSize {
		return nil
	}
	return Frame(sf[StampedHeaderSize:])
}

// Version returns the format version byte.
func (sf StampedFrame) Version() int {
	if len(sf) == 0 {
		return 0
	}
	return int(sf[0])
}

// Stamp returns the timestamp embedded in this frame.
func (sf StampedFrame) Stamp() EpochMillis {
	if len(sf) < StampedHeaderSize {
		return 0
	}
	var buf [8]byte
	copy(buf[1:], sf[1:8])
	return EpochMillis(binary.BigEndian.Uint64(buf[:]))
}

// Duration returns the duration of the embedded Opus frame.
func (sf StampedFrame) Duration() time.Duration {
	return sf.Frame().Duration()
}

// FromStamped extracts the frame and timestamp from stamped data.
// Returns ok=false if the data is invalid.
func FromStamped(b []byte) (frame Frame, ts EpochMillis, ok bool) {
	if len(b) < StampedHeaderSize {
		return nil, 0, false
	}
	if b[0] != FrameVersion {
		return nil, 0, false
	}
	var buf [8]byte
	copy(buf[1:], b[1:8])
	ts = EpochMillis(binary.BigEndian.Uint64(buf[:]))
	frame = Frame(b[StampedHeaderSize:])
	if len(frame) < 1 {
		return nil, 0, false
	}
	return frame, ts, true
}

// Stamp creates a stamped frame from a frame and timestamp.
// Returns a new byte slice containing the stamped frame.
func Stamp(frame Frame, stamp EpochMillis) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(stamp))
	buf[0] = FrameVersion
	return append(buf[:], frame...)
}

// StampTo writes a stamped frame to dst.
// Panics if dst is too small.
func StampTo(dst []byte, frame Frame, stamp EpochMillis) []byte {
	if len(dst) < len(frame)+StampedHeaderSize {
		panic("opusrt: dst buffer too small")
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(stamp))
	buf[0] = FrameVersion
	copy(dst, buf[:])
	copy(dst[StampedHeaderSize:], frame)
	return dst[:len(frame)+StampedHeaderSize]
}
