package vecid

import "math"

// dbscan runs the DBSCAN clustering algorithm using cosine distance.
//
// Parameters:
//   - vectors: the data points (L2-normalized embeddings)
//   - eps: maximum cosine distance (1 - cosine_similarity) for neighbors
//   - minPts: minimum points to form a dense cluster
//
// Returns cluster labels for each vector. Label -1 means noise (unassigned).
func dbscan(vectors [][]float32, eps float32, minPts int) []int {
	n := len(vectors)
	if n == 0 {
		return nil
	}

	const (
		undefined = 0
		noise     = -1
	)

	labels := make([]int, n)   // 0 = undefined
	clusterID := 0

	for i := 0; i < n; i++ {
		if labels[i] != undefined {
			continue
		}

		neighbors := rangeQuery(vectors, i, eps)
		if len(neighbors) < minPts {
			labels[i] = noise
			continue
		}

		// Start a new cluster.
		clusterID++
		labels[i] = clusterID

		// Seed set: neighbors minus point i.
		seed := make([]int, 0, len(neighbors))
		for _, j := range neighbors {
			if j != i {
				seed = append(seed, j)
			}
		}

		for len(seed) > 0 {
			// Pop first element.
			q := seed[0]
			seed = seed[1:]

			if labels[q] == noise {
				labels[q] = clusterID
			}
			if labels[q] != undefined {
				continue
			}
			labels[q] = clusterID

			qNeighbors := rangeQuery(vectors, q, eps)
			if len(qNeighbors) >= minPts {
				seed = append(seed, qNeighbors...)
			}
		}
	}

	return labels
}

// rangeQuery returns indices of all vectors within eps cosine distance of vectors[idx].
func rangeQuery(vectors [][]float32, idx int, eps float32) []int {
	var result []int
	q := vectors[idx]
	for i, v := range vectors {
		dist := cosineDistance(q, v)
		if dist <= eps {
			result = append(result, i)
		}
	}
	return result
}

// cosineDistance returns 1 - cosine_similarity.
// For L2-normalized vectors, cosine_similarity = dot product.
func cosineDistance(a, b []float32) float32 {
	return 1.0 - cosineSim(a, b)
}

// cosineSim computes cosine similarity between two vectors.
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

// l2Norm normalizes a vector to unit length in-place.
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
