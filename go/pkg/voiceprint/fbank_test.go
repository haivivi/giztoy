package voiceprint

import (
	"math"
	"testing"
)

func TestComputeFbankBasic(t *testing.T) {
	cfg := DefaultFbankConfig()

	// 50ms of 16kHz audio = 800 samples = 1600 bytes.
	// Should produce (800 - 400) / 160 + 1 = 3 frames.
	nSamples := 800
	audio := makeSineWavePCM(440, nSamples, cfg.SampleRate)

	result := ComputeFbank(audio, cfg)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	expectedFrames := (nSamples - cfg.FrameLength) / cfg.FrameShift + 1
	if len(result) != expectedFrames {
		t.Errorf("expected %d frames, got %d", expectedFrames, len(result))
	}

	for i, frame := range result {
		if len(frame) != cfg.NumMels {
			t.Errorf("frame %d: expected %d mels, got %d", i, cfg.NumMels, len(frame))
		}
		// All values should be finite.
		for j, v := range frame {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Errorf("frame %d mel %d: non-finite value %f", i, j, v)
			}
		}
	}
	t.Logf("produced %d frames × %d mels", len(result), cfg.NumMels)
}

func TestComputeFbankTooShort(t *testing.T) {
	cfg := DefaultFbankConfig()

	// Audio shorter than one frame.
	audio := make([]byte, cfg.FrameLength*2-2) // one sample short
	result := ComputeFbank(audio, cfg)
	if result != nil {
		t.Errorf("expected nil for too-short audio, got %d frames", len(result))
	}
}

func TestComputeFbankSineVsSilence(t *testing.T) {
	cfg := DefaultFbankConfig()
	nSamples := 1600 // 100ms

	// Sine wave at 440 Hz.
	sine := makeSineWavePCM(440, nSamples, cfg.SampleRate)
	sineFbank := ComputeFbank(sine, cfg)

	// Silence (all zeros). Silence will hit the energy floor.
	silence := make([]byte, nSamples*2)
	silenceFbank := ComputeFbank(silence, cfg)

	if sineFbank == nil || silenceFbank == nil {
		t.Fatal("expected non-nil results")
	}

	// Sine wave should have higher energy in at least some mel bins.
	var sineSum, silenceSum float64
	for _, frame := range sineFbank {
		for _, v := range frame {
			sineSum += float64(v)
		}
	}
	for _, frame := range silenceFbank {
		for _, v := range frame {
			silenceSum += float64(v)
		}
	}

	if sineSum <= silenceSum {
		t.Errorf("sine energy (%f) should be > silence energy (%f)", sineSum, silenceSum)
	}
	t.Logf("sine energy sum: %f, silence energy sum: %f", sineSum, silenceSum)
}

func TestComputeFbankDifferentFreqs(t *testing.T) {
	cfg := DefaultFbankConfig()
	nSamples := 3200 // 200ms

	// Low frequency (200 Hz) vs high frequency (4000 Hz).
	lowFreq := makeSineWavePCM(200, nSamples, cfg.SampleRate)
	highFreq := makeSineWavePCM(4000, nSamples, cfg.SampleRate)

	lowFbank := ComputeFbank(lowFreq, cfg)
	highFbank := ComputeFbank(highFreq, cfg)

	if lowFbank == nil || highFbank == nil {
		t.Fatal("expected non-nil results")
	}

	// Sum energy in low mel bins (0-20) vs high mel bins (60-80).
	var lowLow, lowHigh, highLow, highHigh float64
	for _, frame := range lowFbank {
		for i := 0; i < 20; i++ {
			lowLow += float64(frame[i])
		}
		for i := 60; i < 80; i++ {
			lowHigh += float64(frame[i])
		}
	}
	for _, frame := range highFbank {
		for i := 0; i < 20; i++ {
			highLow += float64(frame[i])
		}
		for i := 60; i < 80; i++ {
			highHigh += float64(frame[i])
		}
	}

	// 200 Hz should have more energy in low mel bins.
	// 4000 Hz should have more energy in high mel bins.
	t.Logf("200Hz: low_bins=%f high_bins=%f", lowLow, lowHigh)
	t.Logf("4kHz: low_bins=%f high_bins=%f", highLow, highHigh)

	if lowLow <= highLow {
		t.Error("200Hz should have more energy in low mel bins than 4kHz")
	}
}

func TestFFTPowerOfTwo(t *testing.T) {
	// Test with a simple known signal.
	n := 8
	x := make([]complex128, n)
	// Impulse at x[0] = 1.
	x[0] = 1
	fft(x)

	// FFT of impulse should be all 1s.
	for i, v := range x {
		if math.Abs(real(v)-1.0) > 1e-10 || math.Abs(imag(v)) > 1e-10 {
			t.Errorf("FFT[%d] = %v, expected 1+0i", i, v)
		}
	}
}

func TestNextPow2(t *testing.T) {
	tests := []struct{ in, want int }{
		{1, 1}, {2, 2}, {3, 4}, {4, 4}, {5, 8},
		{400, 512}, {512, 512}, {513, 1024},
	}
	for _, tt := range tests {
		got := nextPow2(tt.in)
		if got != tt.want {
			t.Errorf("nextPow2(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestMelConversion(t *testing.T) {
	// Round-trip: Hz → Mel → Hz.
	for _, hz := range []float64{0, 100, 1000, 4000, 8000} {
		mel := hzToMel(hz)
		back := melToHz(mel)
		if math.Abs(back-hz) > 0.01 {
			t.Errorf("round-trip failed: %f Hz → %f mel → %f Hz", hz, mel, back)
		}
	}
}

// makeSineWavePCM generates PCM16 audio of a sine wave.
func makeSineWavePCM(freqHz float64, nSamples, sampleRate int) []byte {
	audio := make([]byte, nSamples*2)
	for i := 0; i < nSamples; i++ {
		t := float64(i) / float64(sampleRate)
		sample := int16(16000 * math.Sin(2*math.Pi*freqHz*t))
		audio[2*i] = byte(sample & 0xFF)
		audio[2*i+1] = byte((sample >> 8) & 0xFF)
	}
	return audio
}

func BenchmarkComputeFbank(b *testing.B) {
	cfg := DefaultFbankConfig()
	// 400ms of audio = 6400 samples = 12800 bytes
	audio := makeSineWavePCM(440, 6400, cfg.SampleRate)
	b.ResetTimer()
	for range b.N {
		ComputeFbank(audio, cfg)
	}
}
