//go:build !js
// +build !js

package resampler

import (
	"fmt"
	"io"
	"sync"
	"unsafe"
)

/*
#cgo pkg-config: soxr
#include <soxr.h>
#include <stdlib.h>
*/
import "C"

// Resampler wraps an io.Reader and resamples audio from srcFmt to dstFmt using
// libsoxr. It supports sample rate conversion and channel conversion
// (mono↔stereo). The resampler must be closed with Close() to release C library
// resources.
type Resampler interface {
	io.ReadCloser
	CloseWithError(error) error
}

// Soxr wraps an io.Reader and resamples audio from srcFmt to dstFmt using
// libsoxr.
type Soxr struct {
	sampleRateRatio float64

	srcFmt Format
	src    io.Reader

	dstFmt  Format
	readBuf []byte

	mu       sync.Mutex
	closeErr error
	soxr     C.soxr_t
}

// New creates a new Resampler that resamples audio from srcFmt to dstFmt. It
// supports sample rate conversion and channel conversion (mono↔stereo). The
// formats must use 16-bit signed integer samples.
func New(src io.Reader, srcFmt, dstFmt Format) (Resampler, error) {
	ioSpec := C.soxr_io_spec(C.soxr_datatype_t(C.SOXR_INT16_I), C.soxr_datatype_t(C.SOXR_INT16_I))

	// Use high quality by default
	qSpec := C.soxr_quality_spec(C.SOXR_HQ, 0)

	var soxrErr C.soxr_error_t
	s := C.soxr_create(
		C.double(srcFmt.SampleRate),
		C.double(dstFmt.SampleRate),
		C.uint(dstFmt.channels()),
		&soxrErr,
		&ioSpec,
		&qSpec,
		nil,
	)

	if s == nil {
		msg := "unknown error"
		if soxrErr != nil {
			msg = C.GoString(soxrErr)
		}
		return nil, fmt.Errorf("soxr_create failed: %s", msg)
	}

	rs := &Soxr{
		sampleRateRatio: float64(srcFmt.SampleRate) / float64(dstFmt.SampleRate),

		srcFmt: srcFmt,
		src:    newSampleReader(src, srcFmt.sampleBytes()),

		dstFmt: dstFmt,

		soxr: s,
	}

	return rs, nil
}

// Read copies resampled audio data into p. It returns the number of bytes
// written and any encountered error. This method is not safe for concurrent
// use.
func (r *Soxr) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if len(p) < r.dstFmt.sampleBytes() {
		return 0, io.ErrShortBuffer
	}

	// Truncate p to a multiple of sampleBytes
	p = p[:len(p)/r.dstFmt.sampleBytes()*r.dstFmt.sampleBytes()]

	r.mu.Lock()
	defer r.mu.Unlock()

	// at most two iterations, first iteration is for the initial read, second
	// iteration is for the EOF case.
	for range 2 {
		if r.soxr == nil {
			return 0, r.closeErr
		}

		if r.closeErr != nil {
			n, err := r.processLocked(nil, p)
			if err != nil {
				return 0, err
			}
			return n, nil
		}
		n, readErr := r.readLocked(p)
		if r.soxr == nil {
			return 0, r.closeErr
		}
		if n == 0 && readErr == nil {
			return 0, nil
		}
		if readErr != nil {
			if readErr == io.ErrShortBuffer {
				return 0, readErr
			}
			r.closeErr = readErr
		}
		if n > 0 {
			// Pad partial sample with zeros (for EOF case where sampleReader
			// may return unaligned data)
			if mod := n % r.dstFmt.sampleBytes(); mod != 0 {
				padding := r.dstFmt.sampleBytes() - mod
				for i := range padding {
					r.readBuf[n+i] = 0
				}
				n += padding
			}
			n, err := r.processLocked(r.readBuf[:n], p)
			if err != nil {
				return 0, err
			}
			return n, nil
		}
	}
	panic("unreachable")
}

