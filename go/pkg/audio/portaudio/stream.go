package portaudio

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
)

// InputStream captures audio from the default input device.
type InputStream struct {
	stream *Stream
	format pcm.Format
	frames int
	mu     sync.Mutex
	closed bool
}

// NewInputStream creates a new input stream for recording.
// format: PCM format (e.g., pcm.L16Mono16K)
// bufferDuration: duration of each read buffer (e.g., 20ms)
func NewInputStream(format pcm.Format, bufferDuration time.Duration) (*InputStream, error) {
	framesPerBuffer := int(format.SamplesInDuration(bufferDuration))

	stream, err := openStream(format.Channels(), 0, float64(format.SampleRate()), framesPerBuffer)
	if err != nil {
		return nil, err
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return nil, err
	}

	return &InputStream{
		stream: stream,
		format: format,
		frames: framesPerBuffer,
	}, nil
}

// Read reads PCM samples into the provided buffer.
// Returns the number of samples read (not bytes).
func (is *InputStream) Read(buf []int16) (int, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.closed {
		return 0, io.EOF
	}

	samples, err := is.stream.Read(is.frames)
	if err != nil {
		return 0, err
	}

	n := copy(buf, samples)
	return n, nil
}

// ReadBytes reads PCM samples as bytes (little-endian int16).
func (is *InputStream) ReadBytes(buf []byte) (int, error) {
	samples := make([]int16, len(buf)/2)
	n, err := is.Read(samples)
	if err != nil {
		return 0, err
	}

	for i := 0; i < n; i++ {
		buf[i*2] = byte(samples[i])
		buf[i*2+1] = byte(samples[i] >> 8)
	}
	return n * 2, nil
}

// ReadChunk reads a PCM chunk of the buffer duration.
func (is *InputStream) ReadChunk() (pcm.Chunk, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.closed {
		return nil, io.EOF
	}

	samples, err := is.stream.Read(is.frames)
	if err != nil {
		return nil, err
	}

	// Convert int16 samples to bytes
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		data[i*2] = byte(s)
		data[i*2+1] = byte(s >> 8)
	}

	return is.format.DataChunk(data), nil
}

// Format returns the PCM format.
func (is *InputStream) Format() pcm.Format {
	return is.format
}

// Close stops and closes the stream.
func (is *InputStream) Close() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.closed {
		return nil
	}
	is.closed = true

	return is.stream.Close()
}

// OutputStream plays audio to the default output device.
type OutputStream struct {
	stream *Stream
	format pcm.Format
	frames int
	buffer []int16
	mu     sync.Mutex
	closed bool
}

// NewOutputStream creates a new output stream for playback.
// format: PCM format (e.g., pcm.L16Mono16K)
// bufferDuration: duration of each write buffer (e.g., 20ms)
func NewOutputStream(format pcm.Format, bufferDuration time.Duration) (*OutputStream, error) {
	framesPerBuffer := int(format.SamplesInDuration(bufferDuration))

	stream, err := openStream(0, format.Channels(), float64(format.SampleRate()), framesPerBuffer)
	if err != nil {
		return nil, err
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return nil, err
	}

	return &OutputStream{
		stream: stream,
		format: format,
		frames: framesPerBuffer,
		buffer: make([]int16, framesPerBuffer*format.Channels()),
	}, nil
}

// Write writes PCM samples to the output.
// Returns the number of samples written.
func (os *OutputStream) Write(samples []int16) (int, error) {
	os.mu.Lock()
	defer os.mu.Unlock()

	if os.closed {
		return 0, errors.New("stream closed")
	}

	n := copy(os.buffer, samples)
	// Zero out the rest if samples is shorter than buffer
	for i := n; i < len(os.buffer); i++ {
		os.buffer[i] = 0
	}

	if err := os.stream.Write(os.buffer); err != nil {
		return 0, err
	}
	return n, nil
}

// WriteBytes writes PCM samples from bytes (little-endian int16).
func (os *OutputStream) WriteBytes(buf []byte) (int, error) {
	samples := make([]int16, len(buf)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(buf[i*2]) | int16(buf[i*2+1])<<8
	}
	n, err := os.Write(samples)
	return n * 2, err
}

// WriteChunk writes a PCM chunk to the output.
func (os *OutputStream) WriteChunk(chunk pcm.Chunk) error {
	if chunk.Format() != os.format {
		return errors.New("portaudio: chunk format mismatch")
	}

	os.mu.Lock()
	defer os.mu.Unlock()

	if os.closed {
		return errors.New("stream closed")
	}

	// Write chunk data to a buffer, then convert to samples
	var buf [4096]byte
	n, err := chunk.WriteTo(byteWriter{buf[:len(os.buffer)*2]})
	if err != nil {
		return err
	}

	// Convert bytes to int16 samples
	for i := 0; i < int(n)/2; i++ {
		os.buffer[i] = int16(buf[i*2]) | int16(buf[i*2+1])<<8
	}
	// Zero out the rest
	for i := int(n) / 2; i < len(os.buffer); i++ {
		os.buffer[i] = 0
	}

	return os.stream.Write(os.buffer)
}

// byteWriter wraps a byte slice as an io.Writer.
type byteWriter struct {
	buf []byte
}

func (w byteWriter) Write(p []byte) (int, error) {
	n := copy(w.buf, p)
	return n, nil
}

// Format returns the PCM format.
func (os *OutputStream) Format() pcm.Format {
	return os.format
}

// Close stops and closes the stream.
func (os *OutputStream) Close() error {
	os.mu.Lock()
	defer os.mu.Unlock()

	if os.closed {
		return nil
	}
	os.closed = true

	return os.stream.Close()
}

// DuplexStream provides full-duplex audio (simultaneous recording and playback).
type DuplexStream struct {
	stream      *Stream
	format      pcm.Format
	frames      int
	mu          sync.Mutex
	closed      bool
	processFunc func(in, out []int16)
}

// NewDuplexStream creates a full-duplex stream.
// processFunc is called for each buffer; it receives input samples and should fill output samples.
func NewDuplexStream(format pcm.Format, bufferDuration time.Duration, processFunc func(in, out []int16)) (*DuplexStream, error) {
	framesPerBuffer := int(format.SamplesInDuration(bufferDuration))

	stream, err := openStream(format.Channels(), format.Channels(), float64(format.SampleRate()), framesPerBuffer)
	if err != nil {
		return nil, err
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return nil, err
	}

	ds := &DuplexStream{
		stream:      stream,
		format:      format,
		frames:      framesPerBuffer,
		processFunc: processFunc,
	}

	// Start processing goroutine
	go ds.processLoop()

	return ds, nil
}

func (ds *DuplexStream) processLoop() {
	out := make([]int16, ds.frames*ds.format.Channels())

	for {
		ds.mu.Lock()
		if ds.closed {
			ds.mu.Unlock()
			return
		}

		in, err := ds.stream.Read(ds.frames)
		if err != nil {
			ds.mu.Unlock()
			return
		}

		if ds.processFunc != nil {
			ds.processFunc(in, out)
		}

		ds.stream.Write(out)
		ds.mu.Unlock()
	}
}

// Format returns the PCM format.
func (ds *DuplexStream) Format() pcm.Format {
	return ds.format
}

// Close stops and closes the stream.
func (ds *DuplexStream) Close() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.closed {
		return nil
	}
	ds.closed = true

	return ds.stream.Close()
}
