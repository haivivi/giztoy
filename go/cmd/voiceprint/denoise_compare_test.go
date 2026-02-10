package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/audio/fbank"
	"github.com/haivivi/giztoy/go/pkg/ncnn"
	ortpkg "github.com/haivivi/giztoy/go/pkg/onnx"
)

// TestDTLNStage1Compare compares DTLN Stage 1 output between ncnn (pnnx-converted)
// and ONNX Runtime (original model) frame by frame.
func TestDTLNStage1Compare(t *testing.T) {
	// Load ONNX model
	onnxPath := os.Getenv("ONNX_DTLN1_PATH")
	if onnxPath == "" {
		t.Skip("skip: set ONNX_DTLN1_PATH")
	}
	onnxData, err := os.ReadFile(onnxPath)
	if err != nil {
		t.Fatal(err)
	}

	env, err := ortpkg.NewEnv("dtln-compare")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	onnxSession, err := env.NewSession(onnxData)
	if err != nil {
		t.Fatal(err)
	}
	defer onnxSession.Close()

	// Load ncnn model
	ncnnNet, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN1)
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnNet.Close()

	// Generate test input: realistic magnitude spectrum
	const halfFFT = 257
	const stateLen = 128

	for frame := 0; frame < 5; frame++ {
		mag := make([]float32, halfFFT)
		for i := range mag {
			mag[i] = float32(math.Exp(-float64(i)*0.01)) * (5 + float32(frame))
		}

		// === ONNX: original model ===
		// input_2: [1, 1, 257], input_3: [1, 2, 128, 2] (zeros for first frame)
		onnxMagData := make([]float32, 1*1*halfFFT)
		copy(onnxMagData, mag)

		onnxMag, err := ortpkg.NewTensor([]int64{1, 1, halfFFT}, onnxMagData)
		if err != nil {
			t.Fatal(err)
		}

		// States: [1, 2, 128, 2] = 512 floats, all zero for first frame
		onnxStateData := make([]float32, 1*2*stateLen*2)
		onnxState, err := ortpkg.NewTensor([]int64{1, 2, stateLen, 2}, onnxStateData)
		if err != nil {
			t.Fatal(err)
		}

		onnxOutputs, err := onnxSession.Run(
			[]string{"input_2", "input_3"},
			[]*ortpkg.Tensor{onnxMag, onnxState},
			[]string{"activation_2", "tf_op_layer_stack_2"},
		)
		if err != nil {
			t.Fatalf("ONNX frame %d: %v", frame, err)
		}

		onnxMaskData, err := onnxOutputs[0].FloatData()
		if err != nil {
			t.Fatal(err)
		}
		// Extract the [1,1,257] → [257]
		onnxMask := onnxMaskData

		onnxMag.Close()
		onnxState.Close()
		onnxOutputs[0].Close()
		onnxOutputs[1].Close()

		// === ncnn: converted model ===
		// in0: [1, 257], in1-in4: [1, 128]
		ncnnMag, err := ncnn.NewMat2D(halfFFT, 1, mag)
		if err != nil {
			t.Fatal(err)
		}
		zeroState := make([]float32, stateLen)
		ncnnH1, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnC1, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnH2, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnC2, _ := ncnn.NewMat2D(stateLen, 1, zeroState)

		ex, _ := ncnnNet.NewExtractor()
		ex.SetInput("in0", ncnnMag)
		ex.SetInput("in1", ncnnH1)
		ex.SetInput("in2", ncnnC1)
		ex.SetInput("in3", ncnnH2)
		ex.SetInput("in4", ncnnC2)

		ncnnMaskMat, err := ex.Extract("out0")
		if err != nil {
			t.Fatalf("ncnn frame %d: %v", frame, err)
		}
		ncnnMask := ncnnMaskMat.FloatData()

		ncnnMaskMat.Close()
		ex.Close()
		ncnnMag.Close()
		ncnnH1.Close()
		ncnnC1.Close()
		ncnnH2.Close()
		ncnnC2.Close()

		// === Compare masks ===
		cosSim := cosineSimF32(onnxMask, ncnnMask)
		maxDiff := maxAbsDiff(onnxMask, ncnnMask)

		t.Logf("frame %d: mask cosine=%.6f maxDiff=%.6f onnx[0..4]=[%.4f,%.4f,%.4f,%.4f,%.4f] ncnn[0..4]=[%.4f,%.4f,%.4f,%.4f,%.4f]",
			frame, cosSim, maxDiff,
			onnxMask[0], onnxMask[1], onnxMask[2], onnxMask[3], onnxMask[4],
			ncnnMask[0], ncnnMask[1], ncnnMask[2], ncnnMask[3], ncnnMask[4])

		if cosSim < 0.99 {
			t.Errorf("frame %d: mask cosine too low: %.6f", frame, cosSim)
		}
	}
}

