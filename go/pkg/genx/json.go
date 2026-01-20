package genx

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/kaptinlin/jsonrepair"
)

// unmarshalJSON unmarshals JSON data into v, attempting to repair malformed JSON.
// If the initial unmarshal fails with a syntax error, it tries to repair the JSON
// using jsonrepair before retrying.
func unmarshalJSON(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	if err == nil {
		return nil
	}
	if _, ok := err.(*json.SyntaxError); ok {
		fixed, err := jsonrepair.JSONRepair(string(data))
		if err != nil {
			return err
		}
		return json.Unmarshal([]byte(fixed), v)
	}
	return err
}

// hexString generates a random 16-character hexadecimal string.
func hexString() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
