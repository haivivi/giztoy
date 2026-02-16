// gen_planes generates the LSH hyperplane matrix for voiceprint hashing
// and writes it to planes_512_16.json.
//
// These planes are a project asset — committed to the repo and loaded by
// both Go and Rust at build time. This ensures cross-language hash consistency
// regardless of RNG implementation differences.
//
// Usage: go run ./data/voiceprint/gen_planes.go
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
)

type PlanesFile struct {
	Dim    int         `json:"dim"`
	Bits   int         `json:"bits"`
	Seed   uint64      `json:"seed"`
	Planes [][]float32 `json:"planes"`
}

func main() {
	dim := 512
	bits := 16
	seed := uint64(42)

	rng := rand.New(rand.NewPCG(seed, seed^0xdeadbeef))
	planes := make([][]float32, bits)
	for i := range planes {
		plane := make([]float32, dim)
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

	pf := PlanesFile{
		Dim:    dim,
		Bits:   bits,
		Seed:   seed,
		Planes: planes,
	}

	f, err := os.Create("data/voiceprint/planes_512_16.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(pf); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote data/voiceprint/planes_512_16.json (dim=%d, bits=%d, seed=%d)\n", dim, bits, seed)

	// Also generate a reference hash for validation.
	// Embedding: [0, 0.01, 0.02, ..., 5.11] (dim=512)
	emb := make([]float32, dim)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}

	nBytes := bits / 8
	if bits%8 != 0 {
		nBytes++
	}
	hashBytes := make([]byte, nBytes)
	for i, plane := range planes {
		var dot float32
		for j := range plane {
			dot += plane[j] * emb[j]
		}
		if dot > 0 {
			hashBytes[i/8] |= 1 << (7 - uint(i%8))
		}
	}
	hash := fmt.Sprintf("%02X%02X", hashBytes[0], hashBytes[1])
	fmt.Printf("reference hash for [0..511]*0.01: %s\n", hash)
}
