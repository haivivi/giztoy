package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/audio/fbank"
	"github.com/haivivi/giztoy/go/pkg/ncnn"
)

// === Step 1: Verify FFT/IFFT round-trip ===

func TestFFTRoundTrip(t *testing.T) {
	// Generate a known signal: 440Hz sine
	n := 512
	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 16000)
	}
	original := make([]float64, n)
	copy(original, re)

	// Forward FFT
	fbank.FFT(re, im)

	// Inverse FFT
	fbank.IFFT(re, im)

	// Should recover original signal
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
	// Full STFT → IFFT with overlap-add, no modification
	const (
		fftSize = 512
		hopSize = 128
	)

	// Generate 1 second of 440Hz + 1000Hz at 16kHz
	numSamples := 16000
	samples := make([]float64, numSamples)
	for i := range samples {
		samples[i] = 0.5*math.Sin(2*math.Pi*440*float64(i)/16000) +
			0.3*math.Sin(2*math.Pi*1000*float64(i)/16000)
	}

	// Hann window
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

		// Forward FFT
		fbank.FFT(re, im)

		// NO modification — just reconstruct

		// Mirror conjugate symmetry (needed for real-valued IFFT)
		halfFFT := fftSize/2 + 1
		for i := 1; i < halfFFT-1; i++ {
			re[fftSize-i] = re[i]
			im[fftSize-i] = -im[i]
		}

		// Inverse FFT
		fbank.IFFT(re, im)

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

	// Compare with original (skip edges where window sum is incomplete)
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

// === Step 2: Verify DTLN1 model output ===

func TestDTLN1ModelOutput(t *testing.T) {
	net, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		t.Fatal(err)
	}
	defer net.Close()

	// Feed a realistic-looking magnitude spectrum
	mag := make([]float32, 257)
	for i := range mag {
		mag[i] = float32(math.Exp(-float64(i) * 0.02)) * 10 // decaying spectrum
	}
	state := make([]float32, 128)

	inMag := ncnn.NewMat2D(257, 1, mag)
	defer inMag.Close()
	inH1 := ncnn.NewMat2D(128, 1, state)
	defer inH1.Close()
	inC1 := ncnn.NewMat2D(128, 1, state)
	defer inC1.Close()
	inH2 := ncnn.NewMat2D(128, 1, state)
	defer inH2.Close()
	inC2 := ncnn.NewMat2D(128, 1, state)
	defer inC2.Close()

	ex, err := net.NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}
	defer ex.Close()
	ex.SetInput("in0", inMag)
	ex.SetInput("in1", inH1)
	ex.SetInput("in2", inC1)
	ex.SetInput("in3", inH2)
	ex.SetInput("in4", inC2)

	// Check out0 (mask)
	outMask, err := ex.Extract("out0")
	if err != nil {
		t.Fatalf("extract out0: %v", err)
	}
	maskData := outMask.FloatData()
	outMask.Close()
	t.Logf("out0 (mask): len=%d", len(maskData))

	maskMin, maskMax, maskAvg := float32(maskData[0]), float32(maskData[0]), float64(0)
	for _, v := range maskData {
		if v < maskMin {
			maskMin = v
		}
		if v > maskMax {
			maskMax = v
		}
		maskAvg += float64(v)
	}
	maskAvg /= float64(len(maskData))
	t.Logf("  mask min=%.6f max=%.6f avg=%.6f", maskMin, maskMax, maskAvg)

	// Mask should be in [0, 1] range (sigmoid output)
	if maskMin < -0.01 || maskMax > 1.01 {
		t.Errorf("mask out of [0,1] range: [%.4f, %.4f]", maskMin, maskMax)
	}

	// Check out1-out4 (LSTM states)
	for _, name := range []string{"out1", "out2", "out3", "out4"} {
		out, err := ex.Extract(name)
		if err != nil {
			t.Logf("  %s: EXTRACT FAILED: %v", name, err)
			continue
		}
		data := out.FloatData()
		out.Close()

		if len(data) == 0 {
			t.Logf("  %s: EMPTY (len=0)", name)
			continue
		}

		mn, mx, sum := float64(data[0]), float64(data[0]), 0.0
		nonZero := 0
		for _, v := range data {
			fv := float64(v)
			sum += fv
			if fv < mn {
				mn = fv
			}
			if fv > mx {
				mx = fv
			}
			if math.Abs(fv) > 1e-10 {
				nonZero++
			}
		}
		t.Logf("  %s: len=%d min=%.6f max=%.6f avg=%.6f nonzero=%d/%d",
			name, len(data), mn, mx, sum/float64(len(data)), nonZero, len(data))
	}
}

