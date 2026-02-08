// Package main demonstrates a chatgear server using Doubao Realtime for speech-to-speech.
//
// This server connects to MQTT and handles audio from a geartest client.
// It supports two modes:
//   - Recording mode (push-to-talk): Manual VAD, user controls speech boundaries
//   - Calling mode (server VAD): Doubao's VAD automatically detects speech
//
// Required environment variables:
//   - DOUBAO_APP_ID: Doubao application ID
//   - DOUBAO_TOKEN: Doubao access token
//
// Usage:
//
//	bazel run //examples/go/chatgear/doubao_realtime_server_port -- \
//	  --mqtt=mqtts://user:pass@host:8883 \
//	  --namespace=prefix \
//	  --gear-id=device123
package main

import (
	"bytes"
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
	"github.com/haivivi/giztoy/go/pkg/audio/resampler"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var (
	mqttAddr  = flag.String("mqtt", "", "MQTT broker URL (e.g., mqtts://user:pass@host:8883)")
	namespace = flag.String("namespace", "", "Topic namespace/scope")
	gearID    = flag.String("gear-id", "", "Device gear ID")
	speaker   = flag.String("speaker", "zh_female_vv_jupiter_bigtts", "TTS speaker voice")
	botName   = flag.String("bot-name", "小助手", "Bot name for Doubao")
	sysRole   = flag.String("system-role", "你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。", "System role prompt")
)

func main() {
	flag.Parse()

	if *mqttAddr == "" || *gearID == "" {
		log.Fatal("--mqtt and --gear-id are required")
	}

	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	if appID == "" || token == "" {
		log.Fatal("DOUBAO_APP_ID and DOUBAO_TOKEN environment variables are required")
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

	// Connect to MQTT as server
	log.Printf("Connecting to MQTT: %s", *mqttAddr)
	conn, err := chatgear.DialMQTTServer(ctx, chatgear.MQTTServerConfig{
		Addr:   *mqttAddr,
		Scope:  *namespace,
		GearID: *gearID,
	})
	if err != nil {
		log.Fatalf("Failed to connect MQTT: %v", err)
	}
	defer conn.Close()
	log.Printf("Connected to MQTT, listening for gear: %s", *gearID)

	// Create Doubao client
	client := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))

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
		ctx:     ctx,
		port:    port,
		client:  client,
		speaker: *speaker,
		botName: *botName,
		sysRole: *sysRole,
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

// realtimeHandler manages the Doubao realtime session
type realtimeHandler struct {
	ctx     context.Context
	port    *chatgear.ServerPort
	client  *doubaospeech.Client
	speaker string
	botName string
	sysRole string

	mu            sync.Mutex
	session       *doubaospeech.RealtimeSession
	currentState  chatgear.State
	decoder       *opus.Decoder
	sessionCancel context.CancelFunc
}

func (h *realtimeHandler) handleState(evt *chatgear.StateEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	prevState := h.currentState
	h.currentState = evt.State

	log.Printf("State: %s -> %s", prevState, evt.State)

	switch evt.State {
	case chatgear.StateRecording:
		// User started recording (push-to-talk mode)
		// Ensure session exists
		h.ensureSession()
		log.Println("Recording started - collecting audio")

	case chatgear.StateWaitingForResponse:
		// User finished recording, trigger response
		if prevState == chatgear.StateRecording {
			log.Println("Recording ended - requesting AI response")
			// In Doubao Realtime, audio is sent incrementally
			// The session should already have received the audio
			// Doubao's VAD should handle end detection
		}

	case chatgear.StateCalling:
		// Enter calling mode with server VAD
		h.ensureSession()
		log.Println("Calling mode started - server VAD active")

	case chatgear.StateReady:
		// Back to ready state
		if prevState == chatgear.StateStreaming || prevState == chatgear.StateWaitingForResponse {
			log.Println("Conversation turn complete")
		}

	case chatgear.StateInterrupted:
		// User interrupted AI
		log.Println("Interrupted - stopping playback")
		h.port.Interrupt()
		if h.session != nil {
			h.session.Interrupt(h.ctx)
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
		return
	}

	// Decode Opus to PCM
	if h.decoder == nil {
		var err error
		h.decoder, err = opus.NewDecoder(48000, 1)
		if err != nil {
			log.Printf("Failed to create decoder: %v", err)
			return
		}
	}

	// Decode frame - returns PCM bytes at 48kHz
	pcm48k, err := h.decoder.Decode(frame.Frame)
	if err != nil {
		log.Printf("Decode error: %v", err)
		return
	}

	// Simple resample 48kHz -> 16kHz (take every 3rd sample)
	// Doubao expects 16kHz PCM
	pcm16k := resample48to16(pcm48k)

	// Send to Doubao session
	if err := h.session.SendAudio(h.ctx, pcm16k); err != nil {
		log.Printf("SendAudio error: %v", err)
	}
}

// resample48to16 resamples 48kHz mono PCM to 16kHz using high-quality resampling.
// Input and output are little-endian int16 bytes.
func resample48to16(pcm48k []byte) []byte {
	if len(pcm48k) == 0 {
		return nil
	}
	srcFmt := resampler.Format{SampleRate: 48000, Stereo: false}
	dstFmt := resampler.Format{SampleRate: 16000, Stereo: false}

	rs, err := resampler.New(bytes.NewReader(pcm48k), srcFmt, dstFmt)
	if err != nil {
		log.Printf("Failed to create resampler: %v", err)
		return nil
	}
	defer rs.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, rs); err != nil {
		log.Printf("Failed to resample: %v", err)
		return nil
	}
	return out.Bytes()
}

func (h *realtimeHandler) ensureSession() {
	if h.session != nil {
		return
	}

	log.Println("Creating Doubao realtime session...")

	sessionCtx, cancel := context.WithCancel(h.ctx)
	h.sessionCancel = cancel

	config := &doubaospeech.RealtimeConfig{
		ASR: doubaospeech.RealtimeASRConfig{
			Extra: map[string]any{
				"end_smooth_window_ms": 200,
			},
		},
		TTS: doubaospeech.RealtimeTTSConfig{
			Speaker: h.speaker,
			AudioConfig: doubaospeech.RealtimeAudioConfig{
				Format:     "pcm_s16le",
				SampleRate: 24000,
				Channel:    1,
			},
		},
		Dialog: doubaospeech.RealtimeDialogConfig{
			BotName:    h.botName,
			SystemRole: h.sysRole,
		},
	}

	session, err := h.client.Realtime.Connect(sessionCtx, config)
	if err != nil {
		log.Printf("Failed to connect Doubao: %v", err)
		cancel()
		return
	}

	h.session = session
	log.Println("Doubao session connected")

	// Start receiving events in background
	go h.receiveEvents(sessionCtx, session)
}

func (h *realtimeHandler) receiveEvents(ctx context.Context, session *doubaospeech.RealtimeSession) {
	// Create foreground track for TTS audio
	track, trackCtrl, err := h.port.NewForegroundTrack()
	if err != nil {
		log.Printf("Failed to create track: %v", err)
		return
	}
	defer trackCtrl.CloseWithError(nil)

	for event, err := range session.Recv() {
		if err != nil {
			if err != io.EOF {
				log.Printf("Recv error: %v", err)
			}
			break
		}

		switch event.Type {
		case doubaospeech.EventASRInfo:
			// First word detected - potential interrupt point
			log.Printf("ASR: first word detected")

		case doubaospeech.EventASRResponse:
			// ASR text result
			if event.Text != "" {
				log.Printf("ASR: %s", event.Text)
			}

		case doubaospeech.EventASREnded:
			log.Println("ASR: speech ended")

		case doubaospeech.EventTTSStarted:
			// TTS started
			if event.TTSInfo != nil {
				log.Printf("TTS: %s", event.TTSInfo.Content)
			}

		case doubaospeech.EventChatResponse:
			// Model text response
			if event.Text != "" {
				log.Printf("AI: %s", event.Text)
			}

		case doubaospeech.EventAudioReceived:
			// Audio chunk - write to track as PCM 24kHz mono
			if len(event.Audio) > 0 {
				chunk := pcm.L16Mono24K.DataChunk(event.Audio)
				if err := track.Write(chunk); err != nil {
					log.Printf("Track write error: %v", err)
				}
			}

		case doubaospeech.EventTTSFinished:
			log.Println("TTS: finished")

		case doubaospeech.EventChatEnded:
			log.Println("Chat: response ended")

		case doubaospeech.EventSessionFinished:
			log.Println("Session: ended")
			return
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
