package fbank

import "math"

// hammingWindow generates a Hamming window of the given length.
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

// melToHz converts mel scale frequency back to Hz.
func melToHz(mel float64) float64 {
	return 700.0 * (math.Pow(10.0, mel/2595.0) - 1.0)
}

// melFilterBank creates the mel filterbank matrix.
// Returns [numMels][halfFFT] where halfFFT = fftSize/2 + 1.
func melFilterBank(numMels, fftSize, sampleRate int, lowFreq, highFreq float64) [][]float64 {
	halfFFT := fftSize/2 + 1
	lowMel := hzToMel(lowFreq)
	highMel := hzToMel(highFreq)

	// numMels + 2 equally spaced mel points
	melPoints := make([]float64, numMels+2)
	step := (highMel - lowMel) / float64(numMels+1)
	for i := range melPoints {
		melPoints[i] = lowMel + float64(i)*step
	}

	// Convert mel points to FFT bin indices (round to nearest)
	bins := make([]int, numMels+2)
	for i, m := range melPoints {
		hz := melToHz(m)
		bin := int(math.Round(hz * float64(fftSize) / float64(sampleRate)))
		if bin >= halfFFT {
			bin = halfFFT - 1
		}
		bins[i] = bin
	}

	// Ensure each filter has at least 1 bin width
	for i := 1; i < len(bins); i++ {
		if bins[i] <= bins[i-1] {
			bins[i] = bins[i-1] + 1
		}
	}

	// Create triangular filters
	bank := make([][]float64, numMels)
	for m := 0; m < numMels; m++ {
		filter := make([]float64, halfFFT)
		left := bins[m]
		center := bins[m+1]
		right := bins[m+2]

		for k := left; k < center && k < halfFFT; k++ {
			if center != left {
				filter[k] = float64(k-left) / float64(center-left)
			}
		}
		for k := center; k <= right && k < halfFFT; k++ {
			if right != center {
				filter[k] = float64(right-k) / float64(right-center)
			}
		}
		bank[m] = filter
	}
	return bank
}
