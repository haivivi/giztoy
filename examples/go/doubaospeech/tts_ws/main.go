// TTS WebSocket V1 Example
//
// Demonstrates the /api/v1/tts/ws_binary endpoint (TTS 1.0)
// using the doubaospeech.TTSService API.
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/tts_ws
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

func main() {
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")

	if appID == "" || token == "" {
		fmt.Println("Please set DOUBAO_APP_ID and DOUBAO_TOKEN")
		os.Exit(1)
	}

	// Test TTS 1.0
	testTTSWebSocket(appID, token, "volcano_tts", "BV001_streaming", "TTS 1.0")

	// Test TTS 2.0 (via Stream API)
	testTTSV2Stream(appID, token)
}

func testTTSWebSocket(appID, token, cluster, voice, label string) {
	fmt.Printf("\n=== Testing %s (cluster=%s, voice=%s) ===\n", label, cluster, voice)

	// Create client
	client := ds.NewClient(appID,
		ds.WithBearerToken(token),
		ds.WithCluster(cluster),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Synthesize text
	req := &ds.TTSRequest{
		Text:      "Hello, this is a test.",
		VoiceType: voice,
		Encoding:  ds.EncodingMP3,
	}

	fmt.Println("Synthesizing...")
	result, err := client.TTS.Synthesize(ctx, req)
	if err != nil {
		fmt.Printf("❌ Synthesis failed: %v\n", err)
		return
	}

	if len(result.Audio) > 0 {
		os.MkdirAll("tmp", 0755)
		filename := fmt.Sprintf("tmp/tts_%s.mp3", cluster)
		os.WriteFile(filename, result.Audio, 0644)
		fmt.Printf("✅ Audio saved: %s (%d bytes)\n", filename, len(result.Audio))
	} else {
		fmt.Println("❌ No audio received")
	}
}

func testTTSV2Stream(appID, token string) {
	fmt.Printf("\n=== Testing TTS 2.0 HTTP Stream ===\n")

	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Synthesize with TTS V2 streaming
	req := &ds.TTSV2Request{
		Text:       "Hello, this is a TTS 2.0 streaming test.",
		Speaker:    "zh_female_vv_uranus_bigtts",
		Format:     "mp3",
		SampleRate: 24000,
	}

	fmt.Printf("Speaker: %s\n", req.Speaker)
	fmt.Println("Synthesizing...")

	var audioData []byte
	for chunk, err := range client.TTSV2.Stream(ctx, req) {
		if err != nil {
			fmt.Printf("❌ Stream error: %v\n", err)
			break
		}
		if len(chunk.Audio) > 0 {
			audioData = append(audioData, chunk.Audio...)
			fmt.Printf("   +%d bytes (total: %.1f KB)\n", len(chunk.Audio), float64(len(audioData))/1024)
		}
		if chunk.IsLast {
			break
		}
	}

	if len(audioData) > 0 {
		os.MkdirAll("tmp", 0755)
		filename := "tmp/tts_v2_stream.mp3"
		os.WriteFile(filename, audioData, 0644)
		fmt.Printf("✅ Audio saved: %s (%.1f KB)\n", filename, float64(len(audioData))/1024)
	} else {
		fmt.Println("❌ No audio received")
	}
}
