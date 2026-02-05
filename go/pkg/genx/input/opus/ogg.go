package opus

import (
	"bytes"
	"io"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/ogg"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// FromOggReader creates a genx.Stream from an OGG Opus container.
// Decodes the OGG container and emits individual Opus frames as MessageChunks.
//
// The returned Stream produces MessageChunks with MIMEType "audio/opus".
func FromOggReader(r io.Reader, role genx.Role, name string) genx.Stream {
	s := &oggStream{
		reader: r,
		role:   role,
		name:   name,
		chunks: buffer.N[*genx.MessageChunk](256),
	}

	go s.readLoop()

	return s
}

type oggStream struct {
	reader io.Reader
	role   genx.Role
	name   string
	chunks *buffer.Buffer[*genx.MessageChunk]
}

// Next returns the next MessageChunk from the stream.
func (s *oggStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.chunks.Next()
	if err != nil {
		if err == buffer.ErrIteratorDone {
			return nil, io.EOF
		}
		return nil, err
	}
	return chunk, nil
}

// Close closes the stream.
func (s *oggStream) Close() error {
	return s.chunks.Close()
}

// CloseWithError closes the stream with an error.
func (s *oggStream) CloseWithError(err error) error {
	return s.chunks.CloseWithError(err)
}

func (s *oggStream) readLoop() {
	defer s.chunks.CloseWrite()

	decoder, err := ogg.NewDecoder(s.reader)
	if err != nil {
		s.chunks.CloseWithError(err)
		return
	}
	defer decoder.Close()

	var stream *ogg.StreamState
	var eos bool

	for {
		// Try to get a packet from the stream
		if stream != nil && !eos {
			var packet ogg.Packet
			err := stream.PacketOut(&packet)
			if err == nil {
				data := packet.Data()

				// Skip Opus headers (OpusHead, OpusTags)
				if isOpusHeader(data) {
					continue
				}

				// Check for end of stream
				if packet.EOS() {
					eos = true
				}

				chunk := &genx.MessageChunk{
					Role: s.role,
					Name: s.name,
					Part: &genx.Blob{
						MIMEType: "audio/opus",
						Data:     OpusFrame(data).Clone(),
					},
				}

				if err := s.chunks.Add(chunk); err != nil {
					return
				}
				continue
			}
			if err != ogg.ErrNoPacket {
				// Hole in data - continue
				continue
			}
		}

		// Need more data - get a page
		page, err := decoder.ReadPage()
		if err != nil {
			if err != io.EOF {
				s.chunks.CloseWithError(err)
			}
			return
		}

		// Check if this is a new stream (BOS = Beginning Of Stream)
		if page.IsBOS() {
			// New stream starting - reset state
			if stream != nil {
				stream.Clear()
			}
			stream, err = ogg.NewStreamState(page.SerialNo())
			if err != nil {
				s.chunks.CloseWithError(err)
				return
			}
			eos = false
		} else if stream == nil {
			// Initialize stream with serial number from first page
			stream, err = ogg.NewStreamState(page.SerialNo())
			if err != nil {
				s.chunks.CloseWithError(err)
				return
			}
		}

		// Submit page to stream
		if err := stream.PageIn(page); err != nil {
			s.chunks.CloseWithError(err)
			return
		}

		// Check for end of stream page
		if page.IsEOS() {
			eos = true
		}
	}
}

// isOpusHeader checks if the packet is an OpusHead or OpusTags header.
func isOpusHeader(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.HasPrefix(data, []byte("OpusHead")) || bytes.HasPrefix(data, []byte("OpusTags"))
}
