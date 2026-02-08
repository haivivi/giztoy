// text_chat demonstrates basic chat completion with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . "Hello, who are you?"
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
	maxTokens := flag.Int("max-tokens", 1000, "Maximum tokens")
	temperature := flag.Float64("temperature", 0.7, "Temperature (0-2)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: text_chat [flags] <message>")
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

	resp, err := client.Text.CreateChatCompletion(ctx, &minimax.ChatCompletionRequest{
		Model: *model,
		Messages: []minimax.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: message},
		},
		MaxTokens:   *maxTokens,
		Temperature: *temperature,
	})
	if err != nil {
		log.Fatalf("Chat completion failed: %v", err)
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message != nil {
		fmt.Println(resp.Choices[0].Message.Content)
	}

	if resp.Usage != nil {
		fmt.Printf("\n--- Usage ---\n")
		fmt.Printf("Prompt tokens: %d\n", resp.Usage.PromptTokens)
		fmt.Printf("Completion tokens: %d\n", resp.Usage.CompletionTokens)
		fmt.Printf("Total tokens: %d\n", resp.Usage.TotalTokens)
	}
}
