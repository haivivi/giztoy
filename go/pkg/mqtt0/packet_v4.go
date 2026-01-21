package mqtt0

import (
	"bufio"
	"bytes"
	"io"
)

// MQTT 3.1.1 Protocol Name and Level.
const (
	protocolNameV4  = "MQTT"
	protocolLevelV4 = 4
)

// V4Packet is the interface for MQTT 3.1.1 packets.
type V4Packet interface {
	packetType() byte
	encode() ([]byte, error)
}

// V4Connect represents a CONNECT packet (MQTT 3.1.1).
type V4Connect struct {
	ClientID     string
	Username     string
	Password     []byte
	CleanSession bool
	KeepAlive    uint16
	WillTopic    string
	WillMessage  []byte
	WillQoS      QoS
	WillRetain   bool
}

func (p *V4Connect) packetType() byte { return PacketConnect }

func (p *V4Connect) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Variable header
	// Protocol name
	if err := writeString(&buf, protocolNameV4); err != nil {
		return nil, err
	}
	// Protocol level
	if err := writeByte(&buf, protocolLevelV4); err != nil {
		return nil, err
	}

	// Connect flags
	var flags byte
	if p.CleanSession {
		flags |= 0x02
	}
	if p.WillTopic != "" {
		flags |= 0x04
		flags |= byte(p.WillQoS) << 3
		if p.WillRetain {
			flags |= 0x20
		}
	}
	if len(p.Password) > 0 {
		flags |= 0x40
	}
	if p.Username != "" {
		flags |= 0x80
	}
	if err := writeByte(&buf, flags); err != nil {
		return nil, err
	}

	// Keep alive
	if err := writeUint16(&buf, p.KeepAlive); err != nil {
		return nil, err
	}

	// Payload
	// Client ID
	if err := writeString(&buf, p.ClientID); err != nil {
		return nil, err
	}
	// Will
	if p.WillTopic != "" {
		if err := writeString(&buf, p.WillTopic); err != nil {
			return nil, err
		}
		if err := writeBytes(&buf, p.WillMessage); err != nil {
			return nil, err
		}
	}
	// Username
	if p.Username != "" {
		if err := writeString(&buf, p.Username); err != nil {
			return nil, err
		}
	}
	// Password
	if len(p.Password) > 0 {
		if err := writeBytes(&buf, p.Password); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketConnect, 0, buf.Bytes()), nil
}

// V4ConnAck represents a CONNACK packet (MQTT 3.1.1).
type V4ConnAck struct {
	SessionPresent bool
	ReturnCode     ConnectReturnCode
}

func (p *V4ConnAck) packetType() byte { return PacketConnAck }

func (p *V4ConnAck) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Acknowledge flags
	var flags byte
	if p.SessionPresent {
		flags |= 0x01
	}
	if err := writeByte(&buf, flags); err != nil {
		return nil, err
	}

	// Return code
	if err := writeByte(&buf, byte(p.ReturnCode)); err != nil {
		return nil, err
	}

	return encodePacket(PacketConnAck, 0, buf.Bytes()), nil
}

// V4Publish represents a PUBLISH packet (MQTT 3.1.1).
type V4Publish struct {
	Topic   string
	Payload []byte
	Retain  bool
	Dup     bool
	QoS     QoS
	PacketID uint16
}

func (p *V4Publish) packetType() byte { return PacketPublish }

func (p *V4Publish) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Topic
	if err := writeString(&buf, p.Topic); err != nil {
		return nil, err
	}

	// Packet ID (only for QoS > 0, but we only support QoS 0)
	if p.QoS > 0 {
		if err := writeUint16(&buf, p.PacketID); err != nil {
			return nil, err
		}
	}

	// Payload
	if _, err := buf.Write(p.Payload); err != nil {
		return nil, err
	}

	// Flags
	var flags byte
	if p.Dup {
		flags |= 0x08
	}
	flags |= byte(p.QoS) << 1
	if p.Retain {
		flags |= 0x01
	}

	return encodePacket(PacketPublish, flags, buf.Bytes()), nil
}

// V4Subscribe represents a SUBSCRIBE packet (MQTT 3.1.1).
type V4Subscribe struct {
	PacketID uint16
	Topics   []string
}

func (p *V4Subscribe) packetType() byte { return PacketSubscribe }

func (p *V4Subscribe) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Topics
	for _, topic := range p.Topics {
		if err := writeString(&buf, topic); err != nil {
			return nil, err
		}
		// QoS (always 0 for this implementation)
		if err := writeByte(&buf, 0); err != nil {
			return nil, err
		}
	}

	// SUBSCRIBE has fixed flags 0x02
	return encodePacket(PacketSubscribe, 0x02, buf.Bytes()), nil
}

