// ASR WebSocket V2 Example
//
// Demonstrates the /api/v2/asr endpoint (ASR V1/V2)
// using the doubaospeech.ASRService API.
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/asr_ws [audio_file]
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
	fmt.Println("           ASR WebSocket V2 Example")
	fmt.Println("═══════════════════════════════════════════════════════════")

	// Determine audio file
	audioFile := "tmp/audio_for_asr.pcm"
	if len(os.Args) > 1 {
		audioFile = os.Args[1]
	}

	// Check audio file
	if _, err := os.Stat(audioFile); os.IsNotExist(err) {
		// Try to convert from TTS output
		ttsOutput := "tmp/tts_output.ogg"
		if _, err := os.Stat(ttsOutput); err == nil {
			fmt.Println("🔄 Converting TTS output to PCM...")
			if err := convertToPCM(ttsOutput, audioFile); err != nil {
				fmt.Printf("❌ Conversion failed: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("❌ Audio file does not exist: %s\n", audioFile)
			fmt.Println("   Please run tts_asr example first to generate audio, or specify an audio file")
			os.Exit(1)
		}
	}

	testASRWebSocket(appID, token, audioFile)
}

func convertToPCM(input, output string) error {
	cmd := exec.Command("ffmpeg", "-y", "-i", input,
		"-f", "s16le", "-acodec", "pcm_s16le",
		"-ar", "16000", "-ac", "1", output)
	return cmd.Run()
}

func testASRWebSocket(appID, token, audioFile string) {
	fmt.Printf("\n📁 Audio file: %s\n", audioFile)

	// Read audio
	audioData, err := os.ReadFile(audioFile)
	if err != nil {
		fmt.Printf("❌ Read audio failed: %v\n", err)
		return
	}
	fmt.Printf("   Size: %d bytes (%.1f seconds @16kHz)\n", len(audioData), float64(len(audioData))/32000)

	// Create client
	client := ds.NewClient(appID,
		ds.WithBearerToken(token),
		ds.WithCluster("volcengine_streaming_common"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Configure ASR
	config := &ds.StreamASRConfig{
		Format:         ds.FormatPCM,
		SampleRate:     ds.SampleRate16000,
		Bits:           16,
		Channel:        1,
		ShowUtterances: true,
	}

	fmt.Println("\n📡 Opening ASR WebSocket session...")
	session, err := client.ASR.OpenStreamSession(ctx, config)
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("✅ Session opened")

	// Start result receiver
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		var finalResult string
		for chunk, err := range session.Recv() {
			if err != nil {
				if err != io.EOF && finalResult == "" {
					errCh <- err
				} else if finalResult != "" {
					resultCh <- finalResult
				}
				return
			}

			if chunk.Text != "" {
				if chunk.IsFinal {
					finalResult = chunk.Text
					fmt.Printf("   [final] %s\n", chunk.Text)
				} else {
					fmt.Printf("   [interim] %s\n", chunk.Text)
				}
			}

			if chunk.IsFinal {
				resultCh <- chunk.Text
				return
			}
		}
	}()

	// Send audio data
	fmt.Println("\n📤 Sending audio data...")
	chunkSize := 3200 // 100ms @16kHz, 16bit, mono
	var totalSent int

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		chunk := audioData[i:end]
		isLast := end >= len(audioData)

		if err := session.SendAudio(ctx, chunk, isLast); err != nil {
			fmt.Printf("❌ Send failed: %v\n", err)
			return
		}
		totalSent += len(chunk)

		// Simulate real-time streaming
		time.Sleep(80 * time.Millisecond)

		if totalSent%32000 == 0 {
			fmt.Printf("      Sent: %d / %d bytes\r", totalSent, len(audioData))
		}
	}
	fmt.Printf("      Sent: %d / %d bytes\n", len(audioData), len(audioData))
	fmt.Println("   Audio send complete")

	// Wait for results
	fmt.Println("\n📥 Waiting for final result...")
	select {
	case result := <-resultCh:
		fmt.Println("\n═══════════════════════════════════════════════════════════")
		fmt.Printf("✅ ASR result: %s\n", result)
		fmt.Println("═══════════════════════════════════════════════════════════")
	case err := <-errCh:
		fmt.Printf("\n❌ Error: %v\n", err)
	case <-time.After(30 * time.Second):
		fmt.Println("\n❌ Timeout")
	}
}

