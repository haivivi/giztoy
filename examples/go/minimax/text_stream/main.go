// text_stream demonstrates streaming chat completion with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . "Write a short story about a robot"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

func main() {
	model := flag.String("model", minimax.ModelM2_1, "Model name")
	maxTokens := flag.Int("max-tokens", 2000, "Maximum tokens")
	temperature := flag.Float64("temperature", 0.7, "Temperature (0-2)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: text_stream [flags] <message>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable not set")
	}

	client := minimax.NewClient(apiKey)
	ctx := context.Background()

	message := flag.Arg(0)

	fmt.Println("Assistant: ", "")

	for chunk, err := range client.Text.CreateChatCompletionStream(ctx, &minimax.ChatCompletionRequest{
		Model: *model,
		Messages: []minimax.Message{
			{Role: "system", Content: "You are a creative storyteller."},
			{Role: "user", Content: message},
		},
		MaxTokens:   *maxTokens,
		Temperature: *temperature,
	}) {
		if err != nil {
			log.Fatalf("Streaming failed: %v", err)
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}

	fmt.Println()
}
