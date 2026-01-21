package speech

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/audio/pcm"

	"google.golang.org/api/iterator"
)

// mockVoiceSegment implements VoiceSegment for testing.
type mockVoiceSegment struct {
	data   []byte
	format pcm.Format
	pos    int
}

func (m *mockVoiceSegment) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockVoiceSegment) Format() pcm.Format {
	return m.format
}

func (m *mockVoiceSegment) Close() error {
	return nil
}

// mockSpeechSegment implements SpeechSegment for testing.
type mockSpeechSegment struct {
	audio      []byte
	transcript string
	format     pcm.Format
}

func (m *mockSpeechSegment) Decode(best pcm.Format) VoiceSegment {
	return &mockVoiceSegment{
		data:   m.audio,
		format: m.format,
	}
}

func (m *mockSpeechSegment) Transcribe() io.ReadCloser {
	return io.NopCloser(strings.NewReader(m.transcript))
}

func (m *mockSpeechSegment) Close() error {
	return nil
}

// mockSpeech implements Speech for testing.
type mockSpeech struct {
	segments []SpeechSegment
	pos      int
}

func (m *mockSpeech) Next() (SpeechSegment, error) {
	if m.pos >= len(m.segments) {
		return nil, iterator.Done
	}
	seg := m.segments[m.pos]
	m.pos++
	return seg, nil
}

func (m *mockSpeech) Close() error {
	return nil
}

// mockSpeechStream implements SpeechStream for testing.
type mockSpeechStream struct {
	speeches []Speech
	pos      int
}

func (m *mockSpeechStream) Next() (Speech, error) {
	if m.pos >= len(m.speeches) {
		return nil, iterator.Done
	}
	sp := m.speeches[m.pos]
	m.pos++
	return sp, nil
}

func (m *mockSpeechStream) Close() error {
	return nil
}

// mockPCMWriter implements pcm.Writer for testing.
type mockPCMWriter struct {
	chunks []pcm.Chunk
}

func (m *mockPCMWriter) Write(chunk pcm.Chunk) error {
	m.chunks = append(m.chunks, chunk)
	return nil
}

func (m *mockPCMWriter) TotalBytes() int64 {
	var total int64
	for _, c := range m.chunks {
		total += c.Len()
	}
	return total
}

func TestCollectSpeech(t *testing.T) {
	format := pcm.L16Mono16K

	// Create a speech stream with 2 speeches, each with 2 segments
	stream := &mockSpeechStream{
		speeches: []Speech{
			&mockSpeech{
				segments: []SpeechSegment{
					&mockSpeechSegment{audio: []byte("audio1"), transcript: "hello", format: format},
					&mockSpeechSegment{audio: []byte("audio2"), transcript: "world", format: format},
				},
			},
			&mockSpeech{
				segments: []SpeechSegment{
					&mockSpeechSegment{audio: []byte("audio3"), transcript: "foo", format: format},
				},
			},
		},
	}

	collected := CollectSpeech(stream)
	defer collected.Close()

	// Should get all 3 segments
	segCount := 0
	for seg, err := range Iter(collected) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		seg.Close()
		segCount++
	}

	if segCount != 3 {
		t.Errorf("CollectSpeech got %d segments; want 3", segCount)
	}
}

func TestCopySpeech(t *testing.T) {
	format := pcm.L16Mono16K

	speech := &mockSpeech{
		segments: []SpeechSegment{
			&mockSpeechSegment{
				audio:      make([]byte, int(format.BytesRate())), // 1 second of audio
				transcript: "hello",
				format:     format,
			},
			&mockSpeechSegment{
				audio:      make([]byte, int(format.BytesRate()/2)), // 0.5 seconds of audio
				transcript: "world",
				format:     format,
			},
		},
	}

	pw := &mockPCMWriter{}
	var tw bytes.Buffer

	duration, err := CopySpeech(pw, &tw, speech)
	if err != nil {
		t.Fatalf("CopySpeech error: %v", err)
	}

	// Check duration (should be ~1.5 seconds)
	expectedDuration := time.Duration(1500) * time.Millisecond
	if duration < expectedDuration-100*time.Millisecond || duration > expectedDuration+100*time.Millisecond {
		t.Errorf("CopySpeech duration = %v; want ~%v", duration, expectedDuration)
	}

	// Check transcript
	transcript := tw.String()
	if !strings.Contains(transcript, "hello") || !strings.Contains(transcript, "world") {
		t.Errorf("CopySpeech transcript = %q; want to contain 'hello' and 'world'", transcript)
	}
}

