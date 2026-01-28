// Package main demonstrates two OpenAI Realtime agents chatting with each other.
//
// This example shows:
// - Creating two Realtime sessions (WebSocket or WebRTC)
// - Different input/output modes: t2t, t2a, a2t, a2a
// - Multi-turn conversation between AI agents
// - Using MiniMax TTS for a2t mode (converts text to audio)
//
// Modes:
//   - t2t: text to text (both agents use text only)
//   - t2a: text to audio (send text, receive audio)
//   - a2t: audio to text (send audio via MiniMax TTS, receive text)
//   - a2a: audio to audio (full audio conversation)
//
// Usage:
//
//	export OPENAI_API_KEY=sk-xxx
//	export MINIMAX_API_KEY=xxx  # Required for a2t mode
//	go run main.go                           # default: t2t mode, 10 rounds
//	go run main.go -mode t2a -rounds 15      # text to audio, 15 rounds
//	go run main.go -mode a2a -rounds 20      # full audio mode
//	go run main.go -mode a2t -rounds 10      # audio to text with MiniMax TTS
//	go run main.go -transport ws,webrtc      # WebSocket vs WebRTC
//	go run main.go -v                        # verbose output
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/audio/resampler"
	"github.com/haivivi/giztoy/go/pkg/minimax"
	openairealtime "github.com/haivivi/giztoy/go/pkg/openai-realtime"
)

var (
	apiKey       = flag.String("api-key", os.Getenv("OPENAI_API_KEY"), "OpenAI API key")
	minimaxKey   = flag.String("minimax-key", os.Getenv("MINIMAX_API_KEY"), "MiniMax API key (for a2t mode)")
	model        = flag.String("model", openairealtime.ModelGPT4oRealtimePreview, "Model to use")
	mode         = flag.String("mode", "t2t", "Conversation mode: t2t, t2a, a2t, a2a")
	transport    = flag.String("transport", "ws,ws", "Transport for each agent: ws,ws or ws,webrtc")
	rounds       = flag.Int("rounds", 10, "Number of conversation rounds")
	prompt       = flag.String("prompt", "你好！我是 Agent A，请问你是谁？让我们开始聊天吧！", "Initial prompt")
	verbose      = flag.Bool("v", false, "Verbose output")
	minimaxURL   = flag.String("minimax-url", "https://api.minimax.chat", "MiniMax API base URL")
	useVAD       = flag.Bool("vad", false, "Use VAD (Voice Activity Detection) mode instead of manual mode")
	vadType      = flag.String("vad-type", "server_vad", "VAD type: server_vad or semantic_vad")
)

// Agent represents a chat agent with its session and configuration.
type Agent struct {
	Name         string
	Session      openairealtime.Session
	EventCh      chan *openairealtime.ServerEvent
	ErrCh        chan error
	InputMode    string // "text" or "audio"
	OutputMode   string // "text" or "audio"
	Voice        string
	Instructions string
}

