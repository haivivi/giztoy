package voiceprint

import (
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/ncnn"
)

const (
	// segFrames is the number of fbank frames per inference segment.
	// 300 frames = 3 seconds at 10ms hop. Matching the training window
	// of speaker embedding models improves stability.
	segFrames = 300

	// hopFrames is the hop between segments for averaging.
	hopFrames = 150
)

// NCNNModel implements [Model] using the ncnn inference engine for speaker
// embedding extraction. It wraps a ncnn.Net and handles the full pipeline
// from PCM audio to embedding vector.
//
// # Model Pipeline
//
//  1. PCM16 audio → [ComputeFbank] → mel filterbank features
//  2. Fbank features → ncnn inference → speaker embedding
//
// # Thread Safety
//
// NCNNModel is safe for concurrent use. The ncnn.Net is loaded once and
// shared; each Extract call creates its own ncnn.Extractor.
// Extract holds a read lock for the entire inference duration to prevent
// Close from destroying the net while inference is in progress.
type NCNNModel struct {
	mu       sync.RWMutex
	net      *ncnn.Net
	dim      int
	fbankCfg FbankConfig
	closed   bool

	inputName  string
	outputName string
}

// NCNNModelOption configures an NCNNModel.
type NCNNModelOption func(*NCNNModel)

// WithNCNNFbankConfig sets the filterbank configuration.
func WithNCNNFbankConfig(cfg FbankConfig) NCNNModelOption {
	return func(m *NCNNModel) {
		m.fbankCfg = cfg
	}
}

// WithNCNNEmbeddingDim overrides the expected embedding dimension.
// Default: 512 (for 3D-Speaker ERes2Net base model).
func WithNCNNEmbeddingDim(dim int) NCNNModelOption {
	return func(m *NCNNModel) {
		if dim > 0 {
			m.dim = dim
		}
	}
}

// WithNCNNBlobNames sets the input and output blob names.
// Default: "in0" and "out0" (PNNX-converted models).
func WithNCNNBlobNames(input, output string) NCNNModelOption {
	return func(m *NCNNModel) {
		m.inputName = input
		m.outputName = output
	}
}

// NewNCNNModel creates a new NCNNModel by loading .param and .bin from files.
// For embedding the model in the binary, use [NewNCNNModelFromMemory] instead.
//
// Note: FP16 is disabled by loading the model data from memory internally.
// This avoids the ncnn limitation where options must be set before loading.
func NewNCNNModel(paramPath, binPath string, opts ...NCNNModelOption) (*NCNNModel, error) {
	m := newNCNNModelDefaults(opts)

	// Read files and load via memory path to ensure FP16 is disabled before load.
	paramData, err := os.ReadFile(paramPath)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: read param: %w", err)
	}
	binData, err := os.ReadFile(binPath)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: read bin: %w", err)
	}

	opt := ncnn.NewOption()
	opt.SetFP16(false)
	net, err := ncnn.NewNetFromMemory(paramData, binData, opt)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: %w", err)
	}
	m.net = net
	return m, nil
}

// NewNCNNModelFromMemory creates a new NCNNModel from in-memory .param and .bin data.
// This is the preferred constructor when the model is embedded in the binary
// via go:embed, producing a single-file deployment with zero external dependencies.
func NewNCNNModelFromMemory(paramData, binData []byte, opts ...NCNNModelOption) (*NCNNModel, error) {
	if len(paramData) == 0 {
		return nil, fmt.Errorf("voiceprint: empty param data")
	}
	if len(binData) == 0 {
		return nil, fmt.Errorf("voiceprint: empty bin data")
	}

	m := newNCNNModelDefaults(opts)

	opt := ncnn.NewOption()
	opt.SetFP16(false)
	net, err := ncnn.NewNetFromMemory(paramData, binData, opt)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: %w", err)
	}
	m.net = net
	return m, nil
}

// NewNCNNModelFromNet creates a new NCNNModel from a pre-loaded ncnn.Net.
// Use this when loading models via [ncnn.LoadModel] from the model registry.
func NewNCNNModelFromNet(net *ncnn.Net, opts ...NCNNModelOption) *NCNNModel {
	m := newNCNNModelDefaults(opts)
	m.net = net
	return m
}

