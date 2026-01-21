package mqtt0

// ProtocolVersion represents the MQTT protocol version.
type ProtocolVersion byte

const (
	// ProtocolV4 is MQTT 3.1.1.
	ProtocolV4 ProtocolVersion = 4
	// ProtocolV5 is MQTT 5.0.
	ProtocolV5 ProtocolVersion = 5
)

func (v ProtocolVersion) String() string {
	switch v {
	case ProtocolV4:
		return "MQTT 3.1.1"
	case ProtocolV5:
		return "MQTT 5.0"
	default:
		return "Unknown"
	}
}

// QoS represents the MQTT Quality of Service level.
// This package only supports QoS 0.
type QoS byte

const (
	// AtMostOnce is QoS 0 - fire and forget.
	AtMostOnce QoS = 0
)

// Message represents an MQTT message.
type Message struct {
	// Topic is the message topic.
	Topic string
	// Payload is the message payload.
	Payload []byte
	// Retain indicates if this is a retained message.
	Retain bool
}

// Authenticator provides authentication and ACL for MQTT clients.
type Authenticator interface {
	// Authenticate validates client credentials.
	// Called when a client sends CONNECT packet.
	// Return true to allow the connection.
	Authenticate(clientID, username string, password []byte) bool

	// ACL checks publish/subscribe permissions.
	// Called when a client publishes or subscribes.
	//   - write=true: client is publishing to the topic
	//   - write=false: client is subscribing to the topic
	//
	// Return true to allow the operation.
	ACL(clientID, topic string, write bool) bool
}

// AllowAll is an authenticator that allows all connections and operations.
type AllowAll struct{}

// Authenticate always returns true.
func (AllowAll) Authenticate(clientID, username string, password []byte) bool {
	return true
}

// ACL always returns true.
func (AllowAll) ACL(clientID, topic string, write bool) bool {
	return true
}

// Handler handles incoming messages on the broker.
type Handler interface {
	// HandleMessage is called for every message received by the broker,
	// after it has been routed to subscribers.
	HandleMessage(clientID string, msg *Message)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(clientID string, msg *Message)

// HandleMessage calls f(clientID, msg).
func (f HandlerFunc) HandleMessage(clientID string, msg *Message) {
	f(clientID, msg)
}

// ConnectReturnCode represents the connection return code for MQTT 3.1.1.
type ConnectReturnCode byte

const (
	ConnectAccepted          ConnectReturnCode = 0x00
	ConnectBadProtocol       ConnectReturnCode = 0x01
	ConnectIDRejected        ConnectReturnCode = 0x02
	ConnectServerUnavailable ConnectReturnCode = 0x03
	ConnectBadCredentials    ConnectReturnCode = 0x04
	ConnectNotAuthorized     ConnectReturnCode = 0x05
)

func (c ConnectReturnCode) String() string {
	switch c {
	case ConnectAccepted:
		return "Connection Accepted"
	case ConnectBadProtocol:
		return "Unacceptable Protocol Version"
	case ConnectIDRejected:
		return "Identifier Rejected"
	case ConnectServerUnavailable:
		return "Server Unavailable"
	case ConnectBadCredentials:
		return "Bad User Name or Password"
	case ConnectNotAuthorized:
		return "Not Authorized"
	default:
		return "Unknown"
	}
}

// ReasonCode represents the reason code for MQTT 5.0.
type ReasonCode byte

// MQTT 5.0 reason codes.
const (
	ReasonSuccess                     ReasonCode = 0x00
	ReasonNormalDisconnection         ReasonCode = 0x00
	ReasonGrantedQoS0                 ReasonCode = 0x00
	ReasonUnspecifiedError            ReasonCode = 0x80
	ReasonMalformedPacket             ReasonCode = 0x81
	ReasonProtocolError               ReasonCode = 0x82
	ReasonImplementationError         ReasonCode = 0x83
	ReasonUnsupportedProtocolVersion  ReasonCode = 0x84
	ReasonClientIDNotValid            ReasonCode = 0x85
	ReasonBadUserNameOrPassword       ReasonCode = 0x86
	ReasonNotAuthorized               ReasonCode = 0x87
	ReasonServerUnavailable           ReasonCode = 0x88
	ReasonServerBusy                  ReasonCode = 0x89
	ReasonBanned                      ReasonCode = 0x8A
	ReasonBadAuthMethod               ReasonCode = 0x8C
	ReasonTopicFilterInvalid          ReasonCode = 0x8F
	ReasonTopicNameInvalid            ReasonCode = 0x90
	ReasonPacketIDInUse               ReasonCode = 0x91
	ReasonPacketIDNotFound            ReasonCode = 0x92
	ReasonReceiveMaximumExceeded      ReasonCode = 0x93
	ReasonTopicAliasInvalid           ReasonCode = 0x94
	ReasonPacketTooLarge              ReasonCode = 0x95
	ReasonMessageRateTooHigh          ReasonCode = 0x96
	ReasonQuotaExceeded               ReasonCode = 0x97
	ReasonAdministrativeAction        ReasonCode = 0x98
	ReasonPayloadFormatInvalid        ReasonCode = 0x99
	ReasonRetainNotSupported          ReasonCode = 0x9A
	ReasonQoSNotSupported             ReasonCode = 0x9B
	ReasonUseAnotherServer            ReasonCode = 0x9C
	ReasonServerMoved                 ReasonCode = 0x9D
	ReasonSharedSubNotSupported       ReasonCode = 0x9E
	ReasonConnectionRateExceeded      ReasonCode = 0x9F
	ReasonMaxConnectTime              ReasonCode = 0xA0
	ReasonSubIDNotSupported           ReasonCode = 0xA1
	ReasonWildcardSubNotSupported     ReasonCode = 0xA2
)

func (r ReasonCode) String() string {
	switch r {
	case ReasonSuccess:
		return "Success"
	case ReasonUnspecifiedError:
		return "Unspecified Error"
	case ReasonMalformedPacket:
		return "Malformed Packet"
	case ReasonProtocolError:
		return "Protocol Error"
	case ReasonNotAuthorized:
		return "Not Authorized"
	case ReasonBadUserNameOrPassword:
		return "Bad User Name or Password"
	default:
		return "Unknown"
	}
}
