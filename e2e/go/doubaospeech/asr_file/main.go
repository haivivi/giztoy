// ASR File Recognition Example
//
// Tests async file recognition API for long audio files.
//
// Usage:
//
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-access-token'
//	export AUDIO_URL='https://example.com/audio.mp3'
//	go run main.go
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
	audioURL := os.Getenv("AUDIO_URL")

	if appID == "" || token == "" {
		fmt.Println("Please set environment variables:")
		fmt.Println("  export DOUBAO_APP_ID='your-app-id'")
		fmt.Println("  export DOUBAO_TOKEN='your-access-token'")
		fmt.Println("  export AUDIO_URL='https://example.com/audio.mp3'")
		os.Exit(1)
	}

	if audioURL == "" {
		// Use a sample audio URL for testing
		audioURL = "https://cdn.example.com/sample.mp3"
		fmt.Printf("Using sample audio URL: %s\n", audioURL)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           ASR File Recognition Example")
	fmt.Println("═══════════════════════════════════════════════════════════")

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx := context.Background()

	// Create async recognition task
	fmt.Println("\n📤 Creating async file recognition task...")
	req := &ds.ASRV2AsyncRequest{
		AudioURL: audioURL,
		Language: string(ds.LanguageZhCN),
		Format:   string(ds.FormatMP3),
	}

	result, err := client.ASRV2.SubmitAsync(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed to create task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Task created: %s\n", result.TaskID)

	// Poll for task completion
	fmt.Println("\n⏳ Waiting for task completion...")
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)

		status, err := client.ASRV2.QueryAsync(ctx, result.TaskID)
		if err != nil {
			fmt.Printf("❌ Failed to get status: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("   Status: %s\n", status.Status)

		if status.Status == "success" {
			fmt.Println("\n✅ Recognition completed!")
			fmt.Println("\n📝 Result:")
			if status.Text != "" {
				fmt.Printf("   Text: %s\n", status.Text)
			}
			if len(status.Utterances) > 0 {
				fmt.Println("   Utterances:")
				for i, u := range status.Utterances {
					fmt.Printf("     [%d] %.2fs - %.2fs: %s\n",
						i+1, float64(u.StartTime)/1000, float64(u.EndTime)/1000, u.Text)
				}
			}
			break
		} else if status.Status == "failed" {
			fmt.Printf("❌ Task failed: %s\n", status.Error)
			os.Exit(1)
		}
		// status == "pending" or "processing" - continue polling
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("                     Complete")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
