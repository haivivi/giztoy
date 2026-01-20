package ogg

/*
#include <ogg/ogg.h>
#include <stdlib.h>
#include <string.h>

static ogg_sync_state* alloc_sync_state() {
    ogg_sync_state* state = (ogg_sync_state*)calloc(1, sizeof(ogg_sync_state));
    if (state) {
        ogg_sync_init(state);
    }
    return state;
}

static void free_sync_state(ogg_sync_state* state) {
    if (state) {
        ogg_sync_clear(state);
        free(state);
    }
}
*/
import "C"
import (
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"unsafe"
)

var (
	// ErrSync indicates a sync error during page extraction.
	ErrSync = errors.New("ogg: sync error")
	// ErrNeedMore indicates more data is needed.
	ErrNeedMore = errors.New("ogg: need more data")
)

// SyncState manages the synchronization and page extraction from an Ogg bitstream.
// Must call Clear() when done to release resources.
type SyncState struct {
	state   *C.ogg_sync_state
	cleared atomic.Bool
	cleanup runtime.Cleanup
}

// NewSyncState creates and initializes a new SyncState.
// Returns an error if memory allocation fails.
func NewSyncState() (*SyncState, error) {
	state := C.alloc_sync_state()
	if state == nil {
		return nil, errors.New("ogg: failed to allocate sync state")
	}
	s := &SyncState{state: state}
	s.cleanup = runtime.AddCleanup(s, freeSyncState, uintptr(unsafe.Pointer(state)))
	return s, nil
}

// freeSyncState releases C resources.
func freeSyncState(ptr uintptr) {
	C.free_sync_state((*C.ogg_sync_state)(unsafe.Pointer(ptr)))
}

// Clear releases resources. Safe to call multiple times.
func (s *SyncState) Clear() {
	if s.cleared.CompareAndSwap(false, true) {
		s.cleanup.Stop()
		C.free_sync_state(s.state)
		s.state = nil
	}
}

// Reset resets the sync state.
func (s *SyncState) Reset() {
	C.ogg_sync_reset(s.state)
}

// Buffer returns a buffer for writing data into the sync state.
// After writing, call Wrote with the number of bytes written.
func (s *SyncState) Buffer(size int) []byte {
	buf := C.ogg_sync_buffer(s.state, C.long(size))
	if buf == nil {
		return nil
	}
	// Create a Go slice backed by the C buffer
	return unsafe.Slice((*byte)(unsafe.Pointer(buf)), size)
}

// Wrote tells the sync state that n bytes were written to the buffer.
func (s *SyncState) Wrote(n int) error {
	if C.ogg_sync_wrote(s.state, C.long(n)) != 0 {
		return ErrSync
	}
	return nil
}

// Write writes data directly to the sync state.
func (s *SyncState) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	buf := s.Buffer(len(data))
	if buf == nil {
		return 0, ErrSync
	}
	copy(buf, data)
	if err := s.Wrote(len(data)); err != nil {
		return 0, err
	}
	return len(data), nil
}

// PageOut attempts to extract a complete page from the sync state.
// Returns the page and nil error on success.
// Returns ErrNeedMore if more data is needed.
// Returns ErrSync on sync error (data skipped).
func (s *SyncState) PageOut(page *Page) error {
	result := C.ogg_sync_pageout(s.state, &page.page)
	switch result {
	case 1:
		return nil
	case 0:
		return ErrNeedMore
	default:
		return ErrSync
	}
}

// PageSeek is similar to PageOut but also returns the number of bytes skipped.
func (s *SyncState) PageSeek(page *Page) (skipped int, err error) {
	result := C.ogg_sync_pageseek(s.state, &page.page)
	if result > 0 {
		return 0, nil
	} else if result == 0 {
		return 0, ErrNeedMore
	}
	return int(-result), ErrSync
}

// Decoder reads Ogg pages from an io.Reader.
type Decoder struct {
	r     io.Reader
	sync  *SyncState
	buf   []byte
	page  Page
	inBOS bool
}

// NewDecoder creates a new Ogg decoder.
func NewDecoder(r io.Reader) (*Decoder, error) {
	sync, err := NewSyncState()
	if err != nil {
		return nil, err
	}
	return &Decoder{
		r:    r,
		sync: sync,
		buf:  make([]byte, 4096),
	}, nil
}

// Close releases resources.
func (d *Decoder) Close() error {
	d.sync.Clear()
	return nil
}

// ReadPage reads the next page from the stream.
func (d *Decoder) ReadPage() (*Page, error) {
	for {
		err := d.sync.PageOut(&d.page)
		if err == nil {
			return &d.page, nil
		}
		if err != ErrNeedMore {
			// Sync error - try to recover
			continue
		}

		// Need more data
		n, err := d.r.Read(d.buf)
		if n > 0 {
			if _, werr := d.sync.Write(d.buf[:n]); werr != nil {
				return nil, werr
			}
		}
		if err != nil {
			return nil, err
		}
	}
}
