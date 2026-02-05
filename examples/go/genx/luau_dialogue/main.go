// Package main demonstrates two agents having a conversation.
//
// This example creates two agent instances and coordinates messages
// between them to simulate a dialogue using OpenAI API.
//
// Note: This is a simplified version that demonstrates the dialogue pattern.
// For full Luau agent runtime integration, see the agent runtime tests.
//
// Usage:
//
//	OPENAI_API_KEY=xxx go run ./examples/go/genx/luau_dialogue
//	OPENAI_API_KEY=xxx bazel run //examples/go/genx/luau_dialogue
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	initialTopic = "你认为人工智能能够真正理解人类的情感吗？请分享你的看法。"
	dialogRounds = 3

	systemPrompt = `你正在参与一场哲学对话。请用简洁有趣的方式回应对方的观点，
每次回复控制在2-3句话以内。可以提出反驳、补充或新的问题来推进讨论。
使用中文回复。`
)

// Agent represents a dialogue participant
type Agent struct {
	name      string
	model     string
	history   []*genx.Message
	generator *genx.OpenAIGenerator
}

// NewAgent creates a new agent
func NewAgent(name string, generator *genx.OpenAIGenerator, model string) *Agent {
	return &Agent{
		name:      name,
		model:     model,
		history:   make([]*genx.Message, 0),
		generator: generator,
	}
}

// Respond generates a response to the given input
func (a *Agent) Respond(ctx context.Context, input string) (string, error) {
	// Add user message to history
	a.history = append(a.history, &genx.Message{
		Role:    genx.RoleUser,
		Payload: genx.Contents{genx.Text(input)},
	})

	// Build model context using builder
	builder := &genx.ModelContextBuilder{}
	builder.PromptText("system", systemPrompt)
	for _, msg := range a.history {
		switch msg.Role {
		case genx.RoleUser:
			if contents, ok := msg.Payload.(genx.Contents); ok {
				for _, part := range contents {
					if text, ok := part.(genx.Text); ok {
						builder.UserText("", string(text))
					}
				}
			}
		case genx.RoleModel:
			if contents, ok := msg.Payload.(genx.Contents); ok {
				for _, part := range contents {
					if text, ok := part.(genx.Text); ok {
						builder.ModelText("", string(text))
					}
				}
			}
		}
	}
	mctx := builder.Build()

	// Generate response
	stream, err := a.generator.GenerateStream(ctx, a.model, mctx)
	if err != nil {
		return "", fmt.Errorf("generate stream: %w", err)
	}
	defer stream.Close()

	var response string
	for {
		chunk, err := stream.Next()
		if err != nil {
			// Check if it's normal end of stream
			errStr := err.Error()
			if errStr == "EOF" || errStr == "buffer is closed" ||
				errStr == "genx: generate done" || errStr == "buffer: read from closed buffer" {
				break
			}
			return "", fmt.Errorf("stream next: %w", err)
		}
		if chunk == nil {
			break
		}
		if text, ok := chunk.Part.(genx.Text); ok {
			response += string(text)
		}
	}

	// Add assistant response to history
	a.history = append(a.history, &genx.Message{
		Role:    genx.RoleModel,
		Payload: genx.Contents{genx.Text(response)},
	})

	return response, nil
}

func main() {
	// Check for API key (supports OpenAI or DeepSeek)
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := "gpt-4o-mini"

	// Fallback to DeepSeek if OpenAI key not available
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
		if apiKey != "" {
			baseURL = "https://api.deepseek.com"
			model = "deepseek-chat"
		}
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: OPENAI_API_KEY or DEEPSEEK_API_KEY environment variable is required")
		os.Exit(1)
	}

	fmt.Println("============================================================")
	fmt.Println("  Agent Dialogue: Alice vs Bob (Go)")
	fmt.Println("============================================================")
	fmt.Println()

	if err := runDialogue(apiKey, baseURL, model); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func runDialogue(apiKey, baseURL, model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create OpenAI-compatible client and generator
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	generator := &genx.OpenAIGenerator{
		Client:        &client,
		Model:         model,
		UseSystemRole: true, // Use "system" role instead of "developer" for compatibility
	}

	// Create agents
	alice := NewAgent("Alice", generator, model)
	bob := NewAgent("Bob", generator, model)

	// Start the conversation
	fmt.Printf("Topic: %s\n\n", initialTopic)

	// Alice responds to initial topic
	aliceResponse, err := alice.Respond(ctx, initialTopic)
	if err != nil {
		return fmt.Errorf("Alice initial response: %w", err)
	}
	fmt.Printf("Alice: %s\n\n", aliceResponse)

	// Dialogue loop
	currentInput := aliceResponse
	for round := 1; round <= dialogRounds; round++ {
		fmt.Printf("--- Round %d ---\n\n", round)

		// Bob responds to Alice
		bobResponse, err := bob.Respond(ctx, currentInput)
		if err != nil {
			return fmt.Errorf("Bob response in round %d: %w", round, err)
		}
		fmt.Printf("Bob: %s\n\n", bobResponse)

		// Alice responds to Bob (if not last round)
		if round < dialogRounds {
			aliceResponse, err := alice.Respond(ctx, bobResponse)
			if err != nil {
				return fmt.Errorf("Alice response in round %d: %w", round, err)
			}
			fmt.Printf("Alice: %s\n\n", aliceResponse)
			currentInput = aliceResponse
		}
	}

	fmt.Println("============================================================")
	fmt.Printf("  Dialogue finished - %d rounds\n", dialogRounds)
	fmt.Println("============================================================")

	return nil
}
