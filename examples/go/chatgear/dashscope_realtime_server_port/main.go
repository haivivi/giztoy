// Package main demonstrates a chatgear server using DashScope Realtime for speech-to-speech.
//
// This server starts an embedded MQTT broker and handles audio from a geartest client.
// It supports two modes:
//   - Recording mode (push-to-talk): Manual mode, user controls speech boundaries
//   - Calling mode (server VAD): DashScope's VAD automatically detects speech
//
// Required environment variables:
//   - DASHSCOPE_API_KEY: DashScope API key
//
// Usage:
//
//	bazel run //examples/go/chatgear/dashscope_realtime_server_port -- \
//	  --port=:1883 \
//	  --namespace=prefix \
//	  --gear-id=device123
//
// Then connect geartest to tcp://localhost:1883 with the same namespace and gear-id.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/dashscope"
)

var (
	listenAddr   = flag.String("port", ":1883", "MQTT broker listen address (e.g., :1883)")
	namespace    = flag.String("namespace", "", "Topic namespace/scope")
	gearID       = flag.String("gear-id", "test-gear", "Device gear ID")
	model        = flag.String("model", dashscope.ModelQwenOmniTurboRealtimeLatest, "DashScope model")
	voice        = flag.String("voice", dashscope.VoiceChelsie, "TTS voice (Chelsie, Cherry, Serena, Ethan)")
	instructions = flag.String("instructions", "你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。", "System instructions")
)

func main() {
	flag.Parse()

	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY environment variable is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Start embedded MQTT broker
	log.Printf("Starting MQTT broker on %s for gear %s", *listenAddr, *gearID)
	conn, err := chatgear.ListenMQTTServer(ctx, chatgear.MQTTServerConfig{
		Addr:   *listenAddr,
		Scope:  *namespace,
		GearID: *gearID,
	})
	if err != nil {
		log.Fatalf("Failed to start MQTT broker: %v", err)
	}
	defer conn.Close()
	log.Printf("MQTT broker started, clients can connect to tcp://localhost%s", *listenAddr)

	// Create DashScope client
	client := dashscope.NewClient(apiKey)

	// Create server port
	port := chatgear.NewServerPort()
	defer port.Close()

	// Start ReadFrom and WriteTo
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := port.ReadFrom(conn); err != nil {
			log.Printf("ReadFrom error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := port.WriteTo(conn); err != nil {
			log.Printf("WriteTo error: %v", err)
		}
	}()

	// Create handler
	handler := &realtimeHandler{
		ctx:          ctx,
		port:         port,
		client:       client,
		model:        *model,
		voice:        *voice,
		instructions: *instructions,
	}

	// Main loop - process uplink data
	log.Println("Server ready, waiting for client...")
	for {
		data, err := port.Poll()
		if err != nil {
			if err == buffer.ErrIteratorDone {
				break
			}
			log.Printf("Poll error: %v", err)
			break
		}

		if data.State != nil {
			handler.handleState(data.State)
		}
		if data.Audio != nil {
			handler.handleAudio(data.Audio)
		}
		if data.StatsChanges != nil {
			log.Printf("Stats changed: %+v", data.StatsChanges)
		}
	}

	port.Close()
	wg.Wait()
	log.Println("Server stopped")
}

// realtimeHandler manages the DashScope realtime session
type realtimeHandler struct {
	ctx          context.Context
	port         *chatgear.ServerPort
	client       *dashscope.Client
	model        string
	voice        string
	instructions string

	mu            sync.Mutex
	session       *dashscope.RealtimeSession
	currentState  chatgear.State
	decoder       *opus.Decoder
	sessionCancel context.CancelFunc
	serverVAD     bool // true for calling mode

	// Track management for audio output (supports interruption)
	track     pcm.Track
	trackCtrl *pcm.TrackCtrl

	// Response ID tracking for audio filtering (ignore cut responses)
	currentResponseID  string
	ignoredResponseIDs map[string]bool

	// Stats for logging
	audioFrameCount int
	audioBytesSent  int
}

