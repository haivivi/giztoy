// Package opus provides Opus audio codec encoding and decoding.
//
// This package implements the Opus codec specification as defined in RFC 6716.
// It provides TOC (Table of Contents) parsing, frame handling, and encoding/decoding
// capabilities using libopus via CGO.
package opus

import (
	"fmt"
	"time"
)

type (
	// TOC represents the table-of-contents (TOC) header that signals which of the
	// various modes and configurations a given packet uses. It is composed
	// of a configuration number, "config", a stereo flag, "s", and a frame
	// count code, "c", arranged as illustrated:
	//
	//            0 1 2 3 4 5 6 7
	//           +-+-+-+-+-+-+-+-+
	//           | config  |s| c |
	//           +-+-+-+-+-+-+-+-+
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
	TOC byte

	// Configuration numbers in each range (e.g., 0...3 for NB SILK-
	// only) correspond to the various choices of frame size, in the same
	// order. For example, configuration 0 has a 10 ms frame size and
	// configuration 3 has a 60 ms frame size.
	//
	// +-----------------------+-----------+-----------+-------------------+
	// | Configuration         | Mode      | Bandwidth | Frame Sizes       |
	// | Number(s)             |           |           |                   |
	// +-----------------------+-----------+-----------+-------------------+
	// | 0...3                 | SILK-only | NB        | 10, 20, 40, 60 ms |
	// | 4...7                 | SILK-only | MB        | 10, 20, 40, 60 ms |
	// | 8...11                | SILK-only | WB        | 10, 20, 40, 60 ms |
	// | 12...13               | Hybrid    | SWB       | 10, 20 ms         |
	// | 14...15               | Hybrid    | FB        | 10, 20 ms         |
	// | 16...19               | CELT-only | NB        | 2.5, 5, 10, 20 ms |
	// | 20...23               | CELT-only | WB        | 2.5, 5, 10, 20 ms |
	// | 24...27               | CELT-only | SWB       | 2.5, 5, 10, 20 ms |
	// | 28...31               | CELT-only | FB        | 2.5, 5, 10, 20 ms |
	// +-----------------------+-----------+-----------+-------------------+
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
	Configuration byte

	// ConfigurationMode represents the operating mode of the Opus codec.
	// The LP (SILK) layer and MDCT (CELT) layer can be combined in three
	// possible operating modes:
	//
	// 1. A SILK-only mode for use in low bitrate connections with an audio
	//    bandwidth of WB or less,
	//
	// 2. A Hybrid (SILK+CELT) mode for SWB or FB speech at medium bitrates, and
	//
	// 3. A CELT-only mode for very low delay speech transmission as well
	//    as music transmission (NB to FB).
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
	ConfigurationMode byte

	// FrameDuration represents the duration of an Opus frame.
	// Opus can encode frames of 2.5, 5, 10, 20, 40, or 60 ms.
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-2.1.4
	FrameDuration byte

	// Bandwidth represents the audio bandwidth of an Opus stream.
	// The codec allows input and output of various audio bandwidths.
	//
	// +----------------------+-----------------+-------------------------+
	// | Abbreviation         | Audio Bandwidth | Sample Rate (Effective) |
	// +----------------------+-----------------+-------------------------+
	// | NB (narrowband)      |           4 kHz |                   8 kHz |
	// | MB (medium-band)     |           6 kHz |                  12 kHz |
	// | WB (wideband)        |           8 kHz |                  16 kHz |
	// | SWB (super-wideband) |          12 kHz |                  24 kHz |
	// | FB (fullband)        |      20 kHz (*) |                  48 kHz |
	// +----------------------+-----------------+-------------------------+
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-2
	Bandwidth byte

	// FrameCode represents the number of frames per packet (codes 0 to 3):
	//
	// - 0: 1 frame in the packet
	// - 1: 2 frames in the packet, each with equal compressed size
	// - 2: 2 frames in the packet, with different compressed sizes
	// - 3: an arbitrary number of frames in the packet
	//
	// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
	FrameCode byte
)

