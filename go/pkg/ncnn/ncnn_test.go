package ncnn

import (
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Error("Version() returned empty string")
	}
	t.Logf("ncnn version: %s", v)
}

// mustMat2D is a test helper that creates a Mat2D or fails the test.
func mustMat2D(tb testing.TB, w, h int, data []float32) *Mat {
	tb.Helper()
	m, err := NewMat2D(w, h, data)
	if err != nil {
		tb.Fatal(err)
	}
	return m
}

func TestMat2D(t *testing.T) {
	data := make([]float32, 80*40) // 40 rows × 80 cols
	for i := range data {
		data[i] = float32(i) * 0.001
	}

	mat := mustMat2D(t, 80, 40, data)
	defer mat.Close()

	if mat.W() != 80 {
		t.Errorf("W() = %d, want 80", mat.W())
	}
	if mat.H() != 40 {
		t.Errorf("H() = %d, want 40", mat.H())
	}
}

func TestMat3D(t *testing.T) {
	data := make([]float32, 10*20*3)
	mat, err := NewMat3D(10, 20, 3, data)
	if err != nil {
		t.Fatal(err)
	}
	defer mat.Close()

	if mat.W() != 10 {
		t.Errorf("W() = %d, want 10", mat.W())
	}
	if mat.H() != 20 {
		t.Errorf("H() = %d, want 20", mat.H())
	}
	if mat.C() != 3 {
		t.Errorf("C() = %d, want 3", mat.C())
	}
}

func TestMatFloatData(t *testing.T) {
	data := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	mat := mustMat2D(t, 5, 1, data)
	defer mat.Close()

	out := mat.FloatData()
	if len(out) != 5 {
		t.Fatalf("FloatData len = %d, want 5", len(out))
	}
	for i, v := range out {
		if v != data[i] {
			t.Errorf("FloatData[%d] = %f, want %f", i, v, data[i])
		}
	}
}

func TestMatDoubleClose(t *testing.T) {
	data := []float32{1, 2, 3}
	mat := mustMat2D(t, 3, 1, data)
	mat.Close()
	mat.Close() // should not panic
}

func TestMat2DEmptyData(t *testing.T) {
	_, err := NewMat2D(0, 0, nil)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestNetClose(t *testing.T) {
	// Close without loading should not panic.
	n := &Net{}
	n.Close()
}

func TestRegisterAndLoadModel(t *testing.T) {
	// Register a fake model (will fail to load, but tests the registry).
	RegisterModel("test-model", []byte("fake-param"), []byte("fake-bin"))
	defer func() {
		// Clean up.
		registryMu.Lock()
		delete(registry, "test-model")
		registryMu.Unlock()
	}()

	ids := ListModels()
	found := false
	for _, id := range ids {
		if id == "test-model" {
			found = true
		}
	}
	if !found {
		t.Error("registered model not found in ListModels()")
	}

	info := GetModelInfo("test-model")
	if info == nil {
		t.Fatal("GetModelInfo returned nil")
	}
	if string(info.ParamData) != "fake-param" {
		t.Errorf("ParamData = %q, want fake-param", info.ParamData)
	}

	// LoadModel will fail because the data isn't valid ncnn format.
	_, err := LoadModel("test-model")
	if err == nil {
		t.Error("expected error loading fake model data")
	}
}

func TestLoadModelNotRegistered(t *testing.T) {
	_, err := LoadModel("nonexistent-model")
	if err == nil {
		t.Error("expected error for unregistered model")
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkLoadModel(b *testing.B) {
	for _, id := range []ModelID{ModelSpeakerERes2Net, ModelVADSilero, ModelDenoiseDTLN1, ModelDenoiseDTLN2} {
		b.Run(string(id), func(b *testing.B) {
			for range b.N {
				net, err := LoadModel(id)
				if err != nil {
					b.Fatal(err)
				}
				net.Close()
			}
		})
	}
}

func BenchmarkSpeakerInference(b *testing.B) {
	net, err := LoadModel(ModelSpeakerERes2Net)
	if err != nil {
		b.Fatal(err)
	}
	defer net.Close()

	data := make([]float32, 40*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		input := mustMat2D(b, 80, 40, data)
		ex, _ := net.NewExtractor()
		ex.SetInput("in0", input)
		output, err := ex.Extract("out0")
		if err != nil {
			b.Fatal(err)
		}
		_ = output.FloatData()
		output.Close()
		ex.Close()
		input.Close()
	}
}

func BenchmarkVADInference(b *testing.B) {
	net, err := LoadModel(ModelVADSilero)
	if err != nil {
		b.Fatal(err)
	}
	defer net.Close()

	audio := make([]float32, 512)
	h := make([]float32, 128)
	c := make([]float32, 128)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		inAudio := mustMat2D(b, 512, 1, audio)
		inH := mustMat2D(b, 128, 1, h)
		inC := mustMat2D(b, 128, 1, c)
		ex, _ := net.NewExtractor()
		ex.SetInput("in0", inAudio)
		ex.SetInput("in1", inH)
		ex.SetInput("in2", inC)
		prob, err := ex.Extract("out0")
		if err != nil {
			b.Fatal(err)
		}
		hOut, _ := ex.Extract("out1")
		cOut, _ := ex.Extract("out2")
		_ = prob.FloatData()
		prob.Close()
		hOut.Close()
		cOut.Close()
		ex.Close()
		inAudio.Close()
		inH.Close()
		inC.Close()
	}
}

func BenchmarkDTLN1Inference(b *testing.B) {
	net, err := LoadModel(ModelDenoiseDTLN1)
	if err != nil {
		b.Fatal(err)
	}
	defer net.Close()

	mag := make([]float32, 257)
	state := make([]float32, 128)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		inMag := mustMat2D(b, 257, 1, mag)
		inH1 := mustMat2D(b, 128, 1, state)
		inC1 := mustMat2D(b, 128, 1, state)
		inH2 := mustMat2D(b, 128, 1, state)
		inC2 := mustMat2D(b, 128, 1, state)
		ex, _ := net.NewExtractor()
		ex.SetInput("in0", inMag)
		ex.SetInput("in1", inH1)
		ex.SetInput("in2", inC1)
		ex.SetInput("in3", inH2)
		ex.SetInput("in4", inC2)
		mask, err := ex.Extract("out0")
		if err != nil {
			b.Fatal(err)
		}
		_ = mask.FloatData()
		mask.Close()
		ex.Close()
		inMag.Close()
		inH1.Close()
		inC1.Close()
		inH2.Close()
		inC2.Close()
	}
}

func BenchmarkDTLN2Inference(b *testing.B) {
	net, err := LoadModel(ModelDenoiseDTLN2)
	if err != nil {
		b.Fatal(err)
	}
	defer net.Close()

	feat := make([]float32, 512)
	state := make([]float32, 128)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		inFeat := mustMat2D(b, 512, 1, feat)
		inH1 := mustMat2D(b, 128, 1, state)
		inC1 := mustMat2D(b, 128, 1, state)
		inH2 := mustMat2D(b, 128, 1, state)
		inC2 := mustMat2D(b, 128, 1, state)
		ex, _ := net.NewExtractor()
		ex.SetInput("in0", inFeat)
		ex.SetInput("in1", inH1)
		ex.SetInput("in2", inC1)
		ex.SetInput("in3", inH2)
		ex.SetInput("in4", inC2)
		out, err := ex.Extract("out0")
		if err != nil {
			b.Fatal(err)
		}
		_ = out.FloatData()
		out.Close()
		ex.Close()
		inFeat.Close()
		inH1.Close()
		inC1.Close()
		inH2.Close()
		inC2.Close()
	}
}

