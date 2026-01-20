// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// OGG media container writer, based on Pion WebRTC project.
package opusrt

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/codec/opus"
)

const (
	pageHeaderTypeContinuationOfStream = 0x00
	pageHeaderTypeBeginningOfStream    = 0x02
	pageHeaderTypeEndOfStream          = 0x04
	defaultPreSkip                     = 3840 // RFC 7845 §5.1 recommends 80ms pre-skip (3840 samples at 48kHz)
	idPageSignature                    = "OpusHead"
	commentPageSignature               = "OpusTags"
	pageHeaderSignature                = "OggS"
	pageHeaderSize                     = 27
)

var (
	errFileNotOpened = errors.New("opusrt: file not opened")
)

// OggWriter writes Opus frames to an OGG container.
type OggWriter struct {
	stream                  io.Writer
	sampleRate              uint32
	channelCount            uint16
	serial                  uint32
	pageIndex               uint32
	checksumTable           *[256]uint32
	previousGranulePosition uint64
	lastPayloadSize         int
}

// NewOggWriter creates a new OGG Opus writer.
func NewOggWriter(out io.Writer, sampleRate int, channelCount int) (*OggWriter, error) {
	if out == nil {
		return nil, errFileNotOpened
	}

	var serial uint32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &serial); err != nil {
		return nil, err
	}

	writer := &OggWriter{
		stream:        out,
		sampleRate:    uint32(sampleRate),
		channelCount:  uint16(channelCount),
		serial:        serial,
		checksumTable: generateChecksumTable(),
	}

	if err := writer.writeHeaders(); err != nil {
		return nil, err
	}

	return writer, nil
}

/*
ref: https://tools.ietf.org/html/rfc7845.html
https://git.xiph.org/?p=opus-tools.git;a=blob;f=src/opus_header.c#l219

	Page 0         Pages 1 ... n        Pages (n+1) ...

+------------+ +---+ +---+ ... +---+ +-----------+ +---------+ +--
|            | |   | |   |     |   | |           | |         | |
|+----------+| |+-----------------+| |+-------------------+ +-----
|||ID Header|| ||  Comment Header || ||Audio Data Packet 1| | ...
|+----------+| |+-----------------+| |+-------------------+ +-----
|            | |   | |   |     |   | |           | |         | |
+------------+ +---+ +---+ ... +---+ +-----------+ +---------+ +--
^      ^                           ^
|      |                           |
|      |                           Mandatory Page Break
|      |
|      ID header is contained on a single page
|
'Beginning Of Stream'

Figure 1: Example Packet Organization for a Logical Ogg Opus Stream
*/
func (w *OggWriter) writeHeaders() error {
	// ID Header
	oggIDHeader := make([]byte, 19)

	copy(oggIDHeader[0:], idPageSignature)                          // Magic Signature 'OpusHead'
	oggIDHeader[8] = 1                                              // Version
	oggIDHeader[9] = uint8(w.channelCount)                          // Channel count
	binary.LittleEndian.PutUint16(oggIDHeader[10:], defaultPreSkip) // pre-skip
	binary.LittleEndian.PutUint32(oggIDHeader[12:], w.sampleRate)   // original sample rate
	binary.LittleEndian.PutUint16(oggIDHeader[16:], 0)              // output gain
	oggIDHeader[18] = 0                                             // channel map 0 = one stream: mono or stereo

	data := w.createPage(oggIDHeader, pageHeaderTypeBeginningOfStream, 0, w.pageIndex)
	if err := w.writeToStream(data); err != nil {
		return err
	}
	w.pageIndex++

	// Comment Header (RFC 7845 §5.2)
	oggCommentHeader := make([]byte, 22)
	copy(oggCommentHeader[0:], commentPageSignature)        // Magic Signature 'OpusTags'
	binary.LittleEndian.PutUint32(oggCommentHeader[8:], 6)  // Vendor Length (UTF-8 string length, no null terminator per RFC)
	copy(oggCommentHeader[12:], "giztoy")                   // Vendor name (6 bytes)
	binary.LittleEndian.PutUint32(oggCommentHeader[18:], 0) // User Comment List Length

	data = w.createPage(oggCommentHeader, pageHeaderTypeContinuationOfStream, 0, w.pageIndex)
	if err := w.writeToStream(data); err != nil {
		return err
	}
	w.pageIndex++

	return nil
}

