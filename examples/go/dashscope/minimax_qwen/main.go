// Package main demonstrates multi-round conversation using MiniMax TTS and DashScope Qwen-Omni.
//
// This example shows:
//  1. Use MiniMax TTS to generate speech from text (16kHz PCM16)
//  2. Send the speech to DashScope Qwen-Omni-Realtime
//  3. Receive AI response (text + audio)
//  4. Repeat for multi-round conversation
//
// Usage:
//
//	go run main.go
//	go run main.go -v  # verbose mode
//
// Environment variables:
//
//	MINIMAX_API_KEY    - MiniMax API key
//	DASHSCOPE_API_KEY  - DashScope API key
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var (
	minimaxKey   = flag.String("minimax-key", os.Getenv("MINIMAX_API_KEY"), "MiniMax API key")
	dashscopeKey = flag.String("dashscope-key", os.Getenv("DASHSCOPE_API_KEY"), "DashScope API key")
	verbose      = flag.Bool("v", false, "Verbose output")
)

// ConversationTurn represents a turn in the conversation.
type ConversationTurn struct {
	UserText    string
	Description string
}

func main() {
	flag.Parse()

	if *minimaxKey == "" {
		log.Fatal("MiniMax API key is required. Use -minimax-key or set MINIMAX_API_KEY")
	}
	if *dashscopeKey == "" {
		log.Fatal("DashScope API key is required. Use -dashscope-key or set DASHSCOPE_API_KEY")
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║     MiniMax TTS + DashScope Qwen-Omni Multi-Round Chat       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ctx := context.Background()

	// Create clients
	fmt.Println("[1/4] Creating MiniMax client...")
	mmClient := minimax.NewClient(*minimaxKey)
	fmt.Println("      ✓ MiniMax client ready")

	fmt.Println("[2/4] Creating DashScope client...")
	dsClient := dashscope.NewClient(*dashscopeKey)
	fmt.Println("      ✓ DashScope client ready")

	// Connect to Qwen-Omni-Realtime
	fmt.Println("[3/4] Connecting to Qwen-Omni-Realtime...")
	session, eventCh, errCh, err := connectDashScope(ctx, dsClient)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()
	fmt.Println("      ✓ Session ready")

	fmt.Println("[4/4] Configuration complete!")
	fmt.Println()
	fmt.Println("═══════════════════ Starting Conversation ═══════════════════")
	fmt.Println()

	// Define conversation turns
	turns := []ConversationTurn{
		{UserText: "你好，请介绍一下你自己。", Description: "Greeting and introduction"},
		{UserText: "今天天气怎么样？", Description: "Ask about weather"},
		{UserText: "谢谢你的回答，再见！", Description: "Say goodbye"},
	}

	for i, turn := range turns {
		fmt.Println("┌─────────────────────────────────────────────────────────────┐")
		fmt.Printf("│ Turn %d/%d: %s\n", i+1, len(turns), turn.Description)
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
		fmt.Println()

		// Generate speech using MiniMax TTS
		fmt.Printf("  📝 User: \"%s\"\n", turn.UserText)
		fmt.Println()
		fmt.Print("  🔊 Generating speech via MiniMax TTS... ")

		audioData, err := generateSpeech(ctx, mmClient, turn.UserText)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}
		fmt.Printf("✓ (%d bytes)\n", len(audioData))

		// Send audio to Qwen-Omni
		fmt.Print("  📤 Sending audio to Qwen-Omni... ")

		if err := sendAudioToSession(session, audioData); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}
		fmt.Println("✓")

		// Wait for and display response
		fmt.Println()
		fmt.Println("  🤖 Qwen Response:")
		fmt.Print("     ")

		responseText, responseAudio, err := receiveResponse(eventCh, errCh)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		if responseText == "" {
			fmt.Println("(no text response received)")
		} else {
			fmt.Println()
		}

		if len(responseAudio) > 0 {
			fmt.Println()
			fmt.Printf("     📢 Audio response: %d bytes\n", len(responseAudio))
		}

		fmt.Println()

		// Small delay between turns
		if i < len(turns)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("═══════════════════ Conversation Complete ═══════════════════")
	fmt.Println()
	fmt.Println("✓ Session closed")
}

// connectDashScope establishes a DashScope Realtime session.
func connectDashScope(ctx context.Context, client *dashscope.Client) (*dashscope.RealtimeSession, <-chan *dashscope.RealtimeEvent, <-chan error, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(connectCtx, &dashscope.RealtimeConfig{
		Model: dashscope.ModelQwenOmniTurboRealtimeLatest,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect failed: %w", err)
	}

	// Start event reader
	eventCh := make(chan *dashscope.RealtimeEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		for event, err := range session.Events() {
			if err != nil {
				errCh <- err
				return
			}
			eventCh <- event
		}
		close(eventCh)
	}()

	// Wait for session.created
	select {
	case event := <-eventCh:
		if event.Type != dashscope.EventTypeSessionCreated {
			session.Close()
			return nil, nil, nil, fmt.Errorf("expected session.created, got %s", event.Type)
		}
		fmt.Printf("      ✓ WebSocket connected\n")
		fmt.Printf("      ✓ Session created: %s\n", session.SessionID())
	case err := <-errCh:
		session.Close()
		return nil, nil, nil, err
	case <-time.After(10 * time.Second):
		session.Close()
		return nil, nil, nil, fmt.Errorf("timeout waiting for session.created")
	}

	// Configure session - use manual mode for precise control
	sessionCfg := &dashscope.SessionConfig{
		Modalities:                    []string{dashscope.ModalityText, dashscope.ModalityAudio},
		Voice:                         dashscope.VoiceChelsie,
		InputAudioFormat:              dashscope.AudioFormatPCM16,
		OutputAudioFormat:             dashscope.AudioFormatPCM16,
		EnableInputAudioTranscription: true,
		InputAudioTranscriptionModel:  "gummy-realtime-v1",
		Instructions:                  "You are a helpful AI assistant. Please respond in Chinese concisely.",
		TurnDetection:                 nil, // Manual mode - no server VAD
	}

	if err := session.UpdateSession(sessionCfg); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("update session failed: %w", err)
	}

	// Wait for session.updated
	select {
	case event := <-eventCh:
		if event.Type == dashscope.EventTypeSessionUpdated {
			fmt.Println("      ✓ Session configured")
		}
	case <-time.After(5 * time.Second):
		if *verbose {
			fmt.Println("      ⚠ Timeout waiting for session.updated (continuing)")
		}
	}

	return session, eventCh, errCh, nil
}