func (h *realtimeHandler) handleState(evt *chatgear.StateEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	prevState := h.currentState
	h.currentState = evt.State

	log.Printf("========== STATE: %s -> %s (ts=%v) ==========", prevState, evt.State, evt.Time.Time().Format("15:04:05.000"))

	switch evt.State {
	case chatgear.StateRecording:
		// User started recording (push-to-talk mode)
		h.serverVAD = false
		h.audioFrameCount = 0
		h.audioBytesSent = 0

		// Close current track to stop AI playback immediately
		if h.trackCtrl != nil {
			h.trackCtrl.Close()
			h.track = nil
			h.trackCtrl = nil
			log.Println("[Recording] Closed track to stop AI playback")
		}

		// Mark current response as ignored (filter out remaining audio from this response)
		if h.currentResponseID != "" {
			if h.ignoredResponseIDs == nil {
				h.ignoredResponseIDs = make(map[string]bool)
			}
			h.ignoredResponseIDs[h.currentResponseID] = true
			log.Printf("[Recording] Ignoring response ID: %s", h.currentResponseID)
			h.currentResponseID = ""
		}

		h.ensureSession()
		log.Println("[Recording] Started - manual VAD mode, collecting audio...")

	case chatgear.StateWaitingForResponse:
		// User finished recording, trigger response
		log.Printf("[WaitingForResponse] Audio collected: %d frames, %d bytes sent to AI", h.audioFrameCount, h.audioBytesSent)
		if prevState == chatgear.StateRecording && h.session != nil {
			log.Println("[WaitingForResponse] Triggering AI response (CommitInput + CreateResponse)")
			// Commit audio buffer and request response
			if err := h.session.CommitInput(); err != nil {
				log.Printf("[WaitingForResponse] CommitInput error: %v", err)
			} else {
				log.Println("[WaitingForResponse] CommitInput OK")
			}
			if err := h.session.CreateResponse(nil); err != nil {
				log.Printf("[WaitingForResponse] CreateResponse error: %v", err)
			} else {
				log.Println("[WaitingForResponse] CreateResponse OK")
			}
		}

	case chatgear.StateCalling:
		// Enter calling mode with server VAD
		h.serverVAD = true
		h.audioFrameCount = 0
		h.audioBytesSent = 0
		h.ensureSession()
		// Update session to use server VAD
		if h.session != nil {
			err := h.session.UpdateSession(&dashscope.SessionConfig{
				TurnDetection: &dashscope.TurnDetection{
					Type: dashscope.VADModeServerVAD,
				},
			})
			if err != nil {
				log.Printf("[Calling] Failed to enable server VAD: %v", err)
			} else {
				log.Println("[Calling] Server VAD enabled - AI will auto-detect speech")
			}
		}

	case chatgear.StateReady:
		// Back to ready state
		log.Printf("[Ready] Previous state was %s", prevState)

	case chatgear.StateStreaming:
		log.Println("[Streaming] AI is generating response...")

	case chatgear.StateInterrupted:
		// User interrupted AI
		log.Println("[Interrupted] User interrupted - canceling AI response")
		h.port.Interrupt()
		if h.session != nil {
			h.session.CancelResponse()
		}
	}
}

func (h *realtimeHandler) handleAudio(frame *chatgear.StampedOpusFrame) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Only process audio in active states
	if h.currentState != chatgear.StateRecording && h.currentState != chatgear.StateCalling {
		return
	}

	// Ensure session exists
	if h.session == nil {
		log.Printf("[Audio] Frame received but no session (state=%s)", h.currentState)
		return
	}

	// Decode Opus to PCM
	if h.decoder == nil {
		var err error
		h.decoder, err = opus.NewDecoder(48000, 1)
		if err != nil {
			log.Printf("[Audio] Failed to create decoder: %v", err)
			return
		}
		log.Println("[Audio] Opus decoder created (48kHz mono)")
	}

	// Decode frame - returns PCM bytes at 48kHz
	pcm48k, err := h.decoder.Decode(frame.Frame)
	if err != nil {
		log.Printf("[Audio] Decode error: %v", err)
		return
	}

	// Resample 48kHz -> 16kHz (DashScope expects 16kHz)
	pcm16k := resample48to16(pcm48k)

	h.audioFrameCount++
	h.audioBytesSent += len(pcm16k)

	// Log every 50 frames (~1 second at 20ms/frame)
	if h.audioFrameCount%50 == 0 {
		log.Printf("[Audio] Sent %d frames, %d bytes total (opus=%d -> pcm48k=%d -> pcm16k=%d)",
			h.audioFrameCount, h.audioBytesSent, len(frame.Frame), len(pcm48k), len(pcm16k))
	}

	// Send to DashScope session
	if err := h.session.AppendAudio(pcm16k); err != nil {
		log.Printf("[Audio] AppendAudio error: %v", err)
	}
}