func TestCopySpeech_NilWriters(t *testing.T) {
	format := pcm.L16Mono16K

	speech := &mockSpeech{
		segments: []SpeechSegment{
			&mockSpeechSegment{
				audio:      make([]byte, 1000),
				transcript: "test",
				format:     format,
			},
		},
	}

	// Should not panic with nil writers
	_, err := CopySpeech(nil, nil, speech)
	if err != nil {
		t.Fatalf("CopySpeech with nil writers error: %v", err)
	}
}

func TestIter(t *testing.T) {
	speech := &mockSpeech{
		segments: []SpeechSegment{
			&mockSpeechSegment{transcript: "one"},
			&mockSpeechSegment{transcript: "two"},
			&mockSpeechSegment{transcript: "three"},
		},
	}

	var transcripts []string
	for seg, err := range Iter(speech) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		r := seg.Transcribe()
		data, _ := io.ReadAll(r)
		r.Close()
		transcripts = append(transcripts, string(data))
		seg.Close()
	}

	if len(transcripts) != 3 {
		t.Errorf("Iter got %d items; want 3", len(transcripts))
	}
	expected := []string{"one", "two", "three"}
	for i, tr := range transcripts {
		if tr != expected[i] {
			t.Errorf("transcript[%d] = %q; want %q", i, tr, expected[i])
		}
	}
}

func TestIter_Empty(t *testing.T) {
	speech := &mockSpeech{segments: nil}

	count := 0
	for _, err := range Iter(speech) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		count++
	}

	if count != 0 {
		t.Errorf("Iter on empty speech got %d items; want 0", count)
	}
}

func TestVoiceSegment_Read(t *testing.T) {
	data := []byte("hello world audio data")
	seg := &mockVoiceSegment{
		data:   data,
		format: pcm.L16Mono16K,
	}

	buf := make([]byte, 5)
	var result []byte

	for {
		n, err := seg.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		result = append(result, buf[:n]...)
	}

	if string(result) != string(data) {
		t.Errorf("Read result = %q; want %q", result, data)
	}
}

func TestASRMux(t *testing.T) {
	mux := NewASRMux()

	// Register a mock transcriber
	called := false
	err := mux.HandleFunc("test/model", func(ctx context.Context, model string, opus opusrt.FrameReader) (SpeechStream, error) {
		called = true
		return &mockSpeechStream{}, nil
	})
	if err != nil {
		t.Fatalf("HandleFunc error: %v", err)
	}

	// Test TranscribeStream
	_, err = mux.TranscribeStream(context.Background(), "test/model", nil)
	if err != nil {
		t.Fatalf("TranscribeStream error: %v", err)
	}
	if !called {
		t.Error("TranscribeStream did not call handler")
	}

	// Test not found
	_, err = mux.TranscribeStream(context.Background(), "unknown", nil)
	if err == nil {
		t.Error("TranscribeStream should return error for unknown model")
	}
}

func TestTTSMux(t *testing.T) {
	mux := NewTTSMux()

	// Register a mock synthesizer
	called := false
	err := mux.HandleFunc("test/voice", func(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
		called = true
		return &mockSpeech{}, nil
	})
	if err != nil {
		t.Fatalf("HandleFunc error: %v", err)
	}

	// Test Synthesize
	_, err = mux.Synthesize(context.Background(), "test/voice", strings.NewReader("hello"), pcm.L16Mono16K)
	if err != nil {
		t.Fatalf("Synthesize error: %v", err)
	}
	if !called {
		t.Error("Synthesize did not call handler")
	}

	// Test not found
	_, err = mux.Synthesize(context.Background(), "unknown", strings.NewReader("hello"), pcm.L16Mono16K)
	if err == nil {
		t.Error("Synthesize should return error for unknown voice")
	}
}
