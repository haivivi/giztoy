package voiceprint

import (
	"math"
	"math/cmplx"
)

// FbankConfig configures mel filterbank feature extraction.
type FbankConfig struct {
	SampleRate  int     // Input sample rate in Hz (default: 16000)
	NumMels     int     // Number of mel filterbank channels (default: 80)
	FrameLength int     // Frame length in samples (default: 400 = 25ms @ 16kHz)
	FrameShift  int     // Frame shift in samples (default: 160 = 10ms @ 16kHz)
	PreEmphasis float64 // Pre-emphasis coefficient (default: 0.97)
	EnergyFloor float64 // Floor for log energy (default: 1e-10)
}

// DefaultFbankConfig returns the default configuration for 16kHz audio.
func DefaultFbankConfig() FbankConfig {
	return FbankConfig{
		SampleRate:  16000,
		NumMels:     80,
		FrameLength: 400,  // 25ms @ 16kHz
		FrameShift:  160,  // 10ms @ 16kHz
		PreEmphasis: 0.97,
		EnergyFloor: 1e-10,
	}
}

// ComputeFbank extracts log mel filterbank features from PCM16 audio.
//
// Input: PCM16 signed little-endian audio bytes at the configured sample rate.
// Output: 2D slice [numFrames][numMels] of log mel filterbank energies.
//
// The algorithm:
//  1. Convert PCM16 bytes to float64 samples
//  2. Apply pre-emphasis filter
//  3. Split into overlapping frames
//  4. Apply Hamming window
//  5. Compute power spectrum via FFT
//  6. Apply mel filterbank
//  7. Take log of energies
func ComputeFbank(audio []byte, cfg FbankConfig) [][]float32 {
	// Convert PCM16 to float64 samples.
	nSamples := len(audio) / 2
	if nSamples < cfg.FrameLength {
		return nil
	}
	samples := make([]float64, nSamples)
	for i := 0; i < nSamples; i++ {
		lo := audio[2*i]
		hi := audio[2*i+1]
		s := int16(lo) | int16(hi)<<8
		samples[i] = float64(s)
	}

	// Pre-emphasis.
	if cfg.PreEmphasis > 0 {
		for i := nSamples - 1; i > 0; i-- {
			samples[i] -= cfg.PreEmphasis * samples[i-1]
		}
		samples[0] *= 1.0 - cfg.PreEmphasis
	}

	// Compute number of frames.
	numFrames := (nSamples - cfg.FrameLength) / cfg.FrameShift + 1
	if numFrames <= 0 {
		return nil
	}

	// FFT size: next power of 2 >= FrameLength.
	fftSize := nextPow2(cfg.FrameLength)
	halfFFT := fftSize/2 + 1

	// Pre-compute Hamming window.
	window := hammingWindow(cfg.FrameLength)

	// Pre-compute mel filterbank.
	filterbank := melFilterbank(cfg.NumMels, fftSize, cfg.SampleRate)

	// Process each frame.
	result := make([][]float32, numFrames)
	fftBuf := make([]complex128, fftSize)

	for f := 0; f < numFrames; f++ {
		offset := f * cfg.FrameShift

		// Apply window and zero-pad to FFT size.
		for i := range fftBuf {
			fftBuf[i] = 0
		}
		for i := 0; i < cfg.FrameLength; i++ {
			fftBuf[i] = complex(samples[offset+i]*window[i], 0)
		}

		// In-place FFT.
		fft(fftBuf)

		// Power spectrum: |X[k]|^2.
		powerSpec := make([]float64, halfFFT)
		for k := 0; k < halfFFT; k++ {
			r := real(fftBuf[k])
			im := imag(fftBuf[k])
			powerSpec[k] = r*r + im*im
		}

		// Apply mel filterbank and take log.
		frame := make([]float32, cfg.NumMels)
		for m := 0; m < cfg.NumMels; m++ {
			var energy float64
			for k, w := range filterbank[m] {
				energy += w * powerSpec[k]
			}
			if energy < cfg.EnergyFloor {
				energy = cfg.EnergyFloor
			}
			frame[m] = float32(math.Log(energy))
		}
		result[f] = frame
	}

	return result
}

// nextPow2 returns the smallest power of 2 >= n.
func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// hammingWindow computes a Hamming window of the given length.
func hammingWindow(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(n-1))
	}
	return w
}

// hzToMel converts frequency in Hz to mel scale.
func hzToMel(hz float64) float64 {
	return 2595.0 * math.Log10(1.0+hz/700.0)
}

// melToHz converts mel scale to frequency in Hz.
func melToHz(mel float64) float64 {
	return 700.0 * (math.Pow(10.0, mel/2595.0) - 1.0)
}

// melFilterbank computes triangular mel filterbank weights.
// Returns [numMels][halfFFT] weights.
func melFilterbank(numMels, fftSize, sampleRate int) [][]float64 {
	halfFFT := fftSize/2 + 1

	// Mel scale boundaries.
	melLow := hzToMel(0)
	melHigh := hzToMel(float64(sampleRate) / 2)

	// Equally spaced mel points.
	melPoints := make([]float64, numMels+2)
	for i := range melPoints {
		melPoints[i] = melLow + float64(i)*(melHigh-melLow)/float64(numMels+1)
	}

	// Convert back to Hz and then to FFT bin indices.
	binIndices := make([]int, numMels+2)
	for i := range melPoints {
		hz := melToHz(melPoints[i])
		binIndices[i] = int(math.Floor(hz * float64(fftSize) / float64(sampleRate)))
		if binIndices[i] >= halfFFT {
			binIndices[i] = halfFFT - 1
		}
	}

	// Build triangular filters.
	fb := make([][]float64, numMels)
	for m := 0; m < numMels; m++ {
		fb[m] = make([]float64, halfFFT)
		left := binIndices[m]
		center := binIndices[m+1]
		right := binIndices[m+2]

		// Rising slope.
		for k := left; k <= center; k++ {
			if center > left {
				fb[m][k] = float64(k-left) / float64(center-left)
			}
		}
		// Falling slope.
		for k := center; k <= right; k++ {
			if right > center {
				fb[m][k] = float64(right-k) / float64(right-center)
			}
		}
	}
	return fb
}

// fft computes the in-place Cooley-Tukey FFT.
// The input length must be a power of 2.
func fft(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation.
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	// Butterfly operations.
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		wn := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < half; k++ {
				u := x[start+k]
				t := w * x[start+k+half]
				x[start+k] = u + t
				x[start+k+half] = u - t
				w *= wn
			}
		}
	}
}