func main() {
	flag.Parse()

	// Configure logging
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	if *apiKey == "" {
		log.Fatal("OpenAI API key is required. Use -api-key or set OPENAI_API_KEY")
	}

	// Parse mode
	inputMode, outputMode := parseMode(*mode)

	// Check MiniMax key for a2t mode
	var mmClient *minimax.Client
	if inputMode == "audio" && *minimaxKey != "" {
		mmClient = minimax.NewClient(*minimaxKey, minimax.WithBaseURL(*minimaxURL))
		log.Printf("MiniMax TTS enabled for audio input generation")
	} else if inputMode == "audio" && *minimaxKey == "" {
		log.Printf("Warning: a2t/a2a mode without MiniMax key - will use available audio or fall back to text")
	}

	log.Printf("=== OpenAI Realtime Dual Chat ===")
	log.Printf("Model: %s", *model)
	log.Printf("Mode: %s (input: %s, output: %s)", *mode, inputMode, outputMode)
	log.Printf("Transport: %s", *transport)
	log.Printf("Rounds: %d", *rounds)
	log.Printf("VAD: %v (type: %s)", *useVAD, *vadType)
	log.Printf("Initial prompt: %s", truncate(*prompt, 50))
	log.Println()

	ctx := context.Background()
	client, err := openairealtime.NewClient(*apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Parse transports
	transports := strings.Split(*transport, ",")
	if len(transports) != 2 {
		log.Fatal("Transport must be in format: ws,ws or ws,webrtc")
	}

	// Create Agent A
	log.Println("Creating Agent A...")
	agentA, err := createAgent(ctx, client, "Agent A", transports[0], inputMode, outputMode,
		openairealtime.VoiceAlloy,
		"你是 Agent A，一个友好的 AI 助手。你正在和另一个 AI (Agent B) 对话。要求：1) 每次回复简短，不超过50字；2) 保持对话有趣；3) 可以提问或回应对方。")
	if err != nil {
		log.Fatalf("Failed to create Agent A: %v", err)
	}
	defer agentA.Session.Close()

	// Create Agent B
	log.Println("Creating Agent B...")
	agentB, err := createAgent(ctx, client, "Agent B", transports[1], inputMode, outputMode,
		openairealtime.VoiceShimmer,
		"你是 Agent B，一个聪明的 AI 助手。你正在和另一个 AI (Agent A) 对话。要求：1) 每次回复简短，不超过50字；2) 保持对话有趣；3) 可以提问或回应对方。")
	if err != nil {
		log.Fatalf("Failed to create Agent B: %v", err)
	}
	defer agentB.Session.Close()

	log.Println()
	log.Println("=== Starting Conversation ===")
	log.Println()

	// Start conversation
	currentMessage := *prompt
	var currentAudio []byte
	successfulRounds := 0

	for round := 1; round <= *rounds; round++ {
		log.Printf("--- Round %d/%d ---", round, *rounds)

		// Determine sender and receiver
		var sender, receiver *Agent
		if round%2 == 1 {
			sender, receiver = agentA, agentB
		} else {
			sender, receiver = agentB, agentA
		}

		// Send message from current context
		log.Printf("[%s -> %s]", sender.Name, receiver.Name)

		// Prepare audio input if needed (using MiniMax TTS for a2t mode)
		if receiver.InputMode == "audio" && len(currentAudio) == 0 && currentMessage != "" && mmClient != nil {
			log.Printf("  [TTS] Converting text to audio via MiniMax...")
			audio, err := textToAudio(ctx, mmClient, currentMessage)
			if err != nil {
				log.Printf("  [TTS] Warning: %v (falling back to text)", err)
			} else {
				currentAudio = audio
				duration := float64(len(currentAudio)) / (24000 * 2)
				log.Printf("  [TTS] Generated %d bytes (%.1fs)", len(currentAudio), duration)
			}
		}

		if receiver.InputMode == "text" || len(currentAudio) == 0 {
			log.Printf("  Input (text): %s", truncate(currentMessage, 80))
		} else {
			duration := float64(len(currentAudio)) / (24000 * 2)
			log.Printf("  Input (audio): %d bytes (%.1fs)", len(currentAudio), duration)
		}

		// Send to receiver and get response
		responseText, responseAudio, err := sendAndReceive(receiver, currentMessage, currentAudio)
		if err != nil {
			log.Printf("  Error: %v", err)
			// Try to continue with a fallback
			if round < *rounds {
				currentMessage = "请继续我们的对话，说点有趣的。"
				currentAudio = nil
				time.Sleep(time.Second)
				continue
			}
			break
		}

		successfulRounds++

		if responseText != "" {
			log.Printf("  Output (text): %s", truncate(responseText, 80))
		}
		if len(responseAudio) > 0 {
			duration := float64(len(responseAudio)) / (24000 * 2)
			log.Printf("  Output (audio): %d bytes (%.1fs)", len(responseAudio), duration)
		}

		// Prepare for next round
		currentMessage = responseText
		currentAudio = responseAudio

		// Handle case where we need text but only have audio
		if currentMessage == "" && len(currentAudio) > 0 {
			currentMessage = "[Audio response]"
		}

		log.Println()
		time.Sleep(300 * time.Millisecond)
	}

	log.Println("=== Conversation Complete ===")
	log.Printf("Successful rounds: %d/%d", successfulRounds, *rounds)
}

func parseMode(mode string) (input, output string) {
	switch mode {
	case "t2t":
		return "text", "text"
	case "t2a":
		return "text", "audio"
	case "a2t":
		return "audio", "text"
	case "a2a":
		return "audio", "audio"
	default:
		log.Fatalf("Invalid mode: %s (must be t2t, t2a, a2t, or a2a)", mode)
		return "", ""
	}
}

// textToAudio converts text to audio using MiniMax TTS and resamples to 24kHz.
func textToAudio(ctx context.Context, client *minimax.Client, text string) ([]byte, error) {
	req := &minimax.SpeechRequest{
		Model: minimax.ModelSpeech26HD,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: "female-shaonv",
			Speed:   1.1,
			Vol:     1.0,
		},
		AudioSetting: &minimax.AudioSetting{
			Format:     "pcm",
			SampleRate: 24000, // OpenAI Realtime expects 24kHz
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

// resampleTo24kHz resamples 16kHz PCM to 24kHz PCM.
func resampleTo24kHz(audio []byte) ([]byte, error) {
	srcFmt := resampler.Format{SampleRate: 16000, Stereo: false}
	dstFmt := resampler.Format{SampleRate: 24000, Stereo: false}

	r, err := resampler.New(bytes.NewReader(audio), srcFmt, dstFmt)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

func createAgent(ctx context.Context, client *openairealtime.Client, name, transport, inputMode, outputMode, voice, instructions string) (*Agent, error) {
	var session openairealtime.Session
	var err error

	// Connect based on transport type
	config := &openairealtime.ConnectConfig{Model: *model}

	switch transport {
	case "ws", "websocket":
		session, err = client.ConnectWebSocket(ctx, config)
	case "webrtc":
		session, err = client.ConnectWebRTC(ctx, config)
	default:
		return nil, fmt.Errorf("invalid transport: %s (must be ws or webrtc)", transport)
	}

	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	// Start event reader
	eventCh := make(chan *openairealtime.ServerEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		for event, err := range session.Events() {
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			select {
			case eventCh <- event:
			default:
				// Log warning when dropping events (channel buffer full)
				log.Printf("  Warning: dropping event %s (channel full)", event.Type)
			}
		}
		close(eventCh)
	}()

	// Wait for session.created
	if err := waitForEvent(eventCh, errCh, openairealtime.EventTypeSessionCreated, 15*time.Second); err != nil {
		session.Close()
		return nil, fmt.Errorf("session creation failed: %w", err)
	}

	log.Printf("  %s session created: %s (%s)", name, session.SessionID(), transport)

	// Configure session modalities
	// For multi-turn conversation, we need both text and audio when audio is involved
	modalities := []string{openairealtime.ModalityText}
	if outputMode == "audio" || inputMode == "audio" {
		modalities = append(modalities, openairealtime.ModalityAudio)
	}

	sessionConfig := &openairealtime.SessionConfig{
		Modalities:        modalities,
		Voice:             voice,
		InputAudioFormat:  openairealtime.AudioFormatPCM16,
		OutputAudioFormat: openairealtime.AudioFormatPCM16,
		Instructions:      instructions,
	}

	// Configure VAD or manual mode
	if *useVAD && inputMode == "audio" {
		// Enable VAD for automatic turn detection
		sessionConfig.TurnDetection = &openairealtime.TurnDetection{
			Type:              *vadType,
			Threshold:         0.5,
			PrefixPaddingMs:   300,
			SilenceDurationMs: 800, // Wait 800ms of silence before triggering response
		}
		log.Printf("  %s VAD enabled (type: %s)", name, *vadType)
	} else {
		// Disable VAD for manual control
		sessionConfig.TurnDetectionDisabled = true
	}

	// Enable transcription if we need text from audio input
	if inputMode == "audio" {
		sessionConfig.InputAudioTranscription = &openairealtime.TranscriptionConfig{
			Model: "whisper-1",
		}
	}

	if err := session.UpdateSession(sessionConfig); err != nil {
		session.Close()
		return nil, fmt.Errorf("session update failed: %w", err)
	}

	// Wait for session.updated
	waitForEvent(eventCh, errCh, openairealtime.EventTypeSessionUpdated, 5*time.Second)

	log.Printf("  %s configured (input: %s, output: %s, voice: %s)", name, inputMode, outputMode, voice)

	return &Agent{
		Name:         name,
		Session:      session,
		EventCh:      eventCh,
		ErrCh:        errCh,
		InputMode:    inputMode,
		OutputMode:   outputMode,
		Voice:        voice,
		Instructions: instructions,
	}, nil
}

func sendAndReceive(agent *Agent, textInput string, audioInput []byte) (string, []byte, error) {
	// Drain any pending events
	drainEvents(agent.EventCh, 100*time.Millisecond)

	// Send input based on agent's input mode and available data
	if agent.InputMode == "audio" && len(audioInput) > 0 {
		if *useVAD {
			// VAD mode: send audio in real-time with silence padding
			return sendWithVAD(agent, audioInput)
		}
		// Manual mode: send audio in chunks then commit
		const chunkSize = 4800 // 100ms at 24kHz
		for i := 0; i < len(audioInput); i += chunkSize {
			end := i + chunkSize
			if end > len(audioInput) {
				end = len(audioInput)
			}
			if err := agent.Session.AppendAudio(audioInput[i:end]); err != nil {
				return "", nil, fmt.Errorf("append audio failed: %w", err)
			}
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		if err := agent.Session.CommitInput(); err != nil {
			return "", nil, fmt.Errorf("commit failed: %w", err)
		}
	} else {
		// Fall back to text input
		if err := agent.Session.AddUserMessage(textInput); err != nil {
			return "", nil, fmt.Errorf("add message failed: %w", err)
		}
	}

	// Request response (manual mode only - VAD auto-creates response)
	if err := agent.Session.CreateResponse(nil); err != nil {
		return "", nil, fmt.Errorf("create response failed: %w", err)
	}

	// Collect response
	return collectResponse(agent, 60*time.Second)
}

// sendWithVAD sends audio using real-time streaming with VAD via pcm.Mixer.
// The Mixer automatically outputs silence when no track data is available.
// Flow: [silence] → [speech audio written to track in real-time] → [silence after track closes]
// VAD will automatically detect speech end and trigger response.
func sendWithVAD(agent *Agent, audioInput []byte) (string, []byte, error) {
	const (
		chunkDuration   = 20 * time.Millisecond // 20ms per chunk
		leadingSilence  = 500 * time.Millisecond
		trailingSilence = 1000 * time.Millisecond // Must be > VAD silence_duration_ms (800ms)
	)

	// Create mixer with silence gap - it will output silence when no tracks have data
	mixer := pcm.NewMixer(pcm.L16Mono24K, pcm.WithSilenceGap(time.Hour))
	defer mixer.Close()

	// Calculate chunk size in bytes (20ms at 24kHz 16-bit mono = 960 bytes)
	chunkBytes := int(pcm.L16Mono24K.BytesInDuration(chunkDuration))
	readBuf := make([]byte, chunkBytes)

	// Context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Error channel for goroutine communication
	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	// Goroutine 1: Read from mixer and send to OpenAI at real-time rate
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(chunkDuration)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := mixer.Read(readBuf)
				if err != nil {
					if err != io.EOF {
						errCh <- fmt.Errorf("mixer read failed: %w", err)
					}
					return
				}
				if n > 0 {
					if err := agent.Session.AppendAudio(readBuf[:n]); err != nil {
						errCh <- fmt.Errorf("append audio failed: %w", err)
						return
					}
				}
			}
		}
	}()

	// Calculate timing
	speechDuration := pcm.L16Mono24K.Duration(int64(len(audioInput)))
	totalDuration := leadingSilence + speechDuration + trailingSilence

	log.Printf("  [VAD] Using pcm.Mixer for real-time audio streaming")
	log.Printf("  [VAD] Leading silence: %v, Speech: %v, Trailing silence: %v",
		leadingSilence, speechDuration, trailingSilence)

	// Goroutine 2: Write speech audio to track at REAL-TIME rate
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for leading silence (mixer outputs silence automatically)
		time.Sleep(leadingSilence)

		// Create track for writing speech audio
		track, trackCtrl, err := mixer.CreateTrack()
		if err != nil {
			errCh <- fmt.Errorf("create track failed: %w", err)
			return
		}

		log.Printf("  [VAD] Writing speech audio to track in real-time...")

		// Write audio in chunks at real-time rate
		// This ensures the mixer doesn't get all data at once
		ticker := time.NewTicker(chunkDuration)
		defer ticker.Stop()

		offset := 0
		for offset < len(audioInput) {
			select {
			case <-ctx.Done():
				trackCtrl.CloseWrite()
				return
			case <-ticker.C:
				end := offset + chunkBytes
				if end > len(audioInput) {
					end = len(audioInput)
				}
				chunk := pcm.L16Mono24K.DataChunk(audioInput[offset:end])
				if err := track.Write(chunk); err != nil {
					errCh <- fmt.Errorf("write to track failed: %w", err)
					return
				}
				offset = end
			}
		}

		// Close track to signal end of speech (mixer will return to silence)
		trackCtrl.CloseWrite()
		log.Printf("  [VAD] Speech written, waiting for trailing silence...")

		// Wait for trailing silence to be sent
		time.Sleep(trailingSilence)

		// Stop the read goroutine
		cancel()
	}()

	// Wait for audio streaming to complete or error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		cancel()
		return "", nil, err
	case <-done:
		// Audio streaming complete
	case <-time.After(totalDuration + 10*time.Second):
		cancel()
		return "", nil, fmt.Errorf("timeout sending audio")
	}

	log.Printf("  [VAD] Audio stream complete, waiting for VAD response...")

	// Collect response (VAD should auto-trigger response.create)
	return collectResponse(agent, 60*time.Second)
}

