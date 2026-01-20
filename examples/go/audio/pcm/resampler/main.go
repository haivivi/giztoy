// Package main demonstrates the SoXR audio resampler.
//
// This example shows how to:
//   - Create a resampler for sample rate conversion
//   - Convert audio between different sample rates
//   - Handle stereo/mono conversion
//
// Usage:
//
//	go run main.go
package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/haivivi/giztoy/pkg/audio/resampler"
)

func main() {
	fmt.Println("=== SoXR Resampler Example ===")
	fmt.Println()

	// Example 1: Upsample from 8kHz to 16kHz
	fmt.Println("Example 1: Upsample 8kHz → 16kHz")
	demonstrateResampling(8000, 16000, false)

	fmt.Println()

	// Example 2: Downsample from 48kHz to 16kHz
	fmt.Println("Example 2: Downsample 48kHz → 16kHz")
	demonstrateResampling(48000, 16000, false)

	fmt.Println()

	// Example 3: Stereo to Mono conversion with resampling
	fmt.Println("Example 3: Stereo 44.1kHz → Mono 16kHz")
	demonstrateResampling(44100, 16000, true)

	fmt.Println()
	fmt.Println("Done!")
}

func demonstrateResampling(srcRate, dstRate int, stereoToMono bool) {
	// Generate a 440Hz sine wave at source sample rate (0.1 seconds)
	duration := 0.1
	var srcData []byte
	var srcChannels int

	if stereoToMono {
		srcChannels = 2
		srcData = generateStereoSineWave(440, srcRate, duration)
	} else {
		srcChannels = 1
		srcData = generateSineWave(440, srcRate, duration)
	}

	srcFormat := resampler.Format{
		SampleRate: srcRate,
		Stereo:     stereoToMono,
	}
	dstFormat := resampler.Format{
		SampleRate: dstRate,
		Stereo:     false, // Always mono output in this example
	}

	fmt.Printf("  Source: %d Hz, %d channels, %d bytes\n", srcRate, srcChannels, len(srcData))
	fmt.Printf("  Target: %d Hz, 1 channel\n", dstRate)

	// Create resampler
	src := bytes.NewReader(srcData)
	rs, err := resampler.New(src, srcFormat, dstFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error creating resampler: %v\n", err)
		return
	}
	defer rs.Close()

	// Read all resampled data
	var output bytes.Buffer
	buf := make([]byte, 1024)
	for {
		n, err := rs.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error reading: %v\n", err)
			return
		}
	}

	// Calculate expected output size
	expectedSamples := int(float64(srcRate) * duration * float64(dstRate) / float64(srcRate))
	expectedBytes := expectedSamples * 2 // 16-bit mono

	fmt.Printf("  Output: %d bytes (expected ~%d bytes)\n", output.Len(), expectedBytes)
	fmt.Printf("  Ratio: %.2fx\n", float64(output.Len())/float64(len(srcData)))
}

// generateSineWave generates mono 16-bit PCM samples of a sine wave.
func generateSineWave(freq float64, sampleRate int, duration float64) []byte {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]byte, numSamples*2) // 16-bit = 2 bytes per sample

	amplitude := 16000.0
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		sample := int16(amplitude * math.Sin(2*math.Pi*freq*t))

		// Little-endian 16-bit
		samples[i*2] = byte(sample)
		samples[i*2+1] = byte(sample >> 8)
	}

	return samples
}

// generateStereoSineWave generates stereo 16-bit PCM samples.
// Left channel: original frequency, Right channel: frequency * 1.5 (perfect fifth)
func generateStereoSineWave(freq float64, sampleRate int, duration float64) []byte {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]byte, numSamples*4) // 16-bit stereo = 4 bytes per frame

	amplitude := 16000.0
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Left channel: base frequency
		left := int16(amplitude * math.Sin(2*math.Pi*freq*t))
		// Right channel: perfect fifth above (1.5x frequency)
		right := int16(amplitude * math.Sin(2*math.Pi*freq*1.5*t))

		// Little-endian 16-bit, interleaved L-R
		j := i * 4
		samples[j] = byte(left)
		samples[j+1] = byte(left >> 8)
		samples[j+2] = byte(right)
		samples[j+3] = byte(right >> 8)
	}

	return samples
}
