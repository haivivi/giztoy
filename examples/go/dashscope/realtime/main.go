// Package main provides a multi-turn conversation test between MiniMax and DashScope Realtime.
//
// Flow: LLM -> TTS -> DashScope Omni -> Text -> LLM -> TTS -> DashScope Omni -> ...
//
// Usage:
//
//	go run main.go -minimax-key=xxx -dashscope-key=xxx -rounds=10
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var (
	minimaxKey   = flag.String("minimax-key", os.Getenv("MINIMAX_API_KEY"), "MiniMax API key")
	dashscopeKey = flag.String("dashscope-key", os.Getenv("DASHSCOPE_API_KEY"), "DashScope API key")
	rounds       = flag.Int("rounds", 10, "Number of conversation rounds")
	prompt       = flag.String("prompt", "你好，请问你是谁？", "Initial prompt")
	verbose      = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()

	// Configure slog based on verbose flag
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	if *minimaxKey == "" {
		log.Fatal("MiniMax API key is required. Use -minimax-key or set MINIMAX_API_KEY")
	}
	if *dashscopeKey == "" {
		log.Fatal("DashScope API key is required. Use -dashscope-key or set DASHSCOPE_API_KEY")
	}

	log.Printf("=== Multi-Turn Conversation Test ===")
	log.Printf("Rounds: %d", *rounds)
	log.Printf("Initial prompt: %s", *prompt)
	log.Println()

	// Create clients
	mmClient := minimax.NewClient(*minimaxKey, minimax.WithBaseURL("https://api.minimaxi.chat"))
	dsClient := dashscope.NewClient(*dashscopeKey)

	ctx := context.Background()

	// Connect to DashScope Realtime (keep session for multi-turn)
	session, eventCh, errCh, err := connectDashScope(ctx, dsClient)
	if err != nil {
		log.Fatalf("Failed to connect DashScope: %v", err)
	}
	defer session.Close()

	// Start conversation
	currentText := *prompt
	totalInputTokens := 0
	totalOutputTokens := 0
	successfulRounds := 0

	for round := 1; round <= *rounds; round++ {
		log.Printf("\n========== Round %d/%d ==========", round, *rounds)

		// Step 1: MiniMax LLM processes the text
		log.Printf("[MiniMax LLM] Input: %s", truncate(currentText, 100))
		llmResponse, err := callMiniMaxLLM(ctx, mmClient, currentText)
		if err != nil {
			log.Printf("[Error] MiniMax LLM failed: %v", err)
			break
		}
		log.Printf("[MiniMax LLM] Output: %s", truncate(llmResponse, 100))

		// Step 2: Convert LLM response to audio via TTS
		log.Printf("[MiniMax TTS] Converting to audio...")
		audioData, err := callMiniMaxTTS(ctx, mmClient, llmResponse)
		if err != nil {
			log.Printf("[Error] MiniMax TTS failed: %v", err)
			break
		}
		log.Printf("[MiniMax TTS] Audio: %d bytes (%.1f seconds)", len(audioData), float64(len(audioData))/32000)

		// Step 3: Send audio to DashScope and get response
		log.Printf("[DashScope] Sending audio and waiting for response...")
		omniResponse, usage, err := sendAndReceive(session, eventCh, errCh, audioData)
		if err != nil {
			// Check if connection was closed, try to reconnect
			if strings.Contains(err.Error(), "close") || strings.Contains(err.Error(), "websocket") {
				log.Printf("[Info] WebSocket closed, reconnecting...")
				session.Close()
				session, eventCh, errCh, err = connectDashScope(ctx, dsClient)
				if err != nil {
					log.Printf("[Error] Reconnect failed: %v", err)
					break
				}
				// Retry this round
				omniResponse, usage, err = sendAndReceive(session, eventCh, errCh, audioData)
				if err != nil {
					log.Printf("[Error] DashScope failed after reconnect: %v", err)
					break
				}
			} else {
				log.Printf("[Error] DashScope failed: %v", err)
				break
			}
		}

		successfulRounds++

		if omniResponse == "" {
			log.Printf("[Warning] DashScope returned empty text response")
			// Try to continue with a default response
			omniResponse = "我没有听清楚，请再说一遍。"
		}

		log.Printf("[DashScope] Response: %s", truncate(omniResponse, 100))
		if usage != nil {
			totalInputTokens += usage.InputTokens
			totalOutputTokens += usage.OutputTokens
			log.Printf("[DashScope] Tokens: input=%d, output=%d", usage.InputTokens, usage.OutputTokens)
		}

		// Use DashScope's response as the next input
		currentText = omniResponse

		// Small delay between rounds
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("\n========== Conversation Complete ==========")
	log.Printf("Successful rounds: %d/%d", successfulRounds, *rounds)
	log.Printf("Total tokens: input=%d, output=%d", totalInputTokens, totalOutputTokens)
}

// connectDashScope establishes a DashScope Realtime session
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
			return nil, nil, nil, fmt.Errorf("expected session.created, got %s", event.Type)
		}
		log.Printf("[DashScope] Session created: %s", session.SessionID())
	case err := <-errCh:
		session.Close()
		return nil, nil, nil, err
	case <-time.After(10 * time.Second):
		session.Close()
		return nil, nil, nil, fmt.Errorf("timeout waiting for session.created")
	}

	// Configure session for multi-turn conversation
	sessionCfg := &dashscope.SessionConfig{
		Modalities:                    []string{dashscope.ModalityText, dashscope.ModalityAudio},
		Voice:                         dashscope.VoiceChelsie,
		InputAudioFormat:              dashscope.AudioFormatPCM16,
		OutputAudioFormat:             dashscope.AudioFormatPCM16,
		EnableInputAudioTranscription: true,
		Instructions:                  "你是Qwen AI，正在和MiniMax AI进行对话。要求：1）每次回复不超过30字；2）保持对话有趣；3）可以提问或回应对方。",
		TurnDetection:                 nil, // Manual mode for precise control
	}

	if err := session.UpdateSession(sessionCfg); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("update session failed: %w", err)
	}

	// Wait for session.updated
	select {
	case event := <-eventCh:
		if event.Type == dashscope.EventTypeSessionUpdated {
			log.Printf("[DashScope] Session configured (manual mode)")
		}
	case <-time.After(5 * time.Second):
		log.Println("[DashScope] Timeout waiting for session.updated (continuing)")
	}

	return session, eventCh, errCh, nil
}

