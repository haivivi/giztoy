package modelloader

import (
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
)

func registerASRBySchema(cfg ConfigFile) ([]string, error) {
	// Parse schema to determine provider
	parts := strings.Split(cfg.Schema, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid schema: %s", cfg.Schema)
	}
	provider := parts[0]

	switch provider {
	case "doubao":
		return registerDoubaoASR(cfg)
	default:
		return nil, fmt.Errorf("unknown ASR provider: %s", provider)
	}
}

func registerDoubaoASR(cfg ConfigFile) ([]string, error) {
	if cfg.AppID == "" {
		return nil, fmt.Errorf("app_id is required for doubao ASR")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required for doubao ASR")
	}

	// Create Doubao client
	client := doubaospeech.NewClient(cfg.AppID, doubaospeech.WithBearerToken(cfg.Token))

	// Extract default params
	var opts []transformers.DoubaoASRSAUCOption
	if cfg.DefaultParams != nil {
		if format, ok := cfg.DefaultParams["format"].(string); ok {
			opts = append(opts, transformers.WithDoubaoASRSAUCFormat(format))
		}
		if sampleRate, ok := cfg.DefaultParams["sample_rate"].(float64); ok {
			opts = append(opts, transformers.WithDoubaoASRSAUCSampleRate(int(sampleRate)))
		}
		if bits, ok := cfg.DefaultParams["bits"].(float64); ok {
			opts = append(opts, transformers.WithDoubaoASRSAUCBits(int(bits)))
		}
		if channel, ok := cfg.DefaultParams["channel"].(float64); ok {
			opts = append(opts, transformers.WithDoubaoASRSAUCChannels(int(channel)))
		}
		if language, ok := cfg.DefaultParams["language"].(string); ok {
			opts = append(opts, transformers.WithDoubaoASRSAUCLanguage(language))
		}
	}

	var names []string

	// Register ASR models from Models field (reusing Entry struct)
	for _, m := range cfg.Models {
		if m.Name == "" {
			return nil, fmt.Errorf("asr model entry missing name")
		}

		// Create ASR transformer with the resource options
		asr := transformers.NewDoubaoASRSAUC(client, opts...)
		// Register to both ASRMux and DefaultMux for compatibility
		transformers.HandleASR(m.Name, asr)
		transformers.Handle(m.Name, asr)
		names = append(names, m.Name)
	}

	return names, nil
}
