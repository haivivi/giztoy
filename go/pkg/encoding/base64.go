// Package encoding provides JSON-serializable encoding types.
package encoding

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// StdBase64Data is a byte slice that serializes to/from standard base64 in JSON.
type StdBase64Data []byte

// MarshalJSON implements json.Marshaler.
func (b StdBase64Data) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(b) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (b *StdBase64Data) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("unmarshal json base64 data: empty data")
	}
	switch data[0] {
	case 'n': // null
		return nil
	case '"':
		if len(data) < 2 || data[len(data)-1] != '"' {
			return errors.New("unmarshal json base64 data: invalid string")
		}
		decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
		if err != nil {
			return err
		}
		*b = decoded
		return nil
	default:
		return fmt.Errorf("invalid base64 data: %s", string(data))
	}
}

// String returns the base64-encoded string representation.
func (b StdBase64Data) String() string {
	return base64.StdEncoding.EncodeToString(b)
}

// HexData is a byte slice that serializes to/from hexadecimal in JSON.
type HexData []byte

// MarshalJSON implements json.Marshaler.
func (h HexData) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(h) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (h *HexData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("unmarshal json hex data: empty data")
	}
	switch data[0] {
	case 'n': // null
		return nil
	case '"':
		if len(data) < 2 || data[len(data)-1] != '"' {
			return errors.New("unmarshal json hex data: invalid string")
		}
		decoded, err := hex.DecodeString(string(data[1 : len(data)-1]))
		if err != nil {
			return err
		}
		*h = decoded
		return nil
	default:
		return fmt.Errorf("invalid hex data: %s", string(data))
	}
}

// String returns the hex-encoded string representation.
func (h HexData) String() string {
	return hex.EncodeToString(h)
}
