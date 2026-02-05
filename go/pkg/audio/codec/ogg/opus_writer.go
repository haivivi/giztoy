package ogg

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

const (
	pageHeaderTypeContinuation = 0x00
	pageHeaderTypeBOS          = 0x02
	pageHeaderTypeEOS          = 0x04
	defaultPreSkip             = 3840 // RFC 7845 §5.1 recommends 80ms pre-skip (3840 samples at 48kHz)
	idPageSignature            = "OpusHead"
	commentPageSignature       = "OpusTags"
	pageHeaderSignature        = "OggS"
	pageHeaderSize             = 27
)

var (
	// ErrWriterClosed is returned when writing to a closed writer.
	ErrWriterClosed = errors.New("ogg: writer is closed")
	// ErrNilWriter is returned when the underlying writer is nil.
	ErrNilWriter = errors.New("ogg: nil writer")
	// ErrInvalidSerialNo is returned when an invalid serial number is used.
	ErrInvalidSerialNo = errors.New("ogg: invalid serial number")
	// ErrStreamEnded is returned when writing to an ended stream.
	ErrStreamEnded = errors.New("ogg: stream already ended")
)

// opusStream represents a single Opus stream within the OGG container.
type opusStream struct {
	serialNo   int32
	sampleRate uint32
	channels   uint16
	granule    int64
	pageIndex  uint32
	ended      bool
}

// OpusWriter writes Opus frames to an OGG container.
// It supports multiple streams with different serial numbers.
type OpusWriter struct {
	mu            sync.Mutex
	w             io.Writer
	streams       map[int32]*opusStream
	defaultStream int32
	checksumTable *[256]uint32
	closed        bool
}

// NewOpusWriter creates a new OGG Opus writer with a default stream.
// The default stream uses a random serial number.
func NewOpusWriter(w io.Writer, sampleRate, channels int) (*OpusWriter, error) {
	if w == nil {
		return nil, ErrNilWriter
	}

	ow := &OpusWriter{
		w:             w,
		streams:       make(map[int32]*opusStream),
		checksumTable: generateChecksumTable(),
	}

	// Create default stream
	serialNo := ow.StreamBegin(sampleRate, channels)
	ow.defaultStream = serialNo

	return ow, nil
}

// StreamBegin creates a new stream with the given sample rate and channels.
// Returns the serial number assigned to this stream.
func (w *OpusWriter) StreamBegin(sampleRate, channels int) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()

	serialNo := w.generateSerialNo()
	stream := &opusStream{
		serialNo:   serialNo,
		sampleRate: uint32(sampleRate),
		channels:   uint16(channels),
		granule:    0,
		pageIndex:  0,
	}
	w.streams[serialNo] = stream

	// Write headers for this stream
	w.writeStreamHeaders(stream)

	return serialNo
}

// generateSerialNo generates a random serial number not already in use.
func (w *OpusWriter) generateSerialNo() int32 {
	for {
		var serialNo int32
		if err := binary.Read(rand.Reader, binary.LittleEndian, &serialNo); err != nil {
			// Fallback to simple increment
			serialNo = int32(len(w.streams) + 1)
		}
		if _, exists := w.streams[serialNo]; !exists {
			return serialNo
		}
	}
}

// writeStreamHeaders writes the OpusHead and OpusTags headers for a stream.
func (w *OpusWriter) writeStreamHeaders(stream *opusStream) error {
	// ID Header (OpusHead)
	idHeader := make([]byte, 19)
	copy(idHeader[0:], idPageSignature)                          // "OpusHead"
	idHeader[8] = 1                                               // Version
	idHeader[9] = uint8(stream.channels)                          // Channel count
	binary.LittleEndian.PutUint16(idHeader[10:], defaultPreSkip)  // Pre-skip
	binary.LittleEndian.PutUint32(idHeader[12:], stream.sampleRate) // Sample rate
	binary.LittleEndian.PutUint16(idHeader[16:], 0)               // Output gain
	idHeader[18] = 0                                               // Channel mapping

	page := w.createPage(idHeader, pageHeaderTypeBOS, 0, stream.pageIndex, stream.serialNo)
	if _, err := w.w.Write(page); err != nil {
		return err
	}
	stream.pageIndex++

	// Comment Header (OpusTags)
	commentHeader := make([]byte, 22)
	copy(commentHeader[0:], commentPageSignature)         // "OpusTags"
	binary.LittleEndian.PutUint32(commentHeader[8:], 6)   // Vendor length
	copy(commentHeader[12:], "giztoy")                     // Vendor name
	binary.LittleEndian.PutUint32(commentHeader[18:], 0)  // User comment list length

	page = w.createPage(commentHeader, pageHeaderTypeContinuation, 0, stream.pageIndex, stream.serialNo)
	if _, err := w.w.Write(page); err != nil {
		return err
	}
	stream.pageIndex++

	return nil
}

