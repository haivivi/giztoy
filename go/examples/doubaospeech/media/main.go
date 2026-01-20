// Media Subtitle Extraction Example
//
// Tests subtitle extraction from video/audio files: ExtractSubtitle, GetSubtitleTask
// Requires a publicly accessible media URL
//
// Usage:
//
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-token'
//	export MEDIA_URL='https://example.com/video.mp4'  # Your media file URL
//	bazel run //go/examples/doubaospeech/media
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
	mediaURL := os.Getenv("MEDIA_URL")

	if appID == "" || token == "" {
		fmt.Println("Please set environment variables:")
		fmt.Println("  export DOUBAO_APP_ID='your-app-id'")
		fmt.Println("  export DOUBAO_TOKEN='your-token'")
		fmt.Println("  export MEDIA_URL='https://example.com/video.mp4'")
		os.Exit(1)
	}

	if mediaURL == "" {
		fmt.Println("⚠️  MEDIA_URL not set")
		fmt.Println("")
		fmt.Println("This example requires a publicly accessible media file URL.")
		fmt.Println("Set the MEDIA_URL environment variable:")
		fmt.Println("  export MEDIA_URL='https://your-server.com/video.mp4'")
		fmt.Println("")
		fmt.Println("Supported formats: mp4, mp3, wav, m4a, flac, avi, mkv")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("              Subtitle Extraction Example")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Media URL: %s\n", mediaURL)

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx := context.Background()

	// Create subtitle extraction task
	fmt.Println("\n📋 [1] Creating subtitle extraction task...")
	task, err := client.Media.ExtractSubtitle(ctx, &ds.SubtitleRequest{
		MediaURL: mediaURL,
		Language: ds.LanguageZhCN,
	})
	if err != nil {
		fmt.Printf("❌ ExtractSubtitle failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Task created: %s\n", task.ID)

	// Poll for result
	fmt.Println("\n📋 [2] Waiting for subtitle extraction...")
	for i := 0; i < 60; i++ { // Max 5 minutes
		status, err := client.Media.GetSubtitleTask(ctx, task.ID)
		if err != nil {
			fmt.Printf("❌ GetSubtitleTask failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\r   Status: %s (attempt %d/60)", status.Status, i+1)

		if status.Status == ds.TaskStatusSuccess {
			fmt.Println("\n\n✅ Subtitle extraction complete!")
			fmt.Println("═══════════════════════════════════════════════════════════")
			fmt.Println("Result:")
			fmt.Println("═══════════════════════════════════════════════════════════")
			if status.Result != nil {
				if status.Result.SubtitleURL != "" {
					fmt.Printf("Subtitle URL: %s\n", status.Result.SubtitleURL)
				}
				if status.Result.Duration > 0 {
					fmt.Printf("Duration: %d seconds\n", status.Result.Duration)
				}
			}
			return
		}

		if status.Status == ds.TaskStatusFailed {
			errMsg := "unknown error"
			if status.Error != nil {
				errMsg = status.Error.Message
			}
			fmt.Printf("\n❌ Task failed: %s\n", errMsg)
			os.Exit(1)
		}

		time.Sleep(5 * time.Second)
	}

	fmt.Println("\n⚠️  Task still processing after 5 minutes. Check manually later.")
}
