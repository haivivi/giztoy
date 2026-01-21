package encoding

import (
	"encoding/json"
	"testing"
)

func TestStdBase64Data_MarshalJSON(t *testing.T) {
	data := StdBase64Data([]byte("hello world"))

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := `"aGVsbG8gd29ybGQ="`
	if string(b) != expected {
		t.Errorf("MarshalJSON = %s; want %s", b, expected)
	}
}

func TestStdBase64Data_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "valid base64",
			input: `"aGVsbG8gd29ybGQ="`,
			want:  []byte("hello world"),
		},
		{
			name:  "empty base64",
			input: `""`,
			want:  []byte{},
		},
		{
			name:  "null",
			input: `null`,
			want:  nil,
		},
		{
			name:    "invalid - number",
			input:   `123`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data StdBase64Data
			err := json.Unmarshal([]byte(tc.input), &data)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}

			if string(data) != string(tc.want) {
				t.Errorf("UnmarshalJSON = %v; want %v", data, tc.want)
			}
		})
	}
}

func TestStdBase64Data_RoundTrip(t *testing.T) {
	original := StdBase64Data([]byte("test data for round trip"))

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored StdBase64Data
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if string(original) != string(restored) {
		t.Errorf("RoundTrip: original=%v, restored=%v", original, restored)
	}
}

func TestStdBase64Data_String(t *testing.T) {
	data := StdBase64Data([]byte("hello"))
	expected := "aGVsbG8="

	if data.String() != expected {
		t.Errorf("String() = %s; want %s", data.String(), expected)
	}
}

func TestHexData_MarshalJSON(t *testing.T) {
	data := HexData([]byte{0xde, 0xad, 0xbe, 0xef})

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := `"deadbeef"`
	if string(b) != expected {
		t.Errorf("MarshalJSON = %s; want %s", b, expected)
	}
}

func TestHexData_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "valid hex",
			input: `"deadbeef"`,
			want:  []byte{0xde, 0xad, 0xbe, 0xef},
		},
		{
			name:  "empty hex",
			input: `""`,
			want:  []byte{},
		},
		{
			name:  "null",
			input: `null`,
			want:  nil,
		},
		{
			name:    "invalid - odd length",
			input:   `"abc"`,
			wantErr: true,
		},
		{
			name:    "invalid - non-hex",
			input:   `"xyz123"`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data HexData
			err := json.Unmarshal([]byte(tc.input), &data)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}

			if string(data) != string(tc.want) {
				t.Errorf("UnmarshalJSON = %v; want %v", data, tc.want)
			}
		})
	}
}

func TestHexData_RoundTrip(t *testing.T) {
	original := HexData([]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef})

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored HexData
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if string(original) != string(restored) {
		t.Errorf("RoundTrip: original=%x, restored=%x", original, restored)
	}
}

func TestHexData_String(t *testing.T) {
	data := HexData([]byte{0xca, 0xfe})
	expected := "cafe"

	if data.String() != expected {
		t.Errorf("String() = %s; want %s", data.String(), expected)
	}
}

func TestInStruct(t *testing.T) {
	type Message struct {
		ID      string        `json:"id"`
		Payload StdBase64Data `json:"payload"`
		Hash    HexData       `json:"hash"`
	}

	msg := Message{
		ID:      "test-123",
		Payload: StdBase64Data([]byte("hello")),
		Hash:    HexData([]byte{0xab, 0xcd}),
	}

	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Message
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.ID != msg.ID {
		t.Errorf("ID = %s; want %s", restored.ID, msg.ID)
	}
	if string(restored.Payload) != string(msg.Payload) {
		t.Errorf("Payload = %v; want %v", restored.Payload, msg.Payload)
	}
	if string(restored.Hash) != string(msg.Hash) {
		t.Errorf("Hash = %v; want %v", restored.Hash, msg.Hash)
	}
}