// sendAndReceive sends audio and receives text response
func sendAndReceive(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, audio []byte) (string, *dashscope.UsageStats, error) {
	// Clear any pending events
	drainEvents(eventCh, 100*time.Millisecond)

	// Send audio in chunks
	const chunkSize = 3200 // 100ms at 16kHz
	for i := 0; i < len(audio); i += chunkSize {
		end := i + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		if err := session.AppendAudio(audio[i:end]); err != nil {
			return "", nil, fmt.Errorf("append audio failed: %w", err)
		}
		time.Sleep(30 * time.Millisecond) // Faster than real-time
	}

	// Commit and request response (manual mode)
	time.Sleep(200 * time.Millisecond)
	if err := session.CommitInput(); err != nil {
		return "", nil, fmt.Errorf("commit failed: %w", err)
	}
	if err := session.CreateResponse(nil); err != nil {
		return "", nil, fmt.Errorf("create response failed: %w", err)
	}

	// Collect response (longer timeout for longer audio)
	audioDuration := float64(len(audio)) / 32000 // seconds
	timeout := time.Duration(audioDuration+30) * time.Second
	if timeout < 45*time.Second {
		timeout = 45 * time.Second
	}
	return collectResponseText(eventCh, errCh, timeout)
}

// collectResponseText collects text from response events
func collectResponseText(eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, timeout time.Duration) (string, *dashscope.UsageStats, error) {
	log.Printf("[Collecting] Waiting up to %v for response...", timeout)
	var textBuilder strings.Builder
	var usage *dashscope.UsageStats
	timer := time.After(timeout)

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return textBuilder.String(), usage, nil
			}

			switch event.Type {
			case dashscope.EventTypeChoicesResponse:
				// DashScope's main response format with choices array
				if event.Delta != "" {
					textBuilder.WriteString(event.Delta)
					if *verbose {
						fmt.Print(event.Delta)
					}
				}
				if event.FinishReason != "" && event.FinishReason != "null" {
					if *verbose {
						fmt.Println()
					}
					return textBuilder.String(), usage, nil
				}

			case dashscope.EventTypeResponseTextDelta:
				textBuilder.WriteString(event.Delta)
				if *verbose {
					fmt.Print(event.Delta)
				}

			case dashscope.EventTypeResponseAudioDelta:
				// Audio data - ignore but continue

			case dashscope.EventTypeResponseTranscriptDelta:
				// Transcript of generated audio
				textBuilder.WriteString(event.Delta)
				if *verbose {
					fmt.Print(event.Delta)
				}

			case dashscope.EventTypeResponseDone:
				if *verbose {
					fmt.Println()
				}
				if event.Usage != nil {
					usage = event.Usage
				}
				return textBuilder.String(), usage, nil

			case dashscope.EventTypeError:
				if event.Error != nil {
					return "", nil, fmt.Errorf("error: %s - %s", event.Error.Code, event.Error.Message)
				}

			case dashscope.EventTypeInputAudioCommitted:
				if *verbose {
					log.Println("[Event] Audio committed")
				}

			case dashscope.EventTypeResponseCreated:
				if *verbose {
					log.Printf("[Event] Response created: %s", event.ResponseID)
				}

			default:
				if *verbose {
					log.Printf("[Event] %s", event.Type)
				}
			}

		case err := <-errCh:
			return textBuilder.String(), usage, err

		case <-timer:
			text := textBuilder.String()
			if text != "" {
				return text, usage, nil
			}
			return "", nil, fmt.Errorf("timeout waiting for response")
		}
	}
}

