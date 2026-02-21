package genx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testdataPath(parts ...string) string {
	return filepath.Join("..", "..", "..", "testdata", "genx", filepath.Join(parts...))
}

func TestStreamCtrl_JSON_BOS(t *testing.T) {
	data, err := os.ReadFile(testdataPath("stream_ctrl", "bos_chunk.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var ctrl StreamCtrl
	if err := json.Unmarshal(data, &ctrl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ctrl.StreamID != "abc123" {
		t.Errorf("StreamID = %q, want %q", ctrl.StreamID, "abc123")
	}
	if !ctrl.BeginOfStream {
		t.Error("BeginOfStream should be true")
	}
	if ctrl.EndOfStream {
		t.Error("EndOfStream should be false")
	}
}

func TestStreamCtrl_JSON_EOS(t *testing.T) {
	data, err := os.ReadFile(testdataPath("stream_ctrl", "eos_chunk.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var ctrl StreamCtrl
	if err := json.Unmarshal(data, &ctrl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ctrl.EndOfStream {
		t.Error("EndOfStream should be true")
	}
	if ctrl.BeginOfStream {
		t.Error("BeginOfStream should be false")
	}
}

func TestStreamCtrl_JSON_Full(t *testing.T) {
	data, err := os.ReadFile(testdataPath("stream_ctrl", "full_chunk.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var ctrl StreamCtrl
	if err := json.Unmarshal(data, &ctrl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ctrl.StreamID != "s1" {
		t.Errorf("StreamID = %q, want %q", ctrl.StreamID, "s1")
	}
	if ctrl.Label != "debug-tag" {
		t.Errorf("Label = %q, want %q", ctrl.Label, "debug-tag")
	}
	if ctrl.BeginOfStream {
		t.Error("BeginOfStream should be false")
	}
	if !ctrl.EndOfStream {
		t.Error("EndOfStream should be true")
	}
	if ctrl.Timestamp != 1700000000000 {
		t.Errorf("Timestamp = %d, want %d", ctrl.Timestamp, 1700000000000)
	}
}

func TestStreamCtrl_JSON_Roundtrip(t *testing.T) {
	original := StreamCtrl{
		StreamID:      "test-id",
		Label:         "test-label",
		BeginOfStream: true,
		EndOfStream:   false,
		Timestamp:     1700000000000,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed StreamCtrl
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed != original {
		t.Errorf("roundtrip failed: got %+v, want %+v", parsed, original)
	}
}

func TestNewBeginOfStream(t *testing.T) {
	chunk := NewBeginOfStream("s1")
	if chunk.Ctrl == nil {
		t.Fatal("Ctrl should not be nil")
	}
	if chunk.Ctrl.StreamID != "s1" {
		t.Errorf("StreamID = %q, want %q", chunk.Ctrl.StreamID, "s1")
	}
	if !chunk.IsBeginOfStream() {
		t.Error("should be BOS")
	}
	if chunk.IsEndOfStream() {
		t.Error("should not be EOS")
	}
}

func TestNewEndOfStream(t *testing.T) {
	chunk := NewEndOfStream("audio/pcm")
	if !chunk.IsEndOfStream() {
		t.Error("should be EOS")
	}
	blob, ok := chunk.Part.(*Blob)
	if !ok {
		t.Fatal("Part should be *Blob")
	}
	if blob.MIMEType != "audio/pcm" {
		t.Errorf("MIMEType = %q, want %q", blob.MIMEType, "audio/pcm")
	}
	if len(blob.Data) != 0 {
		t.Error("Data should be empty")
	}
}

func TestNewTextEndOfStream(t *testing.T) {
	chunk := NewTextEndOfStream()
	if !chunk.IsEndOfStream() {
		t.Error("should be EOS")
	}
	text, ok := chunk.Part.(Text)
	if !ok {
		t.Fatal("Part should be Text")
	}
	if text != "" {
		t.Errorf("Text = %q, want empty", text)
	}
}

func TestIsBeginOfStream_NilCtrl(t *testing.T) {
	chunk := &MessageChunk{Role: RoleUser}
	if chunk.IsBeginOfStream() {
		t.Error("nil Ctrl should not be BOS")
	}
}

func TestIsEndOfStream_NilCtrl(t *testing.T) {
	chunk := &MessageChunk{Role: RoleUser}
	if chunk.IsEndOfStream() {
		t.Error("nil Ctrl should not be EOS")
	}
}
