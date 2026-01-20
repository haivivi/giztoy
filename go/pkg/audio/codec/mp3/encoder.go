package mp3

/*
#cgo darwin CFLAGS: -I/opt/homebrew/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -lmp3lame
#cgo linux pkg-config: mp3lame
#include <lame/lame.h>
#include <stdlib.h>

// Wrapper to handle lame_encode_buffer_interleaved with proper typing
static int lame_encode_interleaved(lame_global_flags* gf, const short* pcm, int num_samples, unsigned char* mp3buf, int mp3buf_size) {
    return lame_encode_buffer_interleaved(gf, (short*)pcm, num_samples, mp3buf, mp3buf_size);
}
*/
import "C"
import (
	"errors"
	"io"
	"sync"
	"unsafe"
)

// Quality presets for encoding
type Quality int

const (
	QualityBest   Quality = 0 // ~245 kbps
	QualityHigh   Quality = 2 // ~190 kbps
	QualityMedium Quality = 5 // ~130 kbps
	QualityLow    Quality = 7 // ~100 kbps
	QualityWorst  Quality = 9 // ~65 kbps
)

// Encoder encodes PCM audio to MP3.
type Encoder struct {
	w io.Writer

	mu         sync.Mutex
	lame       *C.lame_global_flags // Use pointer for incomplete C type
	sampleRate int
	channels   int
	quality    Quality
	bitrate    int // 0 = use VBR quality
	inited     bool
	closed     bool

	// Output buffer for encoded MP3 data
	mp3buf []byte
}

// EncoderOption configures the encoder.
type EncoderOption func(*Encoder)

// WithQuality sets the VBR quality (0=best, 9=worst).
func WithQuality(q Quality) EncoderOption {
	return func(e *Encoder) {
		e.quality = q
	}
}

// WithBitrate sets constant bitrate mode (in kbps).
// Common values: 128, 192, 256, 320
func WithBitrate(kbps int) EncoderOption {
	return func(e *Encoder) {
		e.bitrate = kbps
	}
}

// NewEncoder creates a new MP3 encoder writing to w.
//
// Parameters:
//   - w: Output writer for encoded MP3 data
//   - sampleRate: Input sample rate (e.g., 44100, 48000)
//   - channels: Number of input channels (1 or 2)
//   - opts: Optional encoder settings
func NewEncoder(w io.Writer, sampleRate, channels int, opts ...EncoderOption) (*Encoder, error) {
	if channels != 1 && channels != 2 {
		return nil, errors.New("mp3: channels must be 1 or 2")
	}

	e := &Encoder{
		w:          w,
		sampleRate: sampleRate,
		channels:   channels,
		quality:    QualityMedium,
		mp3buf:     make([]byte, 8192), // 8KB output buffer
	}

	for _, opt := range opts {
		opt(e)
	}

	return e, nil
}

// init initializes LAME encoder (called on first write)
func (e *Encoder) init() error {
	if e.inited {
		return nil
	}

	lame := C.lame_init()
	if lame == nil {
		return errors.New("mp3: failed to initialize LAME")
	}

	C.lame_set_in_samplerate(lame, C.int(e.sampleRate))
	C.lame_set_num_channels(lame, C.int(e.channels))

	if e.channels == 1 {
		C.lame_set_mode(lame, C.MONO)
	} else {
		C.lame_set_mode(lame, C.JOINT_STEREO)
	}

	if e.bitrate > 0 {
		// Constant bitrate mode
		C.lame_set_VBR(lame, C.vbr_off)
		C.lame_set_brate(lame, C.int(e.bitrate))
	} else {
		// VBR mode
		C.lame_set_VBR(lame, C.vbr_default)
		C.lame_set_VBR_quality(lame, C.float(e.quality))
	}

	if C.lame_init_params(lame) < 0 {
		C.lame_close(lame)
		return errors.New("mp3: failed to set LAME parameters")
	}

	e.lame = lame
	e.inited = true
	return nil
}

// Write encodes PCM samples to MP3.
// Input should be interleaved int16 samples, little-endian.
func (e *Encoder) Write(pcm []byte) (n int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return 0, errors.New("mp3: encoder is closed")
	}

	if err := e.init(); err != nil {
		return 0, err
	}

	// Convert bytes to samples
	numSamples := len(pcm) / (2 * e.channels) // samples per channel
	if numSamples == 0 {
		return len(pcm), nil
	}

	// Ensure mp3buf is large enough
	// LAME recommends 1.25*num_samples + 7200
	requiredSize := numSamples*5/4 + 7200
	if len(e.mp3buf) < requiredSize {
		e.mp3buf = make([]byte, requiredSize)
	}

	// Encode
	var encoded C.int
	if e.channels == 2 {
		encoded = C.lame_encode_interleaved(
			e.lame,
			(*C.short)(unsafe.Pointer(&pcm[0])),
			C.int(numSamples),
			(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
			C.int(len(e.mp3buf)),
		)
	} else {
		// Mono: use left channel buffer
		encoded = C.lame_encode_buffer(
			e.lame,
			(*C.short)(unsafe.Pointer(&pcm[0])),
			nil,
			C.int(numSamples),
			(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
			C.int(len(e.mp3buf)),
		)
	}

	if encoded < 0 {
		return 0, errors.New("mp3: encode failed")
	}

	if encoded > 0 {
		_, err = e.w.Write(e.mp3buf[:encoded])
		if err != nil {
			return 0, err
		}
	}

	return len(pcm), nil
}

// Flush flushes any remaining encoded data.
// Must be called before Close to ensure all data is written.
func (e *Encoder) Flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.inited || e.closed {
		return nil
	}

	// Flush LAME's internal buffers
	encoded := C.lame_encode_flush(
		e.lame,
		(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
		C.int(len(e.mp3buf)),
	)

	if encoded > 0 {
		_, err := e.w.Write(e.mp3buf[:encoded])
		if err != nil {
			return err
		}
	}

	return nil
}

// Close releases encoder resources.
// Call Flush() before Close() to ensure all data is written.
func (e *Encoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	if e.inited && e.lame != nil {
		C.lame_close(e.lame)
		e.lame = nil
	}

	return nil
}

// EncodePCMStream is a convenience function to encode an entire PCM stream.
// The stream is read from pcm and encoded MP3 is written to w.
func EncodePCMStream(w io.Writer, pcm io.Reader, sampleRate, channels int, opts ...EncoderOption) (written int64, err error) {
	enc, err := NewEncoder(w, sampleRate, channels, opts...)
	if err != nil {
		return 0, err
	}
	defer enc.Close()

	buf := make([]byte, 4096)
	for {
		n, err := pcm.Read(buf)
		if n > 0 {
			wn, werr := enc.Write(buf[:n])
			written += int64(wn)
			if werr != nil {
				return written, werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}

	if err := enc.Flush(); err != nil {
		return written, err
	}

	return written, nil
}