// === Step 3: Verify DTLN2 model output ===

func TestDTLN2ModelOutput(t *testing.T) {
	net, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN2)
	if err != nil {
		t.Fatal(err)
	}
	defer net.Close()

	// Time-domain frame (512 samples of a sine wave)
	frame := make([]float32, 512)
	for i := range frame {
		frame[i] = float32(math.Sin(2*math.Pi*440*float64(i)/16000)) * 0.5
	}
	state := make([]float32, 128)

	inFrame := ncnn.NewMat2D(512, 1, frame)
	defer inFrame.Close()
	inH1 := ncnn.NewMat2D(128, 1, state)
	defer inH1.Close()
	inC1 := ncnn.NewMat2D(128, 1, state)
	defer inC1.Close()
	inH2 := ncnn.NewMat2D(128, 1, state)
	defer inH2.Close()
	inC2 := ncnn.NewMat2D(128, 1, state)
	defer inC2.Close()

	ex, err := net.NewExtractor()
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}
	defer ex.Close()
	ex.SetInput("in0", inFrame)
	ex.SetInput("in1", inH1)
	ex.SetInput("in2", inC1)
	ex.SetInput("in3", inH2)
	ex.SetInput("in4", inC2)

	// Check out0 (enhanced frame)
	outFrame, err := ex.Extract("out0")
	if err != nil {
		t.Fatalf("extract out0: %v", err)
	}
	frameData := outFrame.FloatData()
	outFrame.Close()
	t.Logf("out0 (enhanced frame): len=%d", len(frameData))

	frameMin, frameMax := float64(frameData[0]), float64(frameData[0])
	frameRMS := 0.0
	for _, v := range frameData {
		fv := float64(v)
		if fv < frameMin {
			frameMin = fv
		}
		if fv > frameMax {
			frameMax = fv
		}
		frameRMS += fv * fv
	}
	frameRMS = math.Sqrt(frameRMS / float64(len(frameData)))
	t.Logf("  frame min=%.6f max=%.6f rms=%.6f", frameMin, frameMax, frameRMS)

	// Enhanced frame should have similar magnitude to input, not blow up
	inputRMS := 0.0
	for _, v := range frame {
		inputRMS += float64(v) * float64(v)
	}
	inputRMS = math.Sqrt(inputRMS / float64(len(frame)))
	ratio := frameRMS / inputRMS
	t.Logf("  RMS ratio (output/input): %.4f", ratio)
	if ratio > 10 || ratio < 0.01 {
		t.Errorf("output RMS wildly different from input: ratio=%.4f", ratio)
	}

	// Check state outputs
	for _, name := range []string{"out1", "out2", "out3", "out4"} {
		out, err := ex.Extract(name)
		if err != nil {
			t.Logf("  %s: EXTRACT FAILED: %v", name, err)
			continue
		}
		data := out.FloatData()
		out.Close()
		if len(data) == 0 {
			t.Logf("  %s: EMPTY", name)
			continue
		}
		nonZero := 0
		for _, v := range data {
			if math.Abs(float64(v)) > 1e-10 {
				nonZero++
			}
		}
		t.Logf("  %s: len=%d nonzero=%d/%d", name, len(data), nonZero, len(data))
	}
}

// === Step 4: Multi-frame DTLN1 with state propagation ===

