package mp3

import (
	"bytes"
	"math"
	"testing"
)

func TestEncoderDecoder(t *testing.T) {
	sampleRate := 44100
	channels := 2
	duration := 1.0 // 1 second

	// Generate a 440Hz sine wave
	numSamples := int(float64(sampleRate) * duration)
	pcm := make([]byte, numSamples*channels*2) // 16-bit stereo

	for i := 0; i < numSamples; i++ {
		ti := float64(i) / float64(sampleRate)
		sample := int16(math.Sin(2*math.Pi*440*ti) * 16000)

		// Stereo: same sample for both channels
		offset := i * channels * 2
		pcm[offset] = byte(sample)
		pcm[offset+1] = byte(sample >> 8)
		pcm[offset+2] = byte(sample)
		pcm[offset+3] = byte(sample >> 8)
	}

	// Encode to MP3
	var mp3Buf bytes.Buffer
	enc, err := NewEncoder(&mp3Buf, sampleRate, channels, WithQuality(QualityMedium))
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	_, err = enc.Write(pcm)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	err = enc.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	err = enc.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	t.Logf("Encoded %d bytes PCM to %d bytes MP3 (%.1f%% compression)",
		len(pcm), mp3Buf.Len(), float64(mp3Buf.Len())/float64(len(pcm))*100)

	// Decode back to PCM
	dec := NewDecoder(&mp3Buf)
	defer dec.Close()

	var decodedPCM bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, err := dec.Read(buf)
		if n > 0 {
			decodedPCM.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	t.Logf("Decoded to %d bytes PCM", decodedPCM.Len())
	t.Logf("Sample rate: %d Hz, Channels: %d, Bitrate: %d kbps",
		dec.SampleRate(), dec.Channels(), dec.Bitrate())

	// Verify we got reasonable output
	if decodedPCM.Len() == 0 {
		t.Error("Decoded PCM is empty")
	}

	// MP3 is lossy, so we can't compare exact values
	// Just verify the size is reasonable (should be similar to original)
	ratio := float64(decodedPCM.Len()) / float64(len(pcm))
	if ratio < 0.8 || ratio > 1.2 {
		t.Errorf("Decoded PCM size ratio %f is unexpected (should be close to 1.0)", ratio)
	}
}

func TestEncoderMono(t *testing.T) {
	sampleRate := 16000
	channels := 1
	duration := 0.5 // 0.5 seconds

	numSamples := int(float64(sampleRate) * duration)
	pcm := make([]byte, numSamples*channels*2)

	for i := 0; i < numSamples; i++ {
		ti := float64(i) / float64(sampleRate)
		sample := int16(math.Sin(2*math.Pi*440*ti) * 16000)
		offset := i * 2
		pcm[offset] = byte(sample)
		pcm[offset+1] = byte(sample >> 8)
	}

	var mp3Buf bytes.Buffer
	written, err := EncodePCMStream(&mp3Buf, bytes.NewReader(pcm), sampleRate, channels, WithBitrate(64))
	if err != nil {
		t.Fatalf("EncodePCMStream failed: %v", err)
	}

	t.Logf("Encoded %d bytes mono PCM (wrote %d), got %d bytes MP3", len(pcm), written, mp3Buf.Len())

	if mp3Buf.Len() == 0 {
		t.Error("MP3 output is empty")
	}
}

func TestDecodeFull(t *testing.T) {
	// First, create a small MP3 file
	sampleRate := 44100
	channels := 2
	numSamples := sampleRate / 4 // 0.25 seconds

	pcm := make([]byte, numSamples*channels*2)
	for i := 0; i < numSamples; i++ {
		ti := float64(i) / float64(sampleRate)
		sample := int16(math.Sin(2*math.Pi*880*ti) * 16000)
		offset := i * channels * 2
		pcm[offset] = byte(sample)
		pcm[offset+1] = byte(sample >> 8)
		pcm[offset+2] = byte(sample)
		pcm[offset+3] = byte(sample >> 8)
	}

	var mp3Buf bytes.Buffer
	_, err := EncodePCMStream(&mp3Buf, bytes.NewReader(pcm), sampleRate, channels)
	if err != nil {
		t.Fatalf("EncodePCMStream failed: %v", err)
	}

	// Decode using DecodeFull
	decodedPCM, sr, ch, err := DecodeFull(bytes.NewReader(mp3Buf.Bytes()))
	if err != nil {
		t.Fatalf("DecodeFull failed: %v", err)
	}

	t.Logf("DecodeFull: %d bytes PCM, %d Hz, %d ch", len(decodedPCM), sr, ch)

	if sr != sampleRate {
		t.Errorf("Sample rate mismatch: got %d, want %d", sr, sampleRate)
	}
	if ch != channels {
		t.Errorf("Channels mismatch: got %d, want %d", ch, channels)
	}
}

func TestEncoderQualityPresets(t *testing.T) {
	sampleRate := 44100
	channels := 2
	numSamples := sampleRate / 10 // 0.1 seconds

	pcm := make([]byte, numSamples*channels*2)
	for i := 0; i < numSamples; i++ {
		ti := float64(i) / float64(sampleRate)
		sample := int16(math.Sin(2*math.Pi*440*ti) * 16000)
		offset := i * channels * 2
		pcm[offset] = byte(sample)
		pcm[offset+1] = byte(sample >> 8)
		pcm[offset+2] = byte(sample)
		pcm[offset+3] = byte(sample >> 8)
	}

	qualities := []struct {
		name    string
		quality Quality
	}{
		{"Best", QualityBest},
		{"High", QualityHigh},
		{"Medium", QualityMedium},
		{"Low", QualityLow},
		{"Worst", QualityWorst},
	}

	for _, q := range qualities {
		var mp3Buf bytes.Buffer
		_, err := EncodePCMStream(&mp3Buf, bytes.NewReader(pcm), sampleRate, channels, WithQuality(q.quality))
		if err != nil {
			t.Errorf("Quality %s: EncodePCMStream failed: %v", q.name, err)
			continue
		}
		t.Logf("Quality %s: %d bytes", q.name, mp3Buf.Len())
	}
}
