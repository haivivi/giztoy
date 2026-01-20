// TTS Realtime API Example
//
// Demonstrates the /api/v3/realtime/dialogue endpoint (Speech-to-Speech BigModel)
// using the doubaospeech.RealtimeService.Connect() API.
//
// Doc: https://www.volcengine.com/docs/6561/1354629
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/tts_realtime
package main

import (
	"context"
	"fmt"
	"os"
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

	testRealtimeTTS(appID, token)
}

func testRealtimeTTS(appID, token string) {
	fmt.Println("\n=== Testing TTS via Realtime API (volc.speech.dialog) ===")

	// Create client
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
			BotName:    "Assistant",
			SystemRole: "You are a friendly assistant, respond briefly",
			Extra: map[string]any{
				"input_mod": "keep_alive",
				"model":     "O",
			},
		},
	}

	fmt.Println("[1] Connecting to Realtime API...")
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer session.Close()
	fmt.Println("✅ Connected!")

	// Send hello text to trigger TTS
	fmt.Println("\n[2] Sending hello text to trigger TTS...")
	if err := session.SendText(ctx, "Hello, introduce yourself briefly."); err != nil {
		fmt.Printf("❌ Failed to send text: %v\n", err)
		return
	}
	fmt.Println("   Text sent!")

	// Collect audio data
	fmt.Println("\n[3] Receiving audio data...")
	var audioData []byte
	startTime := time.Now()

	for event, err := range session.Recv() {
		if err != nil {
			fmt.Printf("   Read finished: %v\n", err)
			break
		}

		elapsed := time.Since(startTime).Seconds()

		if len(event.Audio) > 0 {
			audioData = append(audioData, event.Audio...)
			fmt.Printf("   🔊 Audio: +%d bytes (total: %d)\n", len(event.Audio), len(audioData))
		}

		if event.Text != "" {
			fmt.Printf("   📝 Text: %s\n", event.Text)
		}

		fmt.Printf("   [%.1fs] Event type: %d\n", elapsed, event.Type)

		// Check for TTS end event
		if event.Type == ds.EventTTSFinished {
			fmt.Println("   ✅ TTS completed")
			break
		}
	}

	// Save audio
	if len(audioData) > 0 {
		filename := "tmp/tts_realtime.ogg"
		os.MkdirAll("tmp", 0755)
		os.WriteFile(filename, audioData, 0644)
		fmt.Printf("\n✅ Audio saved: %s (%d bytes)\n", filename, len(audioData))
	} else {
		fmt.Println("\n❌ No audio received")
	}

	fmt.Println("\n✅ Example completed!")
}
