package main

import (
	"math"
	"math/cmplx"

	"github.com/haivivi/giztoy/go/pkg/audio/fbank"
)

// spectralDenoise applies spectral subtraction noise reduction to 16kHz mono
// int16 PCM bytes.
//
// Algorithm:
//  1. Estimate noise spectrum from the quietest 20% of frames
//  2. For each frame, subtract the estimated noise power from the signal power
//  3. Apply a soft floor to avoid musical noise artifacts
//  4. Reconstruct the time-domain signal via overlap-add
//
// This is a simple, stateless approach that preserves speaker characteristics
// while reducing stationary background noise.
func spectralDenoise(pcm []byte) []byte {
	const (
		fftSize       = 512
		hopSize       = 128
		halfFFT       = fftSize/2 + 1
		overSubtract  = 2.0  // oversubtraction factor (aggressiveness)
		spectralFloor = 0.02 // minimum gain to avoid musical noise
	)

	numSamples := len(pcm) / 2
	if numSamples < fftSize+hopSize*3 {
		return pcm // too short
	}

	// Convert to float32
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		samples[i] = float32(s) / 32768.0
	}

	// Hann window
	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	numTotalFrames := (numSamples - fftSize) / hopSize + 1

	// === Phase 1: Estimate noise spectrum from quietest frames ===
	type frameSpec struct {
		energy float64
		power  []float64
	}
	specs := make([]frameSpec, numTotalFrames)
	for t := 0; t < numTotalFrames; t++ {
		start := t * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fbank.FFT(re, im)
		pwr := make([]float64, halfFFT)
		energy := 0.0
		for i := 0; i < halfFFT; i++ {
			p := re[i]*re[i] + im[i]*im[i]
			pwr[i] = p
			energy += p
		}
		specs[t] = frameSpec{energy: energy, power: pwr}
	}

	// Sort by energy, take bottom 20% as noise
	energies := make([]float64, numTotalFrames)
	for i, s := range specs {
		energies[i] = s.energy
	}
	threshold := percentile(energies, 20)

	noisePower := make([]float64, halfFFT)
	noiseCount := 0
	for _, s := range specs {
		if s.energy <= threshold {
			for i, p := range s.power {
				noisePower[i] += p
			}
			noiseCount++
		}
	}
	if noiseCount == 0 {
		return pcm // no noise frames found
	}
	for i := range noisePower {
		noisePower[i] /= float64(noiseCount)
	}

	// === Phase 2: Spectral subtraction ===
	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)

	for t := 0; t < numTotalFrames; t++ {
		start := t * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fbank.FFT(re, im)

		// Apply spectral subtraction with soft floor
		for i := 0; i < halfFFT; i++ {
			c := complex(re[i], im[i])
			power := re[i]*re[i] + im[i]*im[i]
			cleanPower := power - overSubtract*noisePower[i]

			gain := spectralFloor
			if power > 1e-10 {
				g := math.Sqrt(math.Max(cleanPower, 0) / power)
				if g > spectralFloor {
					gain = g
				}
			}

			angle := cmplx.Phase(c)
			mag := cmplx.Abs(c) * gain
			cleaned := cmplx.Rect(mag, angle)
			re[i] = real(cleaned)
			im[i] = imag(cleaned)

			if i > 0 && i < halfFFT-1 {
				re[fftSize-i] = re[i]
				im[fftSize-i] = -im[i]
			}
		}

		fbank.IFFT(re, im)

		for i := 0; i < fftSize; i++ {
			idx := start + i
			if idx < numSamples {
				output[idx] += re[i] * hann[i]
				winSum[idx] += hann[i] * hann[i]
			}
		}
	}

	for i := range output {
		if winSum[i] > 1e-8 {
			output[i] /= winSum[i]
		}
	}

	result := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		s := output[i] * 32768.0
		if s > 32767 {
			s = 32767
		} else if s < -32768 {
			s = -32768
		}
		v := int16(s)
		result[i*2] = byte(v)
		result[i*2+1] = byte(v >> 8)
	}
	return result
}

// percentile returns the value at the given percentile (0-100) of data.
func percentile(data []float64, pct int) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	n := len(sorted)
	idx := n * pct / 100
	if idx >= n {
		idx = n - 1
	}
	// Partial sort: find the idx-th smallest
	for i := 0; i <= idx; i++ {
		minIdx := i
		for j := i + 1; j < n; j++ {
			if sorted[j] < sorted[minIdx] {
				minIdx = j
			}
		}
		sorted[i], sorted[minIdx] = sorted[minIdx], sorted[i]
	}
	return sorted[idx]
}
