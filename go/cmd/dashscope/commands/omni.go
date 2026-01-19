package commands

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
	"github.com/haivivi/giztoy/pkg/dashscope"
)

var omniCmd = &cobra.Command{
	Use:   "omni",
	Short: "Qwen-Omni-Realtime multimodal conversation",
	Long: `Qwen-Omni-Realtime multimodal conversation service.

Supports real-time audio conversations with Qwen-Omni model.`,
}

var omniChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive chat session with Qwen-Omni-Realtime.

This is an audio-first API. Use -f to specify a config file, or use flags
to override specific settings.

Audio format: Input 16-bit PCM 16kHz mono, Output 16-bit PCM 24kHz mono

Example config file (omni-chat.yaml):
  model: qwen-omni-turbo-realtime-latest
  voice: Chelsie
  input_audio_format: pcm16
  output_audio_format: pcm16
  enable_input_audio_transcription: true
  input_audio_transcription_model: gummy-realtime-v1
  turn_detection:
    type: server_vad
    threshold: 0.5
    prefix_padding_ms: 300
    silence_duration_ms: 500
  audio_file: input.pcm  # Optional: auto-send this file

Examples:
  # Interactive mode with defaults
  dashscope -c myctx omni chat

  # Use config file
  dashscope -c myctx omni chat -f omni-chat.yaml -o output.pcm

  # Override specific settings
  dashscope -c myctx omni chat --voice Cherry --audio input.pcm -o output.pcm`,
	RunE: runOmniChat,
}

var (
	// Flags can override config file settings
	omniModel        string
	omniVoice        string
	omniAudioFile    string
	omniInstructions string
)

func init() {
	omniChatCmd.Flags().StringVar(&omniModel, "model", "", "Model to use (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniVoice, "voice", "", "Voice for audio output (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniAudioFile, "audio", "", "Input audio file (overrides config file)")
	omniChatCmd.Flags().StringVar(&omniInstructions, "instructions", "", "System instructions (overrides config file)")

	omniCmd.AddCommand(omniChatCmd)
}

func runOmniChat(cmd *cobra.Command, args []string) error {
	ctx, err := getContext()
	if err != nil {
		return err
	}

	// Load config: start with defaults, merge file config, then apply flag overrides
	config := DefaultOmniChatConfig()

	// Load from file if specified
	if inputFile := getInputFile(); inputFile != "" {
		fileConfig := &OmniChatConfig{}
		if err := loadRequest(inputFile, fileConfig); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		// Merge file config into defaults
		mergeConfig(config, fileConfig)
		printVerbose("Loaded config from: %s", inputFile)
	}

	// Apply flag overrides
	if omniModel != "" {
		config.Model = omniModel
	}
	if omniVoice != "" {
		config.Voice = omniVoice
	}
	if omniAudioFile != "" {
		config.AudioFile = omniAudioFile
	}
	if omniInstructions != "" {
		config.Instructions = omniInstructions
	}

	printVerbose("Using context: %s", ctx.Name)
	printVerbose("Model: %s", config.Model)
	printVerbose("Voice: %s", config.Voice)

	// Create DashScope client
	client := createDashScopeClient(ctx)

	// Connect to realtime session
	connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(connectCtx, &dashscope.RealtimeConfig{
		Model: config.Model,
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	cli.PrintSuccess("Connected to Qwen-Omni-Realtime")

	// Start event handler goroutine first to capture session events
	var wg sync.WaitGroup
	sessionCreated := make(chan struct{})
	sessionUpdated := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleEventsWithSignals(session, sessionCreated, sessionUpdated)
	}()

	// Wait for session.created event
	printVerbose("Waiting for session.created...")
	select {
	case <-sessionCreated:
		printVerbose("Session created")
	case <-time.After(5 * time.Second):
		cli.PrintError("Timeout waiting for session.created")
		session.Close()
		return fmt.Errorf("timeout waiting for session.created")
	}

	// Configure session
	printVerbose("Updating session config...")
	if err := session.UpdateSession(config.ToSessionConfig()); err != nil {
		cli.PrintError("Failed to update session: %v", err)
		session.Close()
		return err
	}

	// Wait for session.updated event
	select {
	case <-sessionUpdated:
		printVerbose("Session updated")
	case <-time.After(5 * time.Second):
		printVerbose("Timeout waiting for session.updated (continuing anyway)")
	}

	// Audio file mode
	if config.AudioFile != "" {
		if err := sendAudioFile(session, config.AudioFile); err != nil {
			return err
		}
		// Wait a bit for response
		time.Sleep(5 * time.Second)
		session.Close()
		wg.Wait()
		return nil
	}

	// Interactive mode
	fmt.Println("\nInteractive mode (VAD enabled - speak into your microphone).")
	fmt.Println("Commands:")
	fmt.Println("  /audio <file>  - Send audio file (16-bit PCM, 16kHz)")
	fmt.Println("  /voice <id>    - Change voice")
	fmt.Println("  /exit          - End session")
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

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(session, input) {
				break
			}
			continue
		}

		// Default: treat as audio file path
		if err := sendAudioFile(session, input); err != nil {
			cli.PrintError("Failed to send: %v", err)
		}
	}

	session.Close()
	wg.Wait()
	return nil
}

// mergeConfig merges src into dst (non-zero values only)
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
	if src.EnableInputAudioTranscription {
		dst.EnableInputAudioTranscription = src.EnableInputAudioTranscription
	}
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

func sendAudioFile(session *dashscope.RealtimeSession, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	cli.PrintInfo("Sending audio file: %s", filepath)

	// Read and send in chunks (16kHz * 2 bytes * 0.1s = 3200 bytes per chunk)
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

		// Small delay to simulate real-time streaming
		time.Sleep(100 * time.Millisecond)
	}

	cli.PrintInfo("Sent %d bytes of audio", totalBytes)

	// In server_vad mode, the server will automatically detect speech end
	// and create a response. We just need to wait.

	return nil
}

func handleEventsWithSignals(session *dashscope.RealtimeSession, sessionCreated, sessionUpdated chan struct{}) {
	var audioChunks [][]byte
	var currentText strings.Builder
	var lastText string // Track last text to avoid duplicates
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
			// DashScope "choices" format response
			// Text is cumulative, so we need to print only the new part
			if event.Delta != "" {
				newPart := event.Delta
				if len(newPart) > len(lastText) {
					// Print only the new characters
					fmt.Print(newPart[len(lastText):])
					lastText = newPart
				}
			}
			// Collect audio
			if len(event.Audio) > 0 {
				audioChunks = append(audioChunks, event.Audio)
			}
			// Check if finished
			if event.FinishReason != "" {
				fmt.Println() // New line when done
				// Save audio if we have any
				if len(audioChunks) > 0 {
					saveAudioChunks(audioChunks)
				}
			}

		case dashscope.EventTypeResponseTextDelta:
			// Print text delta incrementally
			fmt.Print(event.Delta)
			currentText.WriteString(event.Delta)

		case dashscope.EventTypeResponseTextDone:
			if currentText.Len() > 0 {
				fmt.Println() // New line after text
			}

		case dashscope.EventTypeResponseAudioDelta:
			// Collect audio chunks
			if len(event.Audio) > 0 {
				audioChunks = append(audioChunks, event.Audio)
			} else if event.AudioBase64 != "" {
				// Decode from base64 if not already decoded
				if decoded, err := base64.StdEncoding.DecodeString(event.AudioBase64); err == nil {
					audioChunks = append(audioChunks, decoded)
				}
			}

		case dashscope.EventTypeResponseAudioDone:
			// Audio complete - save if output file specified
			if len(audioChunks) > 0 {
				saveAudioChunks(audioChunks)
			}

		case dashscope.EventTypeResponseTranscriptDelta:
			// Assistant's speech transcript
			printVerbose("[transcript] %s", event.Delta)

		case dashscope.EventTypeResponseDone:
			if event.Response != nil && event.Response.Status == "failed" {
				if event.Response.StatusDetail != nil && event.Response.StatusDetail.Error != nil {
					cli.PrintError("Response failed: %s", event.Response.StatusDetail.Error.Message)
				}
			}
			if event.Usage != nil {
				printVerbose("Tokens - Input: %d, Output: %d",
					event.Usage.InputTokens, event.Usage.OutputTokens)
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

	outputPath := getOutputFile()
	if outputPath != "" {
		audio := make([]byte, 0, totalSize)
		for _, chunk := range audioChunks {
			audio = append(audio, chunk...)
		}
		if err := cli.OutputBytes(audio, outputPath); err != nil {
			cli.PrintError("Failed to save audio: %v", err)
		} else {
			cli.PrintSuccess("Audio saved to: %s (%d bytes)", outputPath, totalSize)
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
		} else {
			if err := sendAudioFile(session, parts[1]); err != nil {
				cli.PrintError("Failed to send audio: %v", err)
			}
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
			newVoice := parts[1]
			if err := session.UpdateSession(&dashscope.SessionConfig{
				Voice: newVoice,
			}); err != nil {
				cli.PrintError("Failed to change voice: %v", err)
			} else {
				cli.PrintSuccess("Voice changed to: %s", newVoice)
			}
		}

	case "/help":
		fmt.Println("Commands:")
		fmt.Println("  /audio <file> - Send audio file (16-bit PCM, 16kHz)")
		fmt.Println("  /clear        - Clear input buffer")
		fmt.Println("  /voice <id>   - Change voice")
		fmt.Println("  /exit, /quit  - End session")
		fmt.Println("  /help         - Show this help")

	default:
		cli.PrintError("Unknown command: %s (try /help)", cmd)
	}

	return false
}
