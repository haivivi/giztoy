package pcm

import (
	"sync/atomic"
	"time"
)

// TrackCtrl provides control over a track in the mixer, including gain (volume)
// adjustment, fade-out duration, and track lifecycle management.
type TrackCtrl struct {
	label string
	track *track
	next  *TrackCtrl

	gain            AtomicFloat32
	readn           atomic.Int64
	fadeOutDuration atomic.Int32
}

// Label returns the label of the track.
func (tc *TrackCtrl) Label() string {
	return tc.label
}

// SetGainLinearTo linearly fades the track's gain from the current value to
// the target value over the specified duration. The gain is updated in 10ms
// intervals. This method blocks until the fade is complete.
func (tc *TrackCtrl) SetGainLinearTo(to float32, duration time.Duration) {
	from := tc.gain.Load()

	const interval = 10 * time.Millisecond
	steps := int(duration / interval)
	if steps == 0 {
		tc.gain.Store(to)
		return
	}
	for i := range steps {
		time.Sleep(interval)
		tc.gain.Store(from + (to-from)*float32(i+1)/float32(steps))
	}
}

// SetGain sets the gain (volume) of the track. The gain is a linear multiplier
// where 1.0 is full volume, 0.0 is silence, and values greater than 1.0 may
// cause clipping.
func (tc *TrackCtrl) SetGain(volume float32) {
	tc.gain.Store(volume)
}

// SetFadeOutDuration sets the fade-out duration for the track. When the track
// is closed, it will automatically fade out over this duration before actually
// closing. If duration is 0, the track closes immediately.
func (tc *TrackCtrl) SetFadeOutDuration(duration time.Duration) {
	tc.fadeOutDuration.Store(int32(duration / time.Millisecond))
}

// Close closes the track. If a fade-out duration has been set, the track will
// fade out before closing. Otherwise, it closes immediately.
func (tc *TrackCtrl) Close() error {
	if d := tc.fadeOutDuration.Load(); d > 0 {
		go func() {
			tc.SetGainLinearTo(0, time.Duration(d)*time.Millisecond)
			tc.track.Close()
		}()
		return tc.CloseWrite()
	}
	return tc.track.Close()
}

// ReadBytes returns the total number of bytes read from this track.
func (tc *TrackCtrl) ReadBytes() int64 {
	return tc.readn.Load()
}

// CloseWithError closes the track with an error. If a fade-out duration has
// been set, the track will fade out before closing with the error. Otherwise,
// it closes immediately with the error.
func (tc *TrackCtrl) CloseWithError(err error) error {
	if d := tc.fadeOutDuration.Load(); d > 0 {
		go func() {
			tc.SetGainLinearTo(0, time.Duration(d)*time.Millisecond)
			tc.track.CloseWithError(err)
		}()
		return tc.CloseWrite()
	}
	return tc.track.CloseWithError(err)
}

// CloseWriteWithSilence closes writing to the track after appending silence.
func (tc *TrackCtrl) CloseWriteWithSilence(silence time.Duration) error {
	if err := tc.track.Write(tc.track.mx.output.SilenceChunk(silence)); err != nil {
		return err
	}
	return tc.CloseWrite()
}

// CloseWrite closes writing to the track.
func (tc *TrackCtrl) CloseWrite() error {
	return tc.track.CloseWrite()
}

func (tc *TrackCtrl) readFull(p []byte) (bool, error) {
	n, err := readFull(tc.track, p)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	tc.readn.Add(int64(n))
	return true, nil
}