// resample48to16 resamples 48kHz mono PCM to 16kHz by taking every 3rd sample.
func resample48to16(pcm48k []byte) []byte {
	samples48k := len(pcm48k) / 2
	samples16k := samples48k / 3
	pcm16k := make([]byte, samples16k*2)
	for i := 0; i < samples16k; i++ {
		srcIdx := i * 3 * 2
		dstIdx := i * 2
		pcm16k[dstIdx] = pcm48k[srcIdx]
		pcm16k[dstIdx+1] = pcm48k[srcIdx+1]
	}
	return pcm16k
}

func (h *realtimeHandler) ensureSession() {
	if h.session != nil {
		return
	}

	log.Println("Creating DashScope realtime session...")

	sessionCtx, cancel := context.WithCancel(h.ctx)
	h.sessionCancel = cancel

	config := &dashscope.RealtimeConfig{
		Model: h.model,
	}

	session, err := h.client.Realtime.Connect(sessionCtx, config)
	if err != nil {
		log.Printf("Failed to connect DashScope: %v", err)
		cancel()
		return
	}

	h.session = session
	log.Println("DashScope session connected")

	// Wait for session.created and configure
	go h.receiveEvents(sessionCtx, session)
}

func (h *realtimeHandler) receiveEvents(ctx context.Context, session *dashscope.RealtimeSession) {
	// Reset session when this goroutine exits
	defer func() {
		h.mu.Lock()
		if h.session == session {
			h.session = nil
			log.Println("[Session] Session reset due to receiveEvents exit")
		}
		h.mu.Unlock()
	}()

	// Wait for session.created first
	sessionReady := false
	for event, err := range session.Events() {
		if err != nil {
			log.Printf("Event error: %v", err)
			break
		}
		if event.Type == dashscope.EventTypeSessionCreated {
			sessionReady = true
			log.Println("Session created, configuring...")
			break
		}
	}

	if !sessionReady {
		log.Println("Failed to receive session.created")
		return
	}

	// Configure session
	sessionConfig := &dashscope.SessionConfig{
		Voice:                         h.voice,
		Modalities:                    []string{dashscope.ModalityAudio, dashscope.ModalityText},
		Instructions:                  h.instructions,
		EnableInputAudioTranscription: true,
		InputAudioFormat:              dashscope.AudioFormatPCM16,
		OutputAudioFormat:             dashscope.AudioFormatPCM16,
	}

	// Set VAD mode based on current mode
	h.mu.Lock()
	if h.serverVAD {
		sessionConfig.TurnDetection = &dashscope.TurnDetection{
			Type: dashscope.VADModeServerVAD,
		}
	}
	h.mu.Unlock()

	if err := session.UpdateSession(sessionConfig); err != nil {
		log.Printf("UpdateSession error: %v", err)
		return
	}

	log.Println("Session configured")

	// Create initial foreground track for TTS audio
	log.Println("[Track] Creating initial foreground track...")
	if err := h.newTrack(); err != nil {
		log.Printf("[Track] Failed to create initial track: %v", err)
		return
	}

	// Continue receiving events
	var audioChunks int
	var audioBytesOut int
	for event, err := range session.Events() {
		if err != nil {
			if err != io.EOF {
				log.Printf("[Event] Error: %v", err)
			}
			break
		}

		switch event.Type {
		case dashscope.EventTypeSessionUpdated:
			log.Println("[Event] session.updated - configuration applied")

		case dashscope.EventTypeInputSpeechStarted:
			log.Println("[Event] input_audio_buffer.speech_started - VAD detected speech start")
			// Create new track to interrupt current playback (old track fades out)
			if err := h.newTrack(); err != nil {
				log.Printf("[Event] Failed to create new track for interruption: %v", err)
			}

		case dashscope.EventTypeInputSpeechStopped:
			log.Println("[Event] input_audio_buffer.speech_stopped - VAD detected speech end")

		case dashscope.EventTypeInputAudioCommitted:
			log.Println("[Event] input_audio_buffer.committed - audio buffer committed")

		case dashscope.EventTypeInputAudioTranscriptionCompleted:
			// ASR result
			log.Printf("[Event] ASR transcription: \"%s\"", event.Transcript)

		case dashscope.EventTypeResponseCreated:
			log.Printf("[Event] response.created (id=%s) - AI starting response", event.ResponseID)
			audioChunks = 0
			audioBytesOut = 0

			// Check if this response should be ignored (was cut before)
			if h.ignoredResponseIDs[event.ResponseID] {
				log.Printf("[Event] Response %s is in ignore list, skipping", event.ResponseID)
				continue
			}

			// Update current response ID
			h.currentResponseID = event.ResponseID

			// Create new track for this response (enables clean audio switching)
			if err := h.newTrack(); err != nil {
				log.Printf("[Event] Failed to create new track for response: %v", err)
			}

		case dashscope.EventTypeResponseTextDelta:
			// Model text response
			if event.Delta != "" {
				log.Printf("[Event] AI text: %s", event.Delta)
			}

		case dashscope.EventTypeResponseTranscriptDelta:
			// TTS transcript
			if event.Delta != "" {
				log.Printf("[Event] AI transcript: %s", event.Delta)
			}

		case dashscope.EventTypeResponseAudioDelta:
			// Skip audio from ignored responses
			if h.currentResponseID == "" || h.ignoredResponseIDs[h.currentResponseID] {
				continue
			}

			// Audio chunk - write to track as PCM 24kHz mono
			// Note: DashScope outputs 24kHz PCM
			if len(event.Audio) > 0 && h.track != nil {
				audioChunks++
				audioBytesOut += len(event.Audio)
				if audioChunks%20 == 0 {
					log.Printf("[Event] Audio out: %d chunks, %d bytes (%.2fs @ 24kHz)",
						audioChunks, audioBytesOut, float64(audioBytesOut)/48000.0)
				}
				chunk := pcm.L16Mono24K.DataChunk(event.Audio)
				if err := h.track.Write(chunk); err != nil {
					log.Printf("[Event] Track write error (type=%T): %v", err, err)
				} else if audioChunks <= 3 {
					log.Printf("[Event] Track write OK: chunk %d, %d bytes", audioChunks, len(event.Audio))
				}
			}

		case dashscope.EventTypeResponseAudioDone:
			log.Printf("[Event] response.audio.done - total: %d chunks, %d bytes (%.2fs)",
				audioChunks, audioBytesOut, float64(audioBytesOut)/48000.0)

		case dashscope.EventTypeResponseDone:
			log.Println("[Event] response.done - AI finished responding")
			if event.Usage != nil {
				log.Printf("[Event] Usage: input=%d, output=%d, total=%d tokens",
					event.Usage.InputTokens, event.Usage.OutputTokens, event.Usage.TotalTokens)
			}

		case dashscope.EventTypeError:
			if event.Error != nil {
				log.Printf("[Event] ERROR: type=%s code=%s msg=%s", event.Error.Type, event.Error.Code, event.Error.Message)
			}

		default:
			log.Printf("[Event] %s (unhandled)", event.Type)
		}
	}
}

func (h *realtimeHandler) closeSession() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	if h.sessionCancel != nil {
		h.sessionCancel()
		h.sessionCancel = nil
	}
	if h.decoder != nil {
		h.decoder.Close()
		h.decoder = nil
	}
}

// newTrack creates a new foreground track, replacing the old one.
// This enables audio interruption - when a new track is created, the old track
// fades out (200ms) while the new track starts playing immediately.
func (h *realtimeHandler) newTrack() error {
	track, ctrl, err := h.port.NewForegroundTrack()
	if err != nil {
		return err
	}
	h.track = track
	h.trackCtrl = ctrl
	log.Println("[Track] New foreground track created (old track will fade out)")
	return nil
}
