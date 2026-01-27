//go:build !js
// +build !js

package resampler

import (
	"fmt"
	"io"
	"sync"

	resampling "github.com/tphakala/go-audio-resampling"
)

// Resampler wraps an io.Reader and resamples audio from srcFmt to dstFmt.
// It supports sample rate conversion and channel conversion (mono↔stereo).
// The resampler must be closed with Close() to release resources.
type Resampler interface {
	io.ReadCloser
	CloseWithError(error) error
}

// Soxr wraps an io.Reader and resamples audio from srcFmt to dstFmt using
// a pure Go resampler (no CGO/FFI dependencies).
type Soxr struct {
	srcFmt Format
	src    io.Reader

	dstFmt  Format
	readBuf []byte

	mu            sync.Mutex
	closeErr      error
	resampler     resampling.Resampler
	leftover      []byte
	needsResample bool
}

// New creates a new Resampler that resamples audio from srcFmt to dstFmt. It
// supports sample rate conversion and channel conversion (mono↔stereo). The
// formats must use 16-bit signed integer samples.
func New(src io.Reader, srcFmt, dstFmt Format) (Resampler, error) {
	needsResample := srcFmt.SampleRate != dstFmt.SampleRate

	var resampler resampling.Resampler
	if needsResample {
		config := &resampling.Config{
			InputRate:  float64(srcFmt.SampleRate),
			OutputRate: float64(dstFmt.SampleRate),
			Channels:   dstFmt.channels(),
			Quality:    resampling.QualitySpec{Preset: resampling.QualityHigh},
		}
		var err error
		resampler, err = resampling.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create resampler: %w", err)
		}
	}

	rs := &Soxr{
		srcFmt: srcFmt,
		src:    newSampleReader(src, srcFmt.sampleBytes()),

		dstFmt: dstFmt,

		resampler:     resampler,
		needsResample: needsResample,
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

	// First return any leftover data
	if len(r.leftover) > 0 {
		n := copy(p, r.leftover)
		r.leftover = r.leftover[n:]
		return n, nil
	}

	if r.closeErr != nil {
		return 0, r.closeErr
	}

	// Read and process
	return r.readAndProcess(p)
}

// readAndProcess reads from source and processes through resampler.
func (r *Soxr) readAndProcess(p []byte) (int, error) {
	if !r.needsResample {
		// No sample rate conversion, just channel conversion
		return r.readPassthrough(p)
	}

	// Read source data
	// Estimate how much source data we need based on ratio
	ratio := float64(r.srcFmt.SampleRate) / float64(r.dstFmt.SampleRate)
	srcBytesNeeded := int(float64(len(p))*ratio) + r.srcFmt.sampleBytes()*4 // Extra buffer

	if cap(r.readBuf) < srcBytesNeeded {
		r.readBuf = make([]byte, srcBytesNeeded)
	}

	bytesRead, readErr := r.readSourceWithChannelConv(srcBytesNeeded)
	if bytesRead == 0 {
		if readErr != nil {
			return 0, readErr
		}
		return 0, io.EOF
	}

	// Convert bytes to float64 samples (normalized to -1.0 to 1.0)
	numChannels := r.dstFmt.channels()
	numFrames := bytesRead / (2 * numChannels) // 2 bytes per int16, per channel
	input := make([]float64, numFrames*numChannels)

	for i := 0; i < numFrames*numChannels; i++ {
		sample := int16(r.readBuf[i*2]) | int16(r.readBuf[i*2+1])<<8
		input[i] = float64(sample) / 32768.0
	}

	// Resample
	output, err := r.resampler.Process(input)
	if err != nil {
		return 0, fmt.Errorf("resample error: %w", err)
	}

	if len(output) == 0 {
		if readErr != nil {
			return 0, readErr
		}
		return 0, nil
	}

	// Convert back to bytes (int16)
	outputBytes := make([]byte, len(output)*2)
	for i, s := range output {
		sample := int16(s * 32767.0)
		if s > 1.0 {
			sample = 32767
		} else if s < -1.0 {
			sample = -32768
		}
		outputBytes[i*2] = byte(sample)
		outputBytes[i*2+1] = byte(sample >> 8)
	}

	// Ensure output is aligned to sample bytes
	outputLen := (len(outputBytes) / r.dstFmt.sampleBytes()) * r.dstFmt.sampleBytes()
	outputBytes = outputBytes[:outputLen]

	// Copy to output, save leftover
	n := copy(p, outputBytes)
	if len(outputBytes) > n {
		r.leftover = append(r.leftover, outputBytes[n:]...)
	}

	return n, readErr
}

// readPassthrough reads without sample rate conversion.
func (r *Soxr) readPassthrough(p []byte) (int, error) {
	n, err := r.readSourceWithChannelConv(len(p))
	if n == 0 {
		return 0, err
	}
	copy(p, r.readBuf[:n])
	return n, err
}

// readSourceWithChannelConv reads from source and handles channel conversion.
func (r *Soxr) readSourceWithChannelConv(dstLen int) (int, error) {
	if cap(r.readBuf) < dstLen {
		r.readBuf = make([]byte, dstLen)
	}

	if r.srcFmt.Stereo && !r.dstFmt.Stereo {
		// Downmixing stereo to mono: need double the source data
		srcLen := dstLen * 2
		if cap(r.readBuf) < srcLen {
			r.readBuf = make([]byte, srcLen)
		}
		rn, err := r.src.Read(r.readBuf[:srcLen])
		if rn == 0 {
			return 0, err
		}
		return stereoToMono(r.readBuf[:rn]), err
	}

	if r.srcFmt.Stereo == r.dstFmt.Stereo {
		// Same channel count
		return r.src.Read(r.readBuf[:dstLen])
	}

	// Upmixing mono to stereo
	rn, err := r.src.Read(r.readBuf[:dstLen/2])
	if rn == 0 {
		return 0, err
	}
	return monoToStereo(r.readBuf[:rn*2]), err
}

// Close releases resources and marks the resampler as closed.
// Subsequent Read calls will return io.ErrClosedPipe.
func (r *Soxr) Close() error {
	return r.CloseWithError(fmt.Errorf("resampler: %w", io.ErrClosedPipe))
}

// CloseWithError releases resources with a custom error. Subsequent
// Read calls will return the provided error.
func (r *Soxr) CloseWithError(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closeErr == nil {
		r.closeErr = err
	}
	r.resampler = nil
	return nil
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
