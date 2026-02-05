package ogg

import (
	"bytes"
	"io"
	"iter"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// ReadOpusPackets reads Opus packets from an OGG container.
// It returns an iterator that yields OpusPacket and error pairs.
// The caller is responsible for closing the underlying io.Reader.
//
// The iterator supports multiple streams (multiplexed or chained).
// Each packet includes SerialNo to identify which stream it belongs to.
// BOS and EOS flags indicate stream boundaries.
//
// Example:
//
//	for pkt, err := range ogg.ReadOpusPackets(file) {
//	    if err != nil {
//	        return err
//	    }
//	    // process pkt.Frame
//	}
func ReadOpusPackets(r io.Reader) iter.Seq2[*OpusPacket, error] {
	return func(yield func(*OpusPacket, error) bool) {
		decoder, err := NewDecoder(r)
		if err != nil {
			yield(nil, err)
			return
		}
		defer decoder.Close()

		// Map of serial number to stream state
		streams := make(map[int32]*StreamState)
		defer func() {
			for _, s := range streams {
				s.Clear()
			}
		}()

		var packet Packet

		for {
			// Read next page
			page, err := decoder.ReadPage()
			if err != nil {
				if err == io.EOF {
					return
				}
				if !yield(nil, err) {
					return
				}
				continue
			}

			serialNo := page.SerialNo()

			// Handle BOS - create new stream
			if page.IsBOS() {
				// Clean up old stream with same serial if exists
				if old, exists := streams[serialNo]; exists {
					old.Clear()
				}
				stream, err := NewStreamState(serialNo)
				if err != nil {
					if !yield(nil, err) {
						return
					}
					continue
				}
				streams[serialNo] = stream
			}

			// Get or create stream state
			stream := streams[serialNo]
			if stream == nil {
				var err error
				stream, err = NewStreamState(serialNo)
				if err != nil {
					if !yield(nil, err) {
						return
					}
					continue
				}
				streams[serialNo] = stream
			}

			// Submit page to stream
			if err := stream.PageIn(page); err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}

			// Extract all packets from this page
			for {
				err := stream.PacketOut(&packet)
				if err == ErrNoPacket {
					break
				}
				if err == ErrHole {
					// Gap in data, continue
					continue
				}
				if err != nil {
					if !yield(nil, err) {
						return
					}
					break
				}

				data := packet.Data()

				// Skip Opus headers (OpusHead, OpusTags)
				if isOpusHeader(data) {
					continue
				}

				// Skip empty packets (e.g., EOS-only pages)
				if len(data) == 0 {
					continue
				}

				pkt := &OpusPacket{
					Frame:    opus.Frame(data),
					Granule:  packet.GranulePos(),
					SerialNo: serialNo,
					BOS:      packet.BOS(),
					EOS:      packet.EOS(),
				}

				if !yield(pkt, nil) {
					return
				}
			}
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