// drainEvents drains pending events from the channel
func drainEvents(eventCh <-chan *dashscope.RealtimeEvent, timeout time.Duration) {
	timer := time.After(timeout)
	for {
		select {
		case <-eventCh:
			// Discard
		case <-timer:
			return
		}
	}
}

// callMiniMaxLLM calls MiniMax Chat API
func callMiniMaxLLM(ctx context.Context, client *minimax.Client, input string) (string, error) {
	req := &minimax.ChatCompletionRequest{
		Model: "MiniMax-Text-01",
		Messages: []minimax.Message{
			{
				Role:    "system",
				Content: "你是MiniMax AI，正在和Qwen AI进行对话。要求：1）每次回复不超过30字；2）保持对话有趣；3）可以提问或回应对方。",
			},
			{Role: "user", Content: input},
		},
		MaxTokens:   60,
		Temperature: 0.9, // More creative for conversation
	}

	resp, err := client.Text.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type")
	}
	return strings.TrimSpace(content), nil
}

// callMiniMaxTTS calls MiniMax TTS API
func callMiniMaxTTS(ctx context.Context, client *minimax.Client, text string) ([]byte, error) {
	req := &minimax.SpeechRequest{
		Model: minimax.ModelSpeech26HD,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: "female-shaonv",
			Speed:   1.2, // Slightly faster
			Vol:     1.0,
		},
		AudioSetting: &minimax.AudioSetting{
			Format:     "pcm",
			SampleRate: 16000,
			Channel:    1,
		},
		LanguageBoost: "Chinese",
	}

	resp, err := client.Speech.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Audio, nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// Ensure base64 import is used
var _ = base64.StdEncoding
