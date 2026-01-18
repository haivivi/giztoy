package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
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

		var req mm.ChatCompletionRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default model if not specified
		if req.Model == "" {
			if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
				req.Model = defaultModel
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Messages: %d", len(req.Messages))

		// TODO: Implement actual API call
		// For now, show the request that would be made
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
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

		var req mm.ChatCompletionRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default model if not specified
		if req.Model == "" {
			if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
				req.Model = defaultModel
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Streaming mode enabled")

		// TODO: Implement actual streaming API call
		fmt.Println("[Streaming not implemented yet]")
		fmt.Printf("Would stream chat with model %s\n", req.Model)

		return nil
	},
}

func init() {
	textCmd.AddCommand(textChatCmd)
	textCmd.AddCommand(textChatStreamCmd)
}
