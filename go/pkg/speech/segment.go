package speech

import (
	"bytes"
	"io"
	"slices"
	"sync"
	"unicode"
	"unicode/utf8"

	"google.golang.org/api/iterator"
)

var (
	_ SentenceSegmenter = (*DefaultSentenceSegmenter)(nil)
	_ SentenceIterator  = (*defaultSentenceIterator)(nil)
)

// DefaultSentenceSegmenter is the default sentence segmenter.
type DefaultSentenceSegmenter struct {
	// MaxRunesPerSegment specifies the maximum number of runes allowed in each
	// segment. If the text exceeds this limit, it will be split into multiple
	// segments. If this value is not set, it defaults to 256.
	MaxRunesPerSegment int
}

// Segment segments the given reader into sentences.
func (s DefaultSentenceSegmenter) Segment(r io.Reader) (SentenceIterator, error) {
	iter := &defaultSentenceIterator{
		writeNotify: make(chan struct{}, 1),
		buf:         bytes.NewBuffer(nil),
	}
	if s.MaxRunesPerSegment > 0 {
		iter.maxRunesPerSegment = s.MaxRunesPerSegment
	} else {
		iter.maxRunesPerSegment = 256
	}

	go func() {
		defer close(iter.writeNotify)

		// Copy to iter not buf for thread safety
		if _, err := io.Copy(iter, r); err != nil {
			iter.closeWithError(err)
		}
	}()
	return iter, nil
}

type defaultSentenceIterator struct {
	maxRunesPerSegment int

	mu               sync.Mutex
	closed           bool
	firstSegReturned bool
	err              error
	writeNotify      chan struct{}
	buf              *bytes.Buffer
}

func (s *defaultSentenceIterator) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := s.buf.Write(p)
	if err != nil {
		return n, err
	}
	select {
	case s.writeNotify <- struct{}{}:
	default:
	}
	return n, nil
}

func (s *defaultSentenceIterator) nextRunes(move bool) (b []byte, full bool) {
	if move {
		defer func() {
			s.buf.Next(len(b))
		}()
	}
	b = s.buf.Bytes()
	idx := lastRuneIndex(b)
	b = b[:idx]
	if rs := []rune(string(b)); len(rs) >= s.maxRunesPerSegment {
		b = []byte(string(rs[:s.maxRunesPerSegment]))
		full = true
	}
	return
}

func (s *defaultSentenceIterator) Next() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		s.firstSegReturned = true
	}()

	eof := false

	for {
		if s.closed {
			if s.err != nil {
				return "", s.err
			}
			return "", iterator.Done
		}
		if eof {
			if s.buf.Len() > 0 {
				b, _ := s.nextRunes(true)
				return string(b), nil
			}
			return "", iterator.Done
		}
		if b, full := s.nextRunes(false); len(b) > 0 {
			var idx int
			if s.firstSegReturned {
				idx = lastSegmentBoundaryIndex(b)
			} else {
				idx = segmentBoundaryIndex(b)
			}
			switch {
			case idx > 0:
				s.buf.Next(idx)
				return string(b[:idx]), nil
			case idx == 0 && full:
				s.buf.Next(len(b))
				return string(b), nil
			}
		}
		s.mu.Unlock()
		_, ok := <-s.writeNotify
		eof = !ok
		s.mu.Lock()
	}
}

func (s *defaultSentenceIterator) Close() {
	s.closeWithError(nil)
}

func (s *defaultSentenceIterator) closeWithError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.err = err
	s.buf.Reset()
}

// lastRuneIndex returns the index of the last rune in the given byte slice.
func lastRuneIndex(b []byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < utf8.RuneSelf {
			return i + 1
		}
		if !utf8.RuneStart(b[i]) {
			continue
		}
		r, sz := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError {
			return i + sz
		}
		return i
	}
	return 0
}

// lastSegmentBoundaryIndex returns the index of the last boundary rune in the
// given byte slice.
func lastSegmentBoundaryIndex(b []byte) int {
	it := lastRuneIndex(b)
	rs := []rune(string(b[:it]))

	n := 0

	for i, r := range slices.Backward(rs) {
		prev := '0'
		if i > 0 {
			prev = rs[i-1]
		}
		next := '0'
		if i < len(rs)-1 {
			next = rs[i+1]
			n += utf8.RuneLen(next)
		}
		switch r {
		case '.', ':', ',', '：':
			// This case handles 9.9 and 10:15
			if unicode.IsNumber(next) && unicode.IsNumber(prev) {
				continue
			}
			fallthrough
		case '，', '；', '。', '？', '！', '…', '～',
			'?', '!', '¿', '¡', ';', '~',
			'\r', '\n', '„', '・':
			return it - n
		}
	}
	return 0
}

// segmentBoundaryIndex returns the index of the first boundary rune in the
// given byte slice.
func segmentBoundaryIndex(b []byte) int {
	it := lastRuneIndex(b)
	rs := []rune(string(b[:it]))

	n := 0
	for i, r := range rs {
		n += utf8.RuneLen(r)
		prev := '0'
		if i > 0 {
			prev = rs[i-1]
		}
		next := '0'
		if i < len(rs)-1 {
			next = rs[i+1]
		}
		switch r {
		case '.', ':', ',', '：':
			// This case handles 9.9 and 10:15
			if unicode.IsNumber(next) && unicode.IsNumber(prev) {
				continue
			}
			fallthrough
		case '，', '；', '。', '？', '！', '…', '～',
			'?', '!', '¿', '¡', ';', '~',
			'\r', '\n', '„', '・':
			return n
		}
	}
	return 0
}
