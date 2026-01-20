package opus

import (
	"math"
	"testing"
)

func TestEncoderDecoder(t *testing.T) {
	sampleRate := 48000
	channels := 1
	frameSize := sampleRate * 20 / 1000 // 20ms frame

	// Create encoder
	enc, err := NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	// Create decoder
	dec, err := NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}

	// Generate a 440Hz sine wave (20ms)
	pcm := make([]int16, frameSize*channels)
	for i := range pcm {
		t := float64(i) / float64(sampleRate)
		pcm[i] = int16(math.Sin(2*math.Pi*440*t) * 16000)
	}

	// Encode
	frame, err := enc.Encode(pcm, frameSize)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	t.Logf("Encoded %d samples to %d bytes (%.2f%% compression)",
		frameSize, len(frame), float64(len(frame))/float64(frameSize*2)*100)

	// Verify frame TOC
	if len(frame) == 0 {
		t.Fatal("empty frame")
	}
	toc := frame.TOC()
	t.Logf("Frame TOC: %s", toc)

	// Decode
	decoded, err := dec.Decode(frame)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	decodedSamples := len(decoded) / 2 / channels
	if decodedSamples != frameSize {
		t.Errorf("decoded %d samples, want %d", decodedSamples, frameSize)
	}

	t.Logf("Decoded %d bytes back to %d samples", len(frame), decodedSamples)
}

func TestEncoderDecoderStereo(t *testing.T) {
	sampleRate := 48000
	channels := 2
	frameSize := sampleRate * 20 / 1000 // 20ms frame

	// Create encoder
	enc, err := NewAudioEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	// Create decoder
	dec, err := NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}

	// Generate stereo signal (440Hz left, 880Hz right)
	pcm := make([]int16, frameSize*channels)
	for i := 0; i < frameSize; i++ {
		ti := float64(i) / float64(sampleRate)
		pcm[i*2] = int16(math.Sin(2*math.Pi*440*ti) * 16000)   // Left
		pcm[i*2+1] = int16(math.Sin(2*math.Pi*880*ti) * 16000) // Right
	}

	// Encode
	frame, err := enc.Encode(pcm, frameSize)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if !frame.IsStereo() {
		t.Error("expected stereo frame")
	}

	t.Logf("Encoded %d stereo samples to %d bytes", frameSize, len(frame))

	// Decode
	decoded, err := dec.Decode(frame)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	decodedSamples := len(decoded) / 2 / channels
	if decodedSamples != frameSize {
		t.Errorf("decoded %d samples, want %d", decodedSamples, frameSize)
	}
}

func TestDecoderPLC(t *testing.T) {
	sampleRate := 48000
	channels := 1
	frameSize := sampleRate * 20 / 1000 // 20ms frame

	// Create encoder
	enc, err := NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	// Create decoder
	dec, err := NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}

	// Generate a 440Hz sine wave
	pcm := make([]int16, frameSize*channels)
	for i := range pcm {
		ti := float64(i) / float64(sampleRate)
		pcm[i] = int16(math.Sin(2*math.Pi*440*ti) * 16000)
	}

	// Encode and decode first frame normally
	frame, err := enc.Encode(pcm, frameSize)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	_, err = dec.Decode(frame)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Now simulate packet loss with PLC
	plcData, err := dec.DecodePLC(frameSize)
	if err != nil {
		t.Fatalf("PLC decode failed: %v", err)
	}

	plcSamples := len(plcData) / 2 / channels
	if plcSamples != frameSize {
		t.Errorf("PLC generated %d samples, want %d", plcSamples, frameSize)
	}

	t.Logf("PLC generated %d bytes for %d samples", len(plcData), plcSamples)
}

func TestFrameDurationCalculation(t *testing.T) {
	sampleRate := 48000
	channels := 1

	enc, err := NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	// Test different frame durations
	frameSizes := []int{
		sampleRate * 10 / 1000,  // 10ms
		sampleRate * 20 / 1000,  // 20ms
		sampleRate * 40 / 1000,  // 40ms
		sampleRate * 60 / 1000,  // 60ms
	}

	for _, frameSize := range frameSizes {
		pcm := make([]int16, frameSize*channels)
		for i := range pcm {
			ti := float64(i) / float64(sampleRate)
			pcm[i] = int16(math.Sin(2*math.Pi*440*ti) * 16000)
		}

		frame, err := enc.Encode(pcm, frameSize)
		if err != nil {
			t.Errorf("encode failed for frameSize=%d: %v", frameSize, err)
			continue
		}

		expectedDuration := float64(frameSize) / float64(sampleRate) * 1000
		actualDuration := frame.Duration().Seconds() * 1000

		t.Logf("frameSize=%d: expected=%.1fms, actual=%.1fms, bytes=%d",
			frameSize, expectedDuration, actualDuration, len(frame))
	}
}
