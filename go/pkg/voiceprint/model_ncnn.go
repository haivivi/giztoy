package voiceprint

/*
#include <ncnn/c_api.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// NCNNModel implements [Model] using ncnn for speaker embedding inference.
// It loads a pre-converted ncnn model (.param + .bin) and runs inference
// on mel filterbank features extracted from PCM audio.
//
// # Model Pipeline
//
//  1. PCM16 audio → [ComputeFbank] → mel filterbank features
//  2. Fbank features → ncnn inference → speaker embedding
//
// # Thread Safety
//
// NCNNModel is safe for concurrent use. The ncnn net is loaded once and
// shared; each Extract call creates its own extractor.
//
// # Static Linking
//
// ncnn is statically linked (.a) into the Go binary. No external shared
// libraries are needed at runtime.
type NCNNModel struct {
	mu       sync.Mutex
	net      C.ncnn_net_t
	dim      int
	fbankCfg FbankConfig
	closed   bool

	inputName  *C.char
	outputName *C.char
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
		if m.inputName != nil {
			C.free(unsafe.Pointer(m.inputName))
		}
		if m.outputName != nil {
			C.free(unsafe.Pointer(m.outputName))
		}
		m.inputName = C.CString(input)
		m.outputName = C.CString(output)
	}
}

// NewNCNNModel creates a new NCNNModel by loading the given .param and .bin files.
// Returns an error if the model files cannot be loaded.
func NewNCNNModel(paramPath, binPath string, opts ...NCNNModelOption) (*NCNNModel, error) {
	m := &NCNNModel{
		dim:        512,
		fbankCfg:   DefaultFbankConfig(),
		inputName:  C.CString("in0"),
		outputName: C.CString("out0"),
	}
	for _, opt := range opts {
		opt(m)
	}

	// Create and load the ncnn network.
	m.net = C.ncnn_net_create()
	if m.net == nil {
		return nil, fmt.Errorf("voiceprint: ncnn_net_create failed")
	}

	cParam := C.CString(paramPath)
	defer C.free(unsafe.Pointer(cParam))
	if ret := C.ncnn_net_load_param(m.net, cParam); ret != 0 {
		C.ncnn_net_destroy(m.net)
		return nil, fmt.Errorf("voiceprint: ncnn_net_load_param failed: %d", ret)
	}

	cBin := C.CString(binPath)
	defer C.free(unsafe.Pointer(cBin))
	if ret := C.ncnn_net_load_model(m.net, cBin); ret != 0 {
		C.ncnn_net_destroy(m.net)
		return nil, fmt.Errorf("voiceprint: ncnn_net_load_model failed: %d", ret)
	}

	return m, nil
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

	// Step 2: Create ncnn input mat [h=numFrames, w=numMels].
	// Flatten fbank into contiguous float32 buffer.
	flatData := make([]float32, numFrames*numMels)
	for t := 0; t < numFrames; t++ {
		copy(flatData[t*numMels:], fbank[t])
	}

	inputMat := C.ncnn_mat_create_external_2d(
		C.int(numMels),    // w
		C.int(numFrames),  // h
		unsafe.Pointer(&flatData[0]),
		nil, // default allocator
	)
	defer C.ncnn_mat_destroy(inputMat)

	// Step 3: Run inference.
	ex := C.ncnn_extractor_create(net)
	defer C.ncnn_extractor_destroy(ex)

	if ret := C.ncnn_extractor_input(ex, m.inputName, inputMat); ret != 0 {
		return nil, fmt.Errorf("voiceprint: ncnn_extractor_input failed: %d", ret)
	}

	var outputMat C.ncnn_mat_t
	if ret := C.ncnn_extractor_extract(ex, m.outputName, &outputMat); ret != 0 {
		return nil, fmt.Errorf("voiceprint: ncnn_extractor_extract failed: %d", ret)
	}
	defer C.ncnn_mat_destroy(outputMat)

	// Step 4: Copy output embedding.
	outW := int(C.ncnn_mat_get_w(outputMat))
	outData := C.ncnn_mat_get_data(outputMat)
	if outData == nil {
		return nil, fmt.Errorf("voiceprint: ncnn output data is nil")
	}

	n := outW
	if n > m.dim {
		n = m.dim
	}
	embedding := make([]float32, m.dim)
	C.memcpy(unsafe.Pointer(&embedding[0]), outData, C.size_t(n*4))

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
		C.ncnn_net_destroy(m.net)
		m.net = nil
	}
	if m.inputName != nil {
		C.free(unsafe.Pointer(m.inputName))
		m.inputName = nil
	}
	if m.outputName != nil {
		C.free(unsafe.Pointer(m.outputName))
		m.outputName = nil
	}
	return nil
}