// Write writes an Opus frame to the default stream.
func (w *OpusWriter) Write(frame opus.Frame) error {
	return w.StreamWrite(w.defaultStream, frame)
}

// StreamWrite writes an Opus frame to the specified stream.
func (w *OpusWriter) StreamWrite(serialNo int32, frame opus.Frame) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	stream, ok := w.streams[serialNo]
	if !ok {
		return ErrInvalidSerialNo
	}
	if stream.ended {
		return ErrStreamEnded
	}

	// Calculate granule increment from frame duration (always at 48kHz for OGG Opus)
	samples := int64(frame.Duration() * 48000 / time.Second)
	stream.granule += samples

	page := w.createPage(frame, pageHeaderTypeContinuation, uint64(stream.granule), stream.pageIndex, stream.serialNo)
	if _, err := w.w.Write(page); err != nil {
		return err
	}
	stream.pageIndex++

	return nil
}

// Granule returns the current granule position of the default stream.
func (w *OpusWriter) Granule() int64 {
	return w.StreamGranule(w.defaultStream)
}

// StreamGranule returns the current granule position of the specified stream.
func (w *OpusWriter) StreamGranule(serialNo int32) int64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	stream, ok := w.streams[serialNo]
	if !ok {
		return 0
	}
	return stream.granule
}

// SetGranule sets the granule position of the default stream.
func (w *OpusWriter) SetGranule(g int64) {
	w.StreamSetGranule(w.defaultStream, g)
}

// StreamSetGranule sets the granule position of the specified stream.
func (w *OpusWriter) StreamSetGranule(serialNo int32, g int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	stream, ok := w.streams[serialNo]
	if !ok {
		return
	}
	stream.granule = g
}

// StreamEnd ends the specified stream by writing an EOS page.
func (w *OpusWriter) StreamEnd(serialNo int32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	stream, ok := w.streams[serialNo]
	if !ok {
		return ErrInvalidSerialNo
	}
	if stream.ended {
		return nil // Already ended
	}

	// Write an empty EOS page
	page := w.createPage(nil, pageHeaderTypeEOS, uint64(stream.granule), stream.pageIndex, stream.serialNo)
	if _, err := w.w.Write(page); err != nil {
		return err
	}
	stream.ended = true

	return nil
}

// Close ends all streams and closes the writer.
func (w *OpusWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	// End all streams that haven't been ended yet
	for serialNo, stream := range w.streams {
		if !stream.ended {
			page := w.createPage(nil, pageHeaderTypeEOS, uint64(stream.granule), stream.pageIndex, serialNo)
			w.w.Write(page) // Ignore error on close
			stream.ended = true
		}
	}

	w.closed = true

	// Close underlying writer if it implements io.Closer
	if closer, ok := w.w.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

// createPage creates an OGG page with the given payload.
func (w *OpusWriter) createPage(payload []byte, headerType uint8, granulePos uint64, pageIndex uint32, serialNo int32) []byte {
	payloadLen := len(payload)
	nSegments := 1
	if payloadLen > 0 {
		nSegments = (payloadLen / 255) + 1
	}

	page := make([]byte, pageHeaderSize+nSegments+payloadLen)

	copy(page[0:], pageHeaderSignature)                     // "OggS"
	page[4] = 0                                             // Version
	page[5] = headerType                                    // Header type
	binary.LittleEndian.PutUint64(page[6:], granulePos)     // Granule position
	binary.LittleEndian.PutUint32(page[14:], uint32(serialNo)) // Serial number
	binary.LittleEndian.PutUint32(page[18:], pageIndex)     // Page sequence number
	// page[22:26] is checksum, filled later
	page[26] = uint8(nSegments)                             // Number of segments

	// Fill segment table
	if payloadLen > 0 {
		for i := 0; i < nSegments-1; i++ {
			page[pageHeaderSize+i] = 255
		}
		page[pageHeaderSize+nSegments-1] = uint8(payloadLen % 255)
	} else {
		page[pageHeaderSize] = 0
	}

	// Copy payload
	copy(page[pageHeaderSize+nSegments:], payload)

	// Calculate and set checksum
	var checksum uint32
	for i := range page {
		checksum = (checksum << 8) ^ w.checksumTable[byte(checksum>>24)^page[i]]
	}
	binary.LittleEndian.PutUint32(page[22:], checksum)

	return page
}

// generateChecksumTable generates the CRC32 lookup table for OGG.
func generateChecksumTable() *[256]uint32 {
	var table [256]uint32
	const poly = 0x04c11db7

	for i := range table {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if (r & 0x80000000) != 0 {
				r = (r << 1) ^ poly
			} else {
				r <<= 1
			}
			table[i] = r & 0xffffffff
		}
	}
	return &table
}
