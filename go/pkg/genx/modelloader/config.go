package modelloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"google.golang.org/genai"
)

// Verbose enables request body logging for debugging
var Verbose bool

type verboseTransport struct {
	base http.RoundTripper
}

func (t *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(body))

		// Pretty print JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
			log.Printf("\n=== REQUEST TO %s ===\n%s\n=== END REQUEST ===\n", req.URL, prettyJSON.String())
		} else {
			log.Printf("\n=== REQUEST TO %s ===\n%s\n=== END REQUEST ===\n", req.URL, string(body))
		}
	}
	return t.base.RoundTrip(req)
}

type ConfigFile struct {
	// New unified format
	Schema string `json:"schema,omitzero" yaml:"schema,omitzero"` // e.g., "openai/chat/v1", "doubao/seed_tts/v2"
	Type   string `json:"type,omitzero" yaml:"type,omitzero"`     // "generator", "tts", "asr", "realtime"

	// Legacy format (for backward compatibility)
	Kind string `json:"kind,omitzero" yaml:"kind,omitzero"` // "openai", "gemini"

	// Common fields
	APIKey  string `json:"api_key,omitzero" yaml:"api_key,omitzero"` // Can be env var name like "$OPENAI_API_KEY"
	BaseURL string `json:"base_url,omitzero" yaml:"base_url,omitzero"`

	// Generator specific
	Models []Entry `json:"models,omitzero" yaml:"models,omitzero"`

	// TTS specific
	AppID         string         `json:"app_id,omitzero" yaml:"app_id,omitzero"`
	Token         string         `json:"token,omitzero" yaml:"token,omitzero"`
	Cluster       string         `json:"cluster,omitzero" yaml:"cluster,omitzero"`
	Model         string         `json:"model,omitzero" yaml:"model,omitzero"` // For TTS model like "speech-02-hd"
	Voices        []VoiceEntry   `json:"voices,omitzero" yaml:"voices,omitzero"`
	DefaultParams map[string]any `json:"default_params,omitzero" yaml:"default_params,omitzero"`
}

// VoiceEntry represents a TTS voice configuration.
type VoiceEntry struct {
	Name    string `json:"name" yaml:"name"`         // Registration name, e.g., "doubao/cancan"
	VoiceID string `json:"voice_id" yaml:"voice_id"` // Actual voice ID, e.g., "zh_female_cancan"
	Desc    string `json:"desc,omitzero" yaml:"desc,omitzero"`
	Cluster string `json:"cluster,omitzero" yaml:"cluster,omitzero"` // Override cluster for this voice
}

type Entry struct {
	Name              string            `json:"name" yaml:"name"`
	Model             string            `json:"model" yaml:"model"`
	GenerateParams    *genx.ModelParams `json:"generate_params,omitzero" yaml:"generate_params,omitzero"`
	InvokeParams      *genx.ModelParams `json:"invoke_params,omitzero" yaml:"invoke_params,omitzero"`
	SupportJSONOutput bool              `json:"support_json_output,omitzero" yaml:"support_json_output,omitzero"`
	SupportToolCalls  bool              `json:"support_tool_calls,omitzero" yaml:"support_tool_calls,omitzero"`
	SupportTextOnly   bool              `json:"support_text_only,omitzero" yaml:"support_text_only,omitzero"`
	UseSystemRole     bool              `json:"use_system_role,omitzero" yaml:"use_system_role,omitzero"`
	ExtraFields       map[string]any    `json:"extra_fields,omitzero" yaml:"extra_fields,omitzero"`

	// ASR/Realtime specific fields
	Voice      string `json:"voice,omitzero" yaml:"voice,omitzero"`             // Voice ID for realtime
	ResourceID string `json:"resource_id,omitzero" yaml:"resource_id,omitzero"` // Resource ID for ASR
	Desc       string `json:"desc,omitzero" yaml:"desc,omitzero"`               // Description
}

// LoadFromDir loads model configs from dir recursively and registers generators.
// Returns the registered model names.
// Configs with missing credentials (empty API key/token after env expansion) are skipped.
func LoadFromDir(dir string) ([]string, error) {
	var names []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		cfg, err := parseConfig(path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		fileNames, err := registerConfig(*cfg)
		if err != nil {
			// Skip configs with missing credentials
			if strings.Contains(err.Error(), "is required") {
				if Verbose {
					log.Printf("skipping %s: %v", path, err)
				}
				return nil
			}
			return fmt.Errorf("register %s: %w", path, err)
		}
		names = append(names, fileNames...)
		return nil
	})

	return names, err
}

