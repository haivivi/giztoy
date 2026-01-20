// Package mp3 provides MP3 audio encoding and decoding.
//
// Decoding uses minimp3, a lightweight single-header MP3 decoder.
// Encoding uses LAME, the high-quality MP3 encoder.
package mp3

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../third_party/minimp3
#cgo darwin CFLAGS: -I/opt/homebrew/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -lmp3lame
#cgo linux pkg-config: mp3lame

#define MINIMP3_IMPLEMENTATION
#include <minimp3.h>
#include <stdlib.h>
#include <string.h>

// Wrapper struct to hold decoder state and frame info
typedef struct {
    mp3dec_t mp3d;
    mp3dec_frame_info_t info;
    int16_t pcm[MINIMP3_MAX_SAMPLES_PER_FRAME];
    int samples;      // samples per channel in current frame
    int sample_pos;   // current position in pcm buffer
} mp3_decoder_t;

static mp3_decoder_t* mp3_decoder_create() {
    mp3_decoder_t* dec = (mp3_decoder_t*)malloc(sizeof(mp3_decoder_t));
    if (dec) {
        mp3dec_init(&dec->mp3d);
        memset(&dec->info, 0, sizeof(dec->info));
        dec->samples = 0;
        dec->sample_pos = 0;
    }
    return dec;
}

static void mp3_decoder_destroy(mp3_decoder_t* dec) {
    if (dec) {
        free(dec);
    }
}

// Decode a frame from input buffer
// Returns: number of samples per channel decoded (0 if need more data or error)
static int mp3_decoder_decode_frame(mp3_decoder_t* dec, const unsigned char* input, int input_size, int* bytes_consumed) {
    dec->samples = mp3dec_decode_frame(&dec->mp3d, input, input_size, dec->pcm, &dec->info);
    dec->sample_pos = 0;
    *bytes_consumed = dec->info.frame_bytes;
    return dec->samples;
}

// Get frame info after decoding
static int mp3_decoder_get_channels(mp3_decoder_t* dec) {
    return dec->info.channels;
}

static int mp3_decoder_get_hz(mp3_decoder_t* dec) {
    return dec->info.hz;
}

static int mp3_decoder_get_bitrate(mp3_decoder_t* dec) {
    return dec->info.bitrate_kbps;
}

// Read PCM samples from decoded frame
// Returns: number of bytes written to output
static int mp3_decoder_read_pcm(mp3_decoder_t* dec, unsigned char* output, int output_size) {
    int channels = dec->info.channels;
    if (channels == 0) channels = 2;

    int available_samples = dec->samples - dec->sample_pos;
    if (available_samples <= 0) return 0;

    int available_bytes = available_samples * channels * 2; // 16-bit samples
    int to_copy = output_size < available_bytes ? output_size : available_bytes;
    to_copy = (to_copy / (channels * 2)) * (channels * 2); // align to sample boundary

    if (to_copy > 0) {
        memcpy(output, ((unsigned char*)dec->pcm) + dec->sample_pos * channels * 2, to_copy);
        dec->sample_pos += to_copy / (channels * 2);
    }
    return to_copy;
}
*/
import "C"
import (
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Decoder decodes MP3 audio to PCM.
// Must call Close() when done to release resources.
type Decoder struct {
	r io.Reader

	mu      sync.Mutex
	dec     *C.mp3_decoder_t
	buf     []byte // input buffer
	bufPos  int    // current position in buf
	bufLen  int    // valid bytes in buf
	initErr error
	inited  bool
	closed  atomic.Bool
	cleanup runtime.Cleanup

	// Audio format (available after first frame decoded)
	sampleRate int
	channels   int
	bitrate    int
}

// freeMP3Decoder releases C resources.
func freeMP3Decoder(ptr uintptr) {
	C.mp3_decoder_destroy((*C.mp3_decoder_t)(unsafe.Pointer(ptr)))
}

// NewDecoder creates a new MP3 decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:   r,
		buf: make([]byte, 16*1024), // 16KB input buffer
	}
}