// generateSpeech generates speech from text using MiniMax TTS.
func generateSpeech(ctx context.Context, client *minimax.Client, text string) ([]byte, error) {
	req := &minimax.SpeechRequest{
		Model: minimax.ModelSpeech26HD,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: "female-shaonv",
			Speed:   1.0,
			Vol:     1.0,
			Emotion: "happy",
		},
		AudioSetting: &minimax.AudioSetting{
			Format:     "pcm",
			SampleRate: 16000, // 16kHz for DashScope
			Channel:    1,     // Mono
		},
		LanguageBoost: "Chinese",
	}

	resp, err := client.Speech.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Audio, nil
}

// sendAudioToSession sends audio data to the session in chunks.
func sendAudioToSession(session *dashscope.RealtimeSession, audioData []byte) error {
	// Send audio in chunks (100ms chunks at 16kHz, 16-bit = 3200 bytes)
	const chunkSize = 3200
	const chunkIntervalMs = 50 // Faster than realtime to reduce wait time

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		if err := session.AppendAudio(audioData[i:end]); err != nil {
			return fmt.Errorf("append audio failed: %w", err)
		}
		time.Sleep(chunkIntervalMs * time.Millisecond)
	}

	// Small delay before commit
	time.Sleep(200 * time.Millisecond)

	// Commit the audio buffer
	if err := session.CommitInput(); err != nil {
		return fmt.Errorf("commit input failed: %w", err)
	}

	// In manual mode, we need to explicitly request a response
	if err := session.CreateResponse(nil); err != nil {
		return fmt.Errorf("create response failed: %w", err)
	}

	return nil
}

// receiveResponse receives and processes response from Qwen-Omni.
func receiveResponse(eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error) (string, []byte, error) {
	var responseText strings.Builder
	var lastText string
	var responseAudio []byte
	var gotResponse bool
	var gotAnyContent bool

	timeout := 60 * time.Second // Longer timeout for AI response
	timer := time.After(timeout)

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return responseText.String(), responseAudio, nil
			}

			switch event.Type {
			case dashscope.EventTypeInputAudioCommitted:
				fmt.Print("[audio committed] ")

			case dashscope.EventTypeResponseCreated:
				gotResponse = true
				fmt.Print("[response started] ")
				responseText.Reset()
				lastText = ""
				responseAudio = nil

			case dashscope.EventTypeChoicesResponse:
				gotAnyContent = true
				// DashScope's "choices" format
				if event.Delta != "" {
					if len(event.Delta) > len(lastText) {
						newText := event.Delta[len(lastText):]
						fmt.Print(newText)
						lastText = event.Delta
					}
				}
				if event.Audio != nil {
					responseAudio = append(responseAudio, event.Audio...)
				}
				if event.FinishReason != "" && event.FinishReason != "null" {
					responseText.WriteString(lastText)
					fmt.Println()
					return responseText.String(), responseAudio, nil
				}

			case dashscope.EventTypeResponseTextDelta:
				gotAnyContent = true
				fmt.Print(event.Delta)
				responseText.WriteString(event.Delta)

			case dashscope.EventTypeResponseAudioDelta:
				gotAnyContent = true
				if event.Audio != nil {
					responseAudio = append(responseAudio, event.Audio...)
				}

			case dashscope.EventTypeResponseTranscriptDelta:
				gotAnyContent = true
				fmt.Print(event.Delta)
				responseText.WriteString(event.Delta)

			case dashscope.EventTypeResponseDone:
				if gotAnyContent {
					fmt.Println()
				}
				return responseText.String(), responseAudio, nil

			case dashscope.EventTypeError:
				if event.Error != nil {
					return "", nil, fmt.Errorf("error: %s - %s", event.Error.Code, event.Error.Message)
				}

			default:
				if *verbose {
					fmt.Printf("[%s] ", event.Type)
				}
			}

		case err := <-errCh:
			// Check if it's a close frame
			errStr := err.Error()
			if strings.Contains(errStr, "close") || strings.Contains(errStr, "timeout") {
				if gotAnyContent {
					fmt.Println()
				}
				if !gotResponse {
					fmt.Println("(connection closed)")
				}
				return responseText.String(), responseAudio, nil
			}
			return "", nil, err

		case <-timer:
			if gotAnyContent {
				fmt.Println()
			}
			if !gotResponse {
				fmt.Println("(timeout waiting for response)")
			}
			return responseText.String(), responseAudio, nil
		}
	}
}
