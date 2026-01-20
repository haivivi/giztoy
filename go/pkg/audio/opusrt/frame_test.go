package opusrt

import (
	"testing"
	"time"
)

func TestEpochMillis(t *testing.T) {
	now := Now()
	if now <= 0 {
		t.Error("Now() should return positive value")
	}

	d := 100 * time.Millisecond
	ms := FromDuration(d)
	if ms != 100 {
		t.Errorf("FromDuration(%v) = %v, want 100", d, ms)
	}

	if ms.Duration() != d {
		t.Errorf("Duration() = %v, want %v", ms.Duration(), d)
	}
}

func TestStampedFrame(t *testing.T) {
	// Create a simple frame (just TOC byte for testing)
	frame := Frame{0x00} // Narrowband, 10ms, code 0
	stamp := EpochMillis(1234567890123)

	// Stamp the frame
	stamped := Stamp(frame, stamp)
	if len(stamped) != StampedHeaderSize+len(frame) {
		t.Errorf("Stamp() len = %d, want %d", len(stamped), StampedHeaderSize+len(frame))
	}

	// Parse it back
	parsedFrame, parsedStamp, ok := FromStamped(stamped)
	if !ok {
		t.Error("FromStamped() failed")
	}

	if parsedStamp != stamp {
		t.Errorf("Stamp = %v, want %v", parsedStamp, stamp)
	}

	if len(parsedFrame) != len(frame) {
		t.Errorf("Frame len = %d, want %d", len(parsedFrame), len(frame))
	}
}

func TestStampedFrameInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{0x01, 0x02, 0x03}},
		{"wrong version", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"empty frame", []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := FromStamped(tc.data)
			if ok {
				t.Error("FromStamped() should fail for invalid data")
			}
		})
	}
}

func TestStampTo(t *testing.T) {
	frame := Frame{0x00, 0x01, 0x02}
	stamp := EpochMillis(9999)
	dst := make([]byte, 100)

	result := StampTo(dst, frame, stamp)
	if len(result) != StampedHeaderSize+len(frame) {
		t.Errorf("StampTo() len = %d, want %d", len(result), StampedHeaderSize+len(frame))
	}

	parsedFrame, parsedStamp, ok := FromStamped(result)
	if !ok {
		t.Error("FromStamped() failed")
	}
	if parsedStamp != stamp {
		t.Errorf("Stamp = %v, want %v", parsedStamp, stamp)
	}
	if len(parsedFrame) != len(frame) {
		t.Errorf("Frame len = %d, want %d", len(parsedFrame), len(frame))
	}
}

func TestStampedFrameType(t *testing.T) {
	frame := Frame{0x00, 0x01, 0x02}
	stamp := EpochMillis(1000)
	stamped := Stamp(frame, stamp)

	sf := StampedFrame(stamped)
	if sf.Version() != FrameVersion {
		t.Errorf("Version() = %d, want %d", sf.Version(), FrameVersion)
	}
	if sf.Stamp() != stamp {
		t.Errorf("Stamp() = %v, want %v", sf.Stamp(), stamp)
	}
	if len(sf.Frame()) != len(frame) {
		t.Errorf("Frame() len = %d, want %d", len(sf.Frame()), len(frame))
	}
}

func TestFrameDuration(t *testing.T) {
	// Test TOC configurations that we know work
	// The TOC byte encodes: config (5 bits), stereo (1 bit), code (2 bits)
	// Duration depends on the config field
	tests := []struct {
		name  string
		frame Frame
	}{
		{"config 0 mono", Frame{0x00}},
		{"config 0 stereo", Frame{0x04}},
		{"config 8 mono", Frame{0x20}},
		{"config 24 mono", Frame{0x60}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dur := tc.frame.Duration()
			// Just verify it returns a valid duration (not zero for valid frames)
			if dur <= 0 {
				t.Errorf("Duration() = %v, expected positive duration", dur)
			}
			t.Logf("TOC 0x%02x -> %v", tc.frame[0], dur)
		})
	}
}

func TestFrameTOC(t *testing.T) {
	tests := []struct {
		frame Frame
		want  byte
	}{
		{Frame{0x00}, 0x00},
		{Frame{0xFF, 0x01, 0x02}, 0xFF},
		{Frame{0x7F, 0xAB}, 0x7F},
	}

	for _, tc := range tests {
		toc := tc.frame.TOC()
		if byte(toc) != tc.want {
			t.Errorf("TOC() = 0x%02x, want 0x%02x", toc, tc.want)
		}
	}
}

func TestFrameClone(t *testing.T) {
	original := Frame{0x00, 0x01, 0x02, 0x03}
	cloned := original.Clone()

	// Should be equal
	if len(cloned) != len(original) {
		t.Errorf("Clone length = %d, want %d", len(cloned), len(original))
	}

	for i := range original {
		if cloned[i] != original[i] {
			t.Errorf("Clone[%d] = %d, want %d", i, cloned[i], original[i])
		}
	}

	// Modifying clone should not affect original
	cloned[0] = 0xFF
	if original[0] == 0xFF {
		t.Error("Modifying clone affected original")
	}
}

func TestFrameIsStereo(t *testing.T) {
	// TOC byte bit 2 indicates stereo
	tests := []struct {
		toc    byte
		stereo bool
	}{
		{0x00, false}, // mono
		{0x04, true},  // stereo
		{0x08, false}, // mono
		{0x0C, true},  // stereo
	}

	for _, tc := range tests {
		frame := Frame{tc.toc}
		if frame.IsStereo() != tc.stereo {
			t.Errorf("IsStereo(0x%02x) = %v, want %v", tc.toc, frame.IsStereo(), tc.stereo)
		}
	}
}

func TestStampedFrameDuration(t *testing.T) {
	frame := Frame{0x00} // 10ms
	stamp := EpochMillis(1000)
	sf := StampedFrame(Stamp(frame, stamp))

	dur := sf.Duration()
	if dur != 10*time.Millisecond {
		t.Errorf("StampedFrame.Duration() = %v, want 10ms", dur)
	}
}

func TestTimestampOperations(t *testing.T) {
	stamp := EpochMillis(1000)

	// Add
	result := stamp.Add(100 * time.Millisecond)
	if result != 1100 {
		t.Errorf("Add() = %d, want 1100", result)
	}

	// Sub
	diff := EpochMillis(1100).Sub(stamp)
	if diff != 100*time.Millisecond {
		t.Errorf("Sub() = %v, want 100ms", diff)
	}

	// Time conversion
	tm := stamp.Time()
	back := FromTime(tm)
	if back != stamp {
		t.Errorf("Time() -> FromTime() = %d, want %d", back, stamp)
	}
}

func TestEmptyFrame(t *testing.T) {
	// Empty frame should handle gracefully
	frame := Frame{}
	dur := frame.Duration()
	if dur != 0 {
		t.Errorf("Empty frame Duration() = %v, want 0", dur)
	}

	toc := frame.TOC()
	if toc != 0 {
		t.Errorf("Empty frame TOC() = 0x%02x, want 0", toc)
	}
}
