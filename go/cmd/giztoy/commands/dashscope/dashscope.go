// Package dashscope implements the 'giztoy dashscope' subcommand tree.
// It directly calls the go/pkg/dashscope SDK.
package dashscope

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
	"github.com/haivivi/giztoy/go/pkg/cli"
	"github.com/haivivi/giztoy/go/pkg/dashscope"
)

// Cmd is the root 'dashscope' command.
var Cmd = &cobra.Command{
	Use:   "dashscope",
	Short: "DashScope API (omni multimodal chat)",
	Long: `DashScope (Aliyun Model Studio) API client.

Supported services:
  omni  Qwen-Omni-Realtime multimodal conversation (text + audio)

Configuration (dashscope.yaml in context dir):
  api_key: sk-xxx
  workspace: YOUR_WORKSPACE  # optional
  base_url: ...              # optional

Examples:
  giztoy config set dev dashscope api_key sk-xxx
  giztoy dashscope omni chat -f omni-chat.yaml -o output.pcm`,
}

// Flags shared by all dashscope subcommands.
var (
	contextName string
	outputFile  string
	inputFile   string
	outputJSON  bool
)

func init() {
	Cmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")
	Cmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	Cmd.PersistentFlags().StringVarP(&inputFile, "file", "f", "", "input request file (YAML or JSON)")
	Cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output as JSON (for piping)")

	Cmd.AddCommand(omniCmd)
}

// ServiceConfig is the per-context dashscope.yaml schema.
type ServiceConfig struct {
	APIKey    string `yaml:"api_key"`
	Workspace string `yaml:"workspace"`
	BaseURL   string `yaml:"base_url"`
}

func loadServiceConfig() (*ServiceConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	contextDir, err := cfg.ResolveContext(contextName)
	if err != nil {
		if contextName == "" {
			return nil, fmt.Errorf("no context set; use -c flag or 'giztoy config use-context <name>'")
		}
		return nil, err
	}
	svc, err := config.LoadService[ServiceConfig](contextDir, "dashscope")
	if err != nil {
		return nil, fmt.Errorf("dashscope config: %w", err)
	}
	return svc, nil
}

func createClient() (*dashscope.Client, error) {
	svc, err := loadServiceConfig()
	if err != nil {
		return nil, err
	}
	if svc.APIKey == "" {
		return nil, fmt.Errorf("dashscope api_key not configured; run: giztoy config set <context> dashscope api_key <key>")
	}

	var opts []dashscope.Option
	if svc.Workspace != "" {
		opts = append(opts, dashscope.WithWorkspace(svc.Workspace))
	}
	if svc.BaseURL != "" {
		opts = append(opts, dashscope.WithBaseURL(svc.BaseURL))
	}
	return dashscope.NewClient(svc.APIKey, opts...), nil
}

func isVerbose() bool {
	v, _ := Cmd.Root().PersistentFlags().GetBool("verbose")
	return v
}

func printVerbose(format string, args ...any) {
	cli.PrintVerbose(isVerbose(), format, args...)
}

// ---------------------------------------------------------------------------
// Omni Chat
// ---------------------------------------------------------------------------

var omniCmd = &cobra.Command{
	Use:   "omni",
	Short: "Qwen-Omni-Realtime multimodal conversation",
}

var (
	omniModel        string
	omniVoice        string
	omniAudioFile    string
	omniInstructions string
)

var omniChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive chat session with Qwen-Omni-Realtime.

Audio format: Input 16-bit PCM 16kHz mono, Output 16-bit PCM 24kHz mono

Example config file (omni-chat.yaml):
  model: qwen-omni-turbo-realtime-latest
  voice: Chelsie
  audio_file: input.pcm

