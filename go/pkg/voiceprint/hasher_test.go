package voiceprint

import (
	"testing"
)

func TestHasherDeterministic(t *testing.T) {
	h := NewHasher(192, 16, 42)

	// Same embedding → same hash.
	emb := make([]float32, 192)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}

	hash1 := h.Hash(emb)
	hash2 := h.Hash(emb)
	if hash1 != hash2 {
		t.Errorf("same embedding produced different hashes: %q vs %q", hash1, hash2)
	}
	if len(hash1) != 4 { // 16 bits = 4 hex chars
		t.Errorf("expected 4 hex chars, got %d: %q", len(hash1), hash1)
	}
	t.Logf("hash = %s", hash1)
}

func TestHasherSeedMatters(t *testing.T) {
	emb := make([]float32, 192)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}

	h1 := NewHasher(192, 16, 1)
	h2 := NewHasher(192, 16, 2)

	hash1 := h1.Hash(emb)
	hash2 := h2.Hash(emb)

	// Different seeds should (very likely) produce different hashes.
	// Not guaranteed, but extremely unlikely with 16 bits.
	if hash1 == hash2 {
		t.Logf("warning: different seeds produced same hash %q (unlikely but possible)", hash1)
	}
}

func TestHasherSimilarVectors(t *testing.T) {
	h := NewHasher(192, 16, 42)

	// Two very similar embeddings should produce the same hash.
	emb1 := make([]float32, 192)
	emb2 := make([]float32, 192)
	for i := range emb1 {
		emb1[i] = float32(i) * 0.01
		emb2[i] = float32(i)*0.01 + 0.0001 // tiny perturbation
	}

	hash1 := h.Hash(emb1)
	hash2 := h.Hash(emb2)

	if hash1 != hash2 {
		t.Logf("similar vectors got different hashes: %q vs %q (can happen, but rare)", hash1, hash2)
	}

	// Two very different embeddings should (likely) produce different hashes.
	emb3 := make([]float32, 192)
	for i := range emb3 {
		emb3[i] = -float32(i) * 0.05 // opposite direction, different scale
	}
	hash3 := h.Hash(emb3)
	t.Logf("hash1=%s hash3=%s", hash1, hash3)
	if hash1 == hash3 {
		t.Logf("warning: very different vectors got same hash (unlikely)")
	}
}

func TestHasherHexFormat(t *testing.T) {
	h := NewHasher(8, 16, 99)

	emb := []float32{1, 2, 3, 4, 5, 6, 7, 8}
	hash := h.Hash(emb)

	// Must be 4 uppercase hex chars.
	if len(hash) != 4 {
		t.Fatalf("expected length 4, got %d: %q", len(hash), hash)
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("non-uppercase-hex char %c in hash %q", c, hash)
		}
	}
}

func TestHasherMultiPrecision(t *testing.T) {
	h := NewHasher(192, 16, 42)

	emb := make([]float32, 192)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}

	full := h.Hash(emb) // "XXXX"
	if len(full) != 4 {
		t.Fatalf("expected 4 chars, got %d", len(full))
	}

	// Prefix truncation gives coarser matches.
	coarse12 := full[:3] // 12 bit
	coarse8 := full[:2]  // 8 bit
	coarse4 := full[:1]  // 4 bit

	t.Logf("16-bit: %s", full)
	t.Logf("12-bit: %s", coarse12)
	t.Logf(" 8-bit: %s", coarse8)
	t.Logf(" 4-bit: %s", coarse4)

	// They should be proper prefixes.
	if full[:3] != coarse12 || full[:2] != coarse8 || full[:1] != coarse4 {
		t.Error("prefix truncation broken")
	}
}

func TestHasherPanics(t *testing.T) {
	t.Run("bad_bits", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for bits=3")
			}
		}()
		NewHasher(192, 3, 0)
	})

	t.Run("bad_dim", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for dim=0")
			}
		}()
		NewHasher(0, 16, 0)
	})

	t.Run("dim_mismatch", func(t *testing.T) {
		h := NewHasher(192, 16, 0)
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for wrong dim")
			}
		}()
		h.Hash([]float32{1, 2, 3}) // wrong dimension
	})
}

func TestHasherAccessors(t *testing.T) {
	h := NewHasher(192, 16, 0)
	if h.Bits() != 16 {
		t.Errorf("Bits() = %d, want 16", h.Bits())
	}
	if h.Dim() != 192 {
		t.Errorf("Dim() = %d, want 192", h.Dim())
	}
}

func BenchmarkHash(b *testing.B) {
	h := NewHasher(192, 16, 42)
	emb := make([]float32, 192)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}
	b.ResetTimer()
	for range b.N {
		h.Hash(emb)
	}
}
