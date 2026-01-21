package mqtt0

import (
	"bufio"
	"encoding/binary"
	"io"
)

// Maximum packet size (1MB default).
const MaxPacketSize = 1024 * 1024

// MQTT packet types.
const (
	PacketConnect     byte = 1
	PacketConnAck     byte = 2
	PacketPublish     byte = 3
	PacketPubAck      byte = 4
	PacketPubRec      byte = 5
	PacketPubRel      byte = 6
	PacketPubComp     byte = 7
	PacketSubscribe   byte = 8
	PacketSubAck      byte = 9
	PacketUnsubscribe byte = 10
	PacketUnsubAck    byte = 11
	PacketPingReq     byte = 12
	PacketPingResp    byte = 13
	PacketDisconnect  byte = 14
	PacketAuth        byte = 15 // MQTT 5.0 only
)

// PacketTypeName returns the name of a packet type.
func PacketTypeName(t byte) string {
	switch t {
	case PacketConnect:
		return "CONNECT"
	case PacketConnAck:
		return "CONNACK"
	case PacketPublish:
		return "PUBLISH"
	case PacketPubAck:
		return "PUBACK"
	case PacketPubRec:
		return "PUBREC"
	case PacketPubRel:
		return "PUBREL"
	case PacketPubComp:
		return "PUBCOMP"
	case PacketSubscribe:
		return "SUBSCRIBE"
	case PacketSubAck:
		return "SUBACK"
	case PacketUnsubscribe:
		return "UNSUBSCRIBE"
	case PacketUnsubAck:
		return "UNSUBACK"
	case PacketPingReq:
		return "PINGREQ"
	case PacketPingResp:
		return "PINGRESP"
	case PacketDisconnect:
		return "DISCONNECT"
	case PacketAuth:
		return "AUTH"
	default:
		return "UNKNOWN"
	}
}

// readFixedHeader reads the fixed header from a reader.
// Returns packet type, flags, remaining length, and error.
func readFixedHeader(r *bufio.Reader) (packetType byte, flags byte, remainingLength int, err error) {
	// First byte: packet type (4 bits) + flags (4 bits)
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, 0, err
	}
	packetType = b >> 4
	flags = b & 0x0F

	// Remaining length (variable length encoding)
	remainingLength, err = readVariableInt(r)
	if err != nil {
		return 0, 0, 0, err
	}

	return packetType, flags, remainingLength, nil
}

// readVariableInt reads a variable length integer from a reader.
func readVariableInt(r io.ByteReader) (int, error) {
	var value int
	var multiplier = 1

	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		value += int(b&0x7F) * multiplier

		if b&0x80 == 0 {
			return value, nil
		}

		multiplier *= 128
	}

	return 0, &ProtocolError{Message: "malformed variable length integer"}
}

// writeVariableInt writes a variable length integer to a writer.
func writeVariableInt(w io.Writer, value int) error {
	for {
		b := byte(value & 0x7F)
		value >>= 7
		if value > 0 {
			b |= 0x80
		}
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		if value == 0 {
			break
		}
	}
	return nil
}

// variableIntSize returns the size of a variable length integer.
func variableIntSize(value int) int {
	if value < 128 {
		return 1
	}
	if value < 16384 {
		return 2
	}
	if value < 2097152 {
		return 3
	}
	return 4
}

// readString reads a UTF-8 string from a reader.
func readString(r io.Reader) (string, error) {
	length, err := readUint16(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// writeString writes a UTF-8 string to a writer.
func writeString(w io.Writer, s string) error {
	if err := writeUint16(w, uint16(len(s))); err != nil {
		return err
	}
	if len(s) > 0 {
		if _, err := w.Write([]byte(s)); err != nil {
			return err
		}
	}
	return nil
}

// readBytes reads a length-prefixed byte slice from a reader.
func readBytes(r io.Reader) ([]byte, error) {
	length, err := readUint16(r)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeBytes writes a length-prefixed byte slice to a writer.
func writeBytes(w io.Writer, b []byte) error {
	if err := writeUint16(w, uint16(len(b))); err != nil {
		return err
	}
	if len(b) > 0 {
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}

// readUint16 reads a big-endian uint16 from a reader.
func readUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

// writeUint16 writes a big-endian uint16 to a writer.
func writeUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// readUint32 reads a big-endian uint32 from a reader.
func readUint32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

// writeUint32 writes a big-endian uint32 to a writer.
func writeUint32(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// readByte reads a single byte from a reader.
func readByte(r io.Reader) (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// writeByte writes a single byte to a writer.
func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}
