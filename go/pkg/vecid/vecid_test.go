package vecid

import (
	"math"
	"math/rand/v2"
	"testing"
)

// makeCluster generates n embeddings around a centroid with some noise.
func makeCluster(centroid []float32, n int, noise float64, rng *rand.Rand) [][]float32 {
	dim := len(centroid)
	var out [][]float32
	for range n {
		v := make([]float32, dim)
		for d := range v {
			v[d] = centroid[d] + float32(rng.NormFloat64()*noise)
		}
		l2Norm(v)
		out = append(out, v)
	}
	return out
}

// randVec generates a random unit vector.
func randVec(dim int, rng *rand.Rand) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	l2Norm(v)
	return v
}

func TestDBSCAN(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	dim := 32

	// 3 well-separated clusters
	c1 := randVec(dim, rng)
	c2 := randVec(dim, rng)
	c3 := randVec(dim, rng)

	var data [][]float32
	data = append(data, makeCluster(c1, 10, 0.1, rng)...)
	data = append(data, makeCluster(c2, 10, 0.1, rng)...)
	data = append(data, makeCluster(c3, 10, 0.1, rng)...)

	labels := dbscan(data, 0.3, 2) // eps=0.3 cosine distance

	// Count clusters.
	clusters := map[int]int{}
	for _, l := range labels {
		if l > 0 {
			clusters[l]++
		}
	}
	t.Logf("found %d clusters, labels=%v", len(clusters), clusters)

	if len(clusters) < 3 {
		t.Errorf("expected at least 3 clusters, got %d", len(clusters))
	}

	// Check that points in the same original cluster have the same label.
	for i := 0; i < 10; i++ {
		if labels[i] != labels[0] {
			t.Errorf("cluster 1: point %d has label %d, expected %d", i, labels[i], labels[0])
		}
		if labels[10+i] != labels[10] {
			t.Errorf("cluster 2: point %d has label %d, expected %d", 10+i, labels[10+i], labels[10])
		}
		if labels[20+i] != labels[20] {
			t.Errorf("cluster 3: point %d has label %d, expected %d", 20+i, labels[20+i], labels[20])
		}
	}

	// Each cluster should have a different label.
	if labels[0] == labels[10] || labels[0] == labels[20] || labels[10] == labels[20] {
		t.Errorf("clusters should have different labels: %d, %d, %d", labels[0], labels[10], labels[20])
	}
}

func TestDBSCANNoise(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 0))
	dim := 32

	// One cluster + some noise points.
	c := randVec(dim, rng)
	var data [][]float32
	data = append(data, makeCluster(c, 8, 0.05, rng)...)
	// Add 3 noise points far away.
	for range 3 {
		data = append(data, randVec(dim, rng))
	}

	labels := dbscan(data, 0.2, 2)

	clusterCount := 0
	noiseCount := 0
	for _, l := range labels {
		if l > 0 {
			clusterCount++
		} else if l == -1 {
			noiseCount++
		}
	}
	t.Logf("cluster points: %d, noise points: %d", clusterCount, noiseCount)

	if clusterCount < 8 {
		t.Errorf("expected at least 8 cluster points, got %d", clusterCount)
	}
	if noiseCount < 1 {
		t.Errorf("expected at least 1 noise point, got %d", noiseCount)
	}
}

func TestRegistryIdentifyBeforeRecluster(t *testing.T) {
	reg := New(Config{Dim: 32, Threshold: 0.5, Prefix: "test"}, nil)

	emb := make([]float32, 32)
	emb[0] = 1.0

	// Before any recluster, Identify should return unmatched.
	id, conf, ok := reg.Identify(emb)
	if ok {
		t.Errorf("expected no match before recluster, got id=%s conf=%f", id, conf)
	}
	if reg.Len() != 1 {
		t.Errorf("expected 1 stored embedding, got %d", reg.Len())
	}
}

