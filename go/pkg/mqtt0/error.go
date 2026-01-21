package mqtt0

import (
	"errors"
	"fmt"
)

// Common errors.
var (
	// ErrClosed is returned when operating on a closed connection.
	ErrClosed = errors.New("mqtt0: connection closed")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("mqtt0: timeout")

	// ErrAuthFailed is returned when authentication fails.
	ErrAuthFailed = errors.New("mqtt0: authentication failed")

	// ErrACLDenied is returned when ACL check fails.
	ErrACLDenied = errors.New("mqtt0: acl denied")

	// ErrInvalidPacket is returned when a packet is malformed.
	ErrInvalidPacket = errors.New("mqtt0: invalid packet")

	// ErrProtocolViolation is returned when a protocol violation occurs.
	ErrProtocolViolation = errors.New("mqtt0: protocol violation")

	// ErrPacketTooLarge is returned when a packet exceeds the maximum size.
	ErrPacketTooLarge = errors.New("mqtt0: packet too large")

	// ErrInvalidTopic is returned when a topic pattern is invalid.
	ErrInvalidTopic = errors.New("mqtt0: invalid topic")

	// ErrAlreadyRunning is returned when the broker is already running.
	ErrAlreadyRunning = errors.New("mqtt0: already running")
)

// ConnectError represents a connection error with a return code.
type ConnectError struct {
	Code    ConnectReturnCode
	Message string
}

func (e *ConnectError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("mqtt0: connection refused: %s (%s)", e.Code, e.Message)
	}
	return fmt.Sprintf("mqtt0: connection refused: %s", e.Code)
}

// ConnectErrorV5 represents a MQTT 5.0 connection error with a reason code.
type ConnectErrorV5 struct {
	Code    ReasonCode
	Message string
}

func (e *ConnectErrorV5) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("mqtt0: connection refused: %s (%s)", e.Code, e.Message)
	}
	return fmt.Sprintf("mqtt0: connection refused: %s", e.Code)
}

// ProtocolError represents a protocol-level error.
type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("mqtt0: protocol error: %s", e.Message)
}

// UnexpectedPacketError is returned when an unexpected packet type is received.
type UnexpectedPacketError struct {
	Expected string
	Got      string
}

func (e *UnexpectedPacketError) Error() string {
	return fmt.Sprintf("mqtt0: unexpected packet: expected %s, got %s", e.Expected, e.Got)
}
