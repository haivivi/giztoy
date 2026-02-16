// gen_reference generates reference DBSCAN labels from Go for cross-language
// validation. Uses a fixed set of vectors and produces labels + cosine sim values.
//
// Usage: go run ./testdata/compat/dbscan/gen_reference.go
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type DBSCANReference struct {
	Dim       int         `json:"dim"`
	Eps       float32     `json:"eps"`
	MinPts    int         `json:"min_pts"`
	Vectors   [][]float32 `json:"vectors"`
	Labels    []int       `json:"labels"`
}

// cosineSim matches Go vecid implementation (f64 intermediate precision).
func cosineSim(a, b []float32) float32 {
	var dot, na, nb float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	denom := math.Sqrt(na) * math.Sqrt(nb)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

func l2Norm(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	if norm > 0 {
		scale := float32(1.0 / norm)
		for i := range v {
			v[i] *= scale
		}
	}
}

// dbscan matches Go vecid implementation exactly.
func dbscan(vectors [][]float32, eps float32, minPts int) []int {
	n := len(vectors)
	if n == 0 {
		return nil
	}

	const (
		undefined = 0
		noise     = -1
	)

	labels := make([]int, n)
	clusterID := 0

	rangeQuery := func(idx int) []int {
		var result []int
		q := vectors[idx]
		for i, v := range vectors {
			dist := 1.0 - cosineSim(q, v)
			if dist <= eps {
				result = append(result, i)
			}
		}
		return result
	}

	for i := 0; i < n; i++ {
		if labels[i] != undefined {
			continue
		}

		neighbors := rangeQuery(i)
		if len(neighbors) < minPts {
			labels[i] = noise
			continue
		}

		clusterID++
		labels[i] = clusterID

		seed := make([]int, 0, len(neighbors))
		for _, j := range neighbors {
			if j != i {
				seed = append(seed, j)
			}
		}

		for len(seed) > 0 {
			q := seed[0]
			seed = seed[1:]

			if labels[q] == noise {
				labels[q] = clusterID
			}
			if labels[q] != undefined {
				continue
			}
			labels[q] = clusterID

			qNeighbors := rangeQuery(q)
			if len(qNeighbors) >= minPts {
				seed = append(seed, qNeighbors...)
			}
		}
	}

	return labels
}

func main() {
	dim := 8

	// Fixed test vectors: 2 clusters + 1 noise point.
	// Cluster A: near [1,0,0,...], Cluster B: near [0,1,0,...], Noise: random.
	vectors := [][]float32{
		// Cluster A (indices 0-3)
		{1.0, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		{0.99, 0.12, 0.01, 0.0, 0.0, 0.0, 0.0, 0.0},
		{0.98, 0.15, 0.02, 0.01, 0.0, 0.0, 0.0, 0.0},
		{0.97, 0.18, 0.03, 0.0, 0.01, 0.0, 0.0, 0.0},
		// Cluster B (indices 4-7)
		{0.0, 1.0, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0},
		{0.01, 0.99, 0.12, 0.0, 0.0, 0.0, 0.0, 0.0},
		{0.02, 0.98, 0.15, 0.01, 0.0, 0.0, 0.0, 0.0},
		{0.03, 0.97, 0.18, 0.0, 0.01, 0.0, 0.0, 0.0},
		// Noise (index 8)
		{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3},
	}

	// L2 normalize all.
	for i := range vectors {
		l2Norm(vectors[i])
	}

	eps := float32(0.3)
	minPts := 2
	labels := dbscan(vectors, eps, minPts)

	ref := DBSCANReference{
		Dim:     dim,
		Eps:     eps,
		MinPts:  minPts,
		Vectors: vectors,
		Labels:  labels,
	}

	f, err := os.Create("testdata/compat/dbscan/reference.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ref); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("dbscan: %d vectors, dim=%d, eps=%.1f, minPts=%d\n",
		len(vectors), dim, eps, minPts)
	fmt.Printf("labels: %v\n", labels)
	fmt.Println("wrote testdata/compat/dbscan/reference.json")
}
