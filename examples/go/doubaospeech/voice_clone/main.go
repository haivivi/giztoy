// Voice Clone Example
//
// This example demonstrates:
// 1. List existing cloned voices
// 2. Use a cloned voice for TTS
// 3. (Optional) Train a new voice with provided voice ID
//
// Usage:
//
//	# List existing voice clones and test TTS with one
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-token'
//	export DOUBAO_API_KEY='your-api-key'
//	bazel run //go/examples/doubaospeech/voice_clone
//
//	# Train a new voice (requires purchased voice ID)
//	export VOICE_ID='S_xxxxxxxxx'  # Your purchased voice ID
//	bazel run //go/examples/doubaospeech/voice_clone
package main

import (
	"context"
	"fmt"
	"os"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

func main() {
	appID := os.Getenv("DOUBAO_APP_ID")
	token := os.Getenv("DOUBAO_TOKEN")
	apiKey := os.Getenv("DOUBAO_API_KEY")
	voiceID := os.Getenv("VOICE_ID") // Optional: for training new voice

	if appID == "" || token == "" {
		fmt.Println("Please set environment variables:")
		fmt.Println("  export DOUBAO_APP_ID='your-app-id'")
		fmt.Println("  export DOUBAO_TOKEN='your-token'")
		fmt.Println("  export DOUBAO_API_KEY='your-api-key'  # For listing")
		fmt.Println("  export VOICE_ID='S_xxx'  # Optional: for training")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("              Voice Clone Example")
	fmt.Println("═══════════════════════════════════════════════════════════")

	ctx := context.Background()
	client := ds.NewClient(appID,
		ds.WithBearerToken(token),
		ds.WithCluster("volcano_icl"), // For voice clone TTS
	)

	// Step 1: List existing voice clones
	var availableVoices []string
	if apiKey != "" {
		fmt.Println("\n📋 [1] Listing existing voice clones...")
		console := ds.NewConsoleWithAPIKey(apiKey)

		statuses, err := console.ListVoiceCloneStatus(ctx, &ds.ListVoiceCloneStatusRequest{
			AppID:      appID,
			PageNumber: 1,
			PageSize:   50,
		})
		if err != nil {
			fmt.Printf("   ⚠️  ListVoiceCloneStatus failed: %v\n", err)
		} else {
			fmt.Printf("   Found %d voice clone(s)\n", statuses.Total)
			for _, s := range statuses.Statuses {
				status := "⏳"
				if s.State == "Success" {
					status = "✅"
					availableVoices = append(availableVoices, s.SpeakerID)
				} else if s.State == "Failed" {
					status = "❌"
				}
				name := s.Alias
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Printf("   %s %s - %s\n", status, s.SpeakerID, name)
			}
		}
	}

	// Step 2: Test TTS with a cloned voice
	// Use a known working voice ID for testing
	testVoiceID := "S_TR0rbVuI1" // 小茧
	if len(availableVoices) > 0 {
		testVoiceID = availableVoices[0]
	}

	fmt.Printf("\n📋 [2] Testing TTS with cloned voice: %s\n", testVoiceID)

	result, err := client.TTS.Synthesize(ctx, &ds.TTSRequest{
		Text:      "你好，这是使用复刻声音生成的语音测试。声音复刻技术让每个人都可以拥有自己独特的AI声音。",
		VoiceType: testVoiceID,
		Encoding:  ds.EncodingMP3,
	})
	if err != nil {
		fmt.Printf("   ❌ TTS failed: %v\n", err)
		fmt.Println("   💡 Make sure cluster is set to 'volcano_icl' for cloned voices")
	} else {
		// Save audio
		os.MkdirAll("tmp", 0755)
		outputFile := fmt.Sprintf("tmp/voice_clone_%s.mp3", testVoiceID)
		if err := os.WriteFile(outputFile, result.Audio, 0644); err != nil {
			fmt.Printf("   ❌ Failed to save: %v\n", err)
		} else {
			fmt.Printf("   ✅ Generated: %s (%d bytes)\n", outputFile, len(result.Audio))
		}
	}

	// Step 3: Train new voice (if VOICE_ID provided)
	if voiceID != "" {
		fmt.Printf("\n📋 [3] Training new voice: %s\n", voiceID)
		fmt.Println("   ⚠️  Training requires:")
		fmt.Println("      - A purchased voice ID slot")
		fmt.Println("      - Audio file (10-60 seconds)")
		fmt.Println("")
		fmt.Println("   To train, prepare audio and call:")
		fmt.Println("      client.VoiceClone.Train(ctx, &ds.VoiceCloneTrainRequest{")
		fmt.Printf("          SpeakerID: \"%s\",\n", voiceID)
		fmt.Println("          AudioData: [][]byte{wavData},")
		fmt.Println("          Language:  ds.LanguageZhCN,")
		fmt.Println("      })")
	}

	// Summary
	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("💡 Voice Clone Usage:")
	fmt.Println("")
	fmt.Println("   Use cloned voice in TTS:")
	fmt.Println("      Cluster: volcano_icl")
	fmt.Println("      Speaker: S_xxxxxxxxx (your voice ID)")
	fmt.Println("")
	fmt.Println("   Train new voice:")
	fmt.Println("      1. Purchase voice ID slot in Volcengine Console")
	fmt.Println("      2. Prepare audio (10-60 seconds, clear speech)")
	fmt.Println("      3. Call VoiceClone.Train() with your voice ID")
	fmt.Println("")
	fmt.Println("📚 Documentation:")
	fmt.Println("   https://www.volcengine.com/docs/6561/1305191")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