// Configuration returns the configuration number from the TOC byte.
func (t TOC) Configuration() Configuration {
	return Configuration(t >> 3)
}

// IsStereo returns true if the TOC indicates stereo audio.
func (t TOC) IsStereo() bool {
	return (t & 0b00000100) != 0
}

// FrameCode returns the frame count code from the TOC byte.
func (t TOC) FrameCode() FrameCode {
	return FrameCode(t & 0b00000011)
}

// String returns a human-readable representation of the TOC.
func (t TOC) String() string {
	return fmt.Sprintf(
		"opus_toc: stereo=%v, mode=%s, bw=%s, %s, %s",
		t.IsStereo(),
		t.Configuration().Mode(),
		t.Configuration().Bandwidth(),
		t.FrameCode(),
		t.Configuration().FrameDuration(),
	)
}

// Frame code constants.
const (
	OneFrame FrameCode = iota
	TwoEqualFrames
	TwoDifferentFrames
	ArbitraryFrames
)

// String returns a human-readable representation of the FrameCode.
func (c FrameCode) String() string {
	switch c {
	case OneFrame:
		return "One Frame"
	case TwoEqualFrames:
		return "Two Equal Frames"
	case TwoDifferentFrames:
		return "Two Different Frames"
	case ArbitraryFrames:
		return "Arbitrary Frames"
	}
	return "Invalid Frame Code"
}

// Configuration mode constants.
const (
	Silk ConfigurationMode = iota + 1
	CELT
	Hybrid
)

// String returns a human-readable representation of the ConfigurationMode.
func (c ConfigurationMode) String() string {
	switch c {
	case Silk:
		return "Silk"
	case CELT:
		return "CELT"
	case Hybrid:
		return "Hybrid"
	}
	return "Invalid Configuration Mode"
}

// Mode returns the configuration mode (SILK, CELT, or Hybrid) for this configuration.
// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
func (c Configuration) Mode() ConfigurationMode {
	switch {
	case c <= 11:
		return Silk
	case c >= 12 && c <= 15:
		return Hybrid
	case c >= 16 && c <= 31:
		return CELT
	default:
		return 0
	}
}

// Frame duration constants.
const (
	Duration2500us FrameDuration = iota + 1
	Duration5ms
	Duration10ms
	Duration20ms
	Duration40ms
	Duration60ms
)

// String returns a human-readable representation of the FrameDuration.
func (f FrameDuration) String() string {
	switch f {
	case Duration2500us:
		return "2.5ms"
	case Duration5ms:
		return "5ms"
	case Duration10ms:
		return "10ms"
	case Duration20ms:
		return "20ms"
	case Duration40ms:
		return "40ms"
	case Duration60ms:
		return "60ms"
	}
	return "Invalid Frame Duration"
}

// Millis returns the duration in milliseconds.
func (f FrameDuration) Millis() int64 {
	switch f {
	case Duration2500us:
		return 2
	case Duration5ms:
		return 5
	case Duration10ms:
		return 10
	case Duration20ms:
		return 20
	case Duration40ms:
		return 40
	case Duration60ms:
		return 60
	}
	return 0
}

// Duration returns the duration as a time.Duration.
func (f FrameDuration) Duration() time.Duration {
	switch f {
	case Duration2500us:
		return 2500 * time.Microsecond
	case Duration5ms:
		return 5 * time.Millisecond
	case Duration10ms:
		return 10 * time.Millisecond
	case Duration20ms:
		return 20 * time.Millisecond
	case Duration40ms:
		return 40 * time.Millisecond
	case Duration60ms:
		return 60 * time.Millisecond
	}
	return 0
}

// FrameDuration returns the frame duration for this configuration.
// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
func (c Configuration) FrameDuration() FrameDuration {
	switch c {
	case 16, 20, 24, 28:
		return Duration2500us
	case 17, 21, 25, 29:
		return Duration5ms
	case 0, 4, 8, 12, 14, 18, 22, 26, 30:
		return Duration10ms
	case 1, 5, 9, 13, 15, 19, 23, 27, 31:
		return Duration20ms
	case 2, 6, 10:
		return Duration40ms
	case 3, 7, 11:
		return Duration60ms
	}
	return 0
}