// init initializes the decoder (called lazily)
func (d *Decoder) init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.inited {
		return d.initErr
	}
	d.inited = true

	d.dec = C.mp3_decoder_create()
	if d.dec == nil {
		d.initErr = errors.New("mp3: failed to create decoder")
		return d.initErr
	}

	// Register cleanup after successful initialization
	d.cleanup = runtime.AddCleanup(d, freeMP3Decoder, uintptr(unsafe.Pointer(d.dec)))
	return nil
}

// Close releases decoder resources. Safe to call multiple times.
func (d *Decoder) Close() error {
	if d.closed.CompareAndSwap(false, true) {
		d.mu.Lock()
		defer d.mu.Unlock()

		if d.dec != nil {
			d.cleanup.Stop()
			C.mp3_decoder_destroy(d.dec)
			d.dec = nil
		}
	}
	return nil
}

// SampleRate returns the sample rate of the MP3 stream.
// Returns 0 if not yet determined.
func (d *Decoder) SampleRate() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sampleRate
}

// Channels returns the number of channels (1 or 2).
// Returns 0 if not yet determined.
func (d *Decoder) Channels() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channels
}

// Bitrate returns the bitrate in kbps of the last decoded frame.
func (d *Decoder) Bitrate() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bitrate
}

// Read reads decoded PCM data into p.
// The output format is interleaved int16 samples, little-endian.
func (d *Decoder) Read(p []byte) (n int, err error) {
	if err := d.init(); err != nil {
		return 0, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dec == nil {
		return 0, errors.New("mp3: decoder is closed")
	}

	for n < len(p) {
		// Try to read from current decoded frame
		read := C.mp3_decoder_read_pcm(d.dec, (*C.uchar)(unsafe.Pointer(&p[n])), C.int(len(p)-n))
		n += int(read)

		if n >= len(p) {
			break
		}

		// Need to decode more frames
		// First, ensure we have input data
		if d.bufPos >= d.bufLen {
			// Refill input buffer
			d.bufPos = 0
			d.bufLen, err = d.r.Read(d.buf)
			if d.bufLen == 0 {
				if err == nil {
					err = io.EOF
				}
				if n > 0 {
					return n, nil
				}
				return 0, err
			}
		}

		// Decode next frame
		var consumed C.int
		samples := C.mp3_decoder_decode_frame(
			d.dec,
			(*C.uchar)(unsafe.Pointer(&d.buf[d.bufPos])),
			C.int(d.bufLen-d.bufPos),
			&consumed,
		)
		d.bufPos += int(consumed)

		if samples > 0 {
			// Update format info
			d.sampleRate = int(C.mp3_decoder_get_hz(d.dec))
			d.channels = int(C.mp3_decoder_get_channels(d.dec))
			d.bitrate = int(C.mp3_decoder_get_bitrate(d.dec))
		} else if consumed == 0 {
			// Need more data but no bytes consumed - shift buffer
			if d.bufPos > 0 {
				copy(d.buf, d.buf[d.bufPos:d.bufLen])
				d.bufLen -= d.bufPos
				d.bufPos = 0

				// Read more data
				nr, rerr := d.r.Read(d.buf[d.bufLen:])
				d.bufLen += nr
				if nr == 0 && rerr != nil {
					if n > 0 {
						return n, nil
					}
					return 0, rerr
				}
			}
		}
	}

	return n, nil
}

// DecodeFull decodes the entire MP3 stream and returns PCM data.
// This is a convenience function for small files.
func DecodeFull(r io.Reader) (pcm []byte, sampleRate, channels int, err error) {
	dec := NewDecoder(r)
	defer dec.Close()

	var buf []byte
	tmp := make([]byte, 8192)
	for {
		n, err := dec.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, err
		}
	}

	return buf, dec.SampleRate(), dec.Channels(), nil
}
