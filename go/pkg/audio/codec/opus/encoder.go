package opus

// For go build: use pkg-config to find system libopus
// For bazel build: cdeps provides opus headers and library

/*
#cgo pkg-config: opus
#include <opus.h>
#include <stdlib.h>

// Wrapper functions for variadic opus_encoder_ctl
static int opus_encoder_set_bitrate(OpusEncoder *enc, opus_int32 bitrate) {
    return opus_encoder_ctl(enc, OPUS_SET_BITRATE(bitrate));
}

static int opus_encoder_set_complexity(OpusEncoder *enc, opus_int32 complexity) {
    return opus_encoder_ctl(enc, OPUS_SET_COMPLEXITY(complexity));
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Application type constants for encoder initialization.
const (
	// ApplicationVoIP gives best quality at a given bitrate for voice signals.
	ApplicationVoIP = int(C.OPUS_APPLICATION_VOIP)

	// ApplicationAudio gives best quality at a given bitrate for most non-voice signals.
	ApplicationAudio = int(C.OPUS_APPLICATION_AUDIO)

	// ApplicationRestrictedLowdelay configures the minimum possible coding delay.
	ApplicationRestrictedLowdelay = int(C.OPUS_APPLICATION_RESTRICTED_LOWDELAY)
)

// Encoder wraps an Opus encoder.
type Encoder struct {
	sampleRate int
	channels   int
	cEnc       *C.OpusEncoder
}

// NewEncoder creates a new Opus encoder.
//
// Parameters:
//   - sampleRate: Sample rate of input signal (8000, 12000, 16000, 24000, or 48000)
//   - channels: Number of channels (1 or 2)
//   - application: Intended application type (ApplicationVoIP, ApplicationAudio, etc.)
func NewEncoder(sampleRate, channels, application int) (*Encoder, error) {
	var err C.int
	cEnc := C.opus_encoder_create(C.opus_int32(sampleRate), C.int(channels), C.int(application), &err)
	if err != C.OPUS_OK {
		return nil, fmt.Errorf("opus: encoder create failed: %s", C.GoString(C.opus_strerror(err)))
	}
	return &Encoder{
		sampleRate: sampleRate,
		channels:   channels,
		cEnc:       cEnc,
	}, nil
}

// NewVoIPEncoder creates a new Opus encoder optimized for voice.
func NewVoIPEncoder(sampleRate, channels int) (*Encoder, error) {
	return NewEncoder(sampleRate, channels, ApplicationVoIP)
}

// NewAudioEncoder creates a new Opus encoder optimized for music/audio.
func NewAudioEncoder(sampleRate, channels int) (*Encoder, error) {
	return NewEncoder(sampleRate, channels, ApplicationAudio)
}

// Close releases the encoder resources.
func (e *Encoder) Close() {
	if e.cEnc != nil {
		C.opus_encoder_destroy(e.cEnc)
		e.cEnc = nil
	}
}

// Encode encodes PCM samples to an Opus frame.
//
// Parameters:
//   - pcm: Input PCM samples as int16 slice. Must contain frameSize*channels samples.
//   - frameSize: Number of samples per channel in the input signal.
//
// Returns the encoded Opus frame.
func (e *Encoder) Encode(pcm []int16, frameSize int) (Frame, error) {
	if e.cEnc == nil {
		return nil, fmt.Errorf("opus: encoder is closed")
	}

	// Max Opus frame size
	buf := make([]byte, 4000)
	n := C.opus_encode(e.cEnc,
		(*C.opus_int16)(unsafe.Pointer(&pcm[0])), C.int(frameSize),
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.opus_int32(len(buf)))
	if n < 0 {
		return nil, fmt.Errorf("opus: encode failed: %s", C.GoString(C.opus_strerror(n)))
	}
	return buf[:n], nil
}

// EncodeBytes encodes PCM samples from a byte slice to an Opus frame.
// The input should be int16 samples in little-endian format.
func (e *Encoder) EncodeBytes(pcm []byte, frameSize int) (Frame, error) {
	samples := unsafe.Slice((*int16)(unsafe.Pointer(&pcm[0])), len(pcm)/2)
	return e.Encode(samples, frameSize)
}

// EncodeTo encodes PCM samples to the provided buffer.
// Returns the number of bytes written.
func (e *Encoder) EncodeTo(pcm []int16, frameSize int, buf []byte) (int, error) {
	if e.cEnc == nil {
		return 0, fmt.Errorf("opus: encoder is closed")
	}

	n := C.opus_encode(e.cEnc,
		(*C.opus_int16)(unsafe.Pointer(&pcm[0])), C.int(frameSize),
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.opus_int32(len(buf)))
	if n < 0 {
		return 0, fmt.Errorf("opus: encode failed: %s", C.GoString(C.opus_strerror(n)))
	}
	return int(n), nil
}

// SampleRate returns the sample rate of this encoder.
func (e *Encoder) SampleRate() int {
	return e.sampleRate
}

// Channels returns the number of channels of this encoder.
func (e *Encoder) Channels() int {
	return e.channels
}

// SetBitrate sets the target bitrate in bits per second.
func (e *Encoder) SetBitrate(bitrate int) error {
	if e.cEnc == nil {
		return fmt.Errorf("opus: encoder is closed")
	}
	ret := C.opus_encoder_set_bitrate(e.cEnc, C.opus_int32(bitrate))
	if ret != C.OPUS_OK {
		return fmt.Errorf("opus: set bitrate failed: %s", C.GoString(C.opus_strerror(ret)))
	}
	return nil
}

// SetComplexity sets the encoder's computational complexity (0-10).
func (e *Encoder) SetComplexity(complexity int) error {
	if e.cEnc == nil {
		return fmt.Errorf("opus: encoder is closed")
	}
	ret := C.opus_encoder_set_complexity(e.cEnc, C.opus_int32(complexity))
	if ret != C.OPUS_OK {
		return fmt.Errorf("opus: set complexity failed: %s", C.GoString(C.opus_strerror(ret)))
	}
	return nil
}

// FrameSizeForDuration returns the frame size (samples per channel) for a given duration.
func (e *Encoder) FrameSizeForDuration(fd FrameDuration) int {
	return e.sampleRate * int(fd.Millis()) / 1000
}

// FrameSize20ms returns the frame size for 20ms frames (recommended default).
func (e *Encoder) FrameSize20ms() int {
	return e.sampleRate * 20 / 1000
}
