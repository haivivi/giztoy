package voiceprint

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// findModelFiles locates the ncnn model files from Bazel runfiles or a
// well-known path. Returns paramPath, binPath, ok.
func findModelFiles(t *testing.T) (string, string, bool) {
	t.Helper()

	// Try Bazel runfiles first.
	if runfiles := os.Getenv("TEST_SRCDIR"); runfiles != "" {
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace == "" {
			workspace = "_main"
		}
		param := filepath.Join(runfiles, workspace, "devops/tools/pnnx/speaker_model.ncnn.param")
		bin := filepath.Join(runfiles, workspace, "devops/tools/pnnx/speaker_model.ncnn.bin")
		if _, err := os.Stat(param); err == nil {
			return param, bin, true
		}
	}

	// Try relative path (for go test outside Bazel).
	for _, base := range []string{
		"../../../devops/tools/pnnx",
		"/tmp/giztoy-onnx",
	} {
		param := filepath.Join(base, "speaker_model.ncnn.param")
		bin := filepath.Join(base, "speaker_model.ncnn.bin")
		if _, err := os.Stat(param); err == nil {
			return param, bin, true
		}
		// Also check the PNNX default naming.
		param = filepath.Join(base, "3dspeaker.ncnn.param")
		bin = filepath.Join(base, "3dspeaker.ncnn.bin")
		if _, err := os.Stat(param); err == nil {
			return param, bin, true
		}
	}

	return "", "", false
}

func TestNCNNModelLoadAndInfer(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	model, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}
	defer model.Close()

	if model.Dimension() != 512 {
		t.Errorf("Dimension() = %d, want 512", model.Dimension())
	}

	// Generate 400ms of 16kHz sine wave (6400 samples = 12800 bytes).
	audio := makeSineWavePCM(440, 6400, 16000)

	embedding, err := model.Extract(audio)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(embedding) != 512 {
		t.Fatalf("embedding length = %d, want 512", len(embedding))
	}

	// Verify embedding is non-trivial (not all zeros).
	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	t.Logf("embedding norm: %.4f", norm)
	if norm < 0.1 {
		t.Error("embedding norm too small — model may not be working")
	}

	// Verify no NaN/Inf.
	for i, v := range embedding {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("embedding[%d] = %f (NaN/Inf)", i, v)
			break
		}
	}

	t.Logf("first 10: %v", embedding[:10])
}

func TestNCNNModelFromMemory(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	paramData, err := os.ReadFile(paramPath)
	if err != nil {
		t.Fatalf("read param: %v", err)
	}
	binData, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read bin: %v", err)
	}

	model, err := NewNCNNModelFromMemory(paramData, binData)
	if err != nil {
		t.Fatalf("NewNCNNModelFromMemory: %v", err)
	}
	defer model.Close()

	// Same audio as file-based test.
	audio := makeSineWavePCM(440, 6400, 16000)

	embedding, err := model.Extract(audio)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(embedding) != 512 {
		t.Fatalf("embedding length = %d, want 512", len(embedding))
	}

	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	t.Logf("memory model embedding norm: %.4f", norm)
	if norm < 0.1 {
		t.Error("embedding norm too small")
	}
}

func TestNCNNModelConsistency(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	// File-based model.
	fileModel, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}
	defer fileModel.Close()

	// Memory-based model.
	paramData, _ := os.ReadFile(paramPath)
	binData, _ := os.ReadFile(binPath)
	memModel, err := NewNCNNModelFromMemory(paramData, binData)
	if err != nil {
		t.Fatalf("NewNCNNModelFromMemory: %v", err)
	}
	defer memModel.Close()

	audio := makeSineWavePCM(440, 6400, 16000)

	emb1, err := fileModel.Extract(audio)
	if err != nil {
		t.Fatalf("file Extract: %v", err)
	}

	emb2, err := memModel.Extract(audio)
	if err != nil {
		t.Fatalf("memory Extract: %v", err)
	}

	// Both should produce identical embeddings.
	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("embedding[%d]: file=%f memory=%f", i, emb1[i], emb2[i])
			break
		}
	}
	t.Log("file and memory models produce identical embeddings")
}

func TestNCNNModelSpeakerDiscrimination(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	model, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}
	defer model.Close()

	// Two different "speakers" — different frequency content.
	speaker1 := makeSineWavePCM(200, 6400, 16000)  // low voice
	speaker2 := makeSineWavePCM(4000, 6400, 16000) // high voice

	emb1, err := model.Extract(speaker1)
	if err != nil {
		t.Fatalf("Extract speaker1: %v", err)
	}

	emb2, err := model.Extract(speaker2)
	if err != nil {
		t.Fatalf("Extract speaker2: %v", err)
	}

	// Compute cosine similarity.
	var dot, norm1, norm2 float64
	for i := range emb1 {
		dot += float64(emb1[i]) * float64(emb2[i])
		norm1 += float64(emb1[i]) * float64(emb1[i])
		norm2 += float64(emb2[i]) * float64(emb2[i])
	}
	cosine := dot / (math.Sqrt(norm1) * math.Sqrt(norm2))

	t.Logf("cosine similarity between 200Hz and 4kHz: %.4f", cosine)

	// Different frequency content should produce different embeddings.
	// Cosine similarity should be less than 1.0 (not identical).
	if cosine > 0.99 {
		t.Error("embeddings too similar — model may not be discriminating")
	}
}

func TestNCNNModelAudioTooShort(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	model, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}
	defer model.Close()

	// Too short audio (10 samples = 20 bytes).
	_, err = model.Extract(make([]byte, 20))
	if err == nil {
		t.Error("expected error for too-short audio")
	}
}

func TestNCNNModelClose(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	model, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}

	// Close should succeed.
	if err := model.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Double close should be safe.
	if err := model.Close(); err != nil {
		t.Errorf("double Close: %v", err)
	}

	// Extract after close should fail.
	audio := makeSineWavePCM(440, 6400, 16000)
	_, err = model.Extract(audio)
	if err == nil {
		t.Error("expected error after Close")
	}
}

func TestNCNNEndToEndPipeline(t *testing.T) {
	paramPath, binPath, ok := findModelFiles(t)
	if !ok {
		t.Skip("ncnn model files not found; run: bazel build //devops/tools/pnnx:convert")
	}

	model, err := NewNCNNModel(paramPath, binPath)
	if err != nil {
		t.Fatalf("NewNCNNModel: %v", err)
	}
	defer model.Close()

	hasher := NewHasher(model.Dimension(), 16, 42)
	detector := NewDetector(WithWindowSize(3), WithMinRatio(0.6))

	// Feed the same "speaker" multiple times.
	audio := makeSineWavePCM(440, 6400, 16000)

	var lastChunk *SpeakerChunk
	for range 5 {
		emb, err := model.Extract(audio)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		hash := hasher.Hash(emb)
		chunk := detector.Feed(hash)
		if chunk != nil {
			lastChunk = chunk
		}
		t.Logf("hash=%s chunk=%+v", hash, chunk)
	}

	if lastChunk == nil {
		t.Fatal("expected non-nil speaker chunk after 5 feeds")
	}
	if lastChunk.Status != StatusSingle {
		t.Errorf("expected StatusSingle, got %s", lastChunk.Status)
	}
	if lastChunk.Speaker == "" {
		t.Error("expected non-empty speaker label")
	}
	t.Logf("end-to-end: status=%s speaker=%s confidence=%.2f",
		lastChunk.Status, lastChunk.Speaker, lastChunk.Confidence)
}
