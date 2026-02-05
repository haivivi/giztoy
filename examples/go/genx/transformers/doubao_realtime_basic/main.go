// Package main tests Doubao Realtime basic streaming functionality.
//
// This test validates basic text -> Doubao Realtime -> Audio playback pipeline.
//
// Required environment variables:
//   - DOUBAO_APP_ID
//   - DOUBAO_TOKEN
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/doubao_realtime_basic
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
)

var (
	speaker = flag.String("speaker", "zh_female_vv_jupiter_bigtts", "TTS speaker voice (O version)")
	timeout = flag.Duration("timeout", 2*time.Minute, "Test timeout")
)

// Test sentences
var sentences = []string{
	"你好，请用一句话介绍自己。",
	"今天天气怎么样？",
	"给我讲一个笑话。",
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== Doubao Realtime Basic Test ===")
	fmt.Printf("Speaker: %s\n", *speaker)
	fmt.Println()

	// Get API keys
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")

	if appID == "" || token == "" {
		log.Fatal("DOUBAO_APP_ID and DOUBAO_TOKEN required")
	}

	// Create Doubao client
	doubaoClient := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))

	// Create Doubao Realtime transformer (defaults to pcm_s16le @ 24kHz)
	realtime := transformers.NewDoubaoRealtime(doubaoClient,
		transformers.WithDoubaoRealtimeSpeaker(*speaker),
		transformers.WithDoubaoRealtimeSystemRole("你是一个友好的助手，用简短的话回答问题。"),
	)

	// Initialize portaudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream for playback (24kHz mono)
	speakerOut, err := portaudio.NewOutputStream(pcm.L16Mono24K, 20*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to create output stream: %v", err)
	}
	defer speakerOut.Close()

	// Process each sentence
	for i, sentence := range sentences {
		fmt.Printf("[Turn %d] Input: %s\n", i+1, sentence)

		// Create text stream
		input := textToStream(sentence)

		// Connect to Doubao Realtime
		output, err := realtime.Transform(ctx, input)
		if err != nil {
			log.Printf("Doubao Realtime error: %v", err)
			continue
		}

		// Collect and play results
		var llmText string
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
				// ASR result (for audio input mode)
				if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
					fmt.Printf("  ASR: %s\n", text)
				}
			case genx.RoleModel:
				if text, ok := chunk.Part.(genx.Text); ok && len(text) > 0 {
					llmText += string(text)
				} else if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
					audioBytes += len(blob.Data)
					// Play audio (24kHz PCM)
					playPCM(speakerOut, blob.Data)
				}
			}
		}

		fmt.Printf("  LLM: %s\n", truncate(llmText, 80))
		fmt.Printf("  Audio: %.2fs\n", float64(audioBytes)/48000.0) // 24kHz 16-bit mono
		fmt.Println()
	}

	fmt.Println("=== Test Complete ===")
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
