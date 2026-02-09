package genx

import (
	"crypto/rand"
	"time"
)

// epoch2025 is the Unix timestamp for 2025-01-01 00:00:00 UTC.
// Used as the base for StreamID timestamps to keep IDs shorter.
const epoch2025 int64 = 1735689600

// base62 characters for encoding
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// NewStreamID generates a short unique stream identifier.
// Format: base62(seconds_since_2025) + base62(random_6bytes)
// Length: ~14 characters (6 for time + 8 for random)
//
// The time component ensures IDs are roughly time-ordered,
// reducing collision probability in long-running systems.
func NewStreamID() string {
	// Time component: seconds since 2025-01-01
	secs := uint32(time.Now().Unix() - epoch2025)
	timePart := base62EncodeUint32(secs)

	// Random component: 6 bytes
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	randomPart := base62Encode(randomBytes)

	return timePart + randomPart
}

// base62EncodeUint32 encodes a uint32 to base62 string.
func base62EncodeUint32(n uint32) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

// base62Encode encodes bytes to base62 string.
func base62Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Convert bytes to a big integer (simple implementation)
	// For 6 bytes, we can use uint64
	var n uint64
	for _, b := range data {
		n = n*256 + uint64(b)
	}

	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}
