// speech_tts demonstrates text-to-speech synthesis with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . -o output.mp3 "Hello, world!"
//	go run . -stream -o output.mp3 "Hello, world!"
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

func main() {
	model := flag.String("model", minimax.ModelSpeech26HD, "Model name")
	voice := flag.String("voice", minimax.VoiceFemaleShaonv, "Voice ID")
	output := flag.String("o", "output.mp3", "Output file path")
	stream := flag.Bool("stream", false, "Use streaming mode")
	speed := flag.Float64("speed", 1.0, "Speech speed (0.5-2.0)")
	emotion := flag.String("emotion", "", "Emotion: happy, sad, angry, neutral, etc.")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: speech_tts [flags] <text>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable not set")
	}

	client := minimax.NewClient(apiKey)
	ctx := context.Background()

	text := flag.Arg(0)

	req := &minimax.SpeechRequest{
		Model: *model,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: *voice,
			Speed:   *speed,
		},
		AudioSetting: &minimax.AudioSetting{
			Format: minimax.AudioFormatMP3,
		},
	}

	if *emotion != "" {
		req.VoiceSetting.Emotion = *emotion
	}

	var audioData []byte

	if *stream {
		fmt.Println("Using streaming mode...")
		var buf bytes.Buffer
		for chunk, err := range client.Speech.SynthesizeStream(ctx, req) {
			if err != nil {
				log.Fatalf("Streaming failed: %v", err)
			}
			if chunk.Audio != nil {
				buf.Write(chunk.Audio)
				fmt.Printf("\rReceived %d bytes...", buf.Len())
			}
		}
		fmt.Println()
		audioData = buf.Bytes()
	} else {
		fmt.Println("Synthesizing...")
		resp, err := client.Speech.Synthesize(ctx, req)
		if err != nil {
			log.Fatalf("Synthesis failed: %v", err)
		}
		audioData = resp.Audio

		if resp.ExtraInfo != nil {
			fmt.Printf("Duration: %d ms\n", resp.ExtraInfo.AudioLength)
			fmt.Printf("Characters: %d\n", resp.ExtraInfo.UsageCharacters)
		}
	}

	if err := os.WriteFile(*output, audioData, 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Printf("✓ Saved to %s (%d bytes)\n", *output, len(audioData))
}
