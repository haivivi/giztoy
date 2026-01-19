package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/minimax"
)

var textCmd = &cobra.Command{
	Use:   "text",
	Short: "Text generation service",
	Long: `Text generation service (chat completions).

Supports streaming and non-streaming text generation with tool calling.

Example request file (chat.yaml):
  model: MiniMax-M2.1
  messages:
    - role: system
      content: You are a helpful assistant.
    - role: user
      content: Hello, who are you?
  max_tokens: 1000
  temperature: 0.7`,
}

var textChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Create a chat completion",
	Long: `Create a chat completion.

Examples:
  minimax -c myctx text chat -f chat.yaml
  minimax -c myctx text chat -f chat.yaml --json | jq '.choices[0].message'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.ChatCompletionRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default model if not specified
		if req.Model == "" {
			if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
				req.Model = defaultModel
			} else {
				req.Model = minimax.ModelM2_1
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Messages: %d", len(req.Messages))

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Text.CreateChatCompletion(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("chat completion failed: %w", err)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

var textChatStreamCmd = &cobra.Command{
	Use:   "chat-stream",
	Short: "Create a streaming chat completion",
	Long: `Create a streaming chat completion.

The response will be streamed to stdout in real-time.

Examples:
  minimax -c myctx text chat-stream -f chat.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.ChatCompletionRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default model if not specified
		if req.Model == "" {
			if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
				req.Model = defaultModel
			} else {
				req.Model = minimax.ModelM2_1
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Streaming mode enabled")

		// Create API client
		client := createClient(ctx)

		// Call streaming API
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var fullContent string
		for chunk, err := range client.Text.CreateChatCompletionStream(reqCtx, &req) {
			if err != nil {
				return fmt.Errorf("streaming failed: %w", err)
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
				content := chunk.Choices[0].Delta.Content
				fmt.Print(content)
				fullContent += content
			}
		}
		fmt.Println() // New line after streaming

		printVerbose("Total content length: %d characters", len(fullContent))

		return nil
	},
}

func init() {
	textCmd.AddCommand(textChatCmd)
	textCmd.AddCommand(textChatStreamCmd)
}