Examples:
  giztoy dashscope omni chat
  giztoy dashscope omni chat -f omni-chat.yaml -o output.pcm
  giztoy dashscope omni chat --voice Cherry --audio input.pcm -o output.pcm`,
	RunE: runOmniChat,
}

func init() {
	omniChatCmd.Flags().StringVar(&omniModel, "model", "", "Model to use (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniVoice, "voice", "", "Voice for audio output (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniAudioFile, "audio", "", "Input audio file (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniInstructions, "instructions", "", "System instructions")

	omniCmd.AddCommand(omniChatCmd)
}

// OmniChatConfig is the configuration for omni chat command.
type OmniChatConfig struct {
	Model                         string               `yaml:"model" json:"model"`
	Voice                         string               `yaml:"voice" json:"voice"`
	Instructions                  string               `yaml:"instructions" json:"instructions"`
	InputAudioFormat              string               `yaml:"input_audio_format" json:"input_audio_format"`
	OutputAudioFormat             string               `yaml:"output_audio_format" json:"output_audio_format"`
	Modalities                    []string             `yaml:"modalities" json:"modalities"`
	EnableInputAudioTranscription bool                 `yaml:"enable_input_audio_transcription" json:"enable_input_audio_transcription"`
	InputAudioTranscriptionModel  string               `yaml:"input_audio_transcription_model" json:"input_audio_transcription_model"`
	TurnDetection                 *TurnDetectionConfig `yaml:"turn_detection" json:"turn_detection"`
	AudioFile                     string               `yaml:"audio_file" json:"audio_file"`
}

type TurnDetectionConfig struct {
	Type              string  `yaml:"type" json:"type"`
	Threshold         float64 `yaml:"threshold" json:"threshold"`
	PrefixPaddingMs   int     `yaml:"prefix_padding_ms" json:"prefix_padding_ms"`
	SilenceDurationMs int     `yaml:"silence_duration_ms" json:"silence_duration_ms"`
}

func defaultOmniChatConfig() *OmniChatConfig {
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

func (c *OmniChatConfig) toSessionConfig() *dashscope.SessionConfig {
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

func mergeConfig(dst, src *OmniChatConfig) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Voice != "" {
		dst.Voice = src.Voice
	}
	if src.Instructions != "" {
		dst.Instructions = src.Instructions
	}
	if src.InputAudioFormat != "" {
		dst.InputAudioFormat = src.InputAudioFormat
	}
	if src.OutputAudioFormat != "" {
		dst.OutputAudioFormat = src.OutputAudioFormat
	}
	if len(src.Modalities) > 0 {
		dst.Modalities = src.Modalities
	}
	dst.EnableInputAudioTranscription = src.EnableInputAudioTranscription
	if src.InputAudioTranscriptionModel != "" {
		dst.InputAudioTranscriptionModel = src.InputAudioTranscriptionModel
	}
	if src.TurnDetection != nil {
		if dst.TurnDetection == nil {
			dst.TurnDetection = &TurnDetectionConfig{}
		}
		if src.TurnDetection.Type != "" {
			dst.TurnDetection.Type = src.TurnDetection.Type
		}
		if src.TurnDetection.Threshold > 0 {
			dst.TurnDetection.Threshold = src.TurnDetection.Threshold
		}
		if src.TurnDetection.PrefixPaddingMs > 0 {
			dst.TurnDetection.PrefixPaddingMs = src.TurnDetection.PrefixPaddingMs
		}
		if src.TurnDetection.SilenceDurationMs > 0 {
			dst.TurnDetection.SilenceDurationMs = src.TurnDetection.SilenceDurationMs
		}
	}
	if src.AudioFile != "" {
		dst.AudioFile = src.AudioFile
	}
}

func runOmniChat(cmd *cobra.Command, args []string) error {
	// Configure slog based on verbose flag
	logLevel := slog.LevelInfo
	if isVerbose() {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	cfg := defaultOmniChatConfig()

	if inputFile != "" {
		fileCfg := &OmniChatConfig{}
		if err := cli.LoadRequest(inputFile, fileCfg); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		mergeConfig(cfg, fileCfg)
		printVerbose("Loaded config from: %s", inputFile)
	}

	if omniModel != "" {
		cfg.Model = omniModel
	}
	if omniVoice != "" {
		cfg.Voice = omniVoice
	}
	if omniAudioFile != "" {
		cfg.AudioFile = omniAudioFile
	}
	if omniInstructions != "" {
		cfg.Instructions = omniInstructions
	}

	printVerbose("Model: %s", cfg.Model)
	printVerbose("Voice: %s", cfg.Voice)

	client, err := createClient()
	if err != nil {
		return err
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(connectCtx, &dashscope.RealtimeConfig{
		Model: cfg.Model,
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	cli.PrintSuccess("Connected to Qwen-Omni-Realtime")

	var wg sync.WaitGroup
	sessionCreated := make(chan struct{})
	sessionUpdated := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleEventsWithSignals(session, sessionCreated, sessionUpdated)
	}()

	printVerbose("Waiting for session.created...")
	select {
	case <-sessionCreated:
		printVerbose("Session created")
	case <-time.After(5 * time.Second):
		cli.PrintError("Timeout waiting for session.created")
		session.Close()
		return fmt.Errorf("timeout waiting for session.created")
	}

	printVerbose("Updating session config...")
	if err := session.UpdateSession(cfg.toSessionConfig()); err != nil {
		session.Close()
		return fmt.Errorf("update session: %w", err)
	}

	select {
	case <-sessionUpdated:
		printVerbose("Session updated")
	case <-time.After(5 * time.Second):
		printVerbose("Timeout waiting for session.updated (continuing)")
	}

	// Audio file mode
	if cfg.AudioFile != "" {
		if err := sendAudioFile(session, cfg.AudioFile); err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
		session.Close()
		wg.Wait()
		return nil
	}

	// Interactive mode
	fmt.Println("\nInteractive mode (VAD enabled).")
	fmt.Println("Commands: /audio <file>, /voice <id>, /exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			if handleCommand(session, input) {
				break
			}
			continue
		}
		if err := sendAudioFile(session, input); err != nil {
			cli.PrintError("Failed to send: %v", err)
		}
	}

	session.Close()
	wg.Wait()
	return nil
}

func sendAudioFile(session *dashscope.RealtimeSession, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	cli.PrintInfo("Sending audio file: %s", filepath)

	const chunkSize = 3200
	buf := make([]byte, chunkSize)
	totalBytes := 0

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read audio: %w", err)
		}
		if err := session.AppendAudio(buf[:n]); err != nil {
			return fmt.Errorf("failed to send audio: %w", err)
		}
		totalBytes += n
		time.Sleep(100 * time.Millisecond)
	}

	cli.PrintInfo("Sent %d bytes of audio", totalBytes)
	return nil
}

func handleEventsWithSignals(session *dashscope.RealtimeSession, sessionCreated, sessionUpdated chan struct{}) {
	var audioChunks [][]byte
	var currentText strings.Builder
	var lastText string
	createdOnce := sync.Once{}
	updatedOnce := sync.Once{}

	for event, err := range session.Events() {
		if err != nil {
			cli.PrintError("Event error: %v", err)
			return
		}

		switch event.Type {
		case dashscope.EventTypeSessionCreated:
			printVerbose("Session created: %v", event.Session)
			createdOnce.Do(func() { close(sessionCreated) })

		case dashscope.EventTypeSessionUpdated:
			printVerbose("Session updated")
			updatedOnce.Do(func() { close(sessionUpdated) })

		case dashscope.EventTypeResponseCreated:
			printVerbose("Response started")
			currentText.Reset()
			lastText = ""
			audioChunks = nil

		case dashscope.EventTypeChoicesResponse:
			if event.Delta != "" {
				newPart := event.Delta
				if len(newPart) > len(lastText) {
					fmt.Print(newPart[len(lastText):])
					lastText = newPart
				}
			}
			if len(event.Audio) > 0 {
				audioChunks = append(audioChunks, event.Audio)
			}
			if event.FinishReason != "" {
				fmt.Println()
				if len(audioChunks) > 0 {
					saveAudioChunks(audioChunks)
				}
			}

		case dashscope.EventTypeResponseTextDelta:
			fmt.Print(event.Delta)
			currentText.WriteString(event.Delta)

		case dashscope.EventTypeResponseTextDone:
			if currentText.Len() > 0 {
				fmt.Println()
			}

		case dashscope.EventTypeResponseAudioDelta:
			if len(event.Audio) > 0 {
				audioChunks = append(audioChunks, event.Audio)
			} else if event.AudioBase64 != "" {
				if decoded, err := base64.StdEncoding.DecodeString(event.AudioBase64); err == nil {
					audioChunks = append(audioChunks, decoded)
				}
			}

		case dashscope.EventTypeResponseAudioDone:
			if len(audioChunks) > 0 {
				saveAudioChunks(audioChunks)
			}

		case dashscope.EventTypeResponseTranscriptDelta:
			printVerbose("[transcript] %s", event.Delta)

		case dashscope.EventTypeResponseDone:
			if event.Response != nil {
				switch event.Response.Status {
				case "failed":
					if event.Response.StatusDetail != nil && event.Response.StatusDetail.Error != nil {
						cli.PrintError("Response failed: %s", event.Response.StatusDetail.Error.Message)
					} else {
						cli.PrintError("Response failed")
					}
				case "cancelled":
					cli.PrintWarning("Response cancelled")
				case "incomplete":
					cli.PrintWarning("Response incomplete")
				}
			}
			if event.Usage != nil {
				printVerbose("Tokens - Input: %d, Output: %d", event.Usage.InputTokens, event.Usage.OutputTokens)
			}
			fmt.Println()

		case dashscope.EventTypeError:
			if event.Error != nil {
				cli.PrintError("Error: %s - %s", event.Error.Code, event.Error.Message)
			}

		case dashscope.EventTypeInputSpeechStarted:
			printVerbose("Speech started")
		case dashscope.EventTypeInputSpeechStopped:
			printVerbose("Speech stopped")
		case dashscope.EventTypeInputAudioCommitted:
			printVerbose("Audio committed")
		default:
			printVerbose("Event: %s", event.Type)
		}
	}
}

func saveAudioChunks(audioChunks [][]byte) {
	totalSize := 0
	for _, chunk := range audioChunks {
		totalSize += len(chunk)
	}
	printVerbose("Audio received: %d bytes", totalSize)

	if outputFile != "" {
		audio := make([]byte, 0, totalSize)
		for _, chunk := range audioChunks {
			audio = append(audio, chunk...)
		}
		if err := cli.OutputBytes(audio, outputFile); err != nil {
			cli.PrintError("Failed to save audio: %v", err)
		} else {
			cli.PrintSuccess("Audio saved to: %s (%d bytes)", outputFile, totalSize)
		}
	}
}

func handleCommand(session *dashscope.RealtimeSession, input string) bool {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit":
		fmt.Println("Goodbye!")
		return true
	case "/audio":
		if len(parts) < 2 {
			cli.PrintError("Usage: /audio <filepath>")
		} else if err := sendAudioFile(session, parts[1]); err != nil {
			cli.PrintError("Failed to send audio: %v", err)
		}
	case "/clear":
		if err := session.ClearInput(); err != nil {
			cli.PrintError("Failed to clear: %v", err)
		} else {
			cli.PrintInfo("Input buffer cleared")
		}
	case "/voice":
		if len(parts) < 2 {
			cli.PrintInfo("Available voices: %s, %s, %s, %s",
				dashscope.VoiceChelsie, dashscope.VoiceCherry,
				dashscope.VoiceSerena, dashscope.VoiceEthan)
		} else {
			if err := session.UpdateSession(&dashscope.SessionConfig{Voice: parts[1]}); err != nil {
				cli.PrintError("Failed to change voice: %v", err)
			} else {
				cli.PrintSuccess("Voice changed to: %s", parts[1])
			}
		}
	case "/help":
		fmt.Println("Commands: /audio <file>, /clear, /voice <id>, /exit")
	default:
		cli.PrintError("Unknown command: %s (try /help)", cmd)
	}
	return false
}
