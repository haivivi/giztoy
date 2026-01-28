// Console API Tests
//
// Tests Console API：ListTimbres, ListSpeakers, ListVoiceCloneStatus
//
// Usage:
//
//	Method 1: API Key (recommended)
//	export DOUBAO_API_KEY='your-api-key'
//
//	Method 2: AK/SK
//	export VOLC_ACCESS_KEY='your-access-key'
//	export VOLC_SECRET_KEY='your-secret-key'
//
//	go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

func main() {
	// Prefer API Key (recommended)
	apiKey := os.Getenv("DOUBAO_API_KEY")

	// Fallback: use AK/SK
	accessKey := os.Getenv("VOLC_ACCESS_KEY")
	secretKey := os.Getenv("VOLC_SECRET_KEY")

	var console *doubaospeech.Console

	if apiKey != "" {
		fmt.Println("Using API Key authentication")
		console = doubaospeech.NewConsoleWithAPIKey(apiKey)
	} else if accessKey != "" && secretKey != "" {
		fmt.Println("Using AK/SK authentication")
		console = doubaospeech.NewConsole(accessKey, secretKey)
	} else {
		fmt.Println("Please set authentication credentials:")
		fmt.Println("")
		fmt.Println("Method 1: API Key (recommended)")
		fmt.Println("  export DOUBAO_API_KEY='your-api-key'")
		fmt.Println("  Get API Key at: https://console.volcengine.com/speech/new/setting/apikeys")
		fmt.Println("")
		fmt.Println("Method 2: AK/SK")
		fmt.Println("  export VOLC_ACCESS_KEY='your-access-key'")
		fmt.Println("  export VOLC_SECRET_KEY='your-secret-key'")
		fmt.Println("  Get credentials at: https://console.volcengine.com/iam/keymanage/")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           Doubao Speech Console API Tests")
	fmt.Println("═══════════════════════════════════════════════════════════")

	ctx := context.Background()

	// Test 1: List voices (BigModel TTS)
	fmt.Println("\n📋 Tests ListTimbres (ListBigModelTTSTimbres)...")
	timbres, err := console.ListTimbres(ctx, &doubaospeech.ListTimbresRequest{
		PageNumber: 1,
		PageSize:   5,
	})
	if err != nil {
		fmt.Printf("❌ ListTimbres Failed: %v\n", err)
	} else {
		fmt.Printf("✅ ListTimbres Success, total %d voices\n", len(timbres.Timbres))
		for i, t := range timbres.Timbres {
			if i >= 5 {
				fmt.Printf("   ... (showing first 5)\n")
				break
			}
			if len(t.TimbreInfos) > 0 {
				info := t.TimbreInfos[0]
				fmt.Printf("   - %s | %s | %s/%s\n", t.SpeakerID, info.SpeakerName, info.Gender, info.Age)
			} else {
				fmt.Printf("   - %s\n", t.SpeakerID)
			}
		}
	}

	// Test 2: List speakers (new API)
	fmt.Println("\n📋 Tests ListSpeakers...")
	speakers, err := console.ListSpeakers(ctx, &doubaospeech.ListSpeakersRequest{
		PageNumber: 1,
		PageSize:   100,
	})
	if err != nil {
		fmt.Printf("❌ ListSpeakers Failed: %v\n", err)
	} else {
		fmt.Printf("✅ ListSpeakers Success, total %d speakers\n", speakers.Total)
		// Show all with saturn or podcast
		fmt.Println("   === Saturn/Podcast Voices ===")
		for _, s := range speakers.Speakers {
			if strings.Contains(s.VoiceType, "saturn") || strings.Contains(s.VoiceType, "podcast") {
				fmt.Printf("   - %s | %s\n", s.VoiceType, s.Name)
			}
		}
		// Show first 10
		fmt.Println("   === First 10 ===")
		for i, s := range speakers.Speakers {
			if i >= 10 {
				break
			}
			fmt.Printf("   - %s | %s | %s/%s\n", s.VoiceType, s.Name, s.Gender, s.Age)
		}
	}

	// Test 3: List voice clone status
	fmt.Println("\n📋 Tests ListVoiceCloneStatus...")
	cloneStatus, err := console.ListVoiceCloneStatus(ctx, &doubaospeech.ListVoiceCloneStatusRequest{
		AppID:      "9476442538", // Need actual AppID
		PageNumber: 1,
		PageSize:   50,
	})
	if err != nil {
		fmt.Printf("❌ ListVoiceCloneStatus Failed: %v\n", err)
	} else {
		// Count by state
		successCount := 0
		unknownCount := 0
		for _, s := range cloneStatus.Statuses {
			if s.State == "Success" {
				successCount++
			} else if s.State == "Unknown" {
				unknownCount++
			}
		}
		fmt.Printf("✅ ListVoiceCloneStatus Success\n")
		fmt.Printf("   Total slots: %d\n", cloneStatus.Total)
		fmt.Printf("   Trained (Success): %d\n", successCount)
		fmt.Printf("   Available (Unknown): %d\n", unknownCount)
		fmt.Println("\n   === Detailed Status ===")
		for _, s := range cloneStatus.Statuses {
			fmt.Printf("   - %s | Alias: %s | State: %s | Version: %s | ResourceID: %s\n",
				s.SpeakerID, s.Alias, s.State, s.Version, s.ResourceID)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("                     TestsComplete")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
