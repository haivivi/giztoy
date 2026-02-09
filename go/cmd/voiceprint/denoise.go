package main

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/haivivi/giztoy/go/pkg/ncnn"
)

// --------------------------------------------------------------------------
// DTLN Denoiser (two-stage neural network)
// --------------------------------------------------------------------------

type dtlnDenoiser struct {
	net1 *ncnn.Net
	net2 *ncnn.Net
}

func newDTLNDenoiser() (*dtlnDenoiser, error) {
	net1, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		return nil, fmt.Errorf("load DTLN1: %w", err)
	}
	net2, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN2)
	if err != nil {
		net1.Close()
		return nil, fmt.Errorf("load DTLN2: %w", err)
	}
	return &dtlnDenoiser{net1: net1, net2: net2}, nil
}

func (d *dtlnDenoiser) Close() {
	d.net1.Close()
	d.net2.Close()
}

// Denoise runs the full DTLN two-stage pipeline on 16kHz mono int16 PCM.
func (d *dtlnDenoiser) Denoise(pcm []byte) ([]byte, error) {
	const (
		fftSize  = 512
		hopSize  = 128
		halfFFT  = fftSize/2 + 1
		stateLen = 128
	)

	numSamples := len(pcm) / 2
	if numSamples < fftSize {
		return pcm, nil
	}

	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		samples[i] = float32(s) / 32768.0
	}

	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	// LSTM states
	h1 := make([]float32, stateLen)
	c1 := make([]float32, stateLen)
	h2 := make([]float32, stateLen)
	c2 := make([]float32, stateLen)
	h3 := make([]float32, stateLen)
	c3 := make([]float32, stateLen)
	h4 := make([]float32, stateLen)
	c4 := make([]float32, stateLen)

	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)
	numFrames := (numSamples - fftSize) / hopSize + 1

	for t := 0; t < numFrames; t++ {
		start := t * hopSize

		// STFT
		fftR := make([]float64, fftSize)
		fftI := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			fftR[i] = float64(samples[start+i]) * hann[i]
		}
		fftInPlace(fftR, fftI)

		mag := make([]float32, halfFFT)
		phase := make([]complex128, halfFFT)
		for i := 0; i < halfFFT; i++ {
			c := complex(fftR[i], fftI[i])
			mag[i] = float32(cmplx.Abs(c))
			phase[i] = c
		}

		// Stage 1: DTLN1
		mask, nh1, nc1, nh2, nc2, err := d.runStage1(mag, h1, c1, h2, c2)
		if err != nil {
			return nil, fmt.Errorf("DTLN1 frame %d: %w", t, err)
		}
		h1, c1, h2, c2 = nh1, nc1, nh2, nc2

		// Apply mask
		ifftR := make([]float64, fftSize)
		ifftI := make([]float64, fftSize)
		for i := 0; i < halfFFT; i++ {
			enhMag := float64(mag[i]) * float64(mask[i])
			if cmplx.Abs(phase[i]) > 1e-10 {
				angle := cmplx.Phase(phase[i])
				c := cmplx.Rect(enhMag, angle)
				ifftR[i] = real(c)
				ifftI[i] = imag(c)
			}
			if i > 0 && i < halfFFT-1 {
				ifftR[fftSize-i] = ifftR[i]
				ifftI[fftSize-i] = -ifftI[i]
			}
		}
		ifftInPlace(ifftR, ifftI)

		timeFrame := make([]float32, fftSize)
		for i := 0; i < fftSize; i++ {
			timeFrame[i] = float32(ifftR[i])
		}

		// Stage 2: DTLN2
		enhanced, nh3, nc3, nh4, nc4, err := d.runStage2(timeFrame, h3, c3, h4, c4)
		if err != nil {
			return nil, fmt.Errorf("DTLN2 frame %d: %w", t, err)
		}
		h3, c3, h4, c4 = nh3, nc3, nh4, nc4

		// Overlap-add
		for i := 0; i < fftSize; i++ {
			idx := start + i
			if idx < numSamples {
				output[idx] += float64(enhanced[i]) * hann[i]
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
	return result, nil
}

func (d *dtlnDenoiser) runStage1(mag, h1, c1, h2, c2 []float32) (
	mask, nh1, nc1, nh2, nc2 []float32, err error,
) {
	inMag := ncnn.NewMat2D(257, 1, mag)
	defer inMag.Close()
	inH1 := ncnn.NewMat2D(128, 1, h1)
	defer inH1.Close()
	inC1 := ncnn.NewMat2D(128, 1, c1)
	defer inC1.Close()
	inH2 := ncnn.NewMat2D(128, 1, h2)
	defer inH2.Close()
	inC2 := ncnn.NewMat2D(128, 1, c2)
	defer inC2.Close()

	ex, exErr := d.net1.NewExtractor()
	if exErr != nil {
		return nil, nil, nil, nil, nil, exErr
	}
	defer ex.Close()
	ex.SetInput("in0", inMag)
	ex.SetInput("in1", inH1)
	ex.SetInput("in2", inC1)
	ex.SetInput("in3", inH2)
	ex.SetInput("in4", inC2)

	outMask, err := ex.Extract("out0")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	defer outMask.Close()

	z := make([]float32, 128)
	outH1, err1 := ex.Extract("out1")
	if err1 != nil {
		return outMask.FloatData(), z, z, z, z, nil
	}
	defer outH1.Close()
	outC1, err2 := ex.Extract("out2")
	if err2 != nil {
		return outMask.FloatData(), outH1.FloatData(), z, z, z, nil
	}
	defer outC1.Close()
	outH2, err3 := ex.Extract("out3")
	if err3 != nil {
		return outMask.FloatData(), outH1.FloatData(), outC1.FloatData(), z, z, nil
	}
	defer outH2.Close()
	outC2, err4 := ex.Extract("out4")
	if err4 != nil {
		return outMask.FloatData(), outH1.FloatData(), outC1.FloatData(), outH2.FloatData(), z, nil
	}
	defer outC2.Close()

	return outMask.FloatData(), outH1.FloatData(), outC1.FloatData(),
		outH2.FloatData(), outC2.FloatData(), nil
}

func (d *dtlnDenoiser) runStage2(frame, h3, c3, h4, c4 []float32) (
	enhanced, nh3, nc3, nh4, nc4 []float32, err error,
) {
	inFrame := ncnn.NewMat2D(512, 1, frame)
	defer inFrame.Close()
	inH3 := ncnn.NewMat2D(128, 1, h3)
	defer inH3.Close()
	inC3 := ncnn.NewMat2D(128, 1, c3)
	defer inC3.Close()
	inH4 := ncnn.NewMat2D(128, 1, h4)
	defer inH4.Close()
	inC4 := ncnn.NewMat2D(128, 1, c4)
	defer inC4.Close()

	ex, exErr := d.net2.NewExtractor()
	if exErr != nil {
		return nil, nil, nil, nil, nil, exErr
	}
	defer ex.Close()
	ex.SetInput("in0", inFrame)
	ex.SetInput("in1", inH3)
	ex.SetInput("in2", inC3)
	ex.SetInput("in3", inH4)
	ex.SetInput("in4", inC4)

	outFrame, err := ex.Extract("out0")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	defer outFrame.Close()

	z := make([]float32, 128)
	outH3, err3 := ex.Extract("out1")
	if err3 != nil {
		return outFrame.FloatData(), z, z, z, z, nil
	}
	defer outH3.Close()
	outC3, err4 := ex.Extract("out2")
	if err4 != nil {
		return outFrame.FloatData(), outH3.FloatData(), z, z, z, nil
	}
	defer outC3.Close()
	outH4, err5 := ex.Extract("out3")
	if err5 != nil {
		return outFrame.FloatData(), outH3.FloatData(), outC3.FloatData(), z, z, nil
	}
	defer outH4.Close()
	outC4, err6 := ex.Extract("out4")
	if err6 != nil {
		return outFrame.FloatData(), outH3.FloatData(), outC3.FloatData(), outH4.FloatData(), z, nil
	}
	defer outC4.Close()

	return outFrame.FloatData(), outH3.FloatData(), outC3.FloatData(),
		outH4.FloatData(), outC4.FloatData(), nil
}

// DenoiseMaskOnly runs only DTLN Stage 1 (frequency-domain masking),
// skipping Stage 2 (time-domain enhancement). This is more conservative
// and preserves speaker characteristics better.
func (d *dtlnDenoiser) DenoiseMaskOnly(pcm []byte) ([]byte, error) {
	const (
		fftSize = 512
		hopSize = 128
		halfFFT = fftSize/2 + 1
	)

	numSamples := len(pcm) / 2
	if numSamples < fftSize {
		return pcm, nil
	}

	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		samples[i] = float32(s) / 32768.0
	}

	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	h1 := make([]float32, 128)
	c1 := make([]float32, 128)
	h2 := make([]float32, 128)
	c2 := make([]float32, 128)

	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)
	numFrames := (numSamples - fftSize) / hopSize + 1

	for t := 0; t < numFrames; t++ {
		start := t * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fftInPlace(re, im)

		// Magnitude for DTLN1
		mag := make([]float32, halfFFT)
		for i := 0; i < halfFFT; i++ {
			mag[i] = float32(math.Sqrt(re[i]*re[i] + im[i]*im[i]))
		}

		// DTLN1 mask
		mask, nh1, nc1, nh2, nc2, err := d.runStage1(mag, h1, c1, h2, c2)
		if err != nil {
			return nil, fmt.Errorf("DTLN1 frame %d: %w", t, err)
		}
		h1, c1, h2, c2 = nh1, nc1, nh2, nc2

		// Apply mask directly to complex spectrum
		for i := 0; i < halfFFT; i++ {
			g := float64(mask[i])
			re[i] *= g
			im[i] *= g
			if i > 0 && i < halfFFT-1 {
				re[fftSize-i] = re[i]
				im[fftSize-i] = -im[i]
			}
		}

		// IFFT
		ifftInPlace(re, im)

		// Overlap-add
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
	return result, nil
}