func TestDTLN1StatePropagation(t *testing.T) {
	net, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		t.Fatal(err)
	}
	defer net.Close()

	// Run 5 frames, check if states change between frames
	h1 := make([]float32, 128)
	c1 := make([]float32, 128)
	h2 := make([]float32, 128)
	c2 := make([]float32, 128)

	for frame := 0; frame < 5; frame++ {
		mag := make([]float32, 257)
		for i := range mag {
			mag[i] = float32(math.Exp(-float64(i)*0.02)) * (10 + float32(frame))
		}

		inMag := ncnn.NewMat2D(257, 1, mag)
		inH1 := ncnn.NewMat2D(128, 1, h1)
		inC1 := ncnn.NewMat2D(128, 1, c1)
		inH2 := ncnn.NewMat2D(128, 1, h2)
		inC2 := ncnn.NewMat2D(128, 1, c2)

		ex, _ := net.NewExtractor()
		ex.SetInput("in0", inMag)
		ex.SetInput("in1", inH1)
		ex.SetInput("in2", inC1)
		ex.SetInput("in3", inH2)
		ex.SetInput("in4", inC2)

		outMask, _ := ex.Extract("out0")
		maskData := outMask.FloatData()

		maskAvg := 0.0
		for _, v := range maskData {
			maskAvg += float64(v)
		}
		maskAvg /= float64(len(maskData))

		// Try to extract states
		stateOK := true
		outH1, err := ex.Extract("out1")
		if err != nil {
			t.Logf("  frame %d: out1 failed: %v — states NOT propagating", frame, err)
			stateOK = false
		}

		if stateOK {
			h1 = outH1.FloatData()
			outC1, _ := ex.Extract("out2")
			c1 = outC1.FloatData()
			outH2, _ := ex.Extract("out3")
			h2 = outH2.FloatData()
			outC2, _ := ex.Extract("out4")
			c2 = outC2.FloatData()

			h1Norm := vecNorm(h1)
			t.Logf("  frame %d: maskAvg=%.4f h1_norm=%.4f stateOK=true", frame, maskAvg, h1Norm)

			outH1.Close()
			outC1.Close()
			outH2.Close()
			outC2.Close()
		} else {
			t.Logf("  frame %d: maskAvg=%.4f stateOK=false", frame, maskAvg)
		}

		outMask.Close()
		ex.Close()
		inMag.Close()
		inH1.Close()
		inC1.Close()
		inH2.Close()
		inC2.Close()
	}
}

// === Step 5: Full denoise on a known signal ===

func TestDenoiseKnownSignal(t *testing.T) {
	// Generate 0.5s of 440Hz sine at 16kHz with added white noise
	numSamples := 8000
	pcm := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		signal := math.Sin(2*math.Pi*440*float64(i)/16000) * 0.3
		// Simple "noise": high-freq component
		noise := math.Sin(2*math.Pi*7500*float64(i)/16000) * 0.05
		s := int16((signal + noise) * 32767)
		pcm[i*2] = byte(s)
		pcm[i*2+1] = byte(s >> 8)
	}

	beforeRMS := pcmRMSf(pcm)
	t.Logf("input: %d samples, RMS=%.1f", numSamples, beforeRMS)

	// Run spectral denoise (our simpler one)
	denoised := spectralDenoise(pcm)
	afterRMS := pcmRMSf(denoised)
	ratio := afterRMS / beforeRMS
	t.Logf("spectral denoise: RMS=%.1f, ratio=%.4f", afterRMS, ratio)

	if ratio > 2.0 || ratio < 0.1 {
		t.Errorf("spectral denoise RMS ratio abnormal: %.4f", ratio)
	}
}

