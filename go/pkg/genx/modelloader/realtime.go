package modelloader

import (
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
)

func registerRealtimeBySchema(cfg ConfigFile) ([]string, error) {
	// Parse schema to determine provider
	parts := strings.Split(cfg.Schema, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid schema: %s", cfg.Schema)
	}
	provider := parts[0]

	switch provider {
	case "doubao":
		return registerDoubaoRealtime(cfg)
	default:
		return nil, fmt.Errorf("unknown realtime provider: %s", provider)
	}
}

func registerDoubaoRealtime(cfg ConfigFile) ([]string, error) {
	if cfg.AppID == "" {
		return nil, fmt.Errorf("app_id is required for doubao realtime")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required for doubao realtime")
	}

	// Create Doubao client
	client := doubaospeech.NewClient(cfg.AppID, doubaospeech.WithBearerToken(cfg.Token))

	// Extract default params
	var defaultOpts []transformers.DoubaoRealtimeOption
	if cfg.DefaultParams != nil {
		if sampleRate, ok := cfg.DefaultParams["sample_rate"].(float64); ok {
			defaultOpts = append(defaultOpts, transformers.WithDoubaoRealtimeSampleRate(int(sampleRate)))
		}
		if format, ok := cfg.DefaultParams["format"].(string); ok {
			defaultOpts = append(defaultOpts, transformers.WithDoubaoRealtimeFormat(format))
		}
		if model, ok := cfg.DefaultParams["model"].(string); ok {
			defaultOpts = append(defaultOpts, transformers.WithDoubaoRealtimeModel(model))
		}
		if vadWindow, ok := cfg.DefaultParams["vad_window_ms"].(float64); ok {
			defaultOpts = append(defaultOpts, transformers.WithDoubaoRealtimeVADWindow(int(vadWindow)))
		}
	}

	var names []string

	// Register realtime models from Models field
	// Each model has a name and voice
	for _, m := range cfg.Models {
		if m.Name == "" {
			return nil, fmt.Errorf("realtime model entry missing name")
		}

		// Build options for this model
		opts := make([]transformers.DoubaoRealtimeOption, len(defaultOpts))
		copy(opts, defaultOpts)

		// Use Voice field for speaker
		if m.Voice != "" {
			opts = append(opts, transformers.WithDoubaoRealtimeSpeaker(m.Voice))
		}

		// Create realtime transformer
		rt := transformers.NewDoubaoRealtime(client, opts...)
		transformers.Handle(m.Name, rt)
		names = append(names, m.Name)
	}

	return names, nil
}
