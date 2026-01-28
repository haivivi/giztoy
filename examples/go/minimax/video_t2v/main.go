// video_t2v demonstrates text-to-video generation with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . "A cat walking in a garden"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

func main() {
	model := flag.String("model", minimax.ModelHailuo23, "Model name")
	duration := flag.Int("duration", 6, "Video duration in seconds (6 or 10)")
	resolution := flag.String("resolution", minimax.Resolution768P, "Resolution: 768P or 1080P")
	pollInterval := flag.Duration("poll", 10*time.Second, "Poll interval for task status")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: video_t2v [flags] <prompt>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable not set")
	}

	client := minimax.NewClient(apiKey)
	ctx := context.Background()

	prompt := flag.Arg(0)

	fmt.Println("Creating video generation task...")
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Printf("Duration: %ds, Resolution: %s\n", *duration, *resolution)

	task, err := client.Video.CreateTextToVideo(ctx, &minimax.TextToVideoRequest{
		Model:      *model,
		Prompt:     prompt,
		Duration:   *duration,
		Resolution: *resolution,
	})
	if err != nil {
		log.Fatalf("Failed to create task: %v", err)
	}

	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Waiting for completion (polling every %s)...\n", *pollInterval)

	result, err := task.WaitWithInterval(ctx, *pollInterval)
	if err != nil {
		log.Fatalf("Task failed: %v", err)
	}

	fmt.Printf("\n✓ Video generated!\n")
	fmt.Printf("File ID: %s\n", result.FileID)
	if result.VideoWidth > 0 {
		fmt.Printf("Resolution: %dx%d\n", result.VideoWidth, result.VideoHeight)
	}
	if result.DownloadURL != "" {
		fmt.Printf("Download URL: %s\n", result.DownloadURL)
	}
}
