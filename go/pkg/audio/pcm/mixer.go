package pcm

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"
)

// MixerOption is an option for configuring a Mixer.
type MixerOption interface {
	apply(*Mixer)
}

type autoCloseOption struct{}

func (autoCloseOption) apply(mx *Mixer) {
	mx.autoClose = true
}

// WithAutoClose configures the mixer to automatically close writing when all
// tracks are gone. Defaults to false.
func WithAutoClose() MixerOption {
	return autoCloseOption{}
}

type silenceGapOption struct {
	gap time.Duration
}

func (o silenceGapOption) apply(mx *Mixer) {
	mx.silenceGap = o.gap
	// set initial running silence to gap to avoid leading silence.
	// runningSilence will be reset to 0 when a non zero peak is detected.
	mx.runningSilence = o.gap
}

const defaultSilenceGap = 0

// WithSilenceGap sets the duration of silence after which the mixer will close.
// Defaults to 0 (disabled).
func WithSilenceGap(gap time.Duration) MixerOption {
	return silenceGapOption{gap: gap}
}

type onTrackCreatedOption struct {
	fn func()
}

func (o onTrackCreatedOption) apply(mx *Mixer) {
	mx.onTrackCreated = o.fn
}

// WithOnTrackCreated sets a callback that is called when a new track is created.
func WithOnTrackCreated(fn func()) MixerOption {
	return onTrackCreatedOption{fn: fn}
}

type onTrackClosedOption struct {
	fn func()
}

func (o onTrackClosedOption) apply(mx *Mixer) {
	mx.onTrackClosed = o.fn
}

// WithOnTrackClosed sets a callback that is called when a track is closed/removed.
func WithOnTrackClosed(fn func()) MixerOption {
	return onTrackClosedOption{fn: fn}
}

// Mixer is a mixer for audio data. It mixes multiple audio streams into a
// single stream. With TrackCtrl, you can control the play/stop/gain of each
// track.
//
// It is safe to call methods on Mixer from multiple goroutines.
type Mixer struct {
	output    Format
	readChunk int
	autoClose bool

	mu         sync.Mutex
	head       *TrackCtrl
	closeErr   error
	closeWrite bool

	silenceGap     time.Duration
	runningSilence time.Duration

	trackNotify chan struct{}
	writeNotify chan struct{}

	buf      []float32
	trackBuf []byte

	// Track lifecycle callbacks
	onTrackCreated func()
	onTrackClosed  func()
}

// NewMixer creates a new Mixer with the specified output format and options.
// The mixer can be used to mix multiple audio tracks into a single output stream.
func NewMixer(output Format, opts ...MixerOption) *Mixer {
	mx := &Mixer{
		output:      output,
		readChunk:   int(output.BytesInDuration(time.Millisecond * 60)),
		trackNotify: make(chan struct{}, 1),
		writeNotify: make(chan struct{}, 1),

		silenceGap: defaultSilenceGap,
	}
	for _, opt := range opts {
		opt.apply(mx)
	}
	return mx
}

// Output returns the output format of the mixer.
func (mx *Mixer) Output() Format {
	return mx.output
}

// TrackOption is an option for configuring a Track.
type TrackOption interface {
	apply(*TrackCtrl)
}

type trackLabelOption struct {
	label string
}

func (o trackLabelOption) apply(tc *TrackCtrl) {
	tc.label = o.label
}

// WithTrackLabel sets a label for the track.
func WithTrackLabel(label string) TrackOption {
	return trackLabelOption{label: label}
}

