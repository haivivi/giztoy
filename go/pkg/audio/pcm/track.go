package pcm

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/resampler"
)

// Track is a writable audio track in a Mixer.
type Track interface {
	Writer
}

// track represents an audio track in a Mixer. It can accept audio chunks
// in various formats and automatically resamples them to the track's output
// format. Multiple input streams can be written sequentially, and the track
// will read from them in order.
type track struct {
	mx *Mixer

	mu         sync.Mutex
	closeErr   error
	closeWrite bool

	inputs []*trackWriter
}

// Write writes an audio chunk to the track. If the chunk's format differs
// from the current input format, a new input writer is created and the
// previous one is closed. The chunk is automatically resampled to the track's
// output format if necessary.
func (tk *track) Write(chunk Chunk) error {
	input, err := tk.input(chunk.Format())
	if err != nil {
		return err
	}
	_, err = chunk.WriteTo(input)
	return err
}

// Read reads audio data from the track into p. It reads from input writers
// sequentially, moving to the next input when the current one is exhausted.
// Returns io.EOF when all inputs are exhausted.
func (tk *track) Read(p []byte) (n int, err error) {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	if tk.closeErr != nil {
		return 0, tk.closeErr
	}

	for {
		if len(tk.inputs) == 0 {
			return 0, io.EOF
		}
		head := tk.inputs[0]
		rn, err := readFull(head, p)
		p = p[rn:]
		n += rn
		if err != nil {
			if errors.Is(err, io.EOF) {
				head.Close()
				tk.inputs = tk.inputs[1:]
				continue
			}
			return 0, err
		}
		return n, nil
	}
}

// CloseWithError closes the track with the given error. All input writers
// are closed with the same error. Subsequent calls to Write or Read will
// return this error. If err is nil, it is set to io.ErrClosedPipe.
func (tk *track) CloseWithError(err error) error {
	if err == nil {
		err = fmt.Errorf("pcm/track: %w", io.ErrClosedPipe)
	}

	tk.mu.Lock()
	defer tk.mu.Unlock()
	if tk.closeErr != nil {
		return nil
	}

	tk.closeErr = err
	for _, input := range tk.inputs {
		input.CloseWithError(err)
	}
	tk.mx.notifyWrite()
	return nil
}

// CloseWrite closes writing to the current input writer. This allows the
// current input to be drained while preventing new data from being written
// to it. A new input writer will be created automatically if Write is called
// again with a different format.
func (tk *track) CloseWrite() error {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if len(tk.inputs) == 0 {
		return nil
	}
	err := tk.inputs[len(tk.inputs)-1].CloseWrite()
	tk.mx.notifyWrite()
	return err
}

// Close closes the track. It is equivalent to calling CloseWithError with
// io.ErrClosedPipe.
func (tk *track) Close() error {
	return tk.CloseWithError(fmt.Errorf("pcm/track: %w", io.ErrClosedPipe))
}

func (tk *track) input(format Format) (*trackWriter, error) {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	if tk.closeErr != nil {
		return nil, tk.closeErr
	}

	if len(tk.inputs) != 0 {
		last := tk.inputs[len(tk.inputs)-1]
		if last.inputFormat == format {
			return last, nil
		}
		last.CloseWrite()
	}

	newInput, err := tk.newWriter(format)
	if err != nil {
		return nil, err
	}
	tk.inputs = append(tk.inputs, newInput)
	return newInput, nil
}

func (tk *track) newWriter(format Format) (*trackWriter, error) {
	buf := tk.newBuf(format)

	if tk.mx.output == format {
		return &trackWriter{
			inputFormat: format,
			buf:         buf,
		}, nil
	}

	// Need resampling
	rs, err := resampler.New(
		buf,
		resampler.Format{
			SampleRate: format.SampleRate(),
			Stereo:     format.Channels() == 2,
		},
		resampler.Format{
			SampleRate: tk.mx.output.SampleRate(),
			Stereo:     tk.mx.output.Channels() == 2,
		},
	)
	if err != nil {
		return nil, err
	}

	return &trackWriter{
		inputFormat: format,
		buf:         buf,
		resampler:   rs,
	}, nil
}

func (tk *track) newBuf(format Format) *trackRingBuf {
	return &trackRingBuf{
		track:      tk,
		readNotify: make(chan struct{}, 1),
		rb:         make([]byte, format.BytesRate()*10), // buffer of 10seconds
	}
}

// trackWriter wraps a trackBuf and optionally a resampler to provide
// a unified interface for writing and reading audio data. When a resampler
// is present, reads go through the resampler; otherwise, reads come
// directly from the buffer.
type trackWriter struct {
	inputFormat Format
	buf         *trackRingBuf
	resampler   resampler.Resampler
}

// Write writes audio data to the underlying buffer.
func (tw *trackWriter) Write(p []byte) (int, error) {
	return tw.buf.Write(p)
}

// Read reads audio data from the buffer. If a resampler is configured,
// data is read through the resampler; otherwise, it is read directly
// from the buffer.
func (tw *trackWriter) Read(p []byte) (int, error) {
	if tw.resampler != nil {
		return tw.resampler.Read(p)
	}
	return tw.buf.Read(p)
}

