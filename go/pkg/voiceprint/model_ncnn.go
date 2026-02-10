package voiceprint

import (
	"fmt"
	"os"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/ncnn"
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
type NCNNModel struct {
	mu       sync.Mutex
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

// Extract implements [Model]. It converts PCM16 audio to fbank features
// and runs ncnn inference to produce a speaker embedding.
func (m *NCNNModel) Extract(audio []byte) ([]float32, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("voiceprint: model is closed")
	}
	net := m.net
	m.mu.Unlock()

	// Step 1: Compute fbank features.
	fbank := ComputeFbank(audio, m.fbankCfg)
	if fbank == nil || len(fbank) == 0 {
		return nil, fmt.Errorf("voiceprint: audio too short for fbank extraction")
	}

	numFrames := len(fbank)
	numMels := m.fbankCfg.NumMels

	// Step 2: Flatten fbank into [h=numFrames, w=numMels] tensor.
	flatData := make([]float32, numFrames*numMels)
	for t := 0; t < numFrames; t++ {
		copy(flatData[t*numMels:], fbank[t])
	}

	// Step 3: Run inference via ncnn package.
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

	// Step 4: Copy output embedding.
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