// TestDTLNStage2Compare compares DTLN Stage 2 output between ncnn and ONNX.
func TestDTLNStage2Compare(t *testing.T) {
	onnxPath := os.Getenv("ONNX_DTLN2_PATH")
	if onnxPath == "" {
		t.Skip("skip: set ONNX_DTLN2_PATH")
	}
	onnxData, err := os.ReadFile(onnxPath)
	if err != nil {
		t.Fatal(err)
	}

	env, err := ortpkg.NewEnv("dtln-compare")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	onnxSession, err := env.NewSession(onnxData)
	if err != nil {
		t.Fatal(err)
	}
	defer onnxSession.Close()

	ncnnNet, err := ncnn.LoadModel(ncnn.ModelDenoiseDTLN2)
	if err != nil {
		t.Fatal(err)
	}
	defer ncnnNet.Close()

	const frameSize = 512
	const stateLen = 128

	for frame := 0; frame < 5; frame++ {
		// Simulated time-domain frame (after DTLN1 mask + ISTFT)
		timeFrame := make([]float32, frameSize)
		for i := range timeFrame {
			timeFrame[i] = float32(math.Sin(2*math.Pi*440*float64(i)/16000)) * 0.1 * float32(frame+1)
		}

		// === ONNX: input_4 [1, 1, 512], input_5 [1, 2, 128, 2] ===
		onnxFrameData := make([]float32, 1*1*frameSize)
		copy(onnxFrameData, timeFrame)
		onnxFrame, err := ortpkg.NewTensor([]int64{1, 1, int64(frameSize)}, onnxFrameData)
		if err != nil {
			t.Fatal(err)
		}

		onnxStateData := make([]float32, 1*2*stateLen*2)
		onnxState, err := ortpkg.NewTensor([]int64{1, 2, int64(stateLen), 2}, onnxStateData)
		if err != nil {
			t.Fatal(err)
		}

		onnxOutputs, err := onnxSession.Run(
			[]string{"input_4", "input_5"},
			[]*ortpkg.Tensor{onnxFrame, onnxState},
			[]string{"conv1d_3", "tf_op_layer_stack_5"},
		)
		if err != nil {
			t.Fatalf("ONNX frame %d: %v", frame, err)
		}
		onnxOut, err := onnxOutputs[0].FloatData()
		if err != nil {
			t.Fatal(err)
		}

		onnxFrame.Close()
		onnxState.Close()
		onnxOutputs[0].Close()
		onnxOutputs[1].Close()

		// === ncnn ===
		ncnnFrame, _ := ncnn.NewMat2D(frameSize, 1, timeFrame)
		zeroState := make([]float32, stateLen)
		ncnnH1, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnC1, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnH2, _ := ncnn.NewMat2D(stateLen, 1, zeroState)
		ncnnC2, _ := ncnn.NewMat2D(stateLen, 1, zeroState)

		ex, _ := ncnnNet.NewExtractor()
		ex.SetInput("in0", ncnnFrame)
		ex.SetInput("in1", ncnnH1)
		ex.SetInput("in2", ncnnC1)
		ex.SetInput("in3", ncnnH2)
		ex.SetInput("in4", ncnnC2)

		ncnnOutMat, err := ex.Extract("out0")
		if err != nil {
			t.Fatalf("ncnn frame %d: %v", frame, err)
		}
		ncnnOut := ncnnOutMat.FloatData()

		ncnnOutMat.Close()
		ex.Close()
		ncnnFrame.Close()
		ncnnH1.Close()
		ncnnC1.Close()
		ncnnH2.Close()
		ncnnC2.Close()

		// === Compare ===
		cosSim := cosineSimF32(onnxOut, ncnnOut)
		maxDiff := maxAbsDiff(onnxOut, ncnnOut)

		onnxRMS := rmsF32(onnxOut)
		ncnnRMS := rmsF32(ncnnOut)

		t.Logf("frame %d: stage2 cosine=%.6f maxDiff=%.6f onnxRMS=%.6f ncnnRMS=%.6f ratio=%.4f",
			frame, cosSim, maxDiff, onnxRMS, ncnnRMS, ncnnRMS/onnxRMS)

		if cosSim < 0.99 {
			t.Errorf("frame %d: stage2 cosine too low: %.6f", frame, cosSim)
		}
	}
}

