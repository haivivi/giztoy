package voiceprint

import (
	"context"
	"io"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// testModel is a deterministic Model for testing.
// It produces an embedding based on the mean sample value.
type testModel struct {
	dim int
}

func (m *testModel) Extract(audio []byte) ([]float32, error) {
	// Compute mean of PCM16 samples as a simple feature.
	if len(audio) < 2 {
		return make([]float32, m.dim), nil
	}
	var sum float64
	nSamples := len(audio) / 2
	for i := 0; i < len(audio)-1; i += 2 {
		sample := int16(audio[i]) | int16(audio[i+1])<<8
		sum += float64(sample)
	}
	mean := float32(sum / float64(nSamples))

	// Fill embedding with scaled values.
	emb := make([]float32, m.dim)
	for i := range emb {
		emb[i] = mean * float32(i+1) * 0.001
	}
	return emb, nil
}

func (m *testModel) Dimension() int { return m.dim }
func (m *testModel) Close() error   { return nil }

// makePCM creates PCM16 audio filled with a constant sample value.
func makePCM(sampleValue int16, nSamples int) []byte {
	data := make([]byte, nSamples*2)
	lo := byte(sampleValue & 0xFF)
	hi := byte((sampleValue >> 8) & 0xFF)
	for i := 0; i < len(data); i += 2 {
		data[i] = lo
		data[i+1] = hi
	}
	return data
}

// inputStream creates a genx.Stream from a slice of chunks.
func inputStream(chunks []*genx.MessageChunk) genx.Stream {
	buf := buffer.N[*genx.MessageChunk](len(chunks) + 1)
	for _, c := range chunks {
		buf.Add(c)
	}
	buf.CloseWrite()
	return &vpStream{buf: buf}
}

func TestTransformerPassThrough(t *testing.T) {
	model := &testModel{dim: 8}
	hasher := NewHasher(8, 16, 42)
	detector := NewDetector(WithWindowSize(3))

	tr := NewTransformer(model, hasher, detector,
		WithSegmentDuration(100),
		WithSampleRate(16000),
	)

	// 100ms at 16kHz = 1600 samples = 3200 bytes
	pcmData := makePCM(1000, 1600)

	input := inputStream([]*genx.MessageChunk{
		{
			Role: genx.RoleUser,
			Name: "mic",
			Part: &genx.Blob{MIMEType: "audio/pcm", Data: pcmData},
		},
		{
			Role: genx.RoleUser,
			Name: "mic",
			Part: &genx.Blob{MIMEType: "audio/pcm", Data: pcmData},
		},
		// A text chunk should pass through unchanged.
		{
			Role: genx.RoleUser,
			Name: "text",
			Part: genx.Text("hello"),
		},
	})

	out, err := tr.Transform(context.Background(), "", input)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	var chunks []*genx.MessageChunk
	for {
		c, err := out.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		chunks = append(chunks, c)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 output chunks, got %d", len(chunks))
	}

	// First two should be audio with speaker annotation.
	for i := 0; i < 2; i++ {
		blob, ok := chunks[i].Part.(*genx.Blob)
		if !ok {
			t.Errorf("chunk %d: expected Blob, got %T", i, chunks[i].Part)
			continue
		}
		if blob.MIMEType != "audio/pcm" {
			t.Errorf("chunk %d: expected audio/pcm, got %s", i, blob.MIMEType)
		}
		if chunks[i].Role != genx.RoleUser {
			t.Errorf("chunk %d: Role changed to %s", i, chunks[i].Role)
		}
		if chunks[i].Name != "mic" {
			t.Errorf("chunk %d: Name changed to %s", i, chunks[i].Name)
		}
	}

	// Third should be text, passed through.
	text, ok := chunks[2].Part.(genx.Text)
	if !ok {
		t.Errorf("chunk 2: expected Text, got %T", chunks[2].Part)
	} else if text != "hello" {
		t.Errorf("chunk 2: text = %q, want hello", text)
	}
}

func TestTransformerSpeakerDetection(t *testing.T) {
	model := &testModel{dim: 8}
	hasher := NewHasher(8, 16, 42)
	detector := NewDetector(WithWindowSize(3), WithMinRatio(0.6))

	tr := NewTransformer(model, hasher, detector,
		WithSegmentDuration(100),
		WithSampleRate(16000),
	)

	// 100ms at 16kHz = 1600 samples = 3200 bytes
	// Feed the same speaker audio multiple times.
	pcmData := makePCM(5000, 1600)

	var inputChunks []*genx.MessageChunk
	for range 5 {
		inputChunks = append(inputChunks, &genx.MessageChunk{
			Role: genx.RoleUser,
			Name: "mic",
			Part: &genx.Blob{MIMEType: "audio/pcm", Data: pcmData},
		})
	}
	input := inputStream(inputChunks)

	out, err := tr.Transform(context.Background(), "", input)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	var lastLabel string
	for {
		c, err := out.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if c.Ctrl != nil && c.Ctrl.Label != "" {
			lastLabel = c.Ctrl.Label
		}
	}

	// After enough audio, we should have a speaker label.
	if lastLabel == "" {
		t.Error("expected speaker label to be set after 5 audio chunks")
	} else {
		t.Logf("detected speaker: %s", lastLabel)
		if lastLabel[:6] != "voice:" {
			t.Errorf("label should start with 'voice:', got %q", lastLabel)
		}
	}
}

func TestTransformerEoS(t *testing.T) {
	model := &testModel{dim: 8}
	hasher := NewHasher(8, 16, 42)
	detector := NewDetector(WithWindowSize(3))

	tr := NewTransformer(model, hasher, detector,
		WithSegmentDuration(100),
		WithSampleRate(16000),
	)

	pcmData := makePCM(1000, 1600)

	input := inputStream([]*genx.MessageChunk{
		{
			Role: genx.RoleUser,
			Name: "mic",
			Part: &genx.Blob{MIMEType: "audio/pcm", Data: pcmData},
		},
		// Audio EoS.
		{
			Role: genx.RoleUser,
			Name: "mic",
			Part: &genx.Blob{MIMEType: "audio/pcm"},
			Ctrl: &genx.StreamCtrl{EndOfStream: true},
		},
	})

	out, err := tr.Transform(context.Background(), "", input)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	var chunks []*genx.MessageChunk
	for {
		c, err := out.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		chunks = append(chunks, c)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (audio + EoS), got %d", len(chunks))
	}

	// Last chunk should be EoS.
	last := chunks[len(chunks)-1]
	if !last.IsEndOfStream() {
		t.Error("last chunk should be EoS")
	}
	blob, ok := last.Part.(*genx.Blob)
	if !ok {
		t.Error("EoS should have Blob part")
	} else if blob.MIMEType != "audio/pcm" {
		t.Errorf("EoS MIME = %q, want audio/pcm", blob.MIMEType)
	}
}

func TestTransformerCancel(t *testing.T) {
	model := &testModel{dim: 8}
	hasher := NewHasher(8, 16, 42)
	detector := NewDetector()

	tr := NewTransformer(model, hasher, detector)

	// Create a slow input stream that blocks.
	buf := buffer.N[*genx.MessageChunk](10)
	input := &vpStream{buf: buf}

	ctx, cancel := context.WithCancel(context.Background())

	out, err := tr.Transform(ctx, "", input)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// Cancel context.
	cancel()

	// Output should return an error.
	_, err = out.Next()
	if err == nil {
		t.Error("expected error after cancel")
	}
}