func vecNorm(v []float32) float64 {
	sum := 0.0
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
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

func TestDenoiseMaskOnly(t *testing.T) {
	// Test DTLN1 mask-only pipeline (no DTLN2), to isolate the problem
	const (
		fftSize = 512
		hopSize = 128
		halfFFT = fftSize/2 + 1
	)

	net1, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		t.Fatal(err)
	}
	defer net1.Close()

	// 0.5s of 440Hz sine at 16kHz
	numSamples := 8000
	samples := make([]float32, numSamples)
	for i := range samples {
		samples[i] = float32(math.Sin(2*math.Pi*440*float64(i)/16000)) * 0.3
	}

	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)
	numFrames := (numSamples - fftSize) / hopSize + 1

	h1, c1, h2, c2 := make([]float32, 128), make([]float32, 128), make([]float32, 128), make([]float32, 128)

	for t_ := 0; t_ < numFrames; t_++ {
		start := t_ * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fbank.FFT(re, im)

		// Get magnitude for DTLN1
		mag := make([]float32, halfFFT)
		for i := 0; i < halfFFT; i++ {
			mag[i] = float32(math.Sqrt(re[i]*re[i] + im[i]*im[i]))
		}

		// DTLN1 mask
		inMag := ncnn.NewMat2D(257, 1, mag)
		inH1 := ncnn.NewMat2D(128, 1, h1)
		inC1 := ncnn.NewMat2D(128, 1, c1)
		inH2 := ncnn.NewMat2D(128, 1, h2)
		inC2 := ncnn.NewMat2D(128, 1, c2)
		ex, _ := net1.NewExtractor()
		ex.SetInput("in0", inMag)
		ex.SetInput("in1", inH1)
		ex.SetInput("in2", inC1)
		ex.SetInput("in3", inH2)
		ex.SetInput("in4", inC2)
		outMask, _ := ex.Extract("out0")
		mask := outMask.FloatData()
		outH1, _ := ex.Extract("out1")
		h1 = outH1.FloatData()
		outC1, _ := ex.Extract("out2")
		c1 = outC1.FloatData()
		outH2, _ := ex.Extract("out3")
		h2 = outH2.FloatData()
		outC2, _ := ex.Extract("out4")
		c2 = outC2.FloatData()

		if t_ == 0 {
			t.Logf("frame 0: mag[0..4]=%.2f,%.2f,%.2f,%.2f,%.2f  mask[0..4]=%.4f,%.4f,%.4f,%.4f,%.4f",
				mag[0], mag[1], mag[2], mag[3], mag[4],
				mask[0], mask[1], mask[2], mask[3], mask[4])
		}

		// Apply mask to complex spectrum
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
		fbank.IFFT(re, im)

		// Overlap-add
		for i := 0; i < fftSize; i++ {
			idx := start + i
			if idx < numSamples {
				output[idx] += re[i] * hann[i]
				winSum[idx] += hann[i] * hann[i]
			}
		}

		outMask.Close()
		outH1.Close()
		outC1.Close()
		outH2.Close()
		outC2.Close()
		ex.Close()
		inMag.Close()
		inH1.Close()
		inC1.Close()
		inH2.Close()
		inC2.Close()
	}

	// Normalize
	for i := range output {
		if winSum[i] > 1e-8 {
			output[i] /= winSum[i]
		}
	}

	// Check output
	inRMS := 0.0
	outRMS := 0.0
	for i := fftSize; i < numSamples-fftSize; i++ {
		inRMS += float64(samples[i]) * float64(samples[i])
		outRMS += output[i] * output[i]
	}
	middle := numSamples - 2*fftSize
	inRMS = math.Sqrt(inRMS / float64(middle))
	outRMS = math.Sqrt(outRMS / float64(middle))
	ratio := outRMS / inRMS
	t.Logf("mask-only: inRMS=%.6f outRMS=%.6f ratio=%.4f", inRMS, outRMS, ratio)
	t.Logf("output[512..516]: %.6f, %.6f, %.6f, %.6f",
		output[512], output[513], output[514], output[515])

	if ratio > 3.0 || ratio < 0.01 {
		t.Errorf("mask-only RMS ratio abnormal: %.4f", ratio)
	}
}

func TestDenoiseRealSpeech(t *testing.T) {
	// Generate speech-like signal: multiple harmonics simulating formants
	// F0=150Hz (male fundamental), F1=500Hz, F2=1500Hz, F3=2500Hz
	numSamples := 16000 // 1 second
	pcm := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		f := float64(i) / 16000.0
		signal := 0.15*math.Sin(2*math.Pi*150*f) + // fundamental
			0.10*math.Sin(2*math.Pi*300*f) + // 2nd harmonic
			0.08*math.Sin(2*math.Pi*450*f) + // 3rd harmonic
			0.12*math.Sin(2*math.Pi*500*f) + // F1
			0.06*math.Sin(2*math.Pi*1500*f) + // F2
			0.04*math.Sin(2*math.Pi*2500*f) // F3
		s := int16(signal * 32767)
		pcm[i*2] = byte(s)
		pcm[i*2+1] = byte(s >> 8)
	}

	denoiser, err := newDTLNDenoiser()
	if err != nil {
		t.Fatal(err)
	}
	defer denoiser.Close()

	denoised, err := denoiser.Denoise(pcm)
	if err != nil {
		t.Fatal(err)
	}

	inRMS := pcmRMSf(pcm)
	outRMS := pcmRMSf(denoised)
	ratio := outRMS / inRMS
	t.Logf("speech-like: inRMS=%.1f outRMS=%.1f ratio=%.4f", inRMS, outRMS, ratio)

	// Check sample correlation
	n := numSamples
	var corrNum, corrDenA, corrDenB float64
	for i := 512; i < n-512; i++ { // skip edges
		a := float64(int16(pcm[i*2]) | int16(pcm[i*2+1])<<8)
		b := float64(int16(denoised[i*2]) | int16(denoised[i*2+1])<<8)
		corrNum += a * b
		corrDenA += a * a
		corrDenB += b * b
	}
	corr := corrNum / (math.Sqrt(corrDenA) * math.Sqrt(corrDenB))
	t.Logf("Pearson correlation with input: %.4f (1.0=identical, 0=unrelated)", corr)

	if ratio > 10 {
		t.Errorf("output blown up: ratio=%.2f", ratio)
	}
	if corr < 0.1 {
		t.Errorf("output uncorrelated with input: corr=%.4f", corr)
	}
	_ = fmt.Sprintf("")
	_ = cmplx.Abs(0)
}