// TestDTLNEndToEndCompare runs the full DTLN pipeline through both ONNX (original)
// and Go (ncnn + STFT/ISTFT), comparing the final output.
func TestDTLNEndToEndCompare(t *testing.T) {
	oggPath := os.Getenv("VOICEPRINT_TEST_OGG")
	if oggPath == "" {
		t.Skip("skip: set VOICEPRINT_TEST_OGG")
	}
	onnx1Path := os.Getenv("ONNX_DTLN1_PATH")
	onnx2Path := os.Getenv("ONNX_DTLN2_PATH")
	if onnx1Path == "" || onnx2Path == "" {
		t.Skip("skip: set ONNX_DTLN1_PATH and ONNX_DTLN2_PATH")
	}

	// Decode audio
	pcm16k, err := decodeOGGTo16kMono(oggPath)
	if err != nil {
		t.Fatal(err)
	}
	numSamples := len(pcm16k) / 2
	t.Logf("decoded: %d samples (%.1fs)", numSamples, float64(numSamples)/16000)

	// === Go/ncnn pipeline ===
	denoiser, err := newDTLNDenoiser()
	if err != nil {
		t.Fatal(err)
	}
	defer denoiser.Close()

	goDenoised, err := denoiser.Denoise(pcm16k)
	if err != nil {
		t.Fatal(err)
	}

	goRMS := pcmRMSBytes(goDenoised)
	inRMS := pcmRMSBytes(pcm16k)
	t.Logf("Go pipeline: inRMS=%.1f outRMS=%.1f ratio=%.4f", inRMS, goRMS, goRMS/inRMS)

	// === ONNX pipeline (frame-by-frame, same STFT/ISTFT as Go) ===
	env, err := ortpkg.NewEnv("dtln-e2e")
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()

	onnx1Data, _ := os.ReadFile(onnx1Path)
	onnx2Data, _ := os.ReadFile(onnx2Path)
	onnx1Session, err := env.NewSession(onnx1Data)
	if err != nil {
		t.Fatal(err)
	}
	defer onnx1Session.Close()
	onnx2Session, err := env.NewSession(onnx2Data)
	if err != nil {
		t.Fatal(err)
	}
	defer onnx2Session.Close()

	onnxDenoised := runONNXDTLN(t, pcm16k, onnx1Session, onnx2Session)
	onnxRMS := pcmRMSBytes(onnxDenoised)
	t.Logf("ONNX pipeline: inRMS=%.1f outRMS=%.1f ratio=%.4f", inRMS, onnxRMS, onnxRMS/inRMS)

	// === Compare Go vs ONNX output ===
	corr := pcmCorrelation(goDenoised, onnxDenoised)
	t.Logf("Go vs ONNX correlation: %.6f", corr)

	if corr < 0.9 {
		t.Errorf("Go vs ONNX output poorly correlated: %.4f (expected > 0.9)", corr)
	}
	if math.Abs(goRMS/onnxRMS-1) > 1.0 {
		t.Errorf("RMS ratio diverged: Go=%.1f ONNX=%.1f ratio=%.4f", goRMS, onnxRMS, goRMS/onnxRMS)
	}
}

