package main

import (
	"math"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/audio/fbank"
)

func TestFFTRoundTrip(t *testing.T) {
	n := 512
	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 16000)
	}
	original := make([]float64, n)
	copy(original, re)

	fbank.FFT(re, im)
	fbank.IFFT(re, im)

	maxErr := 0.0
	for i := range re {
		err := math.Abs(re[i] - original[i])
		if err > maxErr {
			maxErr = err
		}
	}
	t.Logf("FFT→IFFT max error: %e", maxErr)
	if maxErr > 1e-10 {
		t.Errorf("FFT round-trip error too large: %e", maxErr)
	}
}

func TestSTFTRoundTrip(t *testing.T) {
	const (
		fftSize = 512
		hopSize = 128
	)

	numSamples := 16000
	samples := make([]float64, numSamples)
	for i := range samples {
		samples[i] = 0.5*math.Sin(2*math.Pi*440*float64(i)/16000) +
			0.3*math.Sin(2*math.Pi*1000*float64(i)/16000)
	}

	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	numFrames := (numSamples - fftSize) / hopSize + 1
	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)

	for f := 0; f < numFrames; f++ {
		start := f * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = samples[start+i] * hann[i]
		}
		fbank.FFT(re, im)

		halfFFT := fftSize/2 + 1
		for i := 1; i < halfFFT-1; i++ {
			re[fftSize-i] = re[i]
			im[fftSize-i] = -im[i]
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

	margin := fftSize
	maxErr := 0.0
	for i := margin; i < numSamples-margin; i++ {
		err := math.Abs(output[i] - samples[i])
		if err > maxErr {
			maxErr = err
		}
	}
	t.Logf("STFT→ISTFT round-trip max error (middle): %e", maxErr)
	if maxErr > 1e-6 {
		t.Errorf("STFT round-trip error too large: %e", maxErr)
	}
}

func TestSpectralDenoise(t *testing.T) {
	// Generate 0.5s of 440Hz sine at 16kHz with added high-freq noise
	numSamples := 8000
	pcm := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		signal := math.Sin(2*math.Pi*440*float64(i)/16000) * 0.3
		noise := math.Sin(2*math.Pi*7500*float64(i)/16000) * 0.05
		s := int16((signal + noise) * 32767)
		pcm[i*2] = byte(s)
		pcm[i*2+1] = byte(s >> 8)
	}

	beforeRMS := pcmRMSf(pcm)
	t.Logf("input: %d samples, RMS=%.1f", numSamples, beforeRMS)

	denoised := spectralDenoise(pcm)
	afterRMS := pcmRMSf(denoised)
	ratio := afterRMS / beforeRMS
	t.Logf("spectral denoise: RMS=%.1f, ratio=%.4f", afterRMS, ratio)

	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		t.Errorf("spectral denoise produced NaN/Inf ratio: %.4f", ratio)
	}
	if ratio > 1.5 {
		t.Errorf("spectral denoise amplified signal too much: ratio=%.4f", ratio)
	}
}

func pcmRMSf(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(n))
}
