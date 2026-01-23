package mqtt0

import (
	"bufio"
	"bytes"
	"io"
)

// MQTT 5.0 Protocol Name and Level.
const (
	protocolNameV5  = "MQTT"
	protocolLevelV5 = 5
)

// V5 Property identifiers.
const (
	propPayloadFormat          byte = 0x01
	propMessageExpiry          byte = 0x02
	propContentType            byte = 0x03
	propResponseTopic          byte = 0x08
	propCorrelationData        byte = 0x09
	propSubscriptionID         byte = 0x0B
	propSessionExpiry          byte = 0x11
	propAssignedClientID       byte = 0x12
	propServerKeepAlive        byte = 0x13
	propAuthMethod             byte = 0x15
	propAuthData               byte = 0x16
	propRequestProblemInfo     byte = 0x17
	propWillDelayInterval      byte = 0x18
	propRequestResponseInfo    byte = 0x19
	propResponseInfo           byte = 0x1A
	propServerReference        byte = 0x1C
	propReasonString           byte = 0x1F
	propReceiveMaximum         byte = 0x21
	propTopicAliasMaximum      byte = 0x22
	propTopicAlias             byte = 0x23
	propMaximumQoS             byte = 0x24
	propRetainAvailable        byte = 0x25
	propUserProperty           byte = 0x26
	propMaximumPacketSize      byte = 0x27
	propWildcardSubAvailable   byte = 0x28
	propSubIDAvailable         byte = 0x29
	propSharedSubAvailable     byte = 0x2A
)

// V5Properties contains MQTT 5.0 properties.
type V5Properties struct {
	SessionExpiry       *uint32
	ReceiveMaximum      *uint16
	MaximumQoS          *byte
	RetainAvailable     *bool
	MaximumPacketSize   *uint32
	AssignedClientID    string
	TopicAliasMaximum   *uint16
	TopicAlias          *uint16 // Topic Alias for PUBLISH packets
	ReasonString        string
	UserProperties      []UserProperty
	WildcardSubAvail    *bool
	SubIDAvail          *bool
	SharedSubAvail      *bool
	ServerKeepAlive     *uint16
	ResponseInfo        string
	ServerReference     string
	AuthMethod          string
	AuthData            []byte
	ContentType         string
	ResponseTopic       string
	CorrelationData     []byte
	SubscriptionID      *uint32
	WillDelayInterval   *uint32
	PayloadFormat       *byte
	MessageExpiry       *uint32
}

// UserProperty represents a MQTT 5.0 user property.
type UserProperty struct {
	Key   string
	Value string
}

// V5Packet is the interface for MQTT 5.0 packets.
type V5Packet interface {
	packetTypeV5() byte
	encodeV5() ([]byte, error)
}

// V5Connect represents a CONNECT packet (MQTT 5.0).
type V5Connect struct {
	ClientID     string
	Username     string
	Password     []byte
	CleanStart   bool
	KeepAlive    uint16
	Properties   *V5Properties
	WillTopic    string
	WillMessage  []byte
	WillQoS      QoS
	WillRetain   bool
	WillProps    *V5Properties
}

func (p *V5Connect) packetTypeV5() byte { return PacketConnect }