func parseConfig(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var cfg ConfigFile
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}
	return &cfg, nil
}

func registerConfig(cfg ConfigFile) ([]string, error) {
	// Expand environment variables
	cfg.APIKey = expandEnv(cfg.APIKey)
	cfg.AppID = expandEnv(cfg.AppID)
	cfg.Token = expandEnv(cfg.Token)

	// New unified format: use schema + type
	if cfg.Schema != "" {
		return registerBySchema(cfg)
	}

	// Legacy format: use kind
	switch strings.ToLower(cfg.Kind) {
	case "openai":
		return registerOpenAI(cfg)
	case "gemini":
		return registerGemini(cfg)
	default:
		return nil, fmt.Errorf("unknown kind: %s", cfg.Kind)
	}
}

func registerBySchema(cfg ConfigFile) ([]string, error) {
	// Schema format: {provider}/{subject}/{version}
	// e.g., "openai/chat/v1", "doubao/seed_tts/v2", "minimax/speech/v1"

	switch cfg.Type {
	case "generator":
		return registerGeneratorBySchema(cfg)
	case "tts":
		return registerTTSBySchema(cfg)
	case "asr":
		return registerASRBySchema(cfg)
	case "realtime":
		return registerRealtimeBySchema(cfg)
	case "segmentor":
		return registerSegmentorBySchema(cfg)
	case "labeler":
		return registerLabelerBySchema(cfg)
	case "profiler":
		return registerProfilerBySchema(cfg)
	default:
		return nil, fmt.Errorf("unknown type: %s", cfg.Type)
	}
}

func registerGeneratorBySchema(cfg ConfigFile) ([]string, error) {
	// Parse schema to determine provider
	parts := strings.Split(cfg.Schema, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid schema: %s", cfg.Schema)
	}
	provider := parts[0]

	switch provider {
	case "openai":
		return registerOpenAI(cfg)
	case "gemini":
		return registerGemini(cfg)
	default:
		return nil, fmt.Errorf("unknown generator provider: %s", provider)
	}
}

// expandEnv expands environment variables in a string.
// Supports formats: $VAR, ${VAR}, and plain values.
// If the value starts with $ but the env var is not set, returns empty string.
func expandEnv(s string) string {
	if s == "" {
		return s
	}
	// Check if it looks like an env var reference
	if strings.HasPrefix(s, "$") {
		return os.ExpandEnv(s)
	}
	return s
}

func registerOpenAI(cfg ConfigFile) ([]string, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required for openai kind")
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if Verbose {
		opts = append(opts, option.WithHTTPClient(&http.Client{
			Transport: &verboseTransport{base: http.DefaultTransport},
		}))
	}
	client := openai.NewClient(opts...)

	var names []string
	for _, m := range cfg.Models {
		if m.Name == "" || m.Model == "" {
			return nil, fmt.Errorf("model entry missing name or model")
		}
		if err := generators.Handle(m.Name, &genx.OpenAIGenerator{
			Client:            &client,
			Model:             m.Model,
			GenerateParams:    m.GenerateParams,
			InvokeParams:      m.InvokeParams,
			SupportJSONOutput: m.SupportJSONOutput,
			SupportToolCalls:  m.SupportToolCalls,
			SupportTextOnly:   m.SupportTextOnly,
			UseSystemRole:     m.UseSystemRole,
			ExtraFields:       m.ExtraFields,
		}); err != nil {
			return nil, fmt.Errorf("register generator %q: %w", m.Name, err)
		}
		names = append(names, m.Name)
	}
	return names, nil
}

func registerGemini(cfg ConfigFile) ([]string, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required for gemini kind")
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, m := range cfg.Models {
		if m.Name == "" || m.Model == "" {
			return nil, fmt.Errorf("model entry missing name or model")
		}
		if err := generators.Handle(m.Name, &genx.GeminiGenerator{
			Client:         client,
			Model:          m.Model,
			GenerateParams: m.GenerateParams,
			InvokeParams:   m.InvokeParams,
		}); err != nil {
			return nil, fmt.Errorf("register generator %q: %w", m.Name, err)
		}
		names = append(names, m.Name)
	}
	return names, nil
}
