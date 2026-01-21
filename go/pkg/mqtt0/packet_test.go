package mqtt0

import (
	"bufio"
	"bytes"
	"testing"
)

func TestV4ConnectEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		packet *V4Connect
	}{
		{
			name: "basic",
			packet: &V4Connect{
				ClientID:     "test-client",
				CleanSession: true,
				KeepAlive:    60,
			},
		},
		{
			name: "with credentials",
			packet: &V4Connect{
				ClientID:     "test-client",
				Username:     "user",
				Password:     []byte("pass"),
				CleanSession: true,
				KeepAlive:    60,
			},
		},
		{
			name: "no clean session",
			packet: &V4Connect{
				ClientID:     "test-client",
				CleanSession: false,
				KeepAlive:    30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.packet.encode()
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			// Decode
			reader := bufio.NewReader(bytes.NewReader(data))
			packet, err := ReadV4Packet(reader, MaxPacketSize)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			connect, ok := packet.(*V4Connect)
			if !ok {
				t.Fatalf("expected V4Connect, got %T", packet)
			}

			// Verify
			if connect.ClientID != tt.packet.ClientID {
				t.Errorf("ClientID: got %q, want %q", connect.ClientID, tt.packet.ClientID)
			}
			if connect.Username != tt.packet.Username {
				t.Errorf("Username: got %q, want %q", connect.Username, tt.packet.Username)
			}
			if !bytes.Equal(connect.Password, tt.packet.Password) {
				t.Errorf("Password: got %q, want %q", connect.Password, tt.packet.Password)
			}
			if connect.CleanSession != tt.packet.CleanSession {
				t.Errorf("CleanSession: got %v, want %v", connect.CleanSession, tt.packet.CleanSession)
			}
			if connect.KeepAlive != tt.packet.KeepAlive {
				t.Errorf("KeepAlive: got %d, want %d", connect.KeepAlive, tt.packet.KeepAlive)
			}
		})
	}
}

func TestV4PublishEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		packet *V4Publish
	}{
		{
			name: "basic",
			packet: &V4Publish{
				Topic:   "test/topic",
				Payload: []byte("hello world"),
			},
		},
		{
			name: "with retain",
			packet: &V4Publish{
				Topic:   "test/topic",
				Payload: []byte("hello"),
				Retain:  true,
			},
		},
		{
			name: "empty payload",
			packet: &V4Publish{
				Topic:   "test/topic",
				Payload: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.packet.encode()
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			// Decode
			reader := bufio.NewReader(bytes.NewReader(data))
			packet, err := ReadV4Packet(reader, MaxPacketSize)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			publish, ok := packet.(*V4Publish)
			if !ok {
				t.Fatalf("expected V4Publish, got %T", packet)
			}

			// Verify
			if publish.Topic != tt.packet.Topic {
				t.Errorf("Topic: got %q, want %q", publish.Topic, tt.packet.Topic)
			}
			if !bytes.Equal(publish.Payload, tt.packet.Payload) {
				t.Errorf("Payload: got %q, want %q", publish.Payload, tt.packet.Payload)
			}
			if publish.Retain != tt.packet.Retain {
				t.Errorf("Retain: got %v, want %v", publish.Retain, tt.packet.Retain)
			}
		})
	}
}

func TestV4SubscribeEncodeDecode(t *testing.T) {
	packet := &V4Subscribe{
		PacketID: 123,
		Topics:   []string{"topic/a", "topic/b", "topic/+/c"},
	}

	// Encode
	data, err := packet.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Decode
	reader := bufio.NewReader(bytes.NewReader(data))
	decoded, err := ReadV4Packet(reader, MaxPacketSize)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	sub, ok := decoded.(*V4Subscribe)
	if !ok {
		t.Fatalf("expected V4Subscribe, got %T", decoded)
	}

	if sub.PacketID != packet.PacketID {
		t.Errorf("PacketID: got %d, want %d", sub.PacketID, packet.PacketID)
	}
	if len(sub.Topics) != len(packet.Topics) {
		t.Fatalf("Topics length: got %d, want %d", len(sub.Topics), len(packet.Topics))
	}
	for i, topic := range sub.Topics {
		if topic != packet.Topics[i] {
			t.Errorf("Topic[%d]: got %q, want %q", i, topic, packet.Topics[i])
		}
	}
}

func TestV4PingReqResp(t *testing.T) {
	// PingReq
	pingReq := &V4PingReq{}
	data, err := pingReq.encode()
	if err != nil {
		t.Fatalf("encode pingreq failed: %v", err)
	}

	reader := bufio.NewReader(bytes.NewReader(data))
	packet, err := ReadV4Packet(reader, MaxPacketSize)
	if err != nil {
		t.Fatalf("decode pingreq failed: %v", err)
	}

	if _, ok := packet.(*V4PingReq); !ok {
		t.Errorf("expected V4PingReq, got %T", packet)
	}

	// PingResp
	pingResp := &V4PingResp{}
	data, err = pingResp.encode()
	if err != nil {
		t.Fatalf("encode pingresp failed: %v", err)
	}

	reader = bufio.NewReader(bytes.NewReader(data))
	packet, err = ReadV4Packet(reader, MaxPacketSize)
	if err != nil {
		t.Fatalf("decode pingresp failed: %v", err)
	}

	if _, ok := packet.(*V4PingResp); !ok {
		t.Errorf("expected V4PingResp, got %T", packet)
	}
}

