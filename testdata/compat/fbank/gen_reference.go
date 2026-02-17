// gen_reference generates reference fbank output from Go for cross-language
// validation. Produces a JSON file with the fbank features for a known PCM input.
//
// Usage: go run ./testdata/compat/fbank/gen_reference.go
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/haivivi/giztoy/go/pkg/voiceprint"
)

type FbankReference struct {
	SampleRate  int         `json:"sample_rate"`
	NumSamples  int         `json:"num_samples"`
	FreqHz      float64     `json:"freq_hz"`
	NumFrames   int         `json:"num_frames"`
	NumMels     int         `json:"num_mels"`
	Features    [][]float32 `json:"features"`
}

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

func main() {
	cfg := voiceprint.DefaultFbankConfig()
	nSamples := 6400 // 400ms @ 16kHz
	freqHz := 440.0

	audio := makeSineWavePCM(freqHz, nSamples, cfg.SampleRate)
	features := voiceprint.ComputeFbank(audio, cfg)

	if features == nil {
		fmt.Fprintln(os.Stderr, "ComputeFbank returned nil")
		os.Exit(1)
	}

	ref := FbankReference{
		SampleRate: cfg.SampleRate,
		NumSamples: nSamples,
		FreqHz:     freqHz,
		NumFrames:  len(features),
		NumMels:    cfg.NumMels,
		Features:   features,
	}

	f, err := os.Create("testdata/compat/fbank/reference.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(ref); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("fbank: %d frames × %d mels, freq=%.0fHz, samples=%d\n",
		len(features), cfg.NumMels, freqHz, nSamples)
	fmt.Printf("first frame first 5 mels: %v\n", features[0][:5])
	fmt.Println("wrote testdata/compat/fbank/reference.json")
}
