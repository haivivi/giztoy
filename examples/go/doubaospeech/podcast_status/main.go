// Podcast Status Example
//
// Tests querying podcast synthesis task status.
//
// Usage:
//
//	export DOUBAO_APP_ID='your-app-id'
//	export DOUBAO_TOKEN='your-access-token'
//	export TASK_ID='your-task-id'
//	go run main.go
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
	taskID := os.Getenv("TASK_ID")

	if appID == "" || token == "" {
		fmt.Println("Please set environment variables:")
		fmt.Println("  export DOUBAO_APP_ID='your-app-id'")
		fmt.Println("  export DOUBAO_TOKEN='your-access-token'")
		fmt.Println("  export TASK_ID='your-task-id'")
		os.Exit(1)
	}

	if taskID == "" {
		fmt.Println("Please set TASK_ID environment variable")
		fmt.Println("  export TASK_ID='your-task-id'")
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("           Podcast Status Example")
	fmt.Println("═══════════════════════════════════════════════════════════")

	client := ds.NewClient(appID, ds.WithBearerToken(token))
	ctx := context.Background()

	fmt.Printf("\n📋 Querying podcast task: %s\n", taskID)

	status, err := client.Podcast.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ Failed to get status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n📊 Task Status:")
	fmt.Printf("   Task ID:    %s\n", status.TaskID)
	fmt.Printf("   Status:     %s\n", status.Status)
	fmt.Printf("   Progress:   %d%%\n", status.Progress)

	switch status.Status {
	case ds.TaskStatusSuccess:
		fmt.Println("\n✅ Task completed!")
		if status.Result != nil && status.Result.AudioURL != "" {
			fmt.Printf("   Audio URL:  %s\n", status.Result.AudioURL)
		}
	case ds.TaskStatusFailed:
		if status.Error != nil {
			fmt.Printf("\n❌ Task failed: %s\n", status.Error.Message)
		} else {
			fmt.Println("\n❌ Task failed")
		}
	default:
		fmt.Println("\n⏳ Task is still processing...")
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("                     Complete")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
