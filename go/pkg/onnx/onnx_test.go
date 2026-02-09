package onnx

import (
	"math"
	"os"
	"testing"
)

// findRunfile locates a file in Bazel runfiles or via env var.
func findRunfile(t *testing.T, rlocation, envVar string) string {
	t.Helper()

	// Check env var first.
	if p := os.Getenv(envVar); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try Bazel TEST_SRCDIR + runfiles.
	srcDir := os.Getenv("TEST_SRCDIR")
	if srcDir != "" {
		p := srcDir + "/" + rlocation
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try relative paths.
	candidates := []string{
		rlocation,
		"../" + rlocation,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	t.Skipf("skip: file %s not found (set %s or run via Bazel)", rlocation, envVar)
	return ""
}

func TestNewEnv(t *testing.T) {
	env, err := NewEnv("test")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	t.Log("created ONNX Runtime environment")
}

func TestNewTensor(t *testing.T) {
	data := []float32{1, 2, 3, 4, 5, 6}
	tensor, err := NewTensor([]int64{2, 3}, data)
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Close()

	shape, err := tensor.Shape()
	if err != nil {
		t.Fatal(err)
	}
	if len(shape) != 2 || shape[0] != 2 || shape[1] != 3 {
		t.Errorf("shape = %v, want [2,3]", shape)
	}

	out, err := tensor.FloatData()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 {
		t.Fatalf("len = %d, want 6", len(out))
	}
	for i, v := range out {
		if v != data[i] {
			t.Errorf("[%d] = %f, want %f", i, v, data[i])
		}
	}
}

func TestTensorEmptyData(t *testing.T) {
	_, err := NewTensor([]int64{0}, nil)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestTensorShortData(t *testing.T) {
	_, err := NewTensor([]int64{2, 3}, []float32{1, 2, 3}) // need 6, got 3
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestEnvDoubleClose(t *testing.T) {
	env, err := NewEnv("test")
	if err != nil {
		t.Fatal(err)
	}
	env.Close()
	env.Close() // should not panic
}

// TestERes2NetONNX loads the ERes2Net ONNX model and runs inference.
// The model file path is provided via ONNX_ERES2NET_PATH env var or
// Bazel runfiles.
func TestERes2NetONNX(t *testing.T) {
	modelPath := findRunfile(t, "+pnnx_ext+onnx_speaker_eres2net/model.onnx", "ONNX_ERES2NET_PATH")

	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read model: %v", err)
	}
	t.Logf("loaded ERes2Net ONNX model: %d bytes", len(modelData))

	env, err := NewEnv("test")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	session, err := env.NewSession(modelData)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Input: [1, T=40, 80] fbank features
	T := 40
	data := make([]float32, 1*T*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}

	input, err := NewTensor([]int64{1, int64(T), 80}, data)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()

	outputs, err := session.Run(
		[]string{"x"}, []*Tensor{input},
		[]string{"embedding"},
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outputs))
	}
	defer outputs[0].Close()

	emb, err := outputs[0].FloatData()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ERes2Net ONNX output: %d dims, first 5: %v", len(emb), emb[:5])

	// Should be 512-dim
	if len(emb) != 512 {
		t.Errorf("expected 512-dim embedding, got %d", len(emb))
	}

	// Should not contain NaN
	for i, v := range emb {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("emb[%d] = %f (NaN/Inf)", i, v)
		}
	}
}
