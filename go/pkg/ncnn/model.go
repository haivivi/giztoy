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

	// ModelDenoiseDTLN1 is the first stage of the DTLN noise suppression model.
	// Input: [1, 1, 257] float32 (STFT magnitude)
	// Output: [1, 1, 257] float32 (mask)
	ModelDenoiseDTLN1 ModelID = "denoise-dtln-1"

	// ModelDenoiseDTLN2 is the second stage of the DTLN noise suppression model.
	// Input: [1, 1, 512] float32 (encoded features)
	// Output: [1, 1, 512] float32 (enhanced features)
	ModelDenoiseDTLN2 ModelID = "denoise-dtln-2"
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
func LoadModel(id ModelID) (*Net, error) {
	registryMu.RLock()
	info, ok := registry[id]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("ncnn: model %q not registered", id)
	}

	return NewNetFromMemory(info.ParamData, info.BinData)
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