func (p *V5Connect) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Variable header
	// Protocol name
	if err := writeString(&buf, protocolNameV5); err != nil {
		return nil, err
	}
	// Protocol level
	if err := writeByte(&buf, protocolLevelV5); err != nil {
		return nil, err
	}

	// Connect flags
	var flags byte
	if p.CleanStart {
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

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	// Payload
	// Client ID
	if err := writeString(&buf, p.ClientID); err != nil {
		return nil, err
	}

	// Will properties and payload
	if p.WillTopic != "" {
		if err := encodeV5Properties(&buf, p.WillProps); err != nil {
			return nil, err
		}
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

// V5ConnAck represents a CONNACK packet (MQTT 5.0).
type V5ConnAck struct {
	SessionPresent bool
	ReasonCode     ReasonCode
	Properties     *V5Properties
}

func (p *V5ConnAck) packetTypeV5() byte { return PacketConnAck }

func (p *V5ConnAck) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Acknowledge flags
	var flags byte
	if p.SessionPresent {
		flags |= 0x01
	}
	if err := writeByte(&buf, flags); err != nil {
		return nil, err
	}

	// Reason code
	if err := writeByte(&buf, byte(p.ReasonCode)); err != nil {
		return nil, err
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	return encodePacket(PacketConnAck, 0, buf.Bytes()), nil
}

// V5Publish represents a PUBLISH packet (MQTT 5.0).
type V5Publish struct {
	Topic      string
	Payload    []byte
	Retain     bool
	Dup        bool
	QoS        QoS
	PacketID   uint16
	Properties *V5Properties
}

func (p *V5Publish) packetTypeV5() byte { return PacketPublish }

func (p *V5Publish) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Topic
	if err := writeString(&buf, p.Topic); err != nil {
		return nil, err
	}

	// Packet ID (only for QoS > 0)
	if p.QoS > 0 {
		if err := writeUint16(&buf, p.PacketID); err != nil {
			return nil, err
		}
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
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

// V5Subscribe represents a SUBSCRIBE packet (MQTT 5.0).
type V5Subscribe struct {
	PacketID   uint16
	Properties *V5Properties
	Topics     []V5SubscribeFilter
}

// V5SubscribeFilter represents a subscribe filter in MQTT 5.0.
type V5SubscribeFilter struct {
	Topic             string
	QoS               QoS
	NoLocal           bool
	RetainAsPublished bool
	RetainHandling    byte
}

func (p *V5Subscribe) packetTypeV5() byte { return PacketSubscribe }

func (p *V5Subscribe) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	// Topics
	for _, filter := range p.Topics {
		if err := writeString(&buf, filter.Topic); err != nil {
			return nil, err
		}
		// Subscription options
		var opts byte = byte(filter.QoS)
		if filter.NoLocal {
			opts |= 0x04
		}
		if filter.RetainAsPublished {
			opts |= 0x08
		}
		opts |= (filter.RetainHandling & 0x03) << 4
		if err := writeByte(&buf, opts); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketSubscribe, 0x02, buf.Bytes()), nil
}

// V5SubAck represents a SUBACK packet (MQTT 5.0).
type V5SubAck struct {
	PacketID    uint16
	Properties  *V5Properties
	ReasonCodes []ReasonCode
}

func (p *V5SubAck) packetTypeV5() byte { return PacketSubAck }

func (p *V5SubAck) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	// Reason codes
	for _, code := range p.ReasonCodes {
		if err := writeByte(&buf, byte(code)); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketSubAck, 0, buf.Bytes()), nil
}

// V5Unsubscribe represents an UNSUBSCRIBE packet (MQTT 5.0).
type V5Unsubscribe struct {
	PacketID   uint16
	Properties *V5Properties
	Topics     []string
}

func (p *V5Unsubscribe) packetTypeV5() byte { return PacketUnsubscribe }

func (p *V5Unsubscribe) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	// Topics
	for _, topic := range p.Topics {
		if err := writeString(&buf, topic); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketUnsubscribe, 0x02, buf.Bytes()), nil
}

// V5UnsubAck represents an UNSUBACK packet (MQTT 5.0).
type V5UnsubAck struct {
	PacketID    uint16
	Properties  *V5Properties
	ReasonCodes []ReasonCode
}

func (p *V5UnsubAck) packetTypeV5() byte { return PacketUnsubAck }

func (p *V5UnsubAck) encodeV5() ([]byte, error) {
	var buf bytes.Buffer

	// Packet ID
	if err := writeUint16(&buf, p.PacketID); err != nil {
		return nil, err
	}

	// Properties
	if err := encodeV5Properties(&buf, p.Properties); err != nil {
		return nil, err
	}

	// Reason codes
	for _, code := range p.ReasonCodes {
		if err := writeByte(&buf, byte(code)); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketUnsubAck, 0, buf.Bytes()), nil
}

// V5PingReq represents a PINGREQ packet.
type V5PingReq struct{}

func (p *V5PingReq) packetTypeV5() byte { return PacketPingReq }

func (p *V5PingReq) encodeV5() ([]byte, error) {
	return encodePacket(PacketPingReq, 0, nil), nil
}

// V5PingResp represents a PINGRESP packet.
type V5PingResp struct{}

func (p *V5PingResp) packetTypeV5() byte { return PacketPingResp }

func (p *V5PingResp) encodeV5() ([]byte, error) {
	return encodePacket(PacketPingResp, 0, nil), nil
}

// V5Disconnect represents a DISCONNECT packet (MQTT 5.0).
type V5Disconnect struct {
	ReasonCode ReasonCode
	Properties *V5Properties
}

func (p *V5Disconnect) packetTypeV5() byte { return PacketDisconnect }

