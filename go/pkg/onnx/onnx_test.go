package onnx

import (
	"math"
	"testing"
)

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
	_, err := NewTensor([]int64{2, 3}, []float32{1, 2, 3})
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
	env.Close()
}

func TestListModels(t *testing.T) {
	models := ListModels()
	t.Logf("registered ONNX models: %v", models)
	if len(models) < 2 {
		t.Fatalf("expected at least 2 models, got %d", len(models))
	}
}

func TestERes2NetONNX(t *testing.T) {
	env, err := NewEnv("test")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	session, err := LoadModel(env, ModelSpeakerERes2Net)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

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
	defer outputs[0].Close()

	emb, err := outputs[0].FloatData()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ERes2Net: %d dims, first 5: %v", len(emb), emb[:5])

	if len(emb) != 512 {
		t.Errorf("expected 512-dim, got %d", len(emb))
	}
	for i, v := range emb {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("emb[%d] = %f (NaN/Inf)", i, v)
		}
	}
}

func TestNSNet2ONNX(t *testing.T) {
	env, err := NewEnv("test")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	session, err := LoadModel(env, ModelDenoiseNSNet2)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Input: [1, 5, 161] — 5 frames of log-power spectrum
	frames := 5
	data := make([]float32, 1*frames*161)
	for i := range data {
		data[i] = float32(i%161) * -0.05
	}

	input, err := NewTensor([]int64{1, int64(frames), 161}, data)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()

	outputs, err := session.Run(
		[]string{"input"}, []*Tensor{input},
		[]string{"output"},
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer outputs[0].Close()

	mask, err := outputs[0].FloatData()
	if err != nil {
		t.Fatal(err)
	}
	expected := frames * 161
	if len(mask) != expected {
		t.Fatalf("mask len = %d, want %d", len(mask), expected)
	}

	// Mask should be in [0, 1] (sigmoid output)
	for i, v := range mask {
		if v < 0 || v > 1 {
			t.Errorf("mask[%d] = %f, out of [0,1]", i, v)
			break
		}
	}
	t.Logf("NSNet2: %d values, first 5: [%.4f, %.4f, %.4f, %.4f, %.4f]",
		len(mask), mask[0], mask[1], mask[2], mask[3], mask[4])
}
