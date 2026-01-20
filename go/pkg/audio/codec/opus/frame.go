package opus

import (
	"slices"
	"time"
)

// Frame represents a raw Opus encoded frame.
type Frame []byte

// Duration returns the total duration of audio contained in this frame.
func (f Frame) Duration() time.Duration {
	if len(f) == 0 {
		return 0
	}
	toc := f.TOC()
	fd := toc.Configuration().FrameDuration()
	switch toc.FrameCode() {
	case OneFrame:
		return fd.Duration()
	case TwoEqualFrames:
		return fd.Duration() * 2
	case TwoDifferentFrames:
		return fd.Duration() * 2
	case ArbitraryFrames:
		if len(f) < 2 {
			return 0
		}
		frameCount := f[1] & 0b00111111
		return fd.Duration() * time.Duration(frameCount)
	}
	return 0
}

// TOC returns the TOC byte of this frame.
func (f Frame) TOC() TOC {
	if len(f) == 0 {
		return 0
	}
	return TOC(f[0])
}

// Clone returns a copy of the frame.
func (f Frame) Clone() Frame {
	return slices.Clone(f)
}

// IsStereo returns true if this frame contains stereo audio.
func (f Frame) IsStereo() bool {
	return f.TOC().IsStereo()
}

// Configuration returns the configuration of this frame.
func (f Frame) Configuration() Configuration {
	return f.TOC().Configuration()
}

// Mode returns the mode (SILK, CELT, or Hybrid) of this frame.
func (f Frame) Mode() ConfigurationMode {
	return f.Configuration().Mode()
}

// Bandwidth returns the bandwidth of this frame.
func (f Frame) Bandwidth() Bandwidth {
	return f.Configuration().Bandwidth()
}

// Samples returns the number of samples in this frame.
func (f Frame) Samples() int {
	return f.Configuration().Samples()
}
