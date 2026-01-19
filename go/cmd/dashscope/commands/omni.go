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

This is an audio-first API. Use --audio to send an audio file, or enter
interactive mode to send audio chunks.

Audio format: 16-bit PCM, 24kHz, mono

Examples:
  dashscope -c myctx omni chat
  dashscope -c myctx omni chat --audio input.pcm -o output.pcm`,
	RunE: runOmniChat,
}

var (
	omniModel        string
	omniVoice        string
	omniAudioFile    string
	omniInstructions string
)

func init() {
	omniChatCmd.Flags().StringVar(&omniModel, "model", dashscope.ModelQwenOmniTurboRealtimeLatest, "Model to use")
	omniChatCmd.Flags().StringVar(&omniVoice, "voice", "Chelsie", "Voice for audio output (Chelsie, Cherry, Serena, Ethan)")
	omniChatCmd.Flags().StringVar(&omniAudioFile, "audio", "", "Input audio file (16-bit PCM, 16kHz)")
	omniChatCmd.Flags().StringVar(&omniInstructions, "instructions", "", "System instructions")

	omniCmd.AddCommand(omniChatCmd)
}

func runOmniChat(cmd *cobra.Command, args []string) error {
	ctx, err := getContext()
	if err != nil {
		return err
	}

	printVerbose("Using context: %s", ctx.Name)
	printVerbose("Model: %s", omniModel)
	printVerbose("Voice: %s", omniVoice)

	// Create DashScope client
	client := createDashScopeClient(ctx)

	// Connect to realtime session
	connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(connectCtx, &dashscope.RealtimeConfig{
		Model: omniModel,
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

	// Configure session with VAD mode (server-side voice activity detection)
	printVerbose("Updating session config...")
	err = session.UpdateSession(&dashscope.SessionConfig{
		Modalities:         []string{dashscope.ModalityText, dashscope.ModalityAudio},
		Voice:              omniVoice,
		InputAudioFormat:   "pcm16",  // 16-bit PCM, 16kHz mono
		OutputAudioFormat:  "pcm16",  // 16-bit PCM, 24kHz mono
		EnableInputAudioTranscription: true,
		InputAudioTranscriptionModel:  "gummy-realtime-v1",
		TurnDetection: &dashscope.TurnDetection{
			Type:              dashscope.VADModeServerVAD,
			Threshold:         0.2,
			PrefixPaddingMs:   300,
			SilenceDurationMs: 800,
		},
	})
	if err != nil {
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
	if omniAudioFile != "" {
		if err := sendAudioFile(session, omniAudioFile); err != nil {
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

func sendAudioFile(session *dashscope.RealtimeSession, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	cli.PrintInfo("Sending audio file: %s", filepath)

	// Read and send in chunks (16kHz * 2 bytes * 0.1s = 3200 bytes per chunk)
	chunkSize := 3200
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
	// If you need manual mode, uncomment the following:
	// time.Sleep(1 * time.Second)
	// session.CommitInput()
	// session.CreateResponse(nil)

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
							cli.PrintSuccess("Audio saved to: %s", outputPath)
						}
					}
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
			cli.PrintInfo("Available voices: Chelsie, Cherry, Serena, Ethan")
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

	// Enable debug mode in verbose mode
	if isVerbose() {
		opts = append(opts, dashscope.WithDebug(true))
	}

	return dashscope.NewClient(ctx.APIKey, opts...)
}