func (p *V5Disconnect) encodeV5() ([]byte, error) {
	// If reason code is 0 and no properties, send empty packet
	if p.ReasonCode == ReasonSuccess && p.Properties == nil {
		return encodePacket(PacketDisconnect, 0, nil), nil
	}

	var buf bytes.Buffer

	// Reason code
	if err := writeByte(&buf, byte(p.ReasonCode)); err != nil {
		return nil, err
	}

	// Properties (only if present)
	if p.Properties != nil {
		if err := encodeV5Properties(&buf, p.Properties); err != nil {
			return nil, err
		}
	}

	return encodePacket(PacketDisconnect, 0, buf.Bytes()), nil
}

// encodeV5Properties encodes MQTT 5.0 properties.
func encodeV5Properties(w io.Writer, props *V5Properties) error {
	if props == nil {
		return writeVariableInt(w, 0)
	}

	var buf bytes.Buffer

	if props.SessionExpiry != nil {
		if err := writeByte(&buf, propSessionExpiry); err != nil {
			return err
		}
		if err := writeUint32(&buf, *props.SessionExpiry); err != nil {
			return err
		}
	}

	if props.ReceiveMaximum != nil {
		if err := writeByte(&buf, propReceiveMaximum); err != nil {
			return err
		}
		if err := writeUint16(&buf, *props.ReceiveMaximum); err != nil {
			return err
		}
	}

	if props.MaximumPacketSize != nil {
		if err := writeByte(&buf, propMaximumPacketSize); err != nil {
			return err
		}
		if err := writeUint32(&buf, *props.MaximumPacketSize); err != nil {
			return err
		}
	}

	if props.TopicAliasMaximum != nil {
		if err := writeByte(&buf, propTopicAliasMaximum); err != nil {
			return err
		}
		if err := writeUint16(&buf, *props.TopicAliasMaximum); err != nil {
			return err
		}
	}

	if props.ServerKeepAlive != nil {
		if err := writeByte(&buf, propServerKeepAlive); err != nil {
			return err
		}
		if err := writeUint16(&buf, *props.ServerKeepAlive); err != nil {
			return err
		}
	}

	if props.AssignedClientID != "" {
		if err := writeByte(&buf, propAssignedClientID); err != nil {
			return err
		}
		if err := writeString(&buf, props.AssignedClientID); err != nil {
			return err
		}
	}

	if props.ReasonString != "" {
		if err := writeByte(&buf, propReasonString); err != nil {
			return err
		}
		if err := writeString(&buf, props.ReasonString); err != nil {
			return err
		}
	}

	if props.ResponseInfo != "" {
		if err := writeByte(&buf, propResponseInfo); err != nil {
			return err
		}
		if err := writeString(&buf, props.ResponseInfo); err != nil {
			return err
		}
	}

	if props.ServerReference != "" {
		if err := writeByte(&buf, propServerReference); err != nil {
			return err
		}
		if err := writeString(&buf, props.ServerReference); err != nil {
			return err
		}
	}

	if props.AuthMethod != "" {
		if err := writeByte(&buf, propAuthMethod); err != nil {
			return err
		}
		if err := writeString(&buf, props.AuthMethod); err != nil {
			return err
		}
	}

	if props.AuthData != nil {
		if err := writeByte(&buf, propAuthData); err != nil {
			return err
		}
		if err := writeBytes(&buf, props.AuthData); err != nil {
			return err
		}
	}

	if props.ContentType != "" {
		if err := writeByte(&buf, propContentType); err != nil {
			return err
		}
		if err := writeString(&buf, props.ContentType); err != nil {
			return err
		}
	}

	if props.ResponseTopic != "" {
		if err := writeByte(&buf, propResponseTopic); err != nil {
			return err
		}
		if err := writeString(&buf, props.ResponseTopic); err != nil {
			return err
		}
	}

	if props.CorrelationData != nil {
		if err := writeByte(&buf, propCorrelationData); err != nil {
			return err
		}
		if err := writeBytes(&buf, props.CorrelationData); err != nil {
			return err
		}
	}

	if props.SubscriptionID != nil {
		if err := writeByte(&buf, propSubscriptionID); err != nil {
			return err
		}
		if err := writeVariableInt(&buf, int(*props.SubscriptionID)); err != nil {
			return err
		}
	}

	if props.WillDelayInterval != nil {
		if err := writeByte(&buf, propWillDelayInterval); err != nil {
			return err
		}
		if err := writeUint32(&buf, *props.WillDelayInterval); err != nil {
			return err
		}
	}

	if props.PayloadFormat != nil {
		if err := writeByte(&buf, propPayloadFormat); err != nil {
			return err
		}
		if err := writeByte(&buf, *props.PayloadFormat); err != nil {
			return err
		}
	}

	if props.MessageExpiry != nil {
		if err := writeByte(&buf, propMessageExpiry); err != nil {
			return err
		}
		if err := writeUint32(&buf, *props.MessageExpiry); err != nil {
			return err
		}
	}

	if props.MaximumQoS != nil {
		if err := writeByte(&buf, propMaximumQoS); err != nil {
			return err
		}
		if err := writeByte(&buf, *props.MaximumQoS); err != nil {
			return err
		}
	}

	if props.RetainAvailable != nil {
		if err := writeByte(&buf, propRetainAvailable); err != nil {
			return err
		}
		b := byte(0)
		if *props.RetainAvailable {
			b = 1
		}
		if err := writeByte(&buf, b); err != nil {
			return err
		}
	}

	if props.WildcardSubAvail != nil {
		if err := writeByte(&buf, propWildcardSubAvailable); err != nil {
			return err
		}
		b := byte(0)
		if *props.WildcardSubAvail {
			b = 1
		}
		if err := writeByte(&buf, b); err != nil {
			return err
		}
	}

	if props.SubIDAvail != nil {
		if err := writeByte(&buf, propSubIDAvailable); err != nil {
			return err
		}
		b := byte(0)
		if *props.SubIDAvail {
			b = 1
		}
		if err := writeByte(&buf, b); err != nil {
			return err
		}
	}

	if props.SharedSubAvail != nil {
		if err := writeByte(&buf, propSharedSubAvailable); err != nil {
			return err
		}
		b := byte(0)
		if *props.SharedSubAvail {
			b = 1
		}
		if err := writeByte(&buf, b); err != nil {
			return err
		}
	}

	for _, up := range props.UserProperties {
		if err := writeByte(&buf, propUserProperty); err != nil {
			return err
		}
		if err := writeString(&buf, up.Key); err != nil {
			return err
		}
		if err := writeString(&buf, up.Value); err != nil {
			return err
		}
	}

	// Write property length
	if err := writeVariableInt(w, buf.Len()); err != nil {
		return err
	}

	// Write properties
	_, err := w.Write(buf.Bytes())
	return err
}

