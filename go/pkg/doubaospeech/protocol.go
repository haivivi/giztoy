package doubaospeech

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
)

// ================== 协议常量 ==================

type protocolVersion byte
type messageType byte
type messageTypeFlags byte
type serializationType byte
type compressionType byte

const (
	protocolVersionV1 protocolVersion = 0b0001

	// Message Types
	msgTypeFullClient      messageType = 0b0001
	msgTypeAudioOnlyClient messageType = 0b0010
	msgTypeFullServer      messageType = 0b1001
	msgTypeAudioOnlyServer messageType = 0b1011
	msgTypeFrontEndResult  messageType = 0b1100
	msgTypeError           messageType = 0b1111

	// Message Type Specific Flags
	msgFlagNoSequence  messageTypeFlags = 0b0000
	msgFlagPosSequence messageTypeFlags = 0b0001
	msgFlagNegSequence messageTypeFlags = 0b0010
	msgFlagNegWithSeq  messageTypeFlags = 0b0011
	msgFlagWithEvent   messageTypeFlags = 0b0100

	// Serialization Types
	serializationNone   serializationType = 0b0000
	serializationJSON   serializationType = 0b0001
	serializationThrift serializationType = 0b0011

	// Compression Types
	compressionNone compressionType = 0b0000
	compressionGzip compressionType = 0b0001

	// Protocol Event Types
	eventSessionStart       int32 = 1
	eventSessionFinish      int32 = 2
	eventConnectionStarted  int32 = 50
	eventConnectionFailed   int32 = 51
	eventConnectionFinished int32 = 52
)

// ================== 协议结构 ==================

// binaryProtocol 二进制协议处理器
//
// 协议格式:
// - Header (4 bytes):
//   - (4bits) version + (4bits) header_size
//   - (4bits) message_type + (4bits) message_type_flags
//   - (4bits) serialization + (4bits) compression
//   - (8bits) reserved
//
// - Payload:
//   - [optional] sequence (4 bytes)
//   - [optional] event (4 bytes)
//   - [optional] session_id (4 bytes len + data)
//   - payload_size (4 bytes) + payload_data
type binaryProtocol struct {
	version       protocolVersion
	headerSize    byte
	compression   compressionType
	serialization serializationType
}

// message 协议消息
type message struct {
	msgType   messageType
	flags     messageTypeFlags
	event     int32
	sessionID string
	connectID string
	sequence  int32
	errorCode uint32
	payload   []byte
}

// newBinaryProtocol 创建协议处理器
func newBinaryProtocol() *binaryProtocol {
	return &binaryProtocol{
		version:       protocolVersionV1,
		headerSize:    1, // 4 bytes
		compression:   compressionNone,
		serialization: serializationJSON,
	}
}

// setCompression 设置压缩方式
func (p *binaryProtocol) setCompression(c compressionType) {
	p.compression = c
}

// setSerialization 设置序列化方式
func (p *binaryProtocol) setSerialization(s serializationType) {
	p.serialization = s
}

// marshal 序列化消息
func (p *binaryProtocol) marshal(msg *message) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Header (4 bytes)
	buf.WriteByte(byte(p.version<<4) | p.headerSize)
	buf.WriteByte(byte(msg.msgType<<4) | byte(msg.flags))
	buf.WriteByte(byte(p.serialization<<4) | byte(p.compression))
	buf.WriteByte(0x00) // reserved

	// Sequence (if needed)
	if msg.flags&msgFlagPosSequence != 0 || msg.flags&msgFlagNegSequence != 0 {
		if err := binary.Write(buf, binary.BigEndian, msg.sequence); err != nil {
			return nil, fmt.Errorf("write sequence: %w", err)
		}
	}

	// Event (if needed)
	if msg.flags&msgFlagWithEvent != 0 {
		if err := binary.Write(buf, binary.BigEndian, msg.event); err != nil {
			return nil, fmt.Errorf("write event: %w", err)
		}

		// Session ID (for non-connection events)
		if msg.event != eventSessionStart && msg.event != eventSessionFinish &&
			msg.event != eventConnectionStarted &&
			msg.event != eventConnectionFailed &&
			msg.event != eventConnectionFinished {
			if err := binary.Write(buf, binary.BigEndian, uint32(len(msg.sessionID))); err != nil {
				return nil, fmt.Errorf("write session id length: %w", err)
			}
			buf.WriteString(msg.sessionID)
		}
	}

	// Payload
	payload := msg.payload
	if p.compression == compressionGzip && len(payload) > 0 {
		compressed, err := gzipCompress(payload)
		if err != nil {
			return nil, fmt.Errorf("gzip compress: %w", err)
		}
		payload = compressed
	}

	if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
		return nil, fmt.Errorf("write payload size: %w", err)
	}
	buf.Write(payload)

	return buf.Bytes(), nil
}