// runONNXDTLN runs the full DTLN pipeline using ONNX Runtime with the same
// STFT/ISTFT logic as the Go pipeline, for apples-to-apples comparison.
func runONNXDTLN(t *testing.T, pcm []byte, stage1, stage2 *ortpkg.Session) []byte {
	t.Helper()
	const (
		fftSize = 512
		hopSize = 128
		halfFFT = fftSize/2 + 1
	)

	numSamples := len(pcm) / 2
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		samples[i] = float32(s) / 32768.0
	}

	hann := make([]float64, fftSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(i)/float64(fftSize)))
	}

	output := make([]float64, numSamples)
	winSum := make([]float64, numSamples)
	numFrames := (numSamples - fftSize) / hopSize + 1

	// ONNX states: [1, 2, 128, 2]
	state1Data := make([]float32, 1*2*128*2)
	state2Data := make([]float32, 1*2*128*2)

	for fr := 0; fr < numFrames; fr++ {
		start := fr * hopSize

		// STFT (same as Go pipeline)
		re := make([]float64, fftSize)
		im := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			re[i] = float64(samples[start+i]) * hann[i]
		}
		fbank.FFT(re, im)

		mag := make([]float32, halfFFT)
		phase := make([]complex128, halfFFT)
		for i := 0; i < halfFFT; i++ {
			c := complex(re[i], im[i])
			mag[i] = float32(cmplx.Abs(c))
			phase[i] = c
		}

		// Stage 1: ONNX
		magInput := make([]float32, 1*1*halfFFT)
		copy(magInput, mag)
		onnxMag, _ := ortpkg.NewTensor([]int64{1, 1, halfFFT}, magInput)
		onnxS1, _ := ortpkg.NewTensor([]int64{1, 2, 128, 2}, state1Data)
		s1Out, err := stage1.Run(
			[]string{"input_2", "input_3"},
			[]*ortpkg.Tensor{onnxMag, onnxS1},
			[]string{"activation_2", "tf_op_layer_stack_2"},
		)
		if err != nil {
			t.Fatalf("ONNX Stage1 frame %d: %v", fr, err)
		}
		mask, _ := s1Out[0].FloatData()
		newState1, _ := s1Out[1].FloatData()
		copy(state1Data, newState1)
		onnxMag.Close()
		onnxS1.Close()
		s1Out[0].Close()
		s1Out[1].Close()

		// Apply mask + ISTFT (same as Go pipeline)
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
		fbank.IFFT(ifftR, ifftI)

		timeFrame := make([]float32, fftSize)
		for i := 0; i < fftSize; i++ {
			timeFrame[i] = float32(ifftR[i])
		}

		// Stage 2: ONNX
		frameInput := make([]float32, 1*1*fftSize)
		copy(frameInput, timeFrame)
		onnxFrame, _ := ortpkg.NewTensor([]int64{1, 1, int64(fftSize)}, frameInput)
		onnxS2, _ := ortpkg.NewTensor([]int64{1, 2, 128, 2}, state2Data)
		s2Out, err := stage2.Run(
			[]string{"input_4", "input_5"},
			[]*ortpkg.Tensor{onnxFrame, onnxS2},
			[]string{"conv1d_3", "tf_op_layer_stack_5"},
		)
		if err != nil {
			t.Fatalf("ONNX Stage2 frame %d: %v", fr, err)
		}
		enhanced, _ := s2Out[0].FloatData()
		newState2, _ := s2Out[1].FloatData()
		copy(state2Data, newState2)
		onnxFrame.Close()
		onnxS2.Close()
		s2Out[0].Close()
		s2Out[1].Close()

		// Overlap-add
		for i := 0; i < fftSize; i++ {
			idx := start + i
			if idx < numSamples {
				output[idx] += float64(enhanced[i]) * hann[i]
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

	// Convert to int16 PCM
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

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func cosineSimF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		normA += x * x
		normB += y * y
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func maxAbsDiff(a, b []float32) float64 {
	maxD := float64(0)
	for i := range a {
		if i >= len(b) {
			break
		}
		d := math.Abs(float64(a[i]) - float64(b[i]))
		if d > maxD {
			maxD = d
		}
	}
	return maxD
}

func rmsF32(v []float32) float64 {
	sum := 0.0
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum / float64(len(v)))
}

func pcmRMSBytes(pcm []byte) float64 {
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

func pcmCorrelation(a, b []byte) float64 {
	n := len(a) / 2
	if n > len(b)/2 {
		n = len(b) / 2
	}
	var num, denA, denB float64
	skip := 512 // skip edges
	for i := skip; i < n-skip; i++ {
		x := float64(int16(a[i*2]) | int16(a[i*2+1])<<8)
		y := float64(int16(b[i*2]) | int16(b[i*2+1])<<8)
		num += x * y
		denA += x * x
		denB += y * y
	}
	if denA == 0 || denB == 0 {
		return 0
	}
	return num / (math.Sqrt(denA) * math.Sqrt(denB))
}

var _ = fmt.Sprintf // avoid unused import
