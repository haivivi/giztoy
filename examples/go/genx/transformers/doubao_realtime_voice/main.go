// Package main tests Doubao Realtime voice switching functionality.
//
// NOTE: Doubao Realtime API does NOT support dynamic session updates like DashScope.
// To switch voices, we need to create a new session with a different speaker.
// This test demonstrates voice switching by creating separate sessions.
//
// Required environment variables:
//   - DOUBAO_APP_ID
//   - DOUBAO_TOKEN
//   - MINIMAX_API_KEY (for TTS)
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/doubao_realtime_voice
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
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var (
	timeout = flag.Duration("timeout", 3*time.Minute, "Test timeout")
)

// Voice configurations to test
var voiceConfigs = []struct {
	Speaker     string
	SystemRole  string
	Description string
}{
	{
		Speaker:     "zh_female_vv_jupiter_bigtts",
		SystemRole:  "你是一个活泼可爱的小姑娘，说话甜美，喜欢用语气词。回答要简短有趣。",
		Description: "甜美女声 - 灿灿",
	},
	{
		Speaker:     "zh_male_yunzhou_jupiter_bigtts",
		SystemRole:  "你是一个阳光开朗的小伙子，说话有活力，喜欢鼓励别人。回答要简短有活力。",
		Description: "阳光男声 - 云舟",
	},
}

// Test sentences for each voice
var sentences = []string{
	"你好呀，给我介绍一下你自己吧！",
	"今天天气真不错，你觉得呢？",
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== Doubao Realtime Voice Switching Test ===")
	fmt.Println()
	fmt.Println("Note: Doubao Realtime does NOT support dynamic voice switching.")
	fmt.Println("      This test creates separate sessions for each voice.")
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

	// Create TTS
	tts := transformers.NewMinimaxTTS(mmClient, "female-shaonv",
		transformers.WithMinimaxTTSFormat("pcm"),
		transformers.WithMinimaxTTSSampleRate(16000),
	)

	// Initialize portaudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream for playback (24kHz mono)
	speaker, err := portaudio.NewOutputStream(pcm.L16Mono24K, 20*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to create output stream: %v", err)
	}
	defer speaker.Close()

	// Test each voice configuration
	for i, voiceConfig := range voiceConfigs {
		fmt.Printf("=== Voice %d: %s ===\n", i+1, voiceConfig.Description)
		fmt.Printf("Speaker: %s\n", voiceConfig.Speaker)
		fmt.Printf("Role: %s\n", truncate(voiceConfig.SystemRole, 50))
		fmt.Println()

		// Create Doubao Realtime transformer with this voice
		realtime := transformers.NewDoubaoRealtime(doubaoClient,
			transformers.WithDoubaoRealtimeSpeaker(voiceConfig.Speaker),
			transformers.WithDoubaoRealtimeFormat("pcm_s16le"),
			transformers.WithDoubaoRealtimeSampleRate(24000),
			transformers.WithDoubaoRealtimeSystemRole(voiceConfig.SystemRole),
		)

		// Process each sentence
		for j, sentence := range sentences {
			fmt.Printf("[Turn %d] Input: %s\n", j+1, sentence)

			// Create TTS stream
			ttsStream, err := tts.Transform(ctx, textToStream(sentence))
			if err != nil {
				log.Printf("TTS error: %v", err)
				continue
			}

			// Connect to Doubao Realtime
			output, err := realtime.Transform(ctx, ttsStream)
			if err != nil {
				log.Printf("Doubao Realtime error: %v", err)
				continue
			}

			// Collect and play results
			var asrText, llmText string
			var audioBytes int

			for {
				chunk, err := output.Next()
				if err == io.EOF {
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
					if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
						asrText = string(text) // Keep last ASR result
					}
				case genx.RoleModel:
					if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
						llmText += string(text)
					} else if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
						audioBytes += len(blob.Data)
						// Play audio
						playPCM(speaker, blob.Data)
					}
				}
			}

			fmt.Printf("  ASR:   %s\n", truncate(asrText, 40))
			fmt.Printf("  LLM:   %s\n", truncate(llmText, 50))
			fmt.Printf("  Audio: %.2fs\n", float64(audioBytes)/48000.0)
			fmt.Println()
		}

		// Small pause between voices
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("=== Voice Switching Test Complete ===")
	fmt.Println()
	fmt.Println("Listen and verify that different voices were used for each section.")
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
