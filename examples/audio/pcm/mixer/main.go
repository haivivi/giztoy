// Package main demonstrates the PCM Mixer for mixing multiple audio tracks.
//
// This example shows how to:
//   - Create a Mixer with a specific output format
//   - Create multiple tracks with different formats
//   - Write audio data to tracks
//   - Read mixed audio output
//
// Usage:
//
//	go run main.go
package main

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
)

func main() {
	// Create a mixer with 16kHz mono output
	outputFormat := pcm.L16Mono16K
	mixer := pcm.NewMixer(outputFormat, pcm.WithAutoClose())
	defer mixer.Close()

	fmt.Printf("Created mixer with output format: %v\n", outputFormat)
	fmt.Printf("  Sample rate: %d Hz\n", outputFormat.SampleRate())
	fmt.Printf("  Channels: %d\n", outputFormat.Channels())
	fmt.Printf("  Bytes per second: %d\n", outputFormat.BytesRate())

	// Create two tracks
	track1, ctrl1, err := mixer.CreateTrack(pcm.WithTrackLabel("track1"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create track1 error: %v\n", err)
		return
	}
	track2, ctrl2, err := mixer.CreateTrack(pcm.WithTrackLabel("track2"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create track2 error: %v\n", err)
		return
	}
	_ = ctrl1 // TrackCtrl can be used to control gain, etc.
	_ = ctrl2

	fmt.Println("\nCreated 2 tracks")

	// Generate a 440Hz sine wave (A4 note) for track 1
	// Duration: 0.5 seconds
	samples1 := generateSineWave(440, 16000, 0.5)
	chunk1 := outputFormat.DataChunk(samples1)
	fmt.Printf("\nTrack 1: 440Hz sine wave, %d bytes (%.2f seconds)\n",
		len(samples1), float64(len(samples1))/float64(outputFormat.BytesRate()))

	// Generate a 880Hz sine wave (A5 note) for track 2
	// Duration: 0.5 seconds
	samples2 := generateSineWave(880, 16000, 0.5)
	chunk2 := outputFormat.DataChunk(samples2)
	fmt.Printf("Track 2: 880Hz sine wave, %d bytes (%.2f seconds)\n",
		len(samples2), float64(len(samples2))/float64(outputFormat.BytesRate()))

	// Write audio data to tracks in goroutines
	go func() {
		if err := track1.Write(chunk1); err != nil {
			fmt.Fprintf(os.Stderr, "track1 write error: %v\n", err)
		}
		ctrl1.Close()
	}()

	go func() {
		if err := track2.Write(chunk2); err != nil {
			fmt.Fprintf(os.Stderr, "track2 write error: %v\n", err)
		}
		ctrl2.Close()
	}()

	// Read mixed output
	fmt.Println("\nReading mixed output...")
	buf := make([]byte, 1024)
	totalBytes := 0
	for {
		n, err := mixer.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "mixer read error: %v\n", err)
			break
		}
		totalBytes += n
	}

	fmt.Printf("\nMixed output: %d bytes (%.2f seconds)\n",
		totalBytes, float64(totalBytes)/float64(outputFormat.BytesRate()))
	fmt.Println("\nDone! The two frequencies are now mixed together.")
}

// generateSineWave generates 16-bit PCM samples of a sine wave.
func generateSineWave(freq float64, sampleRate int, duration float64) []byte {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]byte, numSamples*2) // 16-bit = 2 bytes per sample

	amplitude := 16000.0 // About half of max int16 to leave headroom for mixing
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		sample := int16(amplitude * math.Sin(2*math.Pi*freq*t))

		// Little-endian 16-bit
		samples[i*2] = byte(sample)
		samples[i*2+1] = byte(sample >> 8)
	}

	return samples
}
