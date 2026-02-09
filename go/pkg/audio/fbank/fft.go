package fbank

import "math"

// fft performs an in-place radix-2 Cooley-Tukey FFT.
// real and imag must have the same power-of-2 length.
func fft(real, imag []float64) {
	n := len(real)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			real[i], real[j] = real[j], real[i]
			imag[i], imag[j] = imag[j], imag[i]
		}
		k := n >> 1
		for k <= j {
			j -= k
			k >>= 1
		}
		j += k
	}

	// Cooley-Tukey butterfly
	for size := 2; size <= n; size <<= 1 {
		half := size >> 1
		angle := -2.0 * math.Pi / float64(size)
		wR := math.Cos(angle)
		wI := math.Sin(angle)

		for start := 0; start < n; start += size {
			tR, tI := 1.0, 0.0
			for k := 0; k < half; k++ {
				u := start + k
				v := u + half

				// Butterfly
				tmpR := tR*real[v] - tI*imag[v]
				tmpI := tR*imag[v] + tI*real[v]

				real[v] = real[u] - tmpR
				imag[v] = imag[u] - tmpI
				real[u] += tmpR
				imag[u] += tmpI

				// Twiddle factor update
				tR, tI = tR*wR-tI*wI, tR*wI+tI*wR
			}
		}
	}
}