// Close releases the soxr resources and marks the resampler as closed.
// Subsequent Read calls will return io.ErrClosedPipe.
func (r *Soxr) Close() error {
	return r.CloseWithError(fmt.Errorf("resampler: %w", io.ErrClosedPipe))
}

// CloseWithError releases the soxr resources with a custom error. Subsequent
// Read calls will return the provided error.
func (r *Soxr) CloseWithError(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.soxr == nil {
		return nil
	}
	if r.closeErr == nil {
		r.closeErr = err
	}
	C.soxr_delete(r.soxr)
	r.soxr = nil
	return nil
}

// processLocked processes input samples through soxr and writes output to dst.
func (r *Soxr) processLocked(src, dst []byte) (n int, err error) {
	if r.soxr == nil {
		return 0, r.closeErr
	}
	defer func() {
		flushing := src == nil && n == 0
		if err != nil || flushing {
			C.soxr_delete(r.soxr)
			r.soxr = nil
			if err != nil {
				r.closeErr = err
			} else {
				r.closeErr = io.EOF
			}
		}
	}()

	var (
		optr  = C.soxr_out_t(unsafe.Pointer(&dst[0]))
		osize = C.size_t(len(dst) / r.dstFmt.sampleBytes())
		odone C.size_t

		iptr  C.soxr_in_t
		isize C.size_t
		idone C.size_t
	)
	if src != nil {
		iptr = C.soxr_in_t(unsafe.Pointer(&src[0]))
		isize = C.size_t(len(src) / r.dstFmt.sampleBytes())
	}
	if err := C.soxr_process(
		r.soxr,
		iptr, isize, &idone,
		optr, osize, &odone,
	); err != nil {
		return 0, fmt.Errorf("soxr process error: %s", C.GoString(err))
	}
	return int(odone) * r.dstFmt.sampleBytes(), nil
}

// readLocked reads data from the source reader and resamples it to the
// destination format.
func (r *Soxr) readLocked(dst []byte) (rn int, re error) {
	n := int(float64(len(dst)) * r.sampleRateRatio)

	if n == 0 {
		return 0, io.ErrShortBuffer
	}

	r.mu.Unlock()
	defer r.mu.Lock()

	if r.srcFmt.Stereo && !r.dstFmt.Stereo {
		// Downmixing stereo to mono
		n <<= 1
		if cap(r.readBuf) < n {
			r.readBuf = make([]byte, n)
		}
		rn, err := r.src.Read(r.readBuf[:n])
		return stereoToMono(r.readBuf[:rn]), err
	}

	if cap(r.readBuf) < n {
		r.readBuf = make([]byte, n)
	}

	if r.srcFmt.Stereo == r.dstFmt.Stereo {
		return r.src.Read(r.readBuf[:n])
	}

	// Upmixing mono to stereo
	rn, err := r.src.Read(r.readBuf[:n/2])
	return monoToStereo(r.readBuf[:rn*2]), err
}

// stereoToMono converts stereo 16-bit samples to mono in-place by averaging L
// and R channels.
func stereoToMono(b []byte) int {
	numFrames := len(b) / 4
	for i := range numFrames {
		j := i * 4
		k := i * 2
		l := int16(b[j]) | int16(b[j+1])<<8
		r := int16(b[j+2]) | int16(b[j+3])<<8
		m := int16((int32(l) + int32(r)) / 2)
		b[k] = byte(m)
		b[k+1] = byte(m >> 8)
	}
	return numFrames * 2
}

// monoToStereo converts mono 16-bit samples to stereo in-place by duplicating
// each sample.
func monoToStereo(b []byte) int {
	stereoLen := len(b)
	numSamples := stereoLen / 4
	for i := numSamples - 1; i >= 0; i-- {
		s0, s1 := b[i*2], b[i*2+1]
		j := i * 4
		b[j], b[j+1] = s0, s1
		b[j+2], b[j+3] = s0, s1
	}
	return stereoLen
}