// V4SubAck represents a SUBACK packet (MQTT 3.1.1).
type V4SubAck struct {
	PacketID    uint16
	ReturnCodes []byte // 0x00=QoS0, 0x01=QoS1, 0x02=QoS2, 0x80=Failure
}

func (p *V4SubAck) packetType() byte { return PacketSubAck }

func (p *V4SubAck) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Return codes
	if _, err := buf.Write(p.ReturnCodes); err != nil {
		return nil, err
	}

	return encodePacket(PacketSubAck, 0, buf.Bytes()), nil
}

// V4Unsubscribe represents an UNSUBSCRIBE packet (MQTT 3.1.1).
type V4Unsubscribe struct {
	PacketID uint16
	Topics   []string
}

func (p *V4Unsubscribe) packetType() byte { return PacketUnsubscribe }

func (p *V4Unsubscribe) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Topics
	for _, topic := range p.Topics {
		if err := writeString(&buf, topic); err != nil {
			return nil, err
		}
	}

	// UNSUBSCRIBE has fixed flags 0x02
	return encodePacket(PacketUnsubscribe, 0x02, buf.Bytes()), nil
}

// V4UnsubAck represents an UNSUBACK packet (MQTT 3.1.1).
type V4UnsubAck struct {
	PacketID uint16
}

func (p *V4UnsubAck) packetType() byte { return PacketUnsubAck }

func (p *V4UnsubAck) encode() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	return encodePacket(PacketUnsubAck, 0, buf.Bytes()), nil
}

// V4PingReq represents a PINGREQ packet.
type V4PingReq struct{}

func (p *V4PingReq) packetType() byte { return PacketPingReq }

func (p *V4PingReq) encode() ([]byte, error) {
	return encodePacket(PacketPingReq, 0, nil), nil
}

// V4PingResp represents a PINGRESP packet.
type V4PingResp struct{}

func (p *V4PingResp) packetType() byte { return PacketPingResp }

func (p *V4PingResp) encode() ([]byte, error) {
	return encodePacket(PacketPingResp, 0, nil), nil
}

// V4Disconnect represents a DISCONNECT packet.
type V4Disconnect struct{}

func (p *V4Disconnect) packetType() byte { return PacketDisconnect }

func (p *V4Disconnect) encode() ([]byte, error) {
	return encodePacket(PacketDisconnect, 0, nil), nil
}

// encodePacket creates the final packet bytes with fixed header.
func encodePacket(packetType byte, flags byte, payload []byte) []byte {
	remainingLength := len(payload)
	headerSize := 1 + variableIntSize(remainingLength)

	result := make([]byte, headerSize+remainingLength)
	result[0] = (packetType << 4) | (flags & 0x0F)

	// Write remaining length
	idx := 1
	for {
		b := byte(remainingLength & 0x7F)
		remainingLength >>= 7
		if remainingLength > 0 {
			b |= 0x80
		}
		result[idx] = b
		idx++
		if remainingLength == 0 {
			break
		}
	}

	// Copy payload
	copy(result[idx:], payload)

	return result
}

// ReadV4Packet reads a MQTT 3.1.1 packet from a buffered reader.
func ReadV4Packet(r *bufio.Reader, maxSize int) (V4Packet, error) {
	packetType, flags, remainingLength, err := readFixedHeader(r)
	if err != nil {
		return nil, err
	}

	if remainingLength > maxSize {
		return nil, ErrPacketTooLarge
	}

	// Read the remaining bytes
	payload := make([]byte, remainingLength)
	if remainingLength > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	pr := bytes.NewReader(payload)

	switch packetType {
	case PacketConnect:
		return decodeV4Connect(pr)
	case PacketConnAck:
		return decodeV4ConnAck(pr)
	case PacketPublish:
		return decodeV4Publish(pr, flags, remainingLength)
	case PacketSubAck:
		return decodeV4SubAck(pr, remainingLength)
	case PacketUnsubAck:
		return decodeV4UnsubAck(pr)
	case PacketSubscribe:
		return decodeV4Subscribe(pr, remainingLength)
	case PacketUnsubscribe:
		return decodeV4Unsubscribe(pr, remainingLength)
	case PacketPingReq:
		return &V4PingReq{}, nil
	case PacketPingResp:
		return &V4PingResp{}, nil
	case PacketDisconnect:
		return &V4Disconnect{}, nil
	default:
		return nil, &ProtocolError{Message: "unknown packet type"}
	}
}

