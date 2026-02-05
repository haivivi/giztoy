package runtime

import (
	"crypto/rand"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// builtinUUID implements __builtin.uuid() -> string
func (rt *Runtime) builtinUUID(state *luau.State) int {
	uuid := generateUUID()
	state.PushString(uuid)
	return 1
}

// generateUUID generates a random UUID v4.
func generateUUID() string {
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		// Fallback to timestamp-based if crypto/rand fails
		return fmt.Sprintf("%x", rand.Reader)
	}

	// Set version (4) and variant (2) bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 2

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