func TestV5ConnectEncodeDecode(t *testing.T) {
	sessionExpiry := uint32(3600)

	tests := []struct {
		name   string
		packet *V5Connect
	}{
		{
			name: "basic",
			packet: &V5Connect{
				ClientID:   "test-client",
				CleanStart: true,
				KeepAlive:  60,
			},
		},
		{
			name: "with credentials",
			packet: &V5Connect{
				ClientID:   "test-client",
				Username:   "user",
				Password:   []byte("pass"),
				CleanStart: true,
				KeepAlive:  60,
			},
		},
		{
			name: "with session expiry",
			packet: &V5Connect{
				ClientID:   "test-client",
				CleanStart: false,
				KeepAlive:  60,
				Properties: &V5Properties{
					SessionExpiry: &sessionExpiry,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.packet.encodeV5()
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			// Decode
			reader := bufio.NewReader(bytes.NewReader(data))
			packet, err := ReadV5Packet(reader, MaxPacketSize)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			connect, ok := packet.(*V5Connect)
			if !ok {
				t.Fatalf("expected V5Connect, got %T", packet)
			}

			// Verify
			if connect.ClientID != tt.packet.ClientID {
				t.Errorf("ClientID: got %q, want %q", connect.ClientID, tt.packet.ClientID)
			}
			if connect.Username != tt.packet.Username {
				t.Errorf("Username: got %q, want %q", connect.Username, tt.packet.Username)
			}
			if connect.CleanStart != tt.packet.CleanStart {
				t.Errorf("CleanStart: got %v, want %v", connect.CleanStart, tt.packet.CleanStart)
			}
			if connect.KeepAlive != tt.packet.KeepAlive {
				t.Errorf("KeepAlive: got %d, want %d", connect.KeepAlive, tt.packet.KeepAlive)
			}

			// Check properties
			if tt.packet.Properties != nil && tt.packet.Properties.SessionExpiry != nil {
				if connect.Properties == nil || connect.Properties.SessionExpiry == nil {
					t.Error("SessionExpiry property missing")
				} else if *connect.Properties.SessionExpiry != *tt.packet.Properties.SessionExpiry {
					t.Errorf("SessionExpiry: got %d, want %d",
						*connect.Properties.SessionExpiry, *tt.packet.Properties.SessionExpiry)
				}
			}
		})
	}
}

func TestV5PublishEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		packet *V5Publish
	}{
		{
			name: "basic",
			packet: &V5Publish{
				Topic:   "test/topic",
				Payload: []byte("hello world"),
			},
		},
		{
			name: "with retain",
			packet: &V5Publish{
				Topic:   "test/topic",
				Payload: []byte("hello"),
				Retain:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.packet.encodeV5()
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			// Decode
			reader := bufio.NewReader(bytes.NewReader(data))
			packet, err := ReadV5Packet(reader, MaxPacketSize)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			publish, ok := packet.(*V5Publish)
			if !ok {
				t.Fatalf("expected V5Publish, got %T", packet)
			}

			// Verify
			if publish.Topic != tt.packet.Topic {
				t.Errorf("Topic: got %q, want %q", publish.Topic, tt.packet.Topic)
			}
			if !bytes.Equal(publish.Payload, tt.packet.Payload) {
				t.Errorf("Payload: got %q, want %q", publish.Payload, tt.packet.Payload)
			}
			if publish.Retain != tt.packet.Retain {
				t.Errorf("Retain: got %v, want %v", publish.Retain, tt.packet.Retain)
			}
		})
	}
}

func TestVariableInt(t *testing.T) {
	tests := []struct {
		value int
		size  int
	}{
		{0, 1},
		{127, 1},
		{128, 2},
		{16383, 2},
		{16384, 3},
		{2097151, 3},
		{2097152, 4},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := variableIntSize(tt.value); got != tt.size {
				t.Errorf("variableIntSize(%d) = %d, want %d", tt.value, got, tt.size)
			}

			// Test encode/decode
			var buf bytes.Buffer
			if err := writeVariableInt(&buf, tt.value); err != nil {
				t.Fatalf("writeVariableInt failed: %v", err)
			}

			if buf.Len() != tt.size {
				t.Errorf("encoded size = %d, want %d", buf.Len(), tt.size)
			}

			reader := bufio.NewReader(&buf)
			got, err := readVariableInt(reader)
			if err != nil {
				t.Fatalf("readVariableInt failed: %v", err)
			}
			if got != tt.value {
				t.Errorf("readVariableInt() = %d, want %d", got, tt.value)
			}
		})
	}
}
