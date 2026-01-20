package opus

import (
	"testing"
	"time"
)

func TestTOCConfiguration(t *testing.T) {
	tests := []struct {
		toc    TOC
		config Configuration
		mode   ConfigurationMode
		bw     Bandwidth
		dur    FrameDuration
	}{
		// SILK NB 10ms
		{TOC(0 << 3), 0, Silk, NB, Duration10ms},
		// SILK NB 20ms
		{TOC(1 << 3), 1, Silk, NB, Duration20ms},
		// SILK WB 20ms
		{TOC(9 << 3), 9, Silk, WB, Duration20ms},
		// Hybrid SWB 20ms
		{TOC(13 << 3), 13, Hybrid, SWB, Duration20ms},
		// CELT FB 20ms
		{TOC(31 << 3), 31, CELT, FB, Duration20ms},
	}

	for _, tt := range tests {
		t.Run(tt.toc.String(), func(t *testing.T) {
			if got := tt.toc.Configuration(); got != tt.config {
				t.Errorf("Configuration() = %v, want %v", got, tt.config)
			}
			if got := tt.toc.Configuration().Mode(); got != tt.mode {
				t.Errorf("Mode() = %v, want %v", got, tt.mode)
			}
			if got := tt.toc.Configuration().Bandwidth(); got != tt.bw {
				t.Errorf("Bandwidth() = %v, want %v", got, tt.bw)
			}
			if got := tt.toc.Configuration().FrameDuration(); got != tt.dur {
				t.Errorf("FrameDuration() = %v, want %v", got, tt.dur)
			}
		})
	}
}

func TestTOCStereo(t *testing.T) {
	mono := TOC(0)
	stereo := TOC(0b00000100)

	if mono.IsStereo() {
		t.Error("mono TOC should not be stereo")
	}
	if !stereo.IsStereo() {
		t.Error("stereo TOC should be stereo")
	}
}

func TestTOCFrameCode(t *testing.T) {
	tests := []struct {
		toc      TOC
		expected FrameCode
	}{
		{TOC(0b00000000), OneFrame},
		{TOC(0b00000001), TwoEqualFrames},
		{TOC(0b00000010), TwoDifferentFrames},
		{TOC(0b00000011), ArbitraryFrames},
	}

	for _, tt := range tests {
		if got := tt.toc.FrameCode(); got != tt.expected {
			t.Errorf("FrameCode() = %v, want %v", got, tt.expected)
		}
	}
}

func TestFrameDuration(t *testing.T) {
	tests := []struct {
		fd       FrameDuration
		duration time.Duration
		millis   int64
	}{
		{Duration2500us, 2500 * time.Microsecond, 2},
		{Duration5ms, 5 * time.Millisecond, 5},
		{Duration10ms, 10 * time.Millisecond, 10},
		{Duration20ms, 20 * time.Millisecond, 20},
		{Duration40ms, 40 * time.Millisecond, 40},
		{Duration60ms, 60 * time.Millisecond, 60},
	}

	for _, tt := range tests {
		t.Run(tt.fd.String(), func(t *testing.T) {
			if got := tt.fd.Duration(); got != tt.duration {
				t.Errorf("Duration() = %v, want %v", got, tt.duration)
			}
			if got := tt.fd.Millis(); got != tt.millis {
				t.Errorf("Millis() = %v, want %v", got, tt.millis)
			}
		})
	}
}

func TestBandwidthSampleRate(t *testing.T) {
	tests := []struct {
		bw         Bandwidth
		sampleRate int
	}{
		{NB, 8000},
		{MB, 12000},
		{WB, 16000},
		{SWB, 24000},
		{FB, 48000},
	}

	for _, tt := range tests {
		t.Run(tt.bw.String(), func(t *testing.T) {
			if got := tt.bw.SampleRate(); got != tt.sampleRate {
				t.Errorf("SampleRate() = %v, want %v", got, tt.sampleRate)
			}
		})
	}
}

func TestParseFrameCountByte(t *testing.T) {
	// VBR, no padding, 10 frames
	isVBR, hasPadding, count := ParseFrameCountByte(0b10001010)
	if !isVBR {
		t.Error("expected VBR")
	}
	if hasPadding {
		t.Error("expected no padding")
	}
	if count != 10 {
		t.Errorf("frame count = %d, want 10", count)
	}

	// CBR, with padding, 5 frames
	isVBR, hasPadding, count = ParseFrameCountByte(0b01000101)
	if isVBR {
		t.Error("expected CBR")
	}
	if !hasPadding {
		t.Error("expected padding")
	}
	if count != 5 {
		t.Errorf("frame count = %d, want 5", count)
	}
}

func TestFrameDuration_Frame(t *testing.T) {
	// Create a simple frame with config 1 (SILK NB 20ms), mono, one frame
	frame := Frame([]byte{1 << 3, 0x00, 0x00}) // config=1, mono, OneFrame

	expected := 20 * time.Millisecond
	if got := frame.Duration(); got != expected {
		t.Errorf("Frame.Duration() = %v, want %v", got, expected)
	}
}
