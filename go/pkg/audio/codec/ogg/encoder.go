package ogg

/*
#include <ogg/ogg.h>
*/
import "C"
import (
	"crypto/rand"
	"encoding/binary"
	"io"
)

// Encoder writes Ogg pages to an io.Writer.
type Encoder struct {
	w        io.Writer
	stream   *StreamState
	packetNo int64
}

// NewEncoder creates a new Ogg encoder with a random serial number.
func NewEncoder(w io.Writer) (*Encoder, error) {
	var serialNo int32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &serialNo); err != nil {
		return nil, err
	}
	return NewEncoderWithSerial(w, serialNo), nil
}

// NewEncoderWithSerial creates a new Ogg encoder with a specific serial number.
func NewEncoderWithSerial(w io.Writer, serialNo int32) *Encoder {
	return &Encoder{
		w:      w,
		stream: NewStreamState(serialNo),
	}
}

// SerialNo returns the stream serial number.
func (e *Encoder) SerialNo() int32 {
	return e.stream.SerialNo()
}

// WritePacket writes a packet to the stream.
// Set bos=true for beginning of stream, eos=true for end of stream.
func (e *Encoder) WritePacket(data []byte, granulePos int64, bos, eos bool) error {
	if err := e.stream.PacketIn(data, granulePos, e.packetNo, bos, eos); err != nil {
		return err
	}
	e.packetNo++

	// Try to output pages
	for {
		header, body, err := e.stream.PageOut()
		if err == ErrNoPacket {
			break
		}
		if err != nil {
			return err
		}
		if _, err := e.w.Write(header); err != nil {
			return err
		}
		if _, err := e.w.Write(body); err != nil {
			return err
		}
	}

	return nil
}

// Flush forces any remaining packets into pages.
func (e *Encoder) Flush() error {
	for {
		header, body, err := e.stream.Flush()
		if err == ErrNoPacket {
			break
		}
		if err != nil {
			return err
		}
		if _, err := e.w.Write(header); err != nil {
			return err
		}
		if _, err := e.w.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes and releases resources.
func (e *Encoder) Close() error {
	err := e.Flush()
	e.stream.Clear()
	return err
}

// PacketWriter is a convenience wrapper that handles granule position tracking.
type PacketWriter struct {
	enc         *Encoder
	granulePos  int64
	granuleInc  int64 // granules per packet
	firstPacket bool
}

// NewPacketWriter creates a PacketWriter with automatic granule position tracking.
func NewPacketWriter(w io.Writer, granuleIncrement int64) (*PacketWriter, error) {
	enc, err := NewEncoder(w)
	if err != nil {
		return nil, err
	}
	return &PacketWriter{
		enc:         enc,
		granuleInc:  granuleIncrement,
		firstPacket: true,
	}, nil
}

// WriteHeader writes a header packet (BOS).
func (pw *PacketWriter) WriteHeader(data []byte) error {
	err := pw.enc.WritePacket(data, 0, pw.firstPacket, false)
	pw.firstPacket = false
	return err
}

// Write writes a data packet.
func (pw *PacketWriter) Write(data []byte) error {
	pw.granulePos += pw.granuleInc
	return pw.enc.WritePacket(data, pw.granulePos, false, false)
}

// WriteEOS writes the final packet with EOS flag.
func (pw *PacketWriter) WriteEOS(data []byte) error {
	pw.granulePos += pw.granuleInc
	return pw.enc.WritePacket(data, pw.granulePos, false, true)
}

// Flush flushes pending data.
func (pw *PacketWriter) Flush() error {
	return pw.enc.Flush()
}

// Close closes the writer.
func (pw *PacketWriter) Close() error {
	return pw.enc.Close()
}

// SerialNo returns the stream serial number.
func (pw *PacketWriter) SerialNo() int32 {
	return pw.enc.SerialNo()
}
