// Package main tests Doubao Realtime VAD (Voice Activity Detection) functionality.
//
// This test validates VAD configuration through the ASR.Extra["end_smooth_window_ms"] parameter.
// Different window sizes affect how quickly the system detects end of speech.
//
// Required environment variables:
//   - DOUBAO_APP_ID
//   - DOUBAO_TOKEN
//   - MINIMAX_API_KEY (for TTS)
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/doubao_realtime_vad -- -vad-window=200
//	bazel run //examples/go/genx/transformers/doubao_realtime_vad -- -vad-window=500
//	bazel run //examples/go/genx/transformers/doubao_realtime_vad -- -vad-window=1000
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/audio/portaudio"
	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/e2e/genx/transformers/internal"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var (
	speaker   = flag.String("speaker", "zh_female_vv_jupiter_bigtts", "TTS speaker voice")
	vadWindow = flag.Int("vad-window", 200, "VAD end detection window in milliseconds")
	timeout   = flag.Duration("timeout", 2*time.Minute, "Test timeout")
)

// Test sentences with silence gaps between them
var sentences = []string{
	"你好，我是小明。",
	"今天天气怎么样？",
	"请给我讲一个笑话。",
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== Doubao Realtime VAD Test ===")
	fmt.Printf("Speaker:    %s\n", *speaker)
	fmt.Printf("VAD Window: %dms\n", *vadWindow)
	fmt.Println()
	fmt.Println("Note: VAD window controls how long silence is needed to detect end of speech.")
	fmt.Println("  - Smaller window (100-200ms): Faster response, may cut off speech")
	fmt.Println("  - Larger window (500-1000ms): More tolerant of pauses, slower response")
	fmt.Println()

	// Get API keys
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	mmKey := os.Getenv("MINIMAX_API_KEY")

	if appID == "" || token == "" {
		log.Fatal("DOUBAO_APP_ID and DOUBAO_TOKEN required")
	}
	if mmKey == "" {
		log.Fatal("MINIMAX_API_KEY required")
	}

	// Create clients
	doubaoClient := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))
	mmClient := minimax.NewClient(mmKey)

	// Create TTS (16kHz PCM for Doubao input)
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Create Doubao Realtime transformer with custom VAD settings
	realtime := newDoubaoRealtimeWithVAD(doubaoClient, *speaker, *vadWindow)

	// Initialize portaudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream for playback (24kHz mono)
	spk, err := portaudio.NewOutputStream(pcm.L16Mono24K, 20*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to create output stream: %v", err)
	}
	defer spk.Close()

	// Build combined TTS streams with silence gaps
	fmt.Println("[1] Building TTS streams with silence gaps...")
	ttsStreams := make([]genx.Stream, len(sentences))
	for i, sentence := range sentences {
		stream, err := tts.Transform(ctx, "", textToStream(sentence))
		if err != nil {
			log.Fatalf("TTS error: %v", err)
		}
		ttsStreams[i] = stream
	}

	// Combine with silence between sentences (2 seconds)
	combined := genx.CompositeSeq(ttsStreams...)
	eosToSilence := internal.NewEOSToSilence(2*time.Second, 16000, 1)
	withSilence, err := eosToSilence.Transform(ctx, combined)
	if err != nil {
		log.Fatalf("EOSToSilence error: %v", err)
	}

	// Connect to Doubao Realtime
	fmt.Println("[2] Connecting to Doubao Realtime...")
	output, err := realtime.Transform(ctx, "", withSilence)
	if err != nil {
		log.Fatalf("Doubao Realtime error: %v", err)
	}

	// Collect results
	fmt.Println("[3] Streaming with VAD detection...")
	fmt.Println()

	type Turn struct {
		ASR       string
		LLMText   string
		AudioSec  float64
		StartTime time.Time
		EndTime   time.Time
	}

	var turns []Turn
	currentTurn := Turn{StartTime: time.Now()}
	turnCount := 0

	for {
		chunk, err := output.Next()
		if err == io.EOF {
			// Save last turn if has content
			if currentTurn.ASR != "" || currentTurn.LLMText != "" {
				currentTurn.EndTime = time.Now()
				turns = append(turns, currentTurn)
			}
			break
		}
		if err != nil {
			log.Printf("Stream error: %v", err)
			break
		}
		if chunk == nil {
			continue
		}

		switch chunk.Role {
		case genx.RoleUser:
			// ASR result
			if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
				// New ASR result might indicate new turn
				if currentTurn.LLMText != "" {
					// Previous turn had LLM response, start new turn
					currentTurn.EndTime = time.Now()
					turns = append(turns, currentTurn)
					turnCount++
					fmt.Printf("  [Turn %d completed in %.2fs]\n", turnCount, currentTurn.EndTime.Sub(currentTurn.StartTime).Seconds())
					currentTurn = Turn{StartTime: time.Now()}
				}
				currentTurn.ASR = string(text)
				fmt.Printf("  ASR: %s\n", text)
			}
		case genx.RoleModel:
			if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
				currentTurn.LLMText += string(text)
			} else if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
				currentTurn.AudioSec += float64(len(blob.Data)) / 48000.0 // 24kHz 16-bit mono
				// Play audio
				playPCM(spk, blob.Data)
			}
		}
	}

	// Print summary
	fmt.Println()
	fmt.Println("=== VAD Test Summary ===")
	fmt.Printf("VAD Window: %dms\n", *vadWindow)
	fmt.Printf("Total Turns: %d\n", len(turns))
	fmt.Println()

	for i, t := range turns {
		duration := t.EndTime.Sub(t.StartTime).Seconds()
		fmt.Printf("[Turn %d] Duration: %.2fs\n", i+1, duration)
		fmt.Printf("  ASR:   %s\n", truncate(t.ASR, 40))
		fmt.Printf("  LLM:   %s\n", truncate(t.LLMText, 40))
		fmt.Printf("  Audio: %.2fs\n", t.AudioSec)
		fmt.Println()
	}

	fmt.Println("=== Test Complete ===")
}

// newDoubaoRealtimeWithVAD creates a Doubao Realtime transformer with custom VAD settings
func newDoubaoRealtimeWithVAD(client *doubaospeech.Client, speakerVoice string, vadWindowMs int) genx.Transformer {
	return transformers.NewDoubaoRealtime(client,
		transformers.WithDoubaoRealtimeSpeaker(speakerVoice),
		transformers.WithDoubaoRealtimeFormat("pcm_s16le"),
		transformers.WithDoubaoRealtimeSampleRate(24000),
		transformers.WithDoubaoRealtimeSystemRole("你是一个友好的助手，用简短的话回答问题。"),
		transformers.WithDoubaoRealtimeVADWindow(vadWindowMs),
	)
}

// playPCM plays PCM audio data in chunks
func playPCM(out *portaudio.OutputStream, data []byte) {
	const chunkSize = 960 // 20ms @ 24kHz
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		out.WriteBytes(data[:n])
		data = data[n:]
	}
}

// textToStream converts text to a single-chunk stream
func textToStream(text string) genx.Stream {
	return &singleTextStream{text: text}
}

type singleTextStream struct {
	text string
	done bool
}

func (s *singleTextStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, io.EOF
	}
	s.done = true
	return &genx.MessageChunk{
		Role: genx.RoleUser,
		Part: genx.Text(s.text),
	}, nil
}

func (s *singleTextStream) Close() error {
	s.done = true
	return nil
}

func (s *singleTextStream) CloseWithError(err error) error {
	s.done = true
	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
