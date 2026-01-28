// V3 API Connectivity Example
//
// Tests V3 API endpoint connectivity using SDK high-level APIs.
//
// Usage:
//
//	export DOUBAO_APP_ID="your_app_id"
//	export DOUBAO_TOKEN="your_token"
//	bazel run //go/examples/doubaospeech/v3_api
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
		os.Exit(1)
	}

	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Token: %s...\n", token[:10])

	// Create client
	client := ds.NewClient(appID, ds.WithBearerToken(token))

	// Test 1: V3 Realtime Dialog
	fmt.Println("\n=== Test 1: V3 Realtime Dialog ===")
	testRealtimeDialog(client)

	// Test 2: V3 TTS Bidirection
	fmt.Println("\n=== Test 2: V3 TTS Bidirection ===")
	testTTSBidirection(client)
}

func testRealtimeDialog(client *ds.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := &ds.RealtimeConfig{
		TTS: ds.RealtimeTTSConfig{
			AudioConfig: ds.RealtimeAudioConfig{
				Format:     "ogg_opus",
				SampleRate: 16000,
				Channel:    1,
			},
		},
		Dialog: ds.RealtimeDialogConfig{
			BotName:    "Test",
			SystemRole: "Test assistant",
		},
	}

	fmt.Println("Connecting to Realtime API...")
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer session.Close()

	fmt.Println("✅ Connection established!")
}

func testTTSBidirection(client *ds.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Opening TTS V3 session...")
	session, err := client.TTSV2.OpenSession(ctx, &ds.TTSV2SessionConfig{
		Speaker:    "zh_female_cancan",
		ResourceID: ds.ResourceTTSV2,
	})
	if err != nil {
		fmt.Printf("❌ Session open failed: %v\n", err)
		return
	}
	defer session.Close()

	fmt.Println("✅ Session established!")
}
