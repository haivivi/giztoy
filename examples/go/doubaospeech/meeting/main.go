// Meeting Transcription Example
//
// Tests meeting transcription: CreateTask, GetTask
// Requires a publicly accessible audio URL
//
// Usage:
//
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-token'
//	export AUDIO_URL='https://example.com/meeting.mp3'  # Your audio file URL
//	bazel run //go/examples/doubaospeech/meeting
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
		fmt.Println("  export DOUBAO_TOKEN='your-token'")
		fmt.Println("  export AUDIO_URL='https://example.com/meeting.mp3'")
		os.Exit(1)
	}

	if audioURL == "" {
		fmt.Println("⚠️  AUDIO_URL not set")
		fmt.Println("")
		fmt.Println("This example requires a publicly accessible audio file URL.")
		fmt.Println("Set the AUDIO_URL environment variable:")
		fmt.Println("  export AUDIO_URL='https://your-server.com/meeting.mp3'")
		fmt.Println("")
		fmt.Println("Supported formats: mp3, wav, m4a, flac, ogg")
		fmt.Println("Maximum duration: depends on your service quota")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("              Meeting Transcription Example")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Audio URL: %s\n", audioURL)

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx := context.Background()

	// Create meeting transcription task
	fmt.Println("\n📋 [1] Creating meeting transcription task...")
	task, err := client.Meeting.CreateTask(ctx, &ds.MeetingTaskRequest{
		AudioURL:                 audioURL,
		Language:                 ds.LanguageZhCN,
		EnableSpeakerDiarization: true,
		EnableTimestamp:          true,
	})
	if err != nil {
		fmt.Printf("❌ CreateTask failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Task created: %s\n", task.ID)

	// Poll for result
	fmt.Println("\n📋 [2] Waiting for transcription...")
	for i := 0; i < 60; i++ { // Max 5 minutes
		status, err := client.Meeting.GetTask(ctx, task.ID)
		if err != nil {
			fmt.Printf("❌ GetTask failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\r   Status: %s (attempt %d/60)", status.Status, i+1)

		if status.Status == ds.TaskStatusSuccess {
			fmt.Println("\n\n✅ Transcription complete!")
			fmt.Println("═══════════════════════════════════════════════════════════")
			fmt.Println("Transcription Result:")
			fmt.Println("═══════════════════════════════════════════════════════════")
			if status.Result != nil {
				fmt.Println(status.Result.Text)
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
