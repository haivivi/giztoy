package fbank

import (
	"math"
	"testing"
)

func TestHammingWindow(t *testing.T) {
	w := hammingWindow(400)
	if len(w) != 400 {
		t.Fatalf("expected 400, got %d", len(w))
	}
	// Hamming window: endpoints should be ~0.08
	if math.Abs(w[0]-0.08) > 0.01 {
		t.Errorf("w[0] = %f, want ~0.08", w[0])
	}
	// Center should be ~1.0
	if math.Abs(w[199]-1.0) > 0.02 {
		t.Errorf("w[199] = %f, want ~1.0", w[199])
	}
}

func TestMelConversion(t *testing.T) {
	// HTK mel scale: 2595 * log10(1 + f/700)
	// hzToMel(1000) = 2595 * log10(1 + 1000/700) ≈ 1000.45
	mel := hzToMel(1000)
	if math.Abs(mel-1000.45) > 1.0 {
		t.Errorf("hzToMel(1000) = %f, want ~1000.45", mel)
	}
	// Round-trip
	hz := melToHz(mel)
	if math.Abs(hz-1000) > 0.1 {
		t.Errorf("melToHz(hzToMel(1000)) = %f, want 1000", hz)
	}
}

func TestMelFilterBank(t *testing.T) {
	bank := melFilterBank(80, 512, 16000, 20, 7600)
	if len(bank) != 80 {
		t.Fatalf("expected 80 filters, got %d", len(bank))
	}
	halfFFT := 512/2 + 1
	for i, f := range bank {
		if len(f) != halfFFT {
			t.Fatalf("filter %d: expected %d bins, got %d", i, halfFFT, len(f))
		}
	}
	// Each filter should have at least one non-zero coefficient
	for i, f := range bank {
		hasNonZero := false
		for _, v := range f {
			if v > 0 {
				hasNonZero = true
				break
			}
		}
		if !hasNonZero {
			t.Errorf("filter %d is all zeros", i)
		}
	}
}

func TestFFT(t *testing.T) {
	// Test with known signal: DC + 1Hz cosine in 8-sample window
	n := 8
	real := make([]float64, n)
	imag := make([]float64, n)
	for i := range real {
		real[i] = 1.0 + math.Cos(2*math.Pi*float64(i)/float64(n))
	}
	FFT(real, imag)

	// DC component should be n (sum of 1.0*8)
	if math.Abs(real[0]-float64(n)) > 0.01 {
		t.Errorf("DC = %f, want %d", real[0], n)
	}
	// First harmonic should be n/2
	if math.Abs(real[1]-float64(n)/2) > 0.01 {
		t.Errorf("H1 real = %f, want %f", real[1], float64(n)/2)
	}
}

func TestExtract(t *testing.T) {
	cfg := DefaultConfig()
	ext := New(cfg)

	// Generate 1 second of 440Hz sine at 16kHz
	n := 16000
	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = float32(math.Sin(2 * math.Pi * 440 * float64(i) / 16000))
	}

	features := ext.Extract(pcm)
	expectedFrames := (n - cfg.WindowSize) / cfg.HopSize + 1
	if len(features) != expectedFrames {
		t.Fatalf("expected %d frames, got %d", expectedFrames, len(features))
	}
	if len(features[0]) != 80 {
		t.Fatalf("expected 80 mels, got %d", len(features[0]))
	}

	// All values should be finite
	for i, f := range features {
		for j, v := range f {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Errorf("features[%d][%d] = %f (not finite)", i, j, v)
			}
		}
	}

	t.Logf("extracted %d frames x %d mels", len(features), len(features[0]))
	t.Logf("first frame range: [%f, %f]", features[0][0], features[0][79])
}

func TestExtractFromInt16(t *testing.T) {
	cfg := DefaultConfig()
	ext := New(cfg)

	// 0.5s of 440Hz at 16kHz, int16
	n := 8000
	pcm := make([]byte, n*2)
	for i := 0; i < n; i++ {
		s := int16(math.Sin(2*math.Pi*440*float64(i)/16000) * 32767)
		pcm[i*2] = byte(s)
		pcm[i*2+1] = byte(s >> 8)
	}

	features := ext.ExtractFromInt16(pcm)
	if len(features) == 0 {
		t.Fatal("no features extracted")
	}
	t.Logf("extracted %d frames from int16 input", len(features))
}

func TestFlatten(t *testing.T) {
	features := [][]float32{{1, 2, 3}, {4, 5, 6}}
	flat := Flatten(features)
	expected := []float32{1, 2, 3, 4, 5, 6}
	if len(flat) != len(expected) {
		t.Fatalf("expected len %d, got %d", len(expected), len(flat))
	}
	for i, v := range flat {
		if v != expected[i] {
			t.Errorf("flat[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestCMVN(t *testing.T) {
	cfg := DefaultConfig()
	ext := New(cfg)

	pcm := make([]float32, 16000)
	for i := range pcm {
		pcm[i] = float32(math.Sin(2*math.Pi*440*float64(i)/16000)) * 0.5
	}

	features := ext.Extract(pcm)
	CMVN(features)

	// After CMVN, each dimension should have mean ~0 and std ~1
	numMels := len(features[0])
	for m := 0; m < numMels; m++ {
		sum := float64(0)
		for _, f := range features {
			sum += float64(f[m])
		}
		mean := sum / float64(len(features))
		if math.Abs(mean) > 0.01 {
			t.Errorf("mel[%d] mean = %f, want ~0", m, mean)
		}

		varSum := float64(0)
		for _, f := range features {
			d := float64(f[m]) - mean
			varSum += d * d
		}
		std := math.Sqrt(varSum / float64(len(features)))
		if math.Abs(std-1.0) > 0.01 {
			t.Errorf("mel[%d] std = %f, want ~1", m, std)
		}
	}
	t.Log("CMVN: all dimensions have mean≈0, std≈1")
}

func BenchmarkExtract(b *testing.B) {
	cfg := DefaultConfig()
	ext := New(cfg)

	// 3 seconds at 16kHz
	pcm := make([]float32, 48000)
	for i := range pcm {
		pcm[i] = float32(math.Sin(2*math.Pi*440*float64(i)/16000)) * 0.5
	}

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_ = ext.Extract(pcm)
	}
}