// decodeV5Properties decodes MQTT 5.0 properties.
func decodeV5Properties(r io.Reader) (*V5Properties, error) {
	length, err := readVariableIntFromReader(r)
	if err != nil {
		return nil, err
	}

	if length == 0 {
		return nil, nil
	}

	// Read all property bytes
	propBytes := make([]byte, length)
	if _, err := io.ReadFull(r, propBytes); err != nil {
		return nil, err
	}

	props := &V5Properties{}
	pr := bytes.NewReader(propBytes)

	for pr.Len() > 0 {
		propID, err := readByte(pr)
		if err != nil {
			return nil, err
		}

		switch propID {
		case propSessionExpiry:
			v, err := readUint32(pr)
			if err != nil {
				return nil, err
			}
			props.SessionExpiry = &v
		case propReceiveMaximum:
			v, err := readUint16(pr)
			if err != nil {
				return nil, err
			}
			props.ReceiveMaximum = &v
		case propMaximumQoS:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			props.MaximumQoS = &v
		case propRetainAvailable:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			b := v != 0
			props.RetainAvailable = &b
		case propMaximumPacketSize:
			v, err := readUint32(pr)
			if err != nil {
				return nil, err
			}
			props.MaximumPacketSize = &v
		case propAssignedClientID:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.AssignedClientID = v
		case propTopicAliasMaximum:
			v, err := readUint16(pr)
			if err != nil {
				return nil, err
			}
			props.TopicAliasMaximum = &v
		case propTopicAlias:
			v, err := readUint16(pr)
			if err != nil {
				return nil, err
			}
			props.TopicAlias = &v
		case propReasonString:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.ReasonString = v
		case propUserProperty:
			key, err := readString(pr)
			if err != nil {
				return nil, err
			}
			value, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.UserProperties = append(props.UserProperties, UserProperty{Key: key, Value: value})
		case propWildcardSubAvailable:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			b := v != 0
			props.WildcardSubAvail = &b
		case propSubIDAvailable:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			b := v != 0
			props.SubIDAvail = &b
		case propSharedSubAvailable:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			b := v != 0
			props.SharedSubAvail = &b
		case propServerKeepAlive:
			v, err := readUint16(pr)
			if err != nil {
				return nil, err
			}
			props.ServerKeepAlive = &v
		case propResponseInfo:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.ResponseInfo = v
		case propServerReference:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.ServerReference = v
		case propAuthMethod:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.AuthMethod = v
		case propAuthData:
			v, err := readBytes(pr)
			if err != nil {
				return nil, err
			}
			props.AuthData = v
		case propContentType:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.ContentType = v
		case propResponseTopic:
			v, err := readString(pr)
			if err != nil {
				return nil, err
			}
			props.ResponseTopic = v
		case propCorrelationData:
			v, err := readBytes(pr)
			if err != nil {
				return nil, err
			}
			props.CorrelationData = v
		case propSubscriptionID:
			v, err := readVariableIntFromReader(pr)
			if err != nil {
				return nil, err
			}
			u := uint32(v)
			props.SubscriptionID = &u
		case propWillDelayInterval:
			v, err := readUint32(pr)
			if err != nil {
				return nil, err
			}
			props.WillDelayInterval = &v
		case propPayloadFormat:
			v, err := readByte(pr)
			if err != nil {
				return nil, err
			}
			props.PayloadFormat = &v
		case propMessageExpiry:
			v, err := readUint32(pr)
			if err != nil {
				return nil, err
			}
			props.MessageExpiry = &v
		default:
			// Unknown property, skip
			return nil, &ProtocolError{Message: "unknown property identifier"}
		}
	}

	return props, nil
}

