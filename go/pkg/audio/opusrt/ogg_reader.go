package opusrt

import (
	"bytes"
	"io"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/codec/ogg"
)

// OggReader reads Opus frames from an OGG container.
type OggReader struct {
	decoder *ogg.Decoder
	stream  *ogg.StreamState
	page    ogg.Page
	packet  ogg.Packet
	eos     bool
}

// NewOggReader creates a new OGG Opus reader.
func NewOggReader(r io.Reader) *OggReader {
	return &OggReader{
		decoder: ogg.NewDecoder(r),
	}
}

// Frame returns the next Opus frame from the OGG container.
func (o *OggReader) Frame() (Frame, time.Duration, error) {
	for {
		if o.eos {
			return nil, 0, io.EOF
		}

		// Try to get a packet from the stream
		if o.stream != nil {
			err := o.stream.PacketOut(&o.packet)
			if err == nil {
				data := o.packet.Data()

				// Skip Opus headers (OpusHead, OpusTags)
				if isOpusHeader(data) {
					continue
				}

				// Check for end of stream
				if o.packet.EOS() {
					o.eos = true
				}

				return Frame(data), 0, nil
			}
			if err != ogg.ErrNoPacket {
				// Hole in data - continue
				continue
			}
		}

		// Need more data - get a page
		page, err := o.decoder.ReadPage()
		if err != nil {
			if err == io.EOF {
				o.eos = true
			}
			return nil, 0, err
		}

		// Initialize stream with serial number from first page
		if o.stream == nil {
			o.stream = ogg.NewStreamState(page.SerialNo())
		}

		// Submit page to stream
		if err := o.stream.PageIn(page); err != nil {
			return nil, 0, err
		}

		// Check for end of stream page
		if page.IsEOS() {
			o.eos = true
		}
	}
}

// Close releases resources.
func (o *OggReader) Close() error {
	if o.stream != nil {
		o.stream.Clear()
	}
	return o.decoder.Close()
}

// isOpusHeader checks if the packet is an OpusHead or OpusTags header.
func isOpusHeader(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.HasPrefix(data, []byte("OpusHead")) || bytes.HasPrefix(data, []byte("OpusTags"))
}

// OggTeeReader reads Opus frames and simultaneously writes them to an OGG file.
type OggTeeReader struct {
	w    io.Writer
	opus FrameReader

	readErr error
	ogg     *OggWriter
}

// NewOggTeeReader creates a reader that tees Opus frames to an OGG file.
func NewOggTeeReader(oggFile io.Writer, r FrameReader) *OggTeeReader {
	return &OggTeeReader{w: oggFile, opus: r}
}

// Frame returns the next frame, also writing it to the OGG file.
func (rd *OggTeeReader) Frame() (frame Frame, d time.Duration, err error) {
	defer func() {
		rd.readErr = err
		if err != nil && rd.ogg != nil {
			rd.ogg.Close()
		}
	}()

	if rd.readErr != nil {
		return nil, 0, rd.readErr
	}

	frame, d, err = rd.opus.Frame()
	if err != nil {
		return nil, 0, err
	}

	// Initialize OGG writer on first frame
	if rd.ogg == nil {
		var ch int
		if frame.TOC().IsStereo() {
			ch = 2
		} else {
			ch = 1
		}
		ow, err := NewOggWriter(rd.w, frame.TOC().Configuration().Bandwidth().SampleRate(), ch)
		if err != nil {
			return nil, 0, err
		}
		rd.ogg = ow
	}

	if err := rd.ogg.Append(frame.Clone(), d); err != nil {
		return nil, 0, err
	}

	return frame, d, nil
}