// unmarshal 反序列化消息
func (p *binaryProtocol) unmarshal(data []byte) (*message, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	buf := bytes.NewBuffer(data)

	// Read header
	versionAndSize, _ := buf.ReadByte()
	typeAndFlags, _ := buf.ReadByte()
	serAndComp, _ := buf.ReadByte()
	_, _ = buf.ReadByte() // reserved

	msg := &message{
		msgType: messageType(typeAndFlags >> 4),
		flags:   messageTypeFlags(typeAndFlags & 0x0f),
	}

	compression := compressionType(serAndComp & 0x0f)

	// Header size (in 4-byte units)
	headerSize := int(versionAndSize & 0x0f)
	if headerSize > 1 {
		// Skip additional header bytes
		buf.Next((headerSize - 1) * 4)
	}

	// Read sequence if present
	if msg.flags&msgFlagPosSequence != 0 || msg.flags&msgFlagNegSequence != 0 {
		if err := binary.Read(buf, binary.BigEndian, &msg.sequence); err != nil {
			return nil, fmt.Errorf("read sequence: %w", err)
		}
	}

	// Read event if present
	if msg.flags&msgFlagWithEvent != 0 {
		if err := binary.Read(buf, binary.BigEndian, &msg.event); err != nil {
			return nil, fmt.Errorf("read event: %w", err)
		}

		// Read session ID (for non-connection events)
		if msg.event != eventSessionStart && msg.event != eventSessionFinish &&
			msg.event != eventConnectionStarted &&
			msg.event != eventConnectionFailed &&
			msg.event != eventConnectionFinished {
			var sessionIDLen uint32
			if err := binary.Read(buf, binary.BigEndian, &sessionIDLen); err != nil {
				return nil, fmt.Errorf("read session id length: %w", err)
			}
			if sessionIDLen > 0 {
				sessionIDBytes := make([]byte, sessionIDLen)
				if _, err := buf.Read(sessionIDBytes); err != nil {
					return nil, fmt.Errorf("read session id: %w", err)
				}
				msg.sessionID = string(sessionIDBytes)
			}
		}

		// Read connect ID for connection events
		if msg.event == eventConnectionStarted ||
			msg.event == eventConnectionFailed ||
			msg.event == eventConnectionFinished {
			var connectIDLen uint32
			if err := binary.Read(buf, binary.BigEndian, &connectIDLen); err != nil {
				return nil, fmt.Errorf("read connect id length: %w", err)
			}
			if connectIDLen > 0 {
				connectIDBytes := make([]byte, connectIDLen)
				if _, err := buf.Read(connectIDBytes); err != nil {
					return nil, fmt.Errorf("read connect id: %w", err)
				}
				msg.connectID = string(connectIDBytes)
			}
		}
	}

	// Read error code for error messages
	if msg.msgType == msgTypeError {
		if err := binary.Read(buf, binary.BigEndian, &msg.errorCode); err != nil {
			return nil, fmt.Errorf("read error code: %w", err)
		}
	}

	// Read payload
	var payloadSize uint32
	if err := binary.Read(buf, binary.BigEndian, &payloadSize); err != nil {
		return nil, fmt.Errorf("read payload size: %w", err)
	}

	if payloadSize > 0 {
		msg.payload = make([]byte, payloadSize)
		if _, err := buf.Read(msg.payload); err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}

		// Decompress if needed
		if compression == compressionGzip {
			decompressed, err := gzipDecompress(msg.payload)
			if err != nil {
				return nil, fmt.Errorf("gzip decompress: %w", err)
			}
			msg.payload = decompressed
		}
	}

	return msg, nil
}

// isAudioOnly 是否为纯音频消息
func (msg *message) isAudioOnly() bool {
	return msg.msgType == msgTypeAudioOnlyServer || msg.msgType == msgTypeAudioOnlyClient
}

// isError 是否为错误消息
func (msg *message) isError() bool {
	return msg.msgType == msgTypeError
}

// isFrontend 是否为前端结果消息
func (msg *message) isFrontend() bool {
	return msg.msgType == msgTypeFrontEndResult
}

// gzipCompress gzip 压缩
func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gzipDecompress gzip 解压
func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
