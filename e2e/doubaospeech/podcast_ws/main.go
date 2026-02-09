// SAMI Podcast WebSocket Example
//
// Demonstrates the /api/v3/sami/podcasttts endpoint (SAMI Podcast BigModel)
// using the doubaospeech.PodcastService.StreamSAMI() API.
//
// Prerequisites:
//   - Enable the service at: https://console.volcengine.com/speech/service/10028
//   - Resource ID: volc.service_type.10050
//
// Note: This is the SAMI Podcast API (not TTS Podcast).
// For TTS Podcast (/api/v3/tts/podcast), see the podcast_tts example.
//
// Doc: https://www.volcengine.com/docs/6561/1668014
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/podcast_ws
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

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           SAMI Podcast WebSocket Example")
	fmt.Println("           /api/v3/sami/podcasttts")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...\n", token[:10])
	fmt.Println()
	fmt.Println("Doc: https://www.volcengine.com/docs/6561/1668014")

	testPodcastV3(appID, token)
}

func testPodcastV3(appID, token string) {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📋 SAMI Podcast WebSocket Streaming")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	// Build SAMI podcast request (action=0: summary generation)
	req := &ds.PodcastSAMIRequest{
		Action:    0,
		InputID:   fmt.Sprintf("test_%d", time.Now().Unix()),
		InputText: "Analyze the current development of large language models, including the latest progress of GPT, Claude, and other models.",
		AudioConfig: &ds.PodcastAudioConfig{
			Format:     "mp3",
			SampleRate: 24000,
			SpeechRate: 0,
		},
		SpeakerInfo: &ds.PodcastSpeakerInfo{
			RandomOrder: true,
			Speakers: []string{
				"zh_male_dayixiansheng_v2_saturn_bigtts",
				"zh_female_mizaitongxue_v2_saturn_bigtts",
			},
		},
		UseHeadMusic: false,
		UseTailMusic: false,
	}

	fmt.Println("\n📡 Opening WebSocket session...")
	fmt.Printf("   Speakers: %v\n", req.SpeakerInfo.Speakers)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	session, err := client.Podcast.StreamSAMI(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed to open session: %v\n", err)
		return
	}
	defer session.Close()

	fmt.Println("✅ Session opened!")

	// Collect audio data
	fmt.Println("\n📥 Receiving audio chunks...")
	var allAudio []byte
	startTime := time.Now()

	for chunk, err := range session.Recv() {
		if err != nil {
			fmt.Printf("❌ Receive error: %v\n", err)
			break
		}

		elapsed := time.Since(startTime).Seconds()

		if len(chunk.Audio) > 0 {
			allAudio = append(allAudio, chunk.Audio...)
			fmt.Printf("   [%.1fs] 🔊 +%d bytes (total: %.1f KB)\n",
				elapsed, len(chunk.Audio), float64(len(allAudio))/1024)
		}

		if chunk.Event != "" {
			fmt.Printf("   [%.1fs] Event: %s", elapsed, chunk.Event)
			if chunk.Text != "" {
				fmt.Printf(" - %s", truncate(chunk.Text, 50))
			}
			if chunk.Message != "" {
				fmt.Printf(" - %s", chunk.Message)
			}
			fmt.Println()
		}

		if chunk.IsLast {
			fmt.Printf("   [%.1fs] ✅ Stream completed\n", elapsed)
			break
		}
	}

	// Save audio
	if len(allAudio) > 0 {
		outputFile := "tmp/podcast_output.mp3"
		os.MkdirAll("tmp", 0755)
		if err := os.WriteFile(outputFile, allAudio, 0644); err != nil {
			fmt.Printf("❌ Failed to save: %v\n", err)
		} else {
			fmt.Printf("\n✅ Podcast audio saved: %s (%.1f KB)\n", outputFile, float64(len(allAudio))/1024)
			fmt.Println("   Play: ffplay tmp/podcast_output.mp3")
		}
	} else {
		fmt.Println("\n❌ No audio data received")
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("                     Example Complete")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
