// Package main demonstrates two Doubao Realtime AI agents having a text-based conversation.
//
// Since Doubao Realtime's audio input mode requires special configuration,
// this example uses text-based conversation between two AI agents.
// Each AI takes turns responding with text, and their audio responses are played.
//
// Required environment variables:
//   - DOUBAO_APP_ID
//   - DOUBAO_TOKEN
//
// Usage:
//
//	bazel run //examples/go/genx/transformers/doubao_realtime_chat -- -rounds=5
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
	rounds  = flag.Int("rounds", 5, "Number of conversation rounds")
	timeout = flag.Duration("timeout", 3*time.Minute, "Test timeout")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("=== Doubao Realtime Chat (Text Mode) ===")
	fmt.Printf("Rounds: %d\n\n", *rounds)

	// Get API keys
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")

	if appID == "" || token == "" {
		log.Fatal("DOUBAO_APP_ID and DOUBAO_TOKEN required")
	}

	// Create client
	client := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))

	// Create two AI agents with different personas
	aiA := transformers.NewDoubaoRealtime(client,
		transformers.WithDoubaoRealtimeSpeaker("zh_female_vv_jupiter_bigtts"),
		transformers.WithDoubaoRealtimeBotName("小红"),
		transformers.WithDoubaoRealtimeSystemRole("你是小红，一个热情开朗的东北大妈。说话直爽幽默，喜欢唠嗑。你正在和邻居小丽聊天，回答要简短有趣，10-20个字。"),
	)

	aiB := transformers.NewDoubaoRealtime(client,
		transformers.WithDoubaoRealtimeSpeaker("zh_male_yunzhou_jupiter_bigtts"),
		transformers.WithDoubaoRealtimeBotName("小丽"),
		transformers.WithDoubaoRealtimeSystemRole("你是小丽，一个温柔细腻的上海阿姨。说话软糯好听，喜欢分享生活。你正在和邻居小红聊天，回答要简短优雅，10-20个字。"),
	)

	// Initialize portaudio
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream for playback
	speaker, err := portaudio.NewOutputStream(pcm.L16Mono24K, 20*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to create output stream: %v", err)
	}
	defer speaker.Close()

	// Start conversation
	currentMessage := "你好呀邻居，今天天气真不错！"
	currentAI := aiA
	aiName := "小红"

	fmt.Printf("=== Starting Conversation ===\n\n")
	fmt.Printf("[Initial] %s: %s\n\n", aiName, currentMessage)

	for round := 1; round <= *rounds; round++ {
		// Switch AI
		if currentAI == aiA {
			currentAI = aiB
			aiName = "小丽"
		} else {
			currentAI = aiA
			aiName = "小红"
		}

		fmt.Printf("[Round %d] %s's turn...\n", round, aiName)

		// Get response from current AI
		response, audioBytes, err := getResponse(ctx, currentAI, currentMessage, speaker)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("  Response: %s\n", response)
		fmt.Printf("  Audio: %.2fs\n\n", float64(audioBytes)/48000.0)

		// Use response as next message
		currentMessage = response
	}

	fmt.Println("=== Conversation Complete ===")
}

// getResponse gets AI response for a message
func getResponse(ctx context.Context, ai *transformers.DoubaoRealtime, message string, speaker *portaudio.OutputStream) (string, int, error) {
	output, err := ai.Transform(ctx, textToStream(message))
	if err != nil {
		return "", 0, err
	}

	var text string
	var audioBytes int

	for {
		chunk, err := output.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return text, audioBytes, err
		}
		if chunk == nil {
			continue
		}

		switch p := chunk.Part.(type) {
		case genx.Text:
			text += string(p)
		case *genx.Blob:
			audioBytes += len(p.Data)
			playPCM(speaker, p.Data)
		}
	}

	return text, audioBytes, nil
}

// playPCM plays PCM audio data
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

// textToStream converts text to a stream
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

func (s *singleTextStream) Close() error               { return nil }
func (s *singleTextStream) CloseWithError(error) error { return nil }