// CreateTrack creates a new writable track in the mixer. It returns the Track
// for writing audio chunks, a TrackCtrl for controlling the track, and an error
// if the mixer is closed or CloseWrite has been called.
func (mx *Mixer) CreateTrack(opts ...TrackOption) (Track, *TrackCtrl, error) {
	mx.mu.Lock()
	defer mx.mu.Unlock()

	if mx.closeErr != nil {
		return nil, nil, mx.closeErr
	}

	if mx.closeWrite {
		return nil, nil, fmt.Errorf("pcm/mixer: cannot create track after CloseWrite: %w", io.ErrClosedPipe)
	}

	tr, err := mx.newTrack()
	if err != nil {
		return nil, nil, err
	}
	mx.head = &TrackCtrl{
		track: tr,
		next:  mx.head,
		gain:  NewAtomicFloat32(1),
	}
	for _, opt := range opts {
		opt.apply(mx.head)
	}
	select {
	case mx.trackNotify <- struct{}{}:
	default:
	}
	if mx.onTrackCreated != nil {
		mx.onTrackCreated()
	}
	return tr, mx.head, nil
}

// Read reads mixed audio data from the mixer into p. It mixes all active tracks
// and returns the mixed audio in the mixer's output format. The method
// implements io.Reader.
func (mx *Mixer) Read(p []byte) (int, error) {
	if len(p) > mx.readChunk {
		// As we always read full of p, we need to limit the maximum size of the
		// buffer to the output format.
		p = p[:mx.readChunk]
	}
	if err := mx.readFull(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// CloseWrite closes writing to the mixer. After calling CloseWrite, no new
// tracks can be added, and Read will return EOF once all tracks are finished.
func (mx *Mixer) CloseWrite() error {
	mx.mu.Lock()
	defer mx.mu.Unlock()
	return mx.closeWriteLocked()
}

// Close closes the mixer. It is equivalent to calling CloseWithError with
// io.ErrClosedPipe.
func (mx *Mixer) Close() error {
	return mx.CloseWithError(fmt.Errorf("pcm/mixer: close: %w", io.ErrClosedPipe))
}

// CloseWithError closes the mixer with an error. All tracks will be closed with
// the same error. If err is nil, it will be set to io.ErrClosedPipe.
func (mx *Mixer) CloseWithError(err error) error {
	if err == nil {
		err = fmt.Errorf("pcm/mixer: %w", io.ErrClosedPipe)
	}
	mx.mu.Lock()
	defer mx.mu.Unlock()
	if mx.closeErr != nil {
		return nil
	}
	mx.closeErr = err

	if !mx.closeWrite {
		mx.closeWrite = true
		close(mx.trackNotify)
		close(mx.writeNotify)
	}

	it := mx.head
	for it != nil {
		it.CloseWithError(err)
		it = it.next
	}

	return nil
}

func (mx *Mixer) closeWriteLocked() error {
	if mx.closeErr != nil {
		return nil
	}
	if mx.closeWrite {
		return nil
	}

	mx.closeWrite = true
	close(mx.trackNotify)
	close(mx.writeNotify)

	it := mx.head
	for it != nil {
		it.CloseWrite()
		it = it.next
	}
	return nil
}

// readFull reads a full buffer of mixed audio data from the mixer into p.
// It mixes all active tracks and converts the result to the mixer's output format (16-bit PCM).
func (mx *Mixer) readFull(p []byte) error {
	// Convert byte buffer to int16 slice (16-bit PCM, 2 bytes per sample)
	i16 := unsafe.Slice((*int16)(unsafe.Pointer(&p[0])), len(p)/2)

	var (
		peak    float32
		read    bool
		silence bool
	)

	mx.mu.Lock()
	defer mx.mu.Unlock()

	// Ensure mixing buffer is large enough
	if len(mx.buf) < len(i16) {
		mx.buf = make([]float32, len(i16))
	}

	// Loop until we get data or an error
	for {
		var err error
		peak, read, silence, err = mx.readFullLocked(p)

		if err != nil {
			return err
		}

		if read || silence {
			break
		}

		// No data available, wait for write notification
		mx.mu.Unlock()
		<-mx.writeNotify
		mx.mu.Lock()
	}

	// Update running silence counter
	if read {
		mx.runningSilence = 0
	} else if silence {
		mx.runningSilence += mx.output.Duration(int64(len(p)))
	}

	// If no peak detected, output silence
	if peak == 0 {
		for i := range i16 {
			i16[i] = 0
		}
		return nil
	}

	// Convert mixed float32 samples back to int16 PCM format
	for i := range i16 {
		t := mx.buf[i]
		// Clip to prevent overflow
		if t > 1 {
			t = 1
		} else if t < -1 {
			t = -1
		}
		// Convert to int16: positive values use 32767, negative use 32768
		if t >= 0 {
			i16[i] = int16(t * 32767)
		} else {
			i16[i] = int16(t * 32768)
		}
	}
	return nil
}

// headTrackLocked returns the head track controller from the mixer's track list.
func (mx *Mixer) headTrackLocked() (head *TrackCtrl, silence bool, err error) {
	for {
		if mx.closeErr != nil {
			return nil, false, mx.closeErr
		}

		if mx.head != nil {
			return mx.head, false, nil
		}

		if mx.closeWrite {
			return nil, false, io.EOF
		}

		if mx.autoClose {
			mx.closeWriteLocked()
			return nil, false, io.EOF
		}

		if mx.runningSilence < mx.silenceGap {
			return nil, true, nil
		}

		mx.mu.Unlock()
		<-mx.trackNotify
		mx.mu.Lock()
	}
}

// readFullLocked reads audio data from all active tracks and mixes them into the buffer p.
func (mx *Mixer) readFullLocked(p []byte) (peak float32, read, silence bool, err error) {
	it, silence, err := mx.headTrackLocked()
	if err != nil {
		return
	}
	if silence {
		return
	}
	// clear the buffer for the next mixing round
	for i := range mx.buf {
		mx.buf[i] = 0
	}

	// Reuse track read buffer (allocated once, grown as needed).
	if len(mx.trackBuf) < len(p) {
		mx.trackBuf = make([]byte, len(p))
	}
	trackBuf := mx.trackBuf[:len(p)]
	trackI16 := unsafe.Slice((*int16)(unsafe.Pointer(&trackBuf[0])), len(trackBuf)/2)

	var prev *TrackCtrl
	for it != nil {
		ok, err := it.readFull(trackBuf)
		if err != nil {
			// Track has an error, close it and remove from linked list
			it.CloseWithError(err)
			it = it.next
			if prev == nil {
				mx.head = it
			} else {
				prev.next = it
			}
			if mx.onTrackClosed != nil {
				mx.onTrackClosed()
			}
			continue
		}
		if ok {
			read = true
			gain := it.gain.Load()
			// Mix this track's audio into the buffer
			for i := range trackI16 {
				if trackI16[i] != 0 {
					// Convert int16 to float32 in range [-1.0, 1.0]
					s := float32(trackI16[i])
					if s >= 0 {
						s /= 32767
					} else {
						s /= 32768
					}
					// Apply track gain
					s *= gain
					// Track peak amplitude (absolute value)
					if s > peak {
						peak = s
					} else if -s > peak {
						peak = -s
					}
					// Accumulate into mixing buffer
					mx.buf[i] += s
				}
			}
		}
		prev = it
		it = it.next
	}
	return
}

func (mx *Mixer) notifyWrite() {
	go func() {
		mx.mu.Lock()
		defer mx.mu.Unlock()
		if mx.closeErr != nil {
			return
		}
		if mx.closeWrite {
			return
		}
		select {
		case mx.writeNotify <- struct{}{}:
		default:
		}
	}()
}

func (mx *Mixer) newTrack() (*track, error) {
	tr := &track{
		mx:     mx,
		inputs: []*trackWriter{},
	}
	const defaultTrackInputFormat = L16Mono16K
	tw, err := tr.newWriter(defaultTrackInputFormat)
	if err != nil {
		return nil, err
	}
	tr.inputs = append(tr.inputs, tw)
	return tr, nil
}
