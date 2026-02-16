package voiceprint

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
)

// Hasher projects high-dimensional embedding vectors into compact
// locality-sensitive hashes using random hyperplane LSH.
//
// Each hash is a hex string whose length depends on the configured
// bit count. For 16 bits the output is 4 hex chars (e.g., "A3F8").
//
// # Algorithm
//
// The hasher stores `bits` random unit-length hyperplanes of dimension
// `dim`. For each hyperplane, the dot product with the input vector
// determines one bit: positive → 1, non-positive → 0.
// The resulting bit vector is encoded as uppercase hex.
//
// Because similar vectors tend to fall on the same side of random
// hyperplanes, nearby embeddings produce identical (or nearly identical)
// hashes with high probability.
//
// # Multi-Level Precision
//
// Callers can truncate the hash string to get coarser matches:
//
//	full  "A3F8" — 16-bit exact
//	[:3]  "A3F"  — 12-bit fuzzy
//	[:2]  "A3"   — 8-bit group
//	[:1]  "A"    — 4-bit coarse
type Hasher struct {
	dim    int
	bits   int
	planes [][]float32 // bits × dim, each row is a unit hyperplane
}

// NewHasher creates a Hasher with the given embedding dimension and
// output bit count. The bits parameter must be a positive multiple of 4
// (for clean hex encoding). The seed controls the random hyperplanes;
// use a fixed seed for reproducible hashes across restarts.
func NewHasher(dim, bits int, seed uint64) *Hasher {
	if bits <= 0 || bits%4 != 0 {
		panic("voiceprint: bits must be a positive multiple of 4")
	}
	if dim <= 0 {
		panic("voiceprint: dim must be positive")
	}

	rng := rand.New(rand.NewPCG(seed, seed^0xdeadbeef))
	planes := make([][]float32, bits)
	for i := range planes {
		plane := make([]float32, dim)
		// Sample from standard normal distribution then normalize.
		var norm float64
		for j := range plane {
			v := float32(rng.NormFloat64())
			plane[j] = v
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			scale := float32(1.0 / norm)
			for j := range plane {
				plane[j] *= scale
			}
		}
		planes[i] = plane
	}
	return &Hasher{dim: dim, bits: bits, planes: planes}
}

// PlanesFile is the JSON format for persisted hyperplane matrices.
// Both Go and Rust load the same file to ensure cross-language hash consistency.
type PlanesFile struct {
	Dim    int         `json:"dim"`
	Bits   int         `json:"bits"`
	Seed   uint64      `json:"seed"`
	Planes [][]float32 `json:"planes"`
}

// NewHasherFromPlanes creates a Hasher from a pre-computed planes matrix.
// Use this for production to ensure deterministic, cross-language-consistent hashes.
//
// The planes data should be loaded from a persisted JSON file
// (e.g., data/voiceprint/planes_512_16.json).
func NewHasherFromPlanes(planes [][]float32) *Hasher {
	bits := len(planes)
	if bits == 0 || bits%4 != 0 {
		panic("voiceprint: planes count must be a positive multiple of 4")
	}
	dim := len(planes[0])
	if dim <= 0 {
		panic("voiceprint: plane dimension must be positive")
	}
	for i, p := range planes {
		if len(p) != dim {
			panic(fmt.Sprintf("voiceprint: plane %d has dimension %d, expected %d", i, len(p), dim))
		}
	}
	return &Hasher{dim: dim, bits: bits, planes: planes}
}

// NewHasherFromJSON creates a Hasher from a JSON-encoded planes file.
func NewHasherFromJSON(data []byte) (*Hasher, error) {
	var pf PlanesFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("voiceprint: parse planes JSON: %w", err)
	}
	if len(pf.Planes) == 0 {
		return nil, fmt.Errorf("voiceprint: empty planes in JSON")
	}
	return NewHasherFromPlanes(pf.Planes), nil
}

// Planes returns the internal hyperplane matrix.
// Useful for serializing to JSON for cross-language sharing.
func (h *Hasher) Planes() [][]float32 { return h.planes }

// Hash projects an embedding vector into a hex hash string.
// The input must have length equal to the hasher's dimension.
// Returns an uppercase hex string of length bits/4.
func (h *Hasher) Hash(embedding []float32) string {
	if len(embedding) != h.dim {
		panic("voiceprint: embedding dimension mismatch")
	}

	// Compute bit vector: one bit per hyperplane.
	nBytes := h.bits / 8
	if h.bits%8 != 0 {
		nBytes++
	}
	hashBytes := make([]byte, nBytes)

	for i, plane := range h.planes {
		dot := dot32(plane, embedding)
		if dot > 0 {
			hashBytes[i/8] |= 1 << (7 - uint(i%8))
		}
	}

	// Encode as uppercase hex and truncate to exact nibble count.
	full := hex.EncodeToString(hashBytes)
	nNibbles := h.bits / 4
	result := make([]byte, nNibbles)
	for i := 0; i < nNibbles; i++ {
		c := full[i]
		// Uppercase hex.
		if c >= 'a' && c <= 'f' {
			c -= 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// Bits returns the number of hash bits.
func (h *Hasher) Bits() int { return h.bits }

// Dim returns the expected embedding dimension.
func (h *Hasher) Dim() int { return h.dim }

// dot32 computes the dot product of two float32 slices.
func dot32(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