func TestDenoiseRealOGG(t *testing.T) {
	oggPath := os.Getenv("VOICEPRINT_TEST_OGG")
	if oggPath == "" {
		t.Skip("skip: set VOICEPRINT_TEST_OGG to an OGG file path to run this test")
	}
	pcm16k, err := decodeOGGTo16kMono(oggPath)
	if err != nil {
		t.Skipf("skip (decode failed): %v", err)
	}
	numSamples := len(pcm16k) / 2
	t.Logf("decoded 胡子: %d samples (%.1fs)", numSamples, float64(numSamples)/16000)

	// Convert to float32 for analysis
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(pcm16k[i*2]) | int16(pcm16k[i*2+1])<<8
		samples[i] = float32(s) / 32768.0
	}

	const (
		fftSize = 512
		hopSize = 128
		halfFFT = fftSize/2 + 1
	)

	// Hann window
	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	// Load DTLN1 only
	net1, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		t.Fatal(err)
	}
	defer net1.Close()

	h1, c1, h2, c2 := make([]float32, 128), make([]float32, 128), make([]float32, 128), make([]float32, 128)

	numFrames := (numSamples - fftSize) / hopSize + 1

	// Analyze first 10 frames
	for f := 0; f < 10 && f < numFrames; f++ {
		start := f * hopSize
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fbank.FFT(re, im)

		mag := make([]float32, halfFFT)
		totalEnergy := 0.0
		for i := 0; i < halfFFT; i++ {
			mag[i] = float32(math.Sqrt(re[i]*re[i] + im[i]*im[i]))
			totalEnergy += float64(mag[i]) * float64(mag[i])
		}

		// DTLN1
		inMag := ncnn.NewMat2D(257, 1, mag)
		inH1 := ncnn.NewMat2D(128, 1, h1)
		inC1 := ncnn.NewMat2D(128, 1, c1)
		inH2 := ncnn.NewMat2D(128, 1, h2)
		inC2 := ncnn.NewMat2D(128, 1, c2)
		ex, _ := net1.NewExtractor()
		ex.SetInput("in0", inMag)
		ex.SetInput("in1", inH1)
		ex.SetInput("in2", inC1)
		ex.SetInput("in3", inH2)
		ex.SetInput("in4", inC2)
		outMask, _ := ex.Extract("out0")
		mask := outMask.FloatData()
		oH1, _ := ex.Extract("out1")
		h1 = oH1.FloatData()
		oC1, _ := ex.Extract("out2")
		c1 = oC1.FloatData()
		oH2, _ := ex.Extract("out3")
		h2 = oH2.FloatData()
		oC2, _ := ex.Extract("out4")
		c2 = oC2.FloatData()

		// Mask stats
		maskMin, maskMax, maskAvg := float64(mask[0]), float64(mask[0]), 0.0
		for _, v := range mask {
			fv := float64(v)
			maskAvg += fv
			if fv < float64(maskMin) {
				maskMin = fv
			}
			if fv > float64(maskMax) {
				maskMax = fv
			}
		}
		maskAvg /= float64(len(mask))

		// Masked energy ratio
		maskedEnergy := 0.0
		for i := 0; i < halfFFT; i++ {
			me := float64(mag[i]) * float64(mask[i])
			maskedEnergy += me * me
		}
		energyRatio := 0.0
		if totalEnergy > 0 {
			energyRatio = maskedEnergy / totalEnergy
		}

		t.Logf("  frame %d: magRMS=%.2f maskAvg=%.4f maskMin=%.4f maskMax=%.4f energyRetained=%.4f",
			f, math.Sqrt(totalEnergy/float64(halfFFT)), maskAvg, maskMin, maskMax, energyRatio)

		outMask.Close()
		oH1.Close()
		oC1.Close()
		oH2.Close()
		oC2.Close()
		ex.Close()
		inMag.Close()
		inH1.Close()
		inC1.Close()
		inH2.Close()
		inC2.Close()
	}
	_ = fmt.Sprintf("")
	_ = cmplx.Abs(0)
}