// Close closes the writer and its underlying resources. It closes the
// resampler if present, then closes the buffer.
func (tw *trackWriter) Close() error {
	if tw.resampler != nil {
		tw.resampler.Close()
	}
	return tw.buf.Close()
}

// CloseWithError closes the writer with the given error. It propagates
// the error to both the resampler (if present) and the buffer.
func (tw *trackWriter) CloseWithError(err error) error {
	if tw.resampler != nil {
		tw.resampler.CloseWithError(err)
	}
	return tw.buf.CloseWithError(err)
}

// CloseWrite closes writing to the underlying buffer while allowing
// reads to continue until the buffer is drained.
func (tw *trackWriter) CloseWrite() error {
	return tw.buf.CloseWrite()
}

// Error returns the error state of the underlying buffer.
func (tw *trackWriter) Error() error {
	return tw.buf.Error()
}

// trackRingBuf is a thread-safe circular buffer for audio data.
type trackRingBuf struct {
	track      *track
	readNotify chan struct{}

	mu sync.Mutex
	rb []byte

	// we need to keep tail always greater or equal to head
	head, tail int

	closeWrite bool
	closeErr   error
}

// write is the internal implementation that performs the actual circular
// buffer write operation.
func (trb *trackRingBuf) write(p []byte) (n int) {
	if trb.tail-trb.head == len(trb.rb) {
		return 0
	}
	if trb.tail < len(trb.rb) {
		n = copy(trb.rb[trb.tail:], p)
		trb.tail += n
	}
	if trb.tail >= len(trb.rb) {
		m := copy(trb.rb[trb.tail-len(trb.rb):trb.head], p[n:])
		trb.tail += m
		n += m
	}
	return n
}

// Write writes audio data to the buffer.
func (trb *trackRingBuf) Write(p []byte) (n int, re error) {
	if len(p) == 0 {
		return 0, nil
	}
	n = len(p)

	trb.mu.Lock()
	defer trb.mu.Unlock()

	for len(p) > 0 {
		if trb.closeErr != nil {
			return 0, trb.closeErr
		}
		if trb.closeWrite {
			return 0, fmt.Errorf("pcm/trackRingBuf: write: %w", io.ErrClosedPipe)
		}
		if trb.tail-trb.head == len(trb.rb) {
			trb.mu.Unlock()
			<-trb.readNotify
			trb.mu.Lock()
			continue
		}
		p = p[trb.write(p):]
		trb.track.mx.notifyWrite()
	}
	return n, nil
}

// read is the internal implementation that performs the actual circular
// buffer read operation.
func (trb *trackRingBuf) read(p []byte) (n int) {
	if trb.tail >= len(trb.rb) {
		n = copy(p, trb.rb[trb.head:])
		trb.head += n
		if trb.head == len(trb.rb) {
			trb.head = 0
			trb.tail -= len(trb.rb)
		}
	}
	if trb.tail < len(trb.rb) {
		m := copy(p[n:], trb.rb[trb.head:trb.tail])
		trb.head += m
		n += m
	}
	return n
}

// Read reads audio data from the buffer into p.
func (trb *trackRingBuf) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}

	trb.mu.Lock()
	defer trb.mu.Unlock()

	if trb.closeErr != nil {
		return 0, trb.closeErr
	}
	if trb.head == trb.tail {
		if trb.closeWrite {
			return 0, io.EOF
		}
		return 0, nil
	}
	n := trb.read(p)
	if trb.closeWrite {
		return n, nil
	}
	select {
	case trb.readNotify <- struct{}{}:
	default:
	}
	return n, nil
}

// CloseWrite closes writing to the buffer.
func (trb *trackRingBuf) CloseWrite() error {
	trb.mu.Lock()
	defer trb.mu.Unlock()
	if trb.closeErr != nil {
		return trb.closeErr
	}
	if !trb.closeWrite {
		trb.closeWrite = true
		close(trb.readNotify)
	}
	return nil
}

// CloseWithError closes the buffer with the given error.
func (trb *trackRingBuf) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	trb.mu.Lock()
	defer trb.mu.Unlock()
	if trb.closeErr != nil {
		return trb.closeErr
	}
	trb.closeErr = err
	if !trb.closeWrite {
		trb.closeWrite = true
		close(trb.readNotify)
	}
	return nil
}

// Close closes the buffer.
func (trb *trackRingBuf) Close() error {
	return trb.CloseWithError(fmt.Errorf("pcm/trackRingBuf: %w", io.ErrClosedPipe))
}

// Error returns the error state of the buffer, if any.
func (trb *trackRingBuf) Error() error {
	trb.mu.Lock()
	defer trb.mu.Unlock()
	return trb.closeErr
}

// readFull reads from r until the buffer p is completely filled or an error occurs.
func readFull(r io.Reader, p []byte) (int, error) {
	var (
		readBytes int
		readErr   error
	)
	for {
		n, err := r.Read(p[readBytes:])
		if err != nil {
			readErr = err
			break
		}
		if n == 0 {
			break
		}
		readBytes += n
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return 0, readErr
	}
	if readBytes == 0 {
		return 0, readErr
	}
	for i := readBytes; i < len(p); i++ {
		p[i] = 0
	}
	return len(p), nil
}