// readVariableIntFromReader reads a variable length integer from an io.Reader.
func readVariableIntFromReader(r io.Reader) (int, error) {
	var value int
	var multiplier = 1

	for i := 0; i < 4; i++ {
		b, err := readByte(r)
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

// ReadV5Packet reads a MQTT 5.0 packet from a buffered reader.
func ReadV5Packet(r *bufio.Reader, maxSize int) (V5Packet, error) {
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
		return decodeV5Connect(pr)
	case PacketConnAck:
		return decodeV5ConnAck(pr)
	case PacketPublish:
		return decodeV5Publish(pr, flags, remainingLength)
	case PacketSubAck:
		return decodeV5SubAck(pr, remainingLength)
	case PacketUnsubAck:
		return decodeV5UnsubAck(pr, remainingLength)
	case PacketSubscribe:
		return decodeV5Subscribe(pr, remainingLength)
	case PacketUnsubscribe:
		return decodeV5Unsubscribe(pr, remainingLength)
	case PacketPingReq:
		return &V5PingReq{}, nil
	case PacketPingResp:
		return &V5PingResp{}, nil
	case PacketDisconnect:
		return decodeV5Disconnect(pr, remainingLength)
	default:
		return nil, &ProtocolError{Message: "unknown packet type"}
	}
}

func decodeV5Connect(r io.Reader) (*V5Connect, error) {
	// Protocol name
	protocolName, err := readString(r)
	if err != nil {
		return nil, err
	}
	if protocolName != protocolNameV5 {
		return nil, &ProtocolError{Message: "invalid protocol name"}
	}

	// Protocol level
	protocolLevel, err := readByte(r)
	if err != nil {
		return nil, err
	}
	if protocolLevel != protocolLevelV5 {
		return nil, &ProtocolError{Message: "unsupported protocol level"}
	}

	// Connect flags
	flags, err := readByte(r)
	if err != nil {
		return nil, err
	}

	cleanStart := flags&0x02 != 0
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

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Client ID
	clientID, err := readString(r)
	if err != nil {
		return nil, err
	}

	p := &V5Connect{
		ClientID:   clientID,
		CleanStart: cleanStart,
		KeepAlive:  keepAlive,
		Properties: props,
	}

	// Will
	if willFlag {
		p.WillProps, err = decodeV5Properties(r)
		if err != nil {
			return nil, err
		}
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

func decodeV5ConnAck(r io.Reader) (*V5ConnAck, error) {
	// Acknowledge flags
	flags, err := readByte(r)
	if err != nil {
		return nil, err
	}

	// Reason code
	reasonCode, err := readByte(r)
	if err != nil {
		return nil, err
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	return &V5ConnAck{
		SessionPresent: flags&0x01 != 0,
		ReasonCode:     ReasonCode(reasonCode),
		Properties:     props,
	}, nil
}

func decodeV5Publish(r *bytes.Reader, flags byte, remainingLength int) (*V5Publish, error) {
	startLen := r.Len()

	dup := flags&0x08 != 0
	qos := QoS((flags >> 1) & 0x03)
	retain := flags&0x01 != 0

	// Topic
	topic, err := readString(r)
	if err != nil {
		return nil, err
	}

	// Packet ID (only for QoS > 0)
	var packetID uint16
	if qos > 0 {
		packetID, err = readUint16(r)
		if err != nil {
			return nil, err
		}
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Calculate bytes read so far
	bytesRead := startLen - r.Len()
	payloadLength := remainingLength - bytesRead

	// Payload
	var payload []byte
	if payloadLength > 0 {
		payload = make([]byte, payloadLength)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &V5Publish{
		Topic:      topic,
		Payload:    payload,
		Retain:     retain,
		Dup:        dup,
		QoS:        qos,
		PacketID:   packetID,
		Properties: props,
	}, nil
}

func decodeV5Subscribe(r *bytes.Reader, remainingLength int) (*V5Subscribe, error) {
	startLen := r.Len()

	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Topics
	var filters []V5SubscribeFilter
	for r.Len() > 0 {
		topic, err := readString(r)
		if err != nil {
			return nil, err
		}

		opts, err := readByte(r)
		if err != nil {
			return nil, err
		}

		filters = append(filters, V5SubscribeFilter{
			Topic:             topic,
			QoS:               QoS(opts & 0x03),
			NoLocal:           opts&0x04 != 0,
			RetainAsPublished: opts&0x08 != 0,
			RetainHandling:    (opts >> 4) & 0x03,
		})
	}

	_ = startLen
	_ = remainingLength

	return &V5Subscribe{
		PacketID:   packetID,
		Properties: props,
		Topics:     filters,
	}, nil
}

func decodeV5SubAck(r *bytes.Reader, remainingLength int) (*V5SubAck, error) {
	startLen := r.Len()

	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Calculate remaining for reason codes
	bytesRead := startLen - r.Len()
	reasonCodeCount := remainingLength - bytesRead

	// Reason codes
	reasonCodes := make([]ReasonCode, reasonCodeCount)
	for i := 0; i < reasonCodeCount; i++ {
		code, err := readByte(r)
		if err != nil {
			return nil, err
		}
		reasonCodes[i] = ReasonCode(code)
	}

	return &V5SubAck{
		PacketID:    packetID,
		Properties:  props,
		ReasonCodes: reasonCodes,
	}, nil
}

func decodeV5Unsubscribe(r *bytes.Reader, remainingLength int) (*V5Unsubscribe, error) {
	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Topics
	var topics []string
	for r.Len() > 0 {
		topic, err := readString(r)
		if err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	_ = remainingLength

	return &V5Unsubscribe{
		PacketID:   packetID,
		Properties: props,
		Topics:     topics,
	}, nil
}

func decodeV5UnsubAck(r *bytes.Reader, remainingLength int) (*V5UnsubAck, error) {
	startLen := r.Len()

	// Packet ID
	packetID, err := readUint16(r)
	if err != nil {
		return nil, err
	}

	// Properties
	props, err := decodeV5Properties(r)
	if err != nil {
		return nil, err
	}

	// Calculate remaining for reason codes
	bytesRead := startLen - r.Len()
	reasonCodeCount := remainingLength - bytesRead

	// Reason codes
	reasonCodes := make([]ReasonCode, reasonCodeCount)
	for i := 0; i < reasonCodeCount; i++ {
		code, err := readByte(r)
		if err != nil {
			return nil, err
		}
		reasonCodes[i] = ReasonCode(code)
	}

	return &V5UnsubAck{
		PacketID:    packetID,
		Properties:  props,
		ReasonCodes: reasonCodes,
	}, nil
}

func decodeV5Disconnect(r *bytes.Reader, remainingLength int) (*V5Disconnect, error) {
	if remainingLength == 0 {
		return &V5Disconnect{ReasonCode: ReasonNormalDisconnection}, nil
	}

	// Reason code
	reasonCode, err := readByte(r)
	if err != nil {
		return nil, err
	}

	// Properties (if remaining length > 1)
	var props *V5Properties
	if remainingLength > 1 {
		props, err = decodeV5Properties(r)
		if err != nil {
			return nil, err
		}
	}

	return &V5Disconnect{
		ReasonCode: ReasonCode(reasonCode),
		Properties: props,
	}, nil
}

// WriteV5Packet writes a MQTT 5.0 packet to a writer.
func WriteV5Packet(w io.Writer, p V5Packet) error {
	data, err := p.encodeV5()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