func BenchmarkConcurrentSpeakerInference(b *testing.B) {
	net, err := LoadModel(ModelSpeakerERes2Net)
	if err != nil {
		b.Fatal(err)
	}
	defer net.Close()

	data := make([]float32, 40*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			input := mustMat2D(b, 80, 40, data)
			ex, _ := net.NewExtractor()
			ex.SetInput("in0", input)
			output, err := ex.Extract("out0")
			if err != nil {
				b.Fatal(err)
			}
			_ = output.FloatData()
			output.Close()
			ex.Close()
			input.Close()
		}
	})
}

func TestLoadEmbeddedModels(t *testing.T) {
	models := ListModels()
	t.Logf("registered models: %v", models)
	if len(models) < 4 {
		t.Fatalf("expected at least 4 registered models, got %d", len(models))
	}

	tests := []struct {
		id     ModelID
		inputW int // width of input mat
	}{
		{ModelSpeakerERes2Net, 80},  // [T=40, 80] fbank
		{ModelVADSilero, 512},       // [1, 512] audio
		{ModelDenoiseDTLN1, 257},    // [1, 257] STFT mag
		{ModelDenoiseDTLN2, 512},    // [1, 512] features
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			net, err := LoadModel(tt.id)
			if err != nil {
				t.Fatalf("LoadModel(%s): %v", tt.id, err)
			}
			defer net.Close()
			t.Logf("loaded %s OK", tt.id)
		})
	}
}

func TestEmbeddedModelInference(t *testing.T) {
	// Test speaker model inference
	net, err := LoadModel(ModelSpeakerERes2Net)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer net.Close()

	// Input: [T=40, 80] fbank features
	data := make([]float32, 40*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}
	input := mustMat2D(t, 80, 40, data)
	defer input.Close()

	ex, err := net.NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}
	defer ex.Close()

	if err := ex.SetInput("in0", input); err != nil {
		t.Fatalf("SetInput: %v", err)
	}

	output, err := ex.Extract("out0")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	defer output.Close()

	embedding := output.FloatData()
	if len(embedding) == 0 {
		t.Fatal("empty output")
	}
	t.Logf("speaker embedding: %d dims, first 5: %v", len(embedding), embedding[:5])
}
