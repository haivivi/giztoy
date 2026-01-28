package commands

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/cli"
	"github.com/haivivi/giztoy/go/pkg/dashscope"
)

// OmniChatConfig is the configuration for omni chat command.
// This can be loaded from a YAML or JSON file using -f flag.
type OmniChatConfig struct {
	// Model is the model ID to use.
	// Default: qwen-omni-turbo-realtime-latest
	Model string `yaml:"model" json:"model"`

	// Voice is the voice ID for TTS output.
	// Options: Chelsie, Cherry, Serena, Ethan
	Voice string `yaml:"voice" json:"voice"`

	// Instructions is the system prompt.
	Instructions string `yaml:"instructions" json:"instructions"`

	// InputAudioFormat specifies the input audio format.
	// Default: pcm16 (16-bit PCM, 16kHz mono)
	InputAudioFormat string `yaml:"input_audio_format" json:"input_audio_format"`

	// OutputAudioFormat specifies the output audio format.
	// Default: pcm16 (16-bit PCM, 24kHz mono)
	OutputAudioFormat string `yaml:"output_audio_format" json:"output_audio_format"`

	// Modalities specifies the output modalities.
	// Default: ["text", "audio"]
	Modalities []string `yaml:"modalities" json:"modalities"`

	// EnableInputAudioTranscription enables transcription of input audio.
	EnableInputAudioTranscription bool `yaml:"enable_input_audio_transcription" json:"enable_input_audio_transcription"`

	// InputAudioTranscriptionModel specifies the model for input transcription.
	InputAudioTranscriptionModel string `yaml:"input_audio_transcription_model" json:"input_audio_transcription_model"`

	// TurnDetection configures voice activity detection.
	TurnDetection *TurnDetectionConfig `yaml:"turn_detection" json:"turn_detection"`

	// AudioFile is the input audio file path (for non-interactive mode).
	AudioFile string `yaml:"audio_file" json:"audio_file"`
}

// TurnDetectionConfig configures VAD settings.
type TurnDetectionConfig struct {
	// Type is the VAD mode: "server_vad" or "disabled".
	// Default: server_vad
	Type string `yaml:"type" json:"type"`

	// Threshold is the VAD sensitivity (0.0-1.0).
	// Default: 0.5
	Threshold float64 `yaml:"threshold" json:"threshold"`

	// PrefixPaddingMs is the padding before speech start (ms).
	// Default: 300
	PrefixPaddingMs int `yaml:"prefix_padding_ms" json:"prefix_padding_ms"`

	// SilenceDurationMs is the silence duration to detect end of speech (ms).
	// Default: 500
	SilenceDurationMs int `yaml:"silence_duration_ms" json:"silence_duration_ms"`
}

// DefaultOmniChatConfig returns default configuration.
func DefaultOmniChatConfig() *OmniChatConfig {
	return &OmniChatConfig{
		Model:                         dashscope.ModelQwenOmniTurboRealtimeLatest,
		Voice:                         dashscope.VoiceChelsie,
		InputAudioFormat:              dashscope.AudioFormatPCM16,
		OutputAudioFormat:             dashscope.AudioFormatPCM16,
		Modalities:                    []string{dashscope.ModalityText, dashscope.ModalityAudio},
		EnableInputAudioTranscription: true,
		InputAudioTranscriptionModel:  "gummy-realtime-v1",
		TurnDetection: &TurnDetectionConfig{
			Type:              dashscope.VADModeServerVAD,
			Threshold:         0.5,
			PrefixPaddingMs:   300,
			SilenceDurationMs: 800,
		},
	}
}

// ToSessionConfig converts to dashscope.SessionConfig.
func (c *OmniChatConfig) ToSessionConfig() *dashscope.SessionConfig {
	cfg := &dashscope.SessionConfig{
		Voice:                         c.Voice,
		InputAudioFormat:              c.InputAudioFormat,
		OutputAudioFormat:             c.OutputAudioFormat,
		Modalities:                    c.Modalities,
		Instructions:                  c.Instructions,
		EnableInputAudioTranscription: c.EnableInputAudioTranscription,
		InputAudioTranscriptionModel:  c.InputAudioTranscriptionModel,
	}

	if c.TurnDetection != nil {
		cfg.TurnDetection = &dashscope.TurnDetection{
			Type:              c.TurnDetection.Type,
			Threshold:         c.TurnDetection.Threshold,
			PrefixPaddingMs:   c.TurnDetection.PrefixPaddingMs,
			SilenceDurationMs: c.TurnDetection.SilenceDurationMs,
		}
	}

	return cfg
}

// loadRequest loads a request from a YAML or JSON file
func loadRequest(path string, v any) error {
	return cli.LoadRequest(path, v)
}

// outputBytes outputs binary data to a file
func outputBytes(data []byte, outputPath string) error {
	return cli.OutputBytes(data, outputPath)
}

// requireInputFile checks if input file is provided
func requireInputFile() error {
	if getInputFile() == "" {
		return fmt.Errorf("input file is required, use -f flag")
	}
	return nil
}

// createDashScopeClient creates a DashScope API client from context configuration
func createDashScopeClient(ctx *cli.Context) *dashscope.Client {
	var opts []dashscope.Option

	// Use workspace if configured
	if workspace := ctx.GetExtra("workspace"); workspace != "" {
		opts = append(opts, dashscope.WithWorkspace(workspace))
	}

	// Use custom base URL if configured
	if ctx.BaseURL != "" {
		opts = append(opts, dashscope.WithBaseURL(ctx.BaseURL))
	}

	return dashscope.NewClient(ctx.APIKey, opts...)
}
