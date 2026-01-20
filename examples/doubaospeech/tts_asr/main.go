// TTS + ASR Integration Example
//
// Demonstrates end-to-end flow: generate audio with TTS, then recognize with ASR
// using doubaospeech.RealtimeService and ASRServiceV2.
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/tts_asr
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

func main() {
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")

	if appID == "" || token == "" {
		fmt.Println("❌ Please set DOUBAO_APP_ID and DOUBAO_TOKEN")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           Doubao TTS + ASR Integration Example")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...%s\n", token[:4], token[len(token)-4:])
	fmt.Println()

	// Test text
	testText := "Hello, this is a test. Doubao text-to-speech sounds great!"
	fmt.Printf("📝 Test text: %s\n", testText)
	fmt.Println()

	// 1. TTS Test
	audioFile, err := testTTS(appID, token, testText)
	if err != nil {
		fmt.Printf("❌ TTS failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ TTS success, audio saved to: %s\n", audioFile)

	// 2. ASR Test
	fmt.Println()
	fmt.Println("🎧 Starting ASR test...")

	result, err := testASR(appID, token, audioFile)
	if err != nil {
		fmt.Printf("❌ ASR failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ ASR success, result: %s\n", result)

	// 3. Compare results
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                     Result Comparison")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Original text: %s\n", testText)
	fmt.Printf("ASR result:    %s\n", result)
	fmt.Println("═══════════════════════════════════════════════════════════")

	// 4. Play audio (optional)
	fmt.Println()
	fmt.Print("Play audio? [y/N]: ")
	var answer string
	fmt.Scanln(&answer)
	if answer == "y" || answer == "Y" {
		playAudio(audioFile)
	}
}

func testTTS(appID, token, text string) (string, error) {
	fmt.Println("📡 Connecting to Realtime API...")

	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Configure realtime session
	config := &ds.RealtimeConfig{
		TTS: ds.RealtimeTTSConfig{
			AudioConfig: ds.RealtimeAudioConfig{
				Channel:    1,
				Format:     "ogg_opus",
				SampleRate: 16000,
			},
		},
		Dialog: ds.RealtimeDialogConfig{
			BotName:    "Test Assistant",
			SystemRole: "You are a TTS test assistant, just read the text provided by the user",
			Extra: map[string]any{
				"input_mod": "keep_alive",
				"model":     "O",
			},
		},
	}

	fmt.Println("   [1] Connecting...")
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer session.Close()
	fmt.Println("   ✅ Connected")

	// Send text for TTS
	fmt.Println("   [2] Sending TTS request...")
	if err := session.SendText(ctx, text); err != nil {
		return "", fmt.Errorf("send text failed: %w", err)
	}

	// Collect audio
	fmt.Println("   [3] Receiving audio data...")
	var audioData []byte
	for event, err := range session.Recv() {
		if err != nil {
			break
		}
		if len(event.Audio) > 0 {
			audioData = append(audioData, event.Audio...)
			fmt.Printf("\r      Received: %d bytes", len(audioData))
		}
		if event.Type == ds.EventTTSFinished {
			fmt.Println("\n      TTS complete")
			break
		}
	}

	if len(audioData) == 0 {
		return "", fmt.Errorf("no audio data received")
	}

	// Save audio
	os.MkdirAll("tmp", 0755)
	audioFile := "tmp/tts_output.ogg"
	if err := os.WriteFile(audioFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("save audio failed: %w", err)
	}

	return audioFile, nil
}

func testASR(appID, token, audioFile string) (string, error) {
	// Convert to PCM
	fmt.Println("   🔄 Converting audio format...")
	pcmFile := "tmp/audio_for_asr.pcm"
	cmd := exec.Command("ffmpeg", "-y", "-i", audioFile, "-f", "s16le", "-ar", "16000", "-ac", "1", pcmFile)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("conversion failed: %w", err)
	}

	pcmData, err := os.ReadFile(pcmFile)
	if err != nil {
		return "", fmt.Errorf("read PCM failed: %w", err)
	}
	fmt.Printf("      PCM size: %d bytes (%.1f seconds)\n", len(pcmData), float64(len(pcmData))/32000)

	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Configure ASR
	config := &ds.ASRV2Config{
		Format:     "pcm",
		SampleRate: 16000,
		Bits:       16,
		Channels:   1,
		EnableITN:  true,
		EnablePunc: true,
	}

	fmt.Println("   📡 Connecting to ASR API...")
	session, err := client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer session.Close()
	fmt.Println("   ✅ Connected")

	// Start result receiver
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		var lastText string
		for result, err := range session.Recv() {
			if err != nil {
				if lastText != "" {
					resultCh <- lastText
				} else {
					errCh <- fmt.Errorf("read failed: %w", err)
				}
				return
			}
			if result.Text != "" {
				lastText = result.Text
				status := "interim"
				if result.IsFinal {
					status = "final"
				}
				fmt.Printf("      [%s] %s\n", status, result.Text)
				if result.IsFinal {
					resultCh <- result.Text
					return
				}
			}
		}
	}()

	// Send audio data
	fmt.Println("   📤 Sending audio data...")
	chunkSize := 3200 // 100ms of 16kHz mono 16bit
	var totalSent int

	for i := 0; i < len(pcmData); i += chunkSize {
		end := i + chunkSize
		if end > len(pcmData) {
			end = len(pcmData)
		}
		chunk := pcmData[i:end]
		isLast := end >= len(pcmData)

		if err := session.SendAudio(ctx, chunk, isLast); err != nil {
			return "", fmt.Errorf("send audio failed: %w", err)
		}
		totalSent += len(chunk)
		fmt.Printf("\r      Sent: %d / %d bytes", totalSent, len(pcmData))
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("\n      Audio send complete\n")

	// Wait for result
	fmt.Println("   📥 Waiting for result...")
	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return "", err
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("timeout")
	}
}

func playAudio(file string) {
	fmt.Printf("🔊 Playing: %s\n", file)
	cmd := exec.Command("ffplay", "-nodisp", "-autoexit", file)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Run()
}