func decodeV4Connect(r io.Reader) (*V4Connect, error) {
	// Protocol name
	protocolName, err := readString(r)
	if err != nil {
		return nil, err
	}
	if protocolName != protocolNameV4 {
		return nil, &ProtocolError{Message: "invalid protocol name"}
	}

	// Protocol level
	protocolLevel, err := readByte(r)
	if err != nil {
		return nil, err
	}
	if protocolLevel != protocolLevelV4 {
		return nil, &ProtocolError{Message: "unsupported protocol level"}
	}

	// Connect flags
	flags, err := readByte(r)
	if err != nil {
		return nil, err
	}

	cleanSession := flags&0x02 != 0
	willFlag := flags&0x04 != 0
	willQoS := QoS((flags >> 3) & 0x03)
	willRetain := flags&0x20 != 0
	passwordFlag := flags&0x40 != 0
	usernameFlag := flags&0x80 != 0

	// Keep alive
	keepAlive, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Client ID
	clientID, err := readString(r)
	if err != nil {
		return nil, err
	}

	p := &V4Connect{
		ClientID:     clientID,
		CleanSession: cleanSession,
		KeepAlive:    keepAlive,
	}

	// Will
	if willFlag {
		p.WillTopic, err = readString(r)
		if err != nil {
			return nil, err
		}
		p.WillMessage, err = readBytes(r)
		if err != nil {
			return nil, err
		}
		p.WillQoS = willQoS
		p.WillRetain = willRetain
	}

	// Username
	if usernameFlag {
		p.Username, err = readString(r)
		if err != nil {
			return nil, err
		}
	}

	// Password
	if passwordFlag {
		p.Password, err = readBytes(r)
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

func decodeV4ConnAck(r io.Reader) (*V4ConnAck, error) {
	// Acknowledge flags
	flags, err := readByte(r)
	if err != nil {
		return nil, err
	}

	// Return code
	returnCode, err := readByte(r)
	if err != nil {
		return nil, err
	}

	return &V4ConnAck{
		SessionPresent: flags&0x01 != 0,
		ReturnCode:     ConnectReturnCode(returnCode),
	}, nil
}

func decodeV4Publish(r io.Reader, flags byte, remainingLength int) (*V4Publish, error) {
	dup := flags&0x08 != 0
	qos := QoS((flags >> 1) & 0x03)
	retain := flags&0x01 != 0

	// Topic
	topic, err := readString(r)
	if err != nil {
		return nil, err
	}

	// Calculate payload length
	payloadLength := remainingLength - 2 - len(topic)

	// Packet ID (only for QoS > 0)
	var packetID uint16
	if qos > 0 {
		packetID, err = readUint16(r)
		if err != nil {
			return nil, err
		}
		payloadLength -= 2
	}

	// Payload
	var payload []byte
	if payloadLength > 0 {
		payload = make([]byte, payloadLength)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &V4Publish{
		Topic:    topic,
		Payload:  payload,
		Retain:   retain,
		Dup:      dup,
		QoS:      qos,
		PacketID: packetID,
	}, nil
}

func decodeV4Subscribe(r io.Reader, remainingLength int) (*V4Subscribe, error) {
	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Topics
	bytesRead := 2
	var topics []string
	for bytesRead < remainingLength {
		topic, err := readString(r)
		if err != nil {
			return nil, err
		}
		bytesRead += 2 + len(topic)

		// QoS (we ignore it, only support QoS 0)
		_, err = readByte(r)
		if err != nil {
			return nil, err
		}
		bytesRead++

		topics = append(topics, topic)
	}

	return &V4Subscribe{
		PacketID: packetID,
		Topics:   topics,
	}, nil
}

func decodeV4SubAck(r io.Reader, remainingLength int) (*V4SubAck, error) {
	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Return codes
	returnCodes := make([]byte, remainingLength-2)
	if _, err := io.ReadFull(r, returnCodes); err != nil {
		return nil, err
	}

	return &V4SubAck{
		PacketID:    packetID,
		ReturnCodes: returnCodes,
	}, nil
}

func decodeV4Unsubscribe(r io.Reader, remainingLength int) (*V4Unsubscribe, error) {
	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Topics
	bytesRead := 2
	var topics []string
	for bytesRead < remainingLength {
		topic, err := readString(r)
		if err != nil {
			return nil, err
		}
		bytesRead += 2 + len(topic)
		topics = append(topics, topic)
	}

	return &V4Unsubscribe{
		PacketID: packetID,
		Topics:   topics,
	}, nil
}

func decodeV4UnsubAck(r io.Reader) (*V4UnsubAck, error) {
	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	return &V4UnsubAck{
		PacketID: packetID,
	}, nil
}

// WriteV4Packet writes a MQTT 3.1.1 packet to a writer.
func WriteV4Packet(w io.Writer, p V4Packet) error {
	data, err := p.encode()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
