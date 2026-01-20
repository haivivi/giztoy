// ASR SAUC BigModel Example
//
// Demonstrates the /api/v3/sauc/bigmodel endpoint (BigModel Speech Recognition)
// using the doubaospeech.ASRServiceV2.OpenStreamSession() API.
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/asr_sauc
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
		fmt.Println("Please set DOUBAO_APP_ID and DOUBAO_TOKEN")
		os.Exit(1)
	}

	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...\n", token[:10])

	// First convert OGG to PCM
	inputFile := "tmp/tts_realtime.ogg"
	pcmFile := "tmp/audio.pcm"

	if err := convertToPCM(inputFile, pcmFile); err != nil {
		fmt.Printf("Failed to convert audio: %v\n", err)
		os.Exit(1)
	}

	testASRSauc(appID, token, pcmFile)
}

func convertToPCM(inputFile, outputFile string) error {
	fmt.Printf("\nConverting %s to PCM...\n", inputFile)

	// Use ffmpeg to convert to 16kHz mono 16-bit PCM
	cmd := exec.Command("ffmpeg", "-y", "-i", inputFile,
		"-ar", "16000", "-ac", "1", "-f", "s16le", outputFile)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v\nOutput: %s", err, string(output))
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		return err
	}

	fmt.Printf("PCM file created: %s (%d bytes)\n", outputFile, info.Size())
	return nil
}

func testASRSauc(appID, token, audioFile string) {
	fmt.Println("\n=== Testing ASR SAUC (volc.bigasr.sauc.duration) ===")

	// Read audio file
	audioData, err := os.ReadFile(audioFile)
	if err != nil {
		fmt.Printf("Failed to read audio file: %v\n", err)
		return
	}
	fmt.Printf("Audio data: %d bytes (%.1f seconds @16kHz)\n", len(audioData), float64(len(audioData))/32000)

	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Configure ASR session
	config := &ds.ASRV2Config{
		Format:     "pcm",
		SampleRate: 16000,
		Bits:       16,
		Channels:   1,
		EnableITN:  true,
		EnablePunc: true,
	}

	fmt.Println("\n[1] Opening ASR stream session...")
	session, err := client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		fmt.Printf("❌ Failed to open session: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("   ✅ Session opened")

	// Start result receiver
	fmt.Println("\n[2] Streaming audio and receiving results...")
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		var lastText string
		for result, err := range session.Recv() {
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				if lastText != "" {
					resultCh <- lastText
				}
				return
			}
			if result.Text != "" {
				lastText = result.Text
				status := "interim"
				if result.IsFinal {
					status = "final"
				}
				fmt.Printf("   [%s] %s\n", status, result.Text)
				if result.IsFinal {
					resultCh <- result.Text
					return
				}
			}
		}
	}()

	// Send audio data in chunks
	chunkSize := 3200 // 100ms of 16kHz 16-bit mono audio
	var totalSent int

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		chunk := audioData[i:end]
		isLast := end >= len(audioData)

		if err := session.SendAudio(ctx, chunk, isLast); err != nil {
			fmt.Printf("❌ Send error: %v\n", err)
			return
		}
		totalSent += len(chunk)

		// Simulate real-time streaming
		time.Sleep(50 * time.Millisecond)

		if totalSent%32000 == 0 {
			fmt.Printf("   Sent: %d / %d bytes\n", totalSent, len(audioData))
		}
	}
	fmt.Printf("   Audio sent: %d bytes\n", totalSent)

	// Wait for final result
	fmt.Println("\n[3] Waiting for final result...")
	select {
	case result := <-resultCh:
		fmt.Printf("\n🎉 Recognition result: %s\n", result)
	case err := <-errCh:
		fmt.Printf("\n❌ Error: %v\n", err)
	case <-time.After(30 * time.Second):
		fmt.Println("\n❌ Timeout waiting for result")
	}
}