func newNCNNModelDefaults(opts []NCNNModelOption) *NCNNModel {
	m := &NCNNModel{
		dim:        512,
		fbankCfg:   DefaultFbankConfig(),
		inputName:  "in0",
		outputName: "out0",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Extract implements [Model]. It converts PCM16 audio to a normalized
// speaker embedding using the following pipeline:
//
//  1. PCM → fbank features (with Kaldi-compatible parameters)
//  2. CMVN normalization (removes channel/environment effects)
//  3. Segment-based inference (300-frame windows with 150-frame hop)
//  4. Average segment embeddings + L2 normalize
//
// The returned embedding is an L2-normalized unit vector suitable for
// cosine similarity comparison via dot product.
func (m *NCNNModel) Extract(audio []byte) ([]float32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, fmt.Errorf("voiceprint: model is closed")
	}
	net := m.net

	// Step 1: Compute fbank features.
	features := ComputeFbank(audio, m.fbankCfg)
	if features == nil || len(features) == 0 {
		return nil, fmt.Errorf("voiceprint: audio too short for fbank extraction")
	}

	// Step 2: CMVN — subtract mean, divide by std per mel bin.
	cmvn(features)

	// Step 3: Segment-based extraction with averaging.
	numFrames := len(features)
	if numFrames <= segFrames {
		// Short audio: single inference.
		emb, err := m.extractSegment(net, features)
		if err != nil {
			return nil, err
		}
		l2Normalize(emb)
		return emb, nil
	}

	// Long audio: sliding window, average all segment embeddings.
	var embeddings [][]float32
	var lastStart int
	for start := 0; start+segFrames <= numFrames; start += hopFrames {
		emb, err := m.extractSegment(net, features[start:start+segFrames])
		if err != nil {
			continue
		}
		l2Normalize(emb)
		embeddings = append(embeddings, emb)
		lastStart = start
	}
	// Ensure the last segment covers the end of the audio.
	if tail := numFrames - segFrames; tail > lastStart {
		emb, err := m.extractSegment(net, features[tail:])
		if err == nil {
			l2Normalize(emb)
			embeddings = append(embeddings, emb)
		}
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("voiceprint: all segments failed")
	}

	// Average all segment embeddings.
	avg := make([]float32, m.dim)
	for _, emb := range embeddings {
		for i, v := range emb {
			avg[i] += v
		}
	}
	n := float32(len(embeddings))
	for i := range avg {
		avg[i] /= n
	}
	l2Normalize(avg)
	return avg, nil
}

// extractSegment runs ncnn inference on a single fbank segment.
func (m *NCNNModel) extractSegment(net *ncnn.Net, features [][]float32) ([]float32, error) {
	numFrames := len(features)
	numMels := len(features[0])

	flatData := make([]float32, numFrames*numMels)
	for t := 0; t < numFrames; t++ {
		copy(flatData[t*numMels:], features[t])
	}

	input, err := ncnn.NewMat2D(numMels, numFrames, flatData)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: create input mat: %w", err)
	}
	defer input.Close()

	ex, err := net.NewExtractor()
	if err != nil {
		return nil, fmt.Errorf("voiceprint: create extractor: %w", err)
	}
	defer ex.Close()

	if err := ex.SetInput(m.inputName, input); err != nil {
		return nil, fmt.Errorf("voiceprint: %w", err)
	}

	output, err := ex.Extract(m.outputName)
	if err != nil {
		return nil, fmt.Errorf("voiceprint: %w", err)
	}
	defer output.Close()

	data := output.FloatData()
	if data == nil {
		return nil, fmt.Errorf("voiceprint: ncnn output data is nil")
	}

	n := len(data)
	if n > m.dim {
		n = m.dim
	}
	embedding := make([]float32, m.dim)
	copy(embedding, data[:n])
	return embedding, nil
}

// cmvn applies Cepstral Mean and Variance Normalization in-place.
// For each mel dimension, subtracts the mean and divides by the standard
// deviation across all frames. This removes channel and environment effects.
func cmvn(features [][]float32) {
	if len(features) == 0 {
		return
	}
	numMels := len(features[0])
	T := float64(len(features))

	for m := 0; m < numMels; m++ {
		var sum float64
		for _, f := range features {
			sum += float64(f[m])
		}
		mean := sum / T

		var varSum float64
		for _, f := range features {
			d := float64(f[m]) - mean
			varSum += d * d
		}
		std := math.Sqrt(varSum / T)
		if std < 1e-10 {
			std = 1e-10
		}

		for _, f := range features {
			f[m] = float32((float64(f[m]) - mean) / std)
		}
	}
}

// l2Normalize normalizes a vector to unit length in-place.
func l2Normalize(v []float32) {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		scale := float32(1.0 / norm)
		for i := range v {
			v[i] *= scale
		}
	}
}

// Dimension implements [Model].
func (m *NCNNModel) Dimension() int {
	return m.dim
}

// Close implements [Model].
func (m *NCNNModel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	if m.net != nil {
		m.net.Close()
		m.net = nil
	}
	return nil
}