// PageGranuleIncrement returns the granule position increment for OGG pages.
func (c Configuration) PageGranuleIncrement() int {
	switch c {
	case 16, 20, 24, 28:
		return 120
	case 17, 21, 25, 29:
		return 240
	case 0, 4, 8, 12, 14, 18, 22, 26, 30:
		return 480
	case 1, 5, 9, 13, 15, 19, 23, 27, 31:
		return 960
	case 2, 6, 10:
		return 1920
	case 3, 7, 11:
		return 2880
	}
	return 0
}

// Bandwidth constants.
const (
	// NB (narrowband) is 4 kHz audio bandwidth, 8 kHz sample rate
	NB Bandwidth = iota + 1
	// MB (medium-band) is 6 kHz audio bandwidth, 12 kHz sample rate
	MB
	// WB (wideband) is 8 kHz audio bandwidth, 16 kHz sample rate
	WB
	// SWB (super-wideband) is 12 kHz audio bandwidth, 24 kHz sample rate
	SWB
	// FB (fullband) is 20 kHz audio bandwidth, 48 kHz sample rate
	FB
)

// Bandwidth returns the audio bandwidth for this configuration.
// https://datatracker.ietf.org/doc/html/rfc6716#section-3.1
func (c Configuration) Bandwidth() Bandwidth {
	switch {
	case c <= 3:
		return NB
	case c <= 7:
		return MB
	case c <= 11:
		return WB
	case c <= 13:
		return SWB
	case c <= 15:
		return FB
	case c <= 19:
		return NB
	case c <= 23:
		return WB
	case c <= 27:
		return SWB
	case c <= 31:
		return FB
	}
	return 0
}

// samples maps configuration numbers to sample counts.
var samples = [32]int{
	/* Silk   NB   0...3 */ 80, 160, 320, 480,
	/* Silk   MB   4...7 */ 120, 240, 480, 720,
	/* Silk   WB   8..11 */ 160, 320, 640, 960,
	/* Hybrid SWB 12..13 */ 240, 480,
	/* Hybrid FB  14..15 */ 480, 960,
	/* CELT   NB  16..19 */ 20, 40, 80, 120,
	/* CELT   WB  20..23 */ 40, 80, 160, 240,
	/* CELT   SWB 24..27 */ 60, 120, 240, 480,
	/* CELT   SWB 28..31 */ 120, 240, 480, 960,
}

// Samples returns the number of samples for this configuration.
func (c Configuration) Samples() int {
	if c > 31 {
		return 0
	}
	return samples[c]
}

// String returns a human-readable representation of the Bandwidth.
func (b Bandwidth) String() string {
	switch b {
	case NB:
		return "Narrowband"
	case MB:
		return "Mediumband"
	case WB:
		return "Wideband"
	case SWB:
		return "Superwideband"
	case FB:
		return "Fullband"
	}
	return "Invalid Bandwidth"
}

// SampleRate returns the effective sample rate for this bandwidth.
func (b Bandwidth) SampleRate() int {
	switch b {
	case NB:
		return 8000
	case MB:
		return 12000
	case WB:
		return 16000
	case SWB:
		return 24000
	case FB:
		return 48000
	}
	return 0
}

// ParseFrameCountByte parses the frame count byte following the TOC byte
// for packets with arbitrary frame counts (code 3).
//
//	    0
//	    0 1 2 3 4 5 6 7
//	   +-+-+-+-+-+-+-+-+
//	   |v|p|     M     |
//	   +-+-+-+-+-+-+-+-+
//
// https://datatracker.ietf.org/doc/html/rfc6716#section-3.2.5
func ParseFrameCountByte(in byte) (isVBR, hasPadding bool, frameCount byte) {
	isVBR = (in & 0b10000000) != 0
	hasPadding = (in & 0b01000000) != 0
	frameCount = in & 0b00111111
	return
}
