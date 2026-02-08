package voiceprint

import (
	"testing"
)

func TestDetectorInsufficientData(t *testing.T) {
	d := NewDetector()
	// First sample: not enough data.
	result := d.Feed("A3F8")
	if result != nil {
		t.Errorf("expected nil for first sample, got %+v", result)
	}
}

func TestDetectorSingleSpeaker(t *testing.T) {
	d := NewDetector(WithWindowSize(5), WithMinRatio(0.6))

	// Feed the same hash 5 times.
	var last *SpeakerChunk
	for range 5 {
		last = d.Feed("A3F8")
	}

	if last == nil {
		t.Fatal("expected non-nil result")
	}
	if last.Status != StatusSingle {
		t.Errorf("expected StatusSingle, got %s", last.Status)
	}
	if last.Speaker != "voice:A3F8" {
		t.Errorf("expected speaker voice:A3F8, got %s", last.Speaker)
	}
	if last.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", last.Confidence)
	}
	if len(last.Candidates) != 1 || last.Candidates[0] != "voice:A3F8" {
		t.Errorf("unexpected candidates: %v", last.Candidates)
	}
}

func TestDetectorOverlap(t *testing.T) {
	d := NewDetector(WithWindowSize(4), WithMinRatio(0.6))

	// Alternate between two speakers.
	d.Feed("AAAA")
	d.Feed("BBBB")
	d.Feed("AAAA")
	result := d.Feed("BBBB")

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != StatusOverlap {
		t.Errorf("expected StatusOverlap, got %s", result.Status)
	}
	if len(result.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d: %v", len(result.Candidates), result.Candidates)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0 (all slots covered by 2 hashes), got %f", result.Confidence)
	}
}

func TestDetectorUnknown(t *testing.T) {
	d := NewDetector(WithWindowSize(5), WithMinRatio(0.6))

	// Feed 5 different hashes — no dominant pattern.
	d.Feed("AAAA")
	d.Feed("BBBB")
	d.Feed("CCCC")
	d.Feed("DDDD")
	result := d.Feed("EEEE")

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != StatusUnknown {
		t.Errorf("expected StatusUnknown, got %s", result.Status)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected no candidates for Unknown, got %v", result.Candidates)
	}
}

func TestDetectorTransition(t *testing.T) {
	d := NewDetector(WithWindowSize(5), WithMinRatio(0.6))

	// Start with speaker A.
	for range 5 {
		d.Feed("AAAA")
	}
	result := d.Feed("AAAA")
	if result.Status != StatusSingle {
		t.Errorf("step 1: expected Single, got %s", result.Status)
	}

	// Transition to speaker B: window becomes mixed.
	d.Feed("BBBB")
	d.Feed("BBBB")
	result = d.Feed("BBBB")
	// Window: [AAAA, AAAA, BBBB, BBBB, BBBB] — B dominates 3/5 = 0.6.
	if result.Status != StatusSingle {
		t.Errorf("step 2: expected Single (B dominates), got %s", result.Status)
	}
	if result.Speaker != "voice:BBBB" {
		t.Errorf("step 2: expected speaker BBBB, got %s", result.Speaker)
	}

	// Continue with B until fully stable.
	d.Feed("BBBB")
	result = d.Feed("BBBB")
	if result.Status != StatusSingle {
		t.Errorf("step 3: expected Single, got %s", result.Status)
	}
	if result.Confidence != 1.0 {
		t.Errorf("step 3: expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestDetectorReset(t *testing.T) {
	d := NewDetector(WithWindowSize(3))

	d.Feed("AAAA")
	d.Feed("AAAA")
	d.Feed("AAAA")

	d.Reset()

	// After reset, first feed should return nil (insufficient data).
	result := d.Feed("BBBB")
	if result != nil {
		t.Errorf("expected nil after reset, got %+v", result)
	}
}

func TestDetectorCustomOptions(t *testing.T) {
	d := NewDetector(WithWindowSize(3), WithMinRatio(0.8))

	// With window=3 and minRatio=0.8, need 3/3 = 1.0 or at least 0.8.
	// 2/3 ≈ 0.67 < 0.8, so not Single yet.
	d.Feed("AAAA")
	d.Feed("AAAA")
	result := d.Feed("BBBB")

	if result == nil {
		t.Fatal("expected non-nil")
	}
	// 2/3 for AAAA < 0.8, combined 3/3 = 1.0 ≥ 0.8 → Overlap.
	if result.Status != StatusOverlap {
		t.Errorf("expected Overlap (2/3 < 0.8), got %s", result.Status)
	}

	// Now make it unanimous — need 3 feeds to overwrite all 3 slots.
	d.Feed("AAAA")
	d.Feed("AAAA")
	result = d.Feed("AAAA") // window: [AAAA, AAAA, AAAA]
	if result.Status != StatusSingle {
		t.Errorf("expected Single (3/3), got %s", result.Status)
	}
}

func TestSpeakerStatusString(t *testing.T) {
	tests := []struct {
		status SpeakerStatus
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusSingle, "single"},
		{StatusOverlap, "overlap"},
		{SpeakerStatus(99), "SpeakerStatus(99)"},
	}
	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("SpeakerStatus(%d).String() = %q, want %q", int(tt.status), got, tt.want)
		}
	}
}

func TestVoiceLabel(t *testing.T) {
	if got := VoiceLabel("A3F8"); got != "voice:A3F8" {
		t.Errorf("VoiceLabel(A3F8) = %q, want voice:A3F8", got)
	}
}
