// Podcast TTS WebSocket Example
//
// Demonstrates the /api/v3/tts/podcast endpoint (TTS Podcast)
// using the doubaospeech.PodcastService.Stream() API.
//
// Doc: https://www.volcengine.com/docs/6561/1356830
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	export DOUBAO_CLUSTER="volcano_tts" # optional
//	bazel run //examples/go/doubaospeech/podcast_tts
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
	cluster := os.Getenv("DOUBAO_CLUSTER")
	if cluster == "" {
		cluster = "volcano_tts"
	}

	if appID == "" || token == "" {
		fmt.Println("Please set DOUBAO_APP_ID and DOUBAO_TOKEN")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           Podcast TTS WebSocket Example")
	fmt.Println("           /api/v3/tts/podcast")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...\n", token[:min(10, len(token))])
	fmt.Printf("Cluster: %s\n", cluster)
	fmt.Println()
	fmt.Println("Doc: https://www.volcengine.com/docs/6561/1356830")

	testPodcastTTS(appID, token, cluster)
}

func testPodcastTTS(appID, token, cluster string) {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📋 TTS Podcast WebSocket Streaming")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Create client
	client := ds.NewClient(appID,
		ds.WithBearerToken(token),
		ds.WithCluster(cluster),
	)

	// Build podcast request with speakers and dialogues
	req := &ds.PodcastStreamRequest{
		Speakers: []ds.PodcastSpeaker{
			{Name: "主持人A", VoiceType: "zh_male_yangguang"},
			{Name: "主持人B", VoiceType: "zh_female_cancan"},
		},
		Dialogues: []ds.PodcastDialogueLine{
			{Speaker: "主持人A", Text: "大家好，欢迎收听今天的节目。我是主持人A。"},
			{Speaker: "主持人B", Text: "大家好，我是主持人B。今天我们要聊一个很有趣的话题。"},
			{Speaker: "主持人A", Text: "没错，今天我们要讨论的是人工智能的最新进展。"},
			{Speaker: "主持人B", Text: "AI技术发展真的是日新月异，让人感叹科技的力量。"},
		},
		Encoding:   ds.EncodingMP3,
		SampleRate: 24000,
	}

	fmt.Println("\n📡 Opening WebSocket session...")
	fmt.Println("   Speakers:")
	for _, sp := range req.Speakers {
		fmt.Printf("     - %s (%s)\n", sp.Name, sp.VoiceType)
	}
	fmt.Printf("   Dialogues: %d lines\n", len(req.Dialogues))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	session, err := client.Podcast.Stream(ctx, req)
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
			fmt.Printf("   [%.1fs] 🔊 Seq=%d Speaker=%s +%d bytes (total: %.1f KB)\n",
				elapsed, chunk.Sequence, chunk.Speaker, len(chunk.Audio), float64(len(allAudio))/1024)
		}

		if chunk.Message != "" {
			fmt.Printf("   [%.1fs] 📌 %s\n", elapsed, chunk.Message)
		}

		if chunk.IsLast {
			fmt.Printf("   [%.1fs] ✅ Stream completed", elapsed)
			if chunk.Duration > 0 {
				fmt.Printf(" (duration: %dms)", chunk.Duration)
			}
			fmt.Println()
			break
		}
	}

	// Save audio
	if len(allAudio) > 0 {
		outputFile := "tmp/podcast_tts_output.mp3"
		os.MkdirAll("tmp", 0755)
		if err := os.WriteFile(outputFile, allAudio, 0644); err != nil {
			fmt.Printf("❌ Failed to save: %v\n", err)
		} else {
			fmt.Printf("\n✅ Podcast audio saved: %s (%.1f KB)\n", outputFile, float64(len(allAudio))/1024)
			fmt.Println("   Play: ffplay tmp/podcast_tts_output.mp3")
		}
	} else {
		fmt.Println("\n❌ No audio data received")
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("                     Example Complete")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
