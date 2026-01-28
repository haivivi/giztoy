// Simultaneous Translation Example
//
// Tests real-time speech translation: OpenSession, SendAudio, Recv
// Requires PCM audio input (16kHz, 16-bit, mono)
//
// Usage:
//
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-token'
//	export AUDIO_FILE='./audio.pcm'  # Optional: your PCM audio file
//	bazel run //go/examples/doubaospeech/translation
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

func main() {
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	audioFile := os.Getenv("AUDIO_FILE")

	if appID == "" || token == "" {
		fmt.Println("Please set environment variables:")
		fmt.Println("  export DOUBAO_APP_ID='your-app-id'")
		fmt.Println("  export DOUBAO_TOKEN='your-token'")
		fmt.Println("  export AUDIO_FILE='./audio.pcm'  # Optional")
		os.Exit(1)
	}

	// If no audio file provided, generate one using TTS
	if audioFile == "" {
		fmt.Println("⚠️  No audio file provided, will generate test audio using TTS...")
		audioFile = "tmp/translation_input.pcm"
		if err := generateTestAudio(appID, token, audioFile); err != nil {
			fmt.Printf("❌ Failed to generate test audio: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("              Simultaneous Translation Example")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Audio file: %s\n", audioFile)

	// Read audio file
	audioData, err := os.ReadFile(audioFile)
	if err != nil {
		fmt.Printf("❌ Failed to read audio file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Audio size: %d bytes\n", len(audioData))

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Open translation session
	fmt.Println("\n📋 [1] Opening translation session...")
	session, err := client.Translation.OpenSession(ctx, &ds.TranslationConfig{
		SourceLanguage: ds.LanguageZhCN,
		TargetLanguage: ds.LanguageEnUS,
		AudioConfig: ds.StreamASRConfig{
			Format:     ds.FormatPCM,
			SampleRate: ds.SampleRate16000,
			Channel:    1,
			Bits:       16,
		},
		EnableTTS: false,
	})
	if err != nil {
		fmt.Printf("❌ OpenSession failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()
	fmt.Println("✅ Session opened")

	// Send audio in chunks
	fmt.Println("\n📋 [2] Sending audio data...")
	chunkSize := 3200 // 100ms at 16kHz, 16-bit
	go func() {
		for i := 0; i < len(audioData); i += chunkSize {
			end := i + chunkSize
			if end > len(audioData) {
				end = len(audioData)
			}
			chunk := audioData[i:end]
			isLast := end >= len(audioData)

			if err := session.SendAudio(ctx, chunk, isLast); err != nil {
				fmt.Printf("❌ SendAudio failed: %v\n", err)
				return
			}

			if !isLast {
				time.Sleep(100 * time.Millisecond) // Simulate real-time
			}
		}
	}()

	// Receive translation results
	fmt.Println("\n📋 [3] Receiving translation results...")
	fmt.Println("═══════════════════════════════════════════════════════════")
	for chunk, err := range session.Recv() {
		if err != nil {
			fmt.Printf("❌ Receive error: %v\n", err)
			break
		}
		if chunk.SourceText != "" {
			fmt.Printf("📝 Source: %s\n", chunk.SourceText)
		}
		if chunk.TargetText != "" {
			fmt.Printf("🌐 Target: %s\n", chunk.TargetText)
		}
		if chunk.IsFinal {
			fmt.Println("\n✅ Translation complete")
			break
		}
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
}

// generateTestAudio generates test audio using TTS
func generateTestAudio(appID, token, outputFile string) error {
	fmt.Println("   Generating test audio with TTS...")

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx := context.Background()

	// Use Realtime TTS to generate audio
	session, err := client.Realtime.Connect(ctx, &ds.RealtimeConfig{
		TTS: ds.RealtimeTTSConfig{
			Speaker: "zh_female_cancan_mars_bigtts",
			AudioConfig: ds.RealtimeAudioConfig{
				Format:     "mp3",
				SampleRate: 24000,
				Channel:    1,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("connect realtime: %w", err)
	}
	defer session.Close()

	text := "今天的天气真好，我们一起去公园散步吧。"
	if err := session.SendText(ctx, text); err != nil {
		return fmt.Errorf("send text: %w", err)
	}

	// Collect MP3 audio
	var mp3Data []byte
	for event, err := range session.Recv() {
		if err != nil {
			break
		}
		if len(event.Audio) > 0 {
			mp3Data = append(mp3Data, event.Audio...)
		}
		if event.Type == ds.EventTTSFinished {
			break
		}
	}

	if len(mp3Data) == 0 {
		return fmt.Errorf("no audio received")
	}

	// Create output directory
	os.MkdirAll("tmp", 0755)

	// Save MP3 first
	mp3File := "tmp/translation_input.mp3"
	if err := os.WriteFile(mp3File, mp3Data, 0644); err != nil {
		return fmt.Errorf("write mp3: %w", err)
	}

	// Convert to PCM using ffmpeg
	cmd := exec.Command("ffmpeg", "-y", "-i", mp3File, "-f", "s16le", "-ar", "16000", "-ac", "1", outputFile)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("convert to PCM: %w", err)
	}

	fmt.Printf("   ✅ Generated: %s\n", outputFile)
	return nil
}