func collectResponse(agent *Agent, timeout time.Duration) (string, []byte, error) {
	var textBuilder strings.Builder
	var audioData []byte
	timer := time.After(timeout)

	for {
		select {
		case event, ok := <-agent.EventCh:
			if !ok {
				return textBuilder.String(), audioData, nil
			}

			switch event.Type {
			case openairealtime.EventTypeResponseTextDelta:
				textBuilder.WriteString(event.Delta)
				if *verbose {
					fmt.Print(event.Delta)
				}

			case openairealtime.EventTypeResponseAudioDelta:
				if len(event.Audio) > 0 {
					audioData = append(audioData, event.Audio...)
				}

			case openairealtime.EventTypeResponseAudioTranscriptDelta:
				textBuilder.WriteString(event.Delta)
				if *verbose {
					fmt.Print(event.Delta)
				}

			case openairealtime.EventTypeResponseDone:
				if *verbose && textBuilder.Len() > 0 {
					fmt.Println()
				}
				return textBuilder.String(), audioData, nil

			case openairealtime.EventTypeError:
				if event.TranscriptionError != nil {
					return "", nil, fmt.Errorf("API error: %s - %s",
						event.TranscriptionError.Code, event.TranscriptionError.Message)
				}

			default:
				if *verbose {
					slog.Debug("event", "type", event.Type)
				}
			}

		case err := <-agent.ErrCh:
			return textBuilder.String(), audioData, err

		case <-timer:
			text := textBuilder.String()
			if text != "" || len(audioData) > 0 {
				return text, audioData, nil
			}
			return "", nil, fmt.Errorf("timeout waiting for response")
		}
	}
}

func waitForEvent(eventCh <-chan *openairealtime.ServerEvent, errCh <-chan error, eventType string, timeout time.Duration) error {
	timer := time.After(timeout)
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return fmt.Errorf("event channel closed")
			}
			if event.Type == eventType {
				return nil
			}
		case err := <-errCh:
			return err
		case <-timer:
			return fmt.Errorf("timeout waiting for %s", eventType)
		}
	}
}

func drainEvents(eventCh <-chan *openairealtime.ServerEvent, timeout time.Duration) {
	timer := time.After(timeout)
	for {
		select {
		case <-eventCh:
		case <-timer:
			return
		}
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
