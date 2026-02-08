// TTS V3 Bidirectional WebSocket Example
//
// Demonstrates the /api/v3/tts/bidirection endpoint (TTS 2.0 BigModel)
// using the doubaospeech.TTSServiceV2.OpenSession() API.
//
// Prerequisites:
//   - Enable BigModel TTS bidirectional streaming service
//   - Resource ID: seed-tts-2.0 or seed-tts-1.0
//
// Note: For simple TTS synthesis, use the HTTP streaming API (TTSV2.Stream())
// which is simpler and doesn't require WebSocket bidirectional setup.
//
// Doc: https://www.volcengine.com/docs/6561/1329505
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/tts_v3
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
		fmt.Println("Please set DOUBAO_APP_ID and DOUBAO_TOKEN environment variables")
		fmt.Println("  export DOUBAO_APP_ID=<your_app_id>")
		fmt.Println("  export DOUBAO_TOKEN=<your_access_token>")
		os.Exit(1)
	}

	fmt.Println("=== TTS 2.0 BigModel Bidirectional WebSocket Example ===")
	fmt.Println("Doc: https://www.volcengine.com/docs/6561/1329505")
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...\n\n", token[:min(10, len(token))])

	testTTSV3(appID, token)
}

func testTTSV3(appID, token string) {
	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Open bidirectional session with speaker configuration
	// Note: seed-tts-2.0 requires "_uranus_bigtts" suffix voices
	// seed-tts-1.0 can use "_moon_bigtts" suffix voices
	resourceID := os.Getenv("DOUBAO_RESOURCE_ID")
	if resourceID == "" {
		resourceID = "seed-tts-2.0" // Default to 2.0
	}

	// Choose speaker based on resource ID
	speaker := "zh_female_xiaohe_uranus_bigtts" // Works with seed-tts-2.0
	if resourceID == "seed-tts-1.0" {
		speaker = "zh_female_shuangkuaisisi_moon_bigtts" // Works with seed-tts-1.0
	}

	fmt.Println("[1] Opening TTS V3 session...")
	fmt.Printf("   Resource ID: %s\n", resourceID)
	fmt.Printf("   Speaker: %s\n", speaker)
	session, err := client.TTSV2.OpenSession(ctx, &ds.TTSV2SessionConfig{
		Speaker:    speaker,
		ResourceID: resourceID,
		Format:     "mp3",
		SampleRate: 24000,
	})
	if err != nil {
		fmt.Printf("❌ Failed to open session: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("   ✅ Session opened")

	// Send text for synthesis
	fmt.Println("\n[2] Sending text for synthesis...")
	text := "Hello, this is a TTS 2.0 BigModel bidirectional streaming example. The weather is nice today, perfect for a walk."
	if err := session.SendText(ctx, text, true); err != nil {
		fmt.Printf("❌ Failed to send text: %v\n", err)
		return
	}
	fmt.Printf("   Text: %s\n", text)

	// Collect audio data
	fmt.Println("\n[3] Receiving audio data...")
	var audioData []byte
	startTime := time.Now()
	chunkCount := 0

	for chunk, err := range session.Recv() {
		chunkCount++
		if err != nil {
			fmt.Printf("❌ Receive error: %v\n", err)
			break
		}

		elapsed := time.Since(startTime).Seconds()

		// Debug: show all received chunks
		fmt.Printf("   [%.1fs] Chunk #%d: audio=%d bytes, isLast=%v, reqID=%s\n",
			elapsed, chunkCount, len(chunk.Audio), chunk.IsLast, chunk.ReqID)
		if len(chunk.Payload) > 0 && len(chunk.Payload) < 200 {
			fmt.Printf("            Payload: %s\n", string(chunk.Payload))
		}

		if len(chunk.Audio) > 0 {
			audioData = append(audioData, chunk.Audio...)
			fmt.Printf("   [%.1fs] 🔊 +%d bytes (total %.1f KB)\n",
				elapsed, len(chunk.Audio), float64(len(audioData))/1024)
		}

		if chunk.IsLast {
			fmt.Printf("   [%.1fs] ✅ Stream completed\n", elapsed)
			break
		}
	}

	fmt.Printf("\n   Total chunks received: %d\n", chunkCount)

	// Save audio
	if len(audioData) > 0 {
		outputFile := "tmp/tts_v3_output.mp3"
		os.MkdirAll("tmp", 0755)
		if err := os.WriteFile(outputFile, audioData, 0644); err != nil {
			fmt.Printf("❌ Save failed: %v\n", err)
		} else {
			fmt.Printf("\n✅ Audio saved: %s (%.1f KB)\n", outputFile, float64(len(audioData))/1024)
			fmt.Printf("   Play command: ffplay '%s'\n", outputFile)
		}
	} else {
		fmt.Println("\n⚠️ No audio data received")
	}
}