// --------------------------------------------------------------------------
// Spectral subtraction (simple, stateless)
// --------------------------------------------------------------------------

// spectralDenoise applies spectral subtraction noise reduction to 16kHz mono
// int16 PCM bytes.
//
// Algorithm:
//  1. Estimate noise spectrum from the first ~200ms (assumed to be silence/noise)
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
		noiseFrames   = 3       // first 3 frames (~24ms each) for noise estimation
		overSubtract  = 2.0     // oversubtraction factor (aggressiveness)
		spectralFloor = 0.02    // minimum gain to avoid musical noise
	)

	numSamples := len(pcm) / 2
	if numSamples < fftSize+hopSize*noiseFrames {
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
	// Compute per-frame energy, find the quietest 20% of frames
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
		fftInPlace(re, im)
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

	// === Phase 2: Spectral subtraction (recompute FFT with phase) ===
	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)

	for t := 0; t < numTotalFrames; t++ {
		start := t * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fftInPlace(re, im)

		// Apply spectral subtraction with soft floor
		for i := 0; i < halfFFT; i++ {
			c := complex(re[i], im[i])
			power := re[i]*re[i] + im[i]*im[i]
			cleanPower := power - overSubtract*noisePower[i]

			// Compute gain
			gain := spectralFloor
			if power > 1e-10 {
				g := math.Sqrt(math.Max(cleanPower, 0) / power)
				if g > spectralFloor {
					gain = g
				}
			}

			// Apply gain preserving phase
			angle := cmplx.Phase(c)
			mag := cmplx.Abs(c) * gain
			cleaned := cmplx.Rect(mag, angle)
			re[i] = real(cleaned)
			im[i] = imag(cleaned)

			// Mirror for conjugate symmetry
			if i > 0 && i < halfFFT-1 {
				re[fftSize-i] = re[i]
				im[fftSize-i] = -im[i]
			}
		}

		// IFFT
		ifftInPlace(re, im)

		// Overlap-add
		for i := 0; i < fftSize; i++ {
			idx := start + i
			if idx < numSamples {
				output[idx] += re[i] * hann[i]
				winSum[idx] += hann[i] * hann[i]
			}
		}
	}

	// Normalize
	for i := range output {
		if winSum[i] > 1e-8 {
			output[i] /= winSum[i]
		}
	}

	// Convert back to int16 LE bytes
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

// percentile returns the value at the given percentile (0-100) of sorted data.
func percentile(data []float64, pct int) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	// Simple selection sort for percentile (good enough for our sizes)
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

// fftInPlace performs an in-place radix-2 Cooley-Tukey FFT.
func fftInPlace(real, imag []float64) {
	n := len(real)
	if n <= 1 {
		return
	}
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
				tmpR := tR*real[v] - tI*imag[v]
				tmpI := tR*imag[v] + tI*real[v]
				real[v] = real[u] - tmpR
				imag[v] = imag[u] - tmpI
				real[u] += tmpR
				imag[u] += tmpI
				tR, tI = tR*wR-tI*wI, tR*wI+tI*wR
			}
		}
	}
}

// ifftInPlace performs an in-place inverse FFT.
func ifftInPlace(real, imag []float64) {
	n := len(real)
	for i := range imag {
		imag[i] = -imag[i]
	}
	fftInPlace(real, imag)
	scale := 1.0 / float64(n)
	for i := range real {
		real[i] *= scale
		imag[i] *= -scale
	}
}
