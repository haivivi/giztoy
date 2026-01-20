package opus

// For go build: use pkg-config to find system libopus
// For bazel build: cdeps provides opus headers and library

/*
#cgo pkg-config: opus
#include <opus.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Decoder wraps an Opus decoder.
type Decoder struct {
	sampleRate int
	channels   int
	cDec       *C.OpusDecoder
}

// NewDecoder creates a new Opus decoder.
//
// Parameters:
//   - sampleRate: Sample rate to decode at (8000, 12000, 16000, 24000, or 48000)
//   - channels: Number of channels (1 or 2)
func NewDecoder(sampleRate, channels int) (*Decoder, error) {
	var err C.int
	cDec := C.opus_decoder_create(C.opus_int32(sampleRate), C.int(channels), &err)
	if err != C.OPUS_OK {
		return nil, fmt.Errorf("opus: decoder create failed: %s", C.GoString(C.opus_strerror(err)))
	}
	return &Decoder{
		sampleRate: sampleRate,
		channels:   channels,
		cDec:       cDec,
	}, nil
}

// Close releases the decoder resources.
func (d *Decoder) Close() {
	if d.cDec != nil {
		C.opus_decoder_destroy(d.cDec)
		d.cDec = nil
	}
}

// Decode decodes an Opus frame to PCM samples.
// Returns the decoded PCM data as bytes (int16 samples, little-endian).
func (d *Decoder) Decode(f Frame) ([]byte, error) {
	if d.cDec == nil {
		return nil, fmt.Errorf("opus: decoder is closed")
	}

	// Max frame size: 120ms at 48kHz stereo = 5760 samples * 2 channels
	maxSamples := 5760 * d.channels
	buf := make([]int16, maxSamples)

	var dataPtr *C.uchar
	var dataLen C.opus_int32
	if len(f) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&f[0]))
		dataLen = C.opus_int32(len(f))
	}

	n := C.opus_decode(d.cDec, dataPtr, dataLen,
		(*C.opus_int16)(unsafe.Pointer(&buf[0])), C.int(maxSamples/d.channels), 0)
	if n < 0 {
		return nil, fmt.Errorf("opus: decode failed: %s", C.GoString(C.opus_strerror(n)))
	}

	return unsafe.Slice((*byte)(unsafe.Pointer(&buf[0])), 2*int(n)*d.channels), nil
}

// DecodeTo decodes an Opus frame to the provided PCM buffer.
// The buffer should be large enough to hold the decoded samples.
// Returns the number of samples per channel decoded.
func (d *Decoder) DecodeTo(f Frame, buf []int16) (int, error) {
	if d.cDec == nil {
		return 0, fmt.Errorf("opus: decoder is closed")
	}

	var dataPtr *C.uchar
	var dataLen C.opus_int32
	if len(f) > 0 {
		dataPtr = (*C.uchar)(unsafe.Pointer(&f[0]))
		dataLen = C.opus_int32(len(f))
	}

	n := C.opus_decode(d.cDec, dataPtr, dataLen,
		(*C.opus_int16)(unsafe.Pointer(&buf[0])), C.int(len(buf)/d.channels), 0)
	if n < 0 {
		return 0, fmt.Errorf("opus: decode failed: %s", C.GoString(C.opus_strerror(n)))
	}
	return int(n), nil
}

// DecodePLC performs packet loss concealment (PLC) to generate samples
// when a packet is lost.
func (d *Decoder) DecodePLC(samples int) ([]byte, error) {
	if d.cDec == nil {
		return nil, fmt.Errorf("opus: decoder is closed")
	}

	buf := make([]int16, samples*d.channels)
	n := C.opus_decode(d.cDec, nil, 0,
		(*C.opus_int16)(unsafe.Pointer(&buf[0])), C.int(samples), 0)
	if n < 0 {
		return nil, fmt.Errorf("opus: PLC decode failed: %s", C.GoString(C.opus_strerror(n)))
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&buf[0])), 2*int(n)*d.channels), nil
}

// SampleRate returns the sample rate of this decoder.
func (d *Decoder) SampleRate() int {
	return d.sampleRate
}

// Channels returns the number of channels of this decoder.
func (d *Decoder) Channels() int {
	return d.channels
}
