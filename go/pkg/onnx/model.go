package onnx

import (
	"fmt"
	"sync"
)

// ModelID identifies a built-in ONNX model.
type ModelID string

const (
	// ModelSpeakerERes2Net is the 3D-Speaker ERes2Net base model.
	// Input "x": [1, T, 80] float32 (mel filterbank features)
	// Output "embedding": [1, 512] float32 (speaker embedding)
	ModelSpeakerERes2Net ModelID = "speaker-eres2net"

	// ModelVADSilero is the Silero VAD v6 model for voice activity detection.
	// Input "input": [batch, sequence] float32 (audio samples, e.g. 512 @ 16kHz)
	// Input "state": [2, batch, 128] float32 (LSTM state)
	// Input "sr": int64 scalar (sample rate, must be 16000)
	// Output "output": [batch, 1] float32 (speech probability)
	// Output "stateN": [2, batch, 128] float32 (updated LSTM state)
	ModelVADSilero ModelID = "vad-silero"

	// ModelDenoiseNSNet2 is Microsoft's NSNet2 noise suppression model.
	// Input "input": [batch, frames, 161] float32 (log-power spectrum)
	// Output "output": [batch, frames, 161] float32 (frequency gain mask)
	ModelDenoiseNSNet2 ModelID = "denoise-nsnet2"
)

// ModelInfo describes a registered ONNX model.
type ModelInfo struct {
	ID   ModelID
	Data []byte // .onnx file content
}

var (
	registryMu sync.RWMutex
	registry   = make(map[ModelID]*ModelInfo)
)

// RegisterModel registers an ONNX model with the given ID and data.
// Typically called from init() in model_embed.go.
func RegisterModel(id ModelID, data []byte) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[id] = &ModelInfo{ID: id, Data: data}
}

// LoadModel loads a registered ONNX model into a Session.
// The env must have been created with [NewEnv].
func LoadModel(env *Env, id ModelID) (*Session, error) {
	registryMu.RLock()
	info, ok := registry[id]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("onnx: model %q not registered", id)
	}
	return env.NewSession(info.Data)
}

// GetModelData returns the raw ONNX model data for a registered model.
func GetModelData(id ModelID) ([]byte, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	info, ok := registry[id]
	if !ok {
		return nil, false
	}
	return info.Data, true
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
