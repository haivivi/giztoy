package onnx

import (
	"math"
	"os"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/ncnn"
)

// TestERes2NetNCNNvsONNX compares the output of ERes2Net model run through
// ncnn (pnnx-converted) and ONNX Runtime (original ONNX) with identical input.
//
// This validates that the pnnx model conversion is correct.
// A cosine similarity > 0.99 between the two outputs indicates correct conversion.
func TestERes2NetNCNNvsONNX(t *testing.T) {
	// Load ONNX model.
	onnxPath := findRunfile(t, "+pnnx_ext+onnx_speaker_eres2net/model.onnx", "ONNX_ERES2NET_PATH")

	onnxData, err := os.ReadFile(onnxPath)
	if err != nil {
		t.Fatal(err)
	}

	env, err := NewEnv("compare")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	onnxSession, err := env.NewSession(onnxData)
	if err != nil {
		t.Fatal(err)
	}
	defer onnxSession.Close()

	// Load ncnn model.
	ncnnNet, err := ncnn.LoadModel(ncnn.ModelSpeakerERes2Net)
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnNet.Close()

	// Prepare identical input: [1, T=40, 80] for ONNX, [T=40, 80] for ncnn.
	T := 40
	data := make([]float32, T*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}

	// === Run ONNX ===
	onnxInput, err := NewTensor([]int64{1, int64(T), 80}, data)
	if err != nil {
		t.Fatal(err)
	}
	defer onnxInput.Close()

	onnxOutputs, err := onnxSession.Run(
		[]string{"x"}, []*Tensor{onnxInput},
		[]string{"embedding"},
	)
	if err != nil {
		t.Fatalf("ONNX Run: %v", err)
	}
	defer onnxOutputs[0].Close()

	onnxEmb, err := onnxOutputs[0].FloatData()
	if err != nil {
		t.Fatal(err)
	}

	// === Run ncnn ===
	ncnnInput, err := ncnn.NewMat2D(80, T, data)
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnInput.Close()

	ncnnEx, err := ncnnNet.NewExtractor()
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnEx.Close()

	if err := ncnnEx.SetInput("in0", ncnnInput); err != nil {
		t.Fatal(err)
	}

	ncnnOutput, err := ncnnEx.Extract("out0")
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnOutput.Close()

	ncnnEmb := ncnnOutput.FloatData()

	// === Compare ===
	if len(onnxEmb) != len(ncnnEmb) {
		t.Fatalf("dimension mismatch: ONNX=%d, ncnn=%d", len(onnxEmb), len(ncnnEmb))
	}
	t.Logf("ONNX embedding: %d dims, first 5: %v", len(onnxEmb), onnxEmb[:5])
	t.Logf("ncnn embedding: %d dims, first 5: %v", len(ncnnEmb), ncnnEmb[:5])

	// Cosine similarity
	var dotProduct, normA, normB float64
	for i := range onnxEmb {
		a, b := float64(onnxEmb[i]), float64(ncnnEmb[i])
		dotProduct += a * b
		normA += a * a
		normB += b * b
	}
	cosSim := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	t.Logf("cosine similarity: %.6f", cosSim)

	// Max absolute difference
	maxDiff := float64(0)
	for i := range onnxEmb {
		diff := math.Abs(float64(onnxEmb[i]) - float64(ncnnEmb[i]))
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	t.Logf("max absolute difference: %.6f", maxDiff)

	// Acceptance criteria:
	// - Cosine similarity > 0.99 (very similar)
	// - Max diff < 0.1 (no catastrophic divergence)
	if cosSim < 0.99 {
		t.Errorf("cosine similarity too low: %.6f (want > 0.99)", cosSim)
	}
	if maxDiff > 0.1 {
		t.Errorf("max difference too large: %.6f (want < 0.1)", maxDiff)
	}
}
