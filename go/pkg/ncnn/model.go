package ncnn

import (
	"fmt"
	"sync"
)

// ModelID identifies a built-in ncnn model.
type ModelID string

const (
	// ModelSpeakerERes2Net is the 3D-Speaker ERes2Net base model for
	// speaker embedding extraction.
	// Input: [T, 80] float32 (mel filterbank features)
	// Output: [512] float32 (speaker embedding)
	ModelSpeakerERes2Net ModelID = "speaker-eres2net"

	// ModelVADSilero is the Silero VAD model for voice activity detection.
	// Input: [batch, sequence] float32 (audio samples)
	// Output: [batch, 1] float32 (speech probability)
	ModelVADSilero ModelID = "vad-silero"

	// ModelDenoiseNSNet2 is Microsoft's NSNet2 noise suppression model.
	// Operates frame-by-frame on log-power spectrum features.
	// Input:  in0 [1, 161] (log-power spectrum), in1 [1, 400] (GRU1 state), in2 [1, 400] (GRU2 state)
	// Output: out0 [1, 161] (frequency gain mask), out1 [1, 400] (GRU1 new state), out2 [1, 400] (GRU2 new state)
	ModelDenoiseNSNet2 ModelID = "denoise-nsnet2"
)

// ModelInfo describes a registered model.
type ModelInfo struct {
	ID        ModelID
	ParamData []byte // .param file content
	BinData   []byte // .bin file content
}

var (
	registryMu sync.RWMutex
	registry   = make(map[ModelID]*ModelInfo)
)

// RegisterModel registers a model with the given ID and data.
// This is typically called from init() in packages that embed model files.
// Registering the same ID twice replaces the previous registration.
func RegisterModel(id ModelID, paramData, binData []byte) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[id] = &ModelInfo{
		ID:        id,
		ParamData: paramData,
		BinData:   binData,
	}
}

// LoadModel loads a built-in model by ID, returning a ready-to-use Net.
// The model must have been previously registered via [RegisterModel].
// FP16 is disabled by default for numerical safety. Use [Net.SetOptFP16]
// to re-enable if the model is known to be FP16-safe.
func LoadModel(id ModelID) (*Net, error) {
	registryMu.RLock()
	info, ok := registry[id]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("ncnn: model %q not registered", id)
	}

	// Disable FP16 by default — some models (Silero VAD) produce
	// intermediate values >65504 which overflow in FP16.
	opt := NewOption()
	if opt == nil {
		return nil, fmt.Errorf("ncnn: option_create failed for model %q", id)
	}
	opt.SetFP16(false)
	return NewNetFromMemory(info.ParamData, info.BinData, opt)
}

// ListModels returns the IDs of all registered models.
func ListModels() []ModelID {
	registryMu.RLock()
	defer registryMu.RUnlock()
	ids := make([]ModelID, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}

// GetModelInfo returns the info for a registered model, or nil if not found.
func GetModelInfo(id ModelID) *ModelInfo {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[id]
}
