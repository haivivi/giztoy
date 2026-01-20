package pcm

import (
	"math"
	"sync/atomic"
)

// AtomicFloat32 provides atomic operations for float32 values.
// It uses atomic uint32 operations internally by bit-casting the float32.
type AtomicFloat32 struct {
	bits uint32
}

// NewAtomicFloat32 creates a new AtomicFloat32 with the given initial value.
func NewAtomicFloat32(val float32) AtomicFloat32 {
	return AtomicFloat32{bits: math.Float32bits(val)}
}

// Load atomically loads and returns the float32 value.
func (af *AtomicFloat32) Load() float32 {
	return math.Float32frombits(atomic.LoadUint32(&af.bits))
}

// Store atomically stores the given float32 value.
func (af *AtomicFloat32) Store(val float32) {
	atomic.StoreUint32(&af.bits, math.Float32bits(val))
}
