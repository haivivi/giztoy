package opus

import (
	"testing"
	"time"

	codecopus "github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

func TestEpochMillis(t *testing.T) {
	// Test Now - compare milliseconds directly
	beforeMs := time.Now().UnixMilli()
	ms := Now()
	afterMs := time.Now().UnixMilli()

	if int64(ms) < beforeMs || int64(ms) > afterMs {
		t.Errorf("Now() returned incorrect time: got %d, expected between %d and %d", ms, beforeMs, afterMs)
	}

	// Test FromTime
	testTime := time.Date(2024, 1, 15, 12, 30, 45, 500*1e6, time.UTC)
	ms = FromTime(testTime)
	if ms.Time().Unix() != testTime.Unix() {
		t.Errorf("FromTime: expected %v, got %v", testTime.Unix(), ms.Time().Unix())
	}

	// Test Duration conversion
	ms = EpochMillis(1500)
	d := ms.Duration()
	if d != 1500*time.Millisecond {
		t.Errorf("Duration: expected 1500ms, got %v", d)
	}

	// Test Add
	ms = EpochMillis(1000)
	ms2 := ms.Add(500 * time.Millisecond)
	if ms2 != 1500 {
		t.Errorf("Add: expected 1500, got %d", ms2)
	}

	// Test Sub
	ms1 := EpochMillis(2000)
	ms2 = EpochMillis(1500)
	diff := ms1.Sub(ms2)
	if diff != 500*time.Millisecond {
		t.Errorf("Sub: expected 500ms, got %v", diff)
	}
}

func TestStampedFrame(t *testing.T) {
	// Test Timestamp
	sf := StampedFrame{
		Frame: OpusFrame{0x01, 0x02, 0x03},
		Stamp: 12345,
	}

	if sf.Timestamp() != 12345 {
		t.Errorf("Timestamp: expected 12345, got %d", sf.Timestamp())
	}
}

func TestParseStamped(t *testing.T) {
	// Create a stamped frame
	frame := OpusFrame{0xf8, 0xff, 0xfe} // silence frame
	stamp := EpochMillis(1234567890123)
	stamped := MakeStamped(frame, stamp)

	// Parse it back
	parsedFrame, parsedStamp, ok := ParseStamped(stamped)
	if !ok {
		t.Fatal("ParseStamped failed")
	}

	if parsedStamp != stamp {
		t.Errorf("ParseStamped stamp: expected %d, got %d", stamp, parsedStamp)
	}

	if len(parsedFrame) != len(frame) {
		t.Errorf("ParseStamped frame length: expected %d, got %d", len(frame), len(parsedFrame))
	}

	for i := range frame {
		if parsedFrame[i] != frame[i] {
			t.Errorf("ParseStamped frame[%d]: expected %d, got %d", i, frame[i], parsedFrame[i])
		}
	}
}

func TestParseStamped_Invalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{0x01, 0x02}},
		{"wrong version", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf8}},
		{"no frame data", []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, ok := ParseStamped(tt.data)
			if ok {
				t.Error("ParseStamped should have failed")
			}
		})
	}
}

func TestOpusFrame_Clone(t *testing.T) {
	original := OpusFrame{0x01, 0x02, 0x03}
	cloned := original.Clone()

	// Modify original
	original[0] = 0xff

	// Clone should be unchanged
	if cloned[0] != 0x01 {
		t.Error("Clone was modified when original was changed")
	}
}

func TestOpusFrame_TOC(t *testing.T) {
	tests := []struct {
		name     string
		frame    OpusFrame
		expected codecopus.TOC
	}{
		{"empty frame", OpusFrame{}, 0},
		{"silence frame", OpusSilence20ms, codecopus.TOC(0xf8)},
		{"custom TOC", OpusFrame{0x48, 0x01, 0x02}, codecopus.TOC(0x48)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.frame.TOC()
			if got != tt.expected {
				t.Errorf("TOC() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOpusFrame_Duration(t *testing.T) {
	tests := []struct {
		name     string
		frame    OpusFrame
		expected time.Duration
	}{
		// Empty frame
		{"empty frame", OpusFrame{}, 0},

		// Silence frame: TOC=0xf8 = config 31 (CELT FB), one frame, 20ms
		// config 31 = 11111 -> FrameDuration 20ms
		{"silence 20ms", OpusSilence20ms, 20 * time.Millisecond},

		// Config 0 (SILK NB 10ms), one frame
		// TOC = 0b00000000 = 0x00
		{"SILK NB 10ms one frame", OpusFrame{0x00, 0x01}, 10 * time.Millisecond},

		// Config 1 (SILK NB 20ms), one frame
		// TOC = 0b00001000 = 0x08
		{"SILK NB 20ms one frame", OpusFrame{0x08, 0x01}, 20 * time.Millisecond},

		// Config 31 (CELT FB 20ms), two equal frames (code 1)
		// TOC = 0b11111001 = 0xf9
		{"CELT FB 20ms two equal frames", OpusFrame{0xf9, 0x01, 0x02}, 40 * time.Millisecond},

		// Config 31 (CELT FB 20ms), two different frames (code 2)
		// TOC = 0b11111010 = 0xfa
		{"CELT FB 20ms two different frames", OpusFrame{0xfa, 0x01, 0x02}, 40 * time.Millisecond},

		// Config 31 (CELT FB 20ms), arbitrary frames (code 3), count=3
		// TOC = 0b11111011 = 0xfb, second byte = 0x03 (count=3)
		{"CELT FB 20ms arbitrary 3 frames", OpusFrame{0xfb, 0x03, 0x01, 0x02, 0x03}, 60 * time.Millisecond},

		// Arbitrary frames but frame too short (only 1 byte)
		{"arbitrary frames too short", OpusFrame{0xfb}, 0},

		// Config 16 (CELT NB 2.5ms), one frame
		// TOC = 0b10000000 = 0x80
		{"CELT NB 2.5ms", OpusFrame{0x80, 0x01}, 2500 * time.Microsecond},

		// Config 17 (CELT NB 5ms), one frame
		// TOC = 0b10001000 = 0x88
		{"CELT NB 5ms", OpusFrame{0x88, 0x01}, 5 * time.Millisecond},

		// Config 2 (SILK NB 40ms), one frame
		// TOC = 0b00010000 = 0x10
		{"SILK NB 40ms", OpusFrame{0x10, 0x01}, 40 * time.Millisecond},

		// Config 3 (SILK NB 60ms), one frame
		// TOC = 0b00011000 = 0x18
		{"SILK NB 60ms", OpusFrame{0x18, 0x01}, 60 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.frame.Duration()
			if got != tt.expected {
				t.Errorf("Duration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOpusFrame_IsStereo(t *testing.T) {
	tests := []struct {
		name     string
		frame    OpusFrame
		expected bool
	}{
		{"empty frame", OpusFrame{}, false},
		// Mono: stereo bit (bit 2) = 0
		// TOC = 0xf8 = 11111000, bit 2 = 0 -> mono
		{"mono frame", OpusFrame{0xf8, 0x01}, false},
		// Stereo: stereo bit (bit 2) = 1
		// TOC = 0xfc = 11111100, bit 2 = 1 -> stereo
		{"stereo frame", OpusFrame{0xfc, 0x01}, true},
		// Another stereo check
		// TOC = 0x04 = 00000100, bit 2 = 1 -> stereo
		{"stereo frame 2", OpusFrame{0x04, 0x01}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.frame.IsStereo()
			if got != tt.expected {
				t.Errorf("IsStereo() = %v, want %v", got, tt.expected)
			}
		})
	}
}