func TestRegistryReclusterAndIdentify(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	dim := 32

	reg := New(Config{Dim: dim, Threshold: 0.5, Prefix: "speaker", MinSamples: 2}, nil)

	// Create 3 clusters.
	c1 := randVec(dim, rng)
	c2 := randVec(dim, rng)
	c3 := randVec(dim, rng)

	for _, emb := range makeCluster(c1, 5, 0.05, rng) {
		reg.Identify(emb)
	}
	for _, emb := range makeCluster(c2, 5, 0.05, rng) {
		reg.Identify(emb)
	}
	for _, emb := range makeCluster(c3, 5, 0.05, rng) {
		reg.Identify(emb)
	}

	if reg.Len() != 15 {
		t.Fatalf("expected 15 stored, got %d", reg.Len())
	}

	// Recluster.
	n := reg.Recluster()
	t.Logf("recluster found %d clusters", n)
	if n < 3 {
		t.Errorf("expected at least 3 clusters, got %d", n)
	}

	buckets := reg.Buckets()
	t.Logf("buckets: %d", len(buckets))
	for _, b := range buckets {
		t.Logf("  %s: count=%d", b.ID, b.Count)
	}

	// Now Identify should match.
	// Generate a new point near c1.
	test := make([]float32, dim)
	copy(test, c1)
	for i := range test {
		test[i] += float32(rng.NormFloat64() * 0.03)
	}
	l2Norm(test)

	id, conf, ok := reg.Identify(test)
	if !ok {
		t.Error("expected match after recluster")
	}
	t.Logf("identified: id=%s confidence=%.3f", id, conf)
	if conf < 0.5 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestRegistryIDStability(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	dim := 32

	reg := New(Config{Dim: dim, Threshold: 0.5, Prefix: "s", MinSamples: 2}, nil)

	c1 := randVec(dim, rng)
	c2 := randVec(dim, rng)

	for _, emb := range makeCluster(c1, 5, 0.05, rng) {
		reg.Identify(emb)
	}
	for _, emb := range makeCluster(c2, 5, 0.05, rng) {
		reg.Identify(emb)
	}

	reg.Recluster()
	firstBuckets := reg.Buckets()
	firstIDs := map[string]string{}
	for _, b := range firstBuckets {
		firstIDs[b.ID] = b.ID
	}

	// Add more data to the same clusters and recluster again.
	for _, emb := range makeCluster(c1, 3, 0.05, rng) {
		reg.Identify(emb)
	}
	for _, emb := range makeCluster(c2, 3, 0.05, rng) {
		reg.Identify(emb)
	}

	reg.Recluster()
	secondBuckets := reg.Buckets()

	// IDs should be preserved.
	secondIDs := map[string]bool{}
	for _, b := range secondBuckets {
		secondIDs[b.ID] = true
	}

	preserved := 0
	for id := range firstIDs {
		if secondIDs[id] {
			preserved++
		}
	}
	t.Logf("first: %v, second: %v, preserved: %d/%d",
		keys(firstIDs), mapKeys(secondIDs), preserved, len(firstIDs))

	if preserved < len(firstIDs) {
		t.Errorf("expected all %d IDs preserved, only %d survived", len(firstIDs), preserved)
	}
}

func TestRegistrySetThreshold(t *testing.T) {
	reg := New(Config{Dim: 8, Threshold: 0.9, Prefix: "t"}, nil)
	reg.SetThreshold(0.3)

	// Verify it took effect via a recluster + identify cycle.
	emb := []float32{1, 0, 0, 0, 0, 0, 0, 0}
	reg.Identify(emb)
	reg.Identify(emb)
	n := reg.Recluster()
	if n != 1 {
		t.Errorf("expected 1 cluster, got %d", n)
	}
}

func TestRegistryReset(t *testing.T) {
	reg := New(Config{Dim: 8, Prefix: "x"}, nil)
	reg.Identify([]float32{1, 0, 0, 0, 0, 0, 0, 0})
	reg.Identify([]float32{1, 0, 0, 0, 0, 0, 0, 0})
	reg.Recluster()

	if reg.Len() == 0 || len(reg.Buckets()) == 0 {
		t.Fatal("expected non-empty before reset")
	}

	reg.Reset()
	if reg.Len() != 0 {
		t.Errorf("expected 0 after reset, got %d", reg.Len())
	}
	if len(reg.Buckets()) != 0 {
		t.Errorf("expected 0 buckets after reset, got %d", len(reg.Buckets()))
	}
}

func TestCosineSim(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	c := []float32{1, 0, 0}

	if sim := cosineSim(a, b); math.Abs(float64(sim)) > 0.01 {
		t.Errorf("orthogonal vectors: expected ~0, got %f", sim)
	}
	if sim := cosineSim(a, c); math.Abs(float64(sim)-1.0) > 0.01 {
		t.Errorf("identical vectors: expected ~1, got %f", sim)
	}
}

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()

	seq, err := s.Append([]float32{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if seq != 1 {
		t.Errorf("expected seq=1, got %d", seq)
	}

	s.Append([]float32{4, 5, 6})

	n, _ := s.Len()
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}

	all, _ := s.All()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
	// Verify it's a copy.
	all[0][0] = 999
	all2, _ := s.All()
	if all2[0][0] == 999 {
		t.Error("All() should return copies, not references")
	}

	s.Clear()
	n, _ = s.Len()
	if n != 0 {
		t.Errorf("expected 0 after clear, got %d", n)
	}
}

// Helpers

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