func (w *OggWriter) createPage(payload []uint8, headerType uint8, granulePos uint64, pageIndex uint32) []byte {
	w.lastPayloadSize = len(payload)
	nSegments := (len(payload) / 255) + 1

	page := make([]byte, pageHeaderSize+w.lastPayloadSize+nSegments)

	copy(page[0:], pageHeaderSignature)                 // 'OggS'
	page[4] = 0                                         // Version
	page[5] = headerType                                // Header type
	binary.LittleEndian.PutUint64(page[6:], granulePos) // Granule position
	binary.LittleEndian.PutUint32(page[14:], w.serial)  // Bitstream serial number
	binary.LittleEndian.PutUint32(page[18:], pageIndex) // Page sequence number
	page[26] = uint8(nSegments)                         // Number of segments

	// Fill segment table
	for i := 0; i < nSegments-1; i++ {
		page[pageHeaderSize+i] = 255
	}
	page[pageHeaderSize+nSegments-1] = uint8(len(payload) % 255)

	copy(page[pageHeaderSize+nSegments:], payload)

	// Calculate checksum
	var checksum uint32
	for index := range page {
		checksum = (checksum << 8) ^ w.checksumTable[byte(checksum>>24)^page[index]]
	}
	binary.LittleEndian.PutUint32(page[22:], checksum)

	return page
}

// ReadFrom reads all frames from a FrameReader and writes them to the OGG container.
func (w *OggWriter) ReadFrom(r FrameReader) (time.Duration, error) {
	var duration time.Duration
	for {
		frame, d, err := r.Frame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return duration, err
		}
		if err := w.Append(frame.Clone(), d); err != nil {
			return duration, err
		}
		if d == 0 {
			d = frame.Duration()
		}
		duration += d
	}
	return duration, nil
}

// Append writes a single frame to the OGG container.
// If loss > 0, the frame is skipped (TODO: handle loss).
func (w *OggWriter) Append(frame Frame, loss time.Duration) error {
	if loss > 0 {
		// TODO: handle loss (e.g., write silence)
		return nil
	}
	w.previousGranulePosition += uint64(opus.TOC(frame[0]).Configuration().PageGranuleIncrement())
	data := w.createPage(frame, pageHeaderTypeContinuationOfStream, w.previousGranulePosition, w.pageIndex)
	w.pageIndex++
	return w.writeToStream(data)
}

// SeekReaderAt combines Seeker and ReaderAt interfaces.
type SeekReaderAt interface {
	io.Seeker
	io.ReaderAt
}

// CloseWrite is the interface for half-closing a connection.
type CloseWrite interface {
	CloseWrite() error
}

// Close finalizes the OGG stream and closes the underlying writer if possible.
func (w *OggWriter) Close() error {
	defer func() {
		w.stream = nil
	}()

	// Try to update the last page with EOS flag
	if sr, ok := w.stream.(SeekReaderAt); ok {
		pageOffset, err := sr.Seek(-1*int64(w.lastPayloadSize+pageHeaderSize+1), 2)
		if err != nil {
			return err
		}

		payload := make([]byte, w.lastPayloadSize)
		if _, err := sr.ReadAt(payload, pageOffset+pageHeaderSize+1); err != nil {
			return err
		}

		data := w.createPage(payload, pageHeaderTypeEndOfStream, w.previousGranulePosition, w.pageIndex-1)
		if err := w.writeToStream(data); err != nil {
			return err
		}
	}

	if closer, ok := w.stream.(CloseWrite); ok {
		return closer.CloseWrite()
	}

	if closer, ok := w.stream.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

func (w *OggWriter) writeToStream(p []byte) error {
	if w.stream == nil {
		return errFileNotOpened
	}
	_, err := w.stream.Write(p)
	return err
}

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

// PCMToOgg encodes PCM data to an OGG Opus stream.
func PCMToOgg(w io.Writer, pcm []byte, sampleRate, channels int) error {
	opusStream, err := EncodePCMStream(bytes.NewReader(pcm), sampleRate, channels)
	if err != nil {
		return err
	}
	return OpusStreamToOgg(w, opusStream, sampleRate, channels)
}

// PCMStreamToOgg encodes a PCM stream to an OGG Opus stream.
func PCMStreamToOgg(w io.Writer, pcm io.Reader, sampleRate, channels int) error {
	opusStream, err := EncodePCMStream(pcm, sampleRate, channels)
	if err != nil {
		return err
	}
	return OpusStreamToOgg(w, opusStream, sampleRate, channels)
}

// OpusStreamToOgg writes an Opus frame stream to an OGG container.
func OpusStreamToOgg(w io.Writer, opusFrames FrameReader, sampleRate, channels int) error {
	ogg, err := NewOggWriter(w, sampleRate, channels)
	if err != nil {
		return err
	}
	defer ogg.Close()
	_, err = ogg.ReadFrom(opusFrames)
	return err
}
