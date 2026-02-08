package vecstore

import (
	"fmt"
	"testing"
)

func TestMemoryInsertAndSearch(t *testing.T) {
	vec := NewMemory()

	_ = vec.Insert("a", []float32{1, 0, 0, 0})
	_ = vec.Insert("b", []float32{0, 1, 0, 0})
	_ = vec.Insert("c", []float32{0.9, 0.1, 0, 0})

	matches, err := vec.Search([]float32{1, 0, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].ID != "a" {
		t.Errorf("top match = %q, want 'a'", matches[0].ID)
	}
	if matches[1].ID != "c" {
		t.Errorf("second match = %q, want 'c'", matches[1].ID)
	}
}

func TestMemoryBatchInsert(t *testing.T) {
	vec := NewMemory()
	ids := []string{"a", "b", "c"}
	vecs := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	}
	if err := vec.BatchInsert(ids, vecs); err != nil {
		t.Fatal(err)
	}
	if vec.Len() != 3 {
		t.Errorf("Len = %d, want 3", vec.Len())
	}
	matches, err := vec.Search([]float32{1, 0, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "a" {
		t.Errorf("expected match 'a', got %v", matches)
	}
}

func TestMemoryDelete(t *testing.T) {
	vec := NewMemory()
	_ = vec.Insert("a", []float32{1, 0})
	if vec.Len() != 1 {
		t.Fatalf("Len = %d, want 1", vec.Len())
	}
	_ = vec.Delete("a")
	if vec.Len() != 0 {
		t.Errorf("Len after delete = %d, want 0", vec.Len())
	}
	// Delete nonexistent should not error.
	if err := vec.Delete("nonexistent"); err != nil {
		t.Fatal(err)
	}
}

func TestMemorySearchEmpty(t *testing.T) {
	vec := NewMemory()
	matches, err := vec.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil for empty index, got %v", matches)
	}
}

func TestMemoryFlush(t *testing.T) {
	if err := NewMemory().Flush(); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryClose(t *testing.T) {
	if err := NewMemory().Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCosineDistance(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float32
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 0},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 1},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, 2},
		{"similar", []float32{1, 0.1, 0}, []float32{1, 0, 0}, 0.005},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineDistance(tt.a, tt.b)
			if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
				t.Errorf("CosineDistance = %f, want ~%f", got, tt.want)
			}
		})
	}
}

func TestCosineDistanceEdgeCases(t *testing.T) {
	// Dimension mismatch.
	d := CosineDistance([]float32{1, 0}, []float32{1, 0, 0})
	if d != 2 {
		t.Errorf("dimension mismatch: got %f, want 2", d)
	}
	// Zero vector.
	d = CosineDistance([]float32{0, 0, 0}, []float32{1, 0, 0})
	if d != 0 {
		t.Errorf("zero vector: got %f, want 0", d)
	}
}

// Ensure Memory implements Index.
var _ Index = (*Memory)(nil)

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkMemorySearch(b *testing.B) {
	vec := NewMemory()
	for i := 0; i < 1000; i++ {
		v := []float32{
			float32(i%7) / 7.0,
			float32(i%11) / 11.0,
			float32(i%13) / 13.0,
			float32(i%17) / 17.0,
		}
		_ = vec.Insert(fmt.Sprintf("v-%d", i), v)
	}

	query := []float32{0.5, 0.5, 0.5, 0.5}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vec.Search(query, 10)
	}
}
