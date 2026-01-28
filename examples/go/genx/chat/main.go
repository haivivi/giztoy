// Example: Two AI generators having a conversation
//
// This example demonstrates how to use the genx package to make
// an OpenAI model and a Gemini model have a conversation with each other.
//
// Usage:
//
//	go run main.go -models-dir=/path/to/models
//	# or with environment variables:
//	export OPENAI_API_KEY=your-openai-key
//	export GEMINI_API_KEY=your-gemini-key
//	go run main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	gai "google.golang.org/genai"
)

var (
	rounds      = flag.Int("rounds", 5, "Number of conversation rounds")
	topic       = flag.String("topic", "the meaning of life", "Topic for discussion")
	oaiModel    = flag.String("oai-model", "gpt-4o-mini", "OpenAI model to use")
	geminiModel = flag.String("gemini-model", "gemini-2.5-flash", "Gemini model to use")
	verbose     = flag.Bool("v", false, "Verbose output (show streaming)")
	modelsDir   = flag.String("models-dir", "", "Path to models config directory (e.g., x/go/tools/modeltest/models)")
)

// ModelConfig represents a model provider configuration
type ModelConfig struct {
	Kind    string        `json:"kind"`
	APIKey  string        `json:"api_key"`
	BaseURL string        `json:"base_url,omitempty"`
	Models  []ModelDetail `json:"models"`
}

type ModelDetail struct {
	Name              string            `json:"name"`
	Model             string            `json:"model"`
	SupportJSONOutput bool              `json:"support_json_output"`
	SupportToolCalls  bool              `json:"support_tool_calls"`
	SupportTextOnly   bool              `json:"support_text_only"`
	UseSystemRole     bool              `json:"use_system_role"`
	GenerateParams    *genx.ModelParams `json:"generate_params,omitempty"`
	InvokeParams      *genx.ModelParams `json:"invoke_params,omitempty"`
}

func loadModelConfig(dir, filename string) (*ModelConfig, error) {
	data, err := os.ReadFile(dir + "/" + filename)
	if err != nil {
		return nil, err
	}
	var cfg ModelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Initialize OpenAI generator
	oaiGen, err := newOpenAIGenerator()
	if err != nil {
		return fmt.Errorf("failed to create OpenAI generator: %w", err)
	}

	// Initialize Gemini generator
	geminiGen, err := newGeminiGenerator(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Gemini generator: %w", err)
	}

	// Create conversation contexts for each participant
	oaiCtx := &genx.ModelContextBuilder{}
	geminiCtx := &genx.ModelContextBuilder{}

	// Set up system prompts
	oaiCtx.PromptText("system", fmt.Sprintf(`You are a thoughtful philosopher named "OpenAI".
You are having a conversation with another AI named "Gemini" about: %s.
Be concise but insightful. Keep responses under 100 words.
Express your unique perspective and ask follow-up questions to continue the dialogue.`, *topic))

	geminiCtx.PromptText("system", fmt.Sprintf(`You are a curious scientist named "Gemini".
You are having a conversation with another AI named "OpenAI" about: %s.
Be concise but analytical. Keep responses under 100 words.
Express your unique perspective and ask follow-up questions to continue the dialogue.`, *topic))

	// Start the conversation
	fmt.Printf("🎭 AI Conversation: %s\n", *topic)
	fmt.Println(strings.Repeat("=", 60))

	// Initial prompt to start the conversation
	initialMessage := fmt.Sprintf("Let's discuss: %s. What are your initial thoughts?", *topic)

	// Add the initial message to OpenAI's context (as if Gemini asked)
	oaiCtx.UserText("Gemini", initialMessage)
	fmt.Printf("\n🟢 Gemini: %s\n", initialMessage)

	// Conversation loop
	for round := 1; round <= *rounds; round++ {
		fmt.Printf("\n--- Round %d/%d ---\n", round, *rounds)

		// OpenAI responds
		oaiResponse, err := generate(ctx, oaiGen, "OpenAI", oaiCtx)
		if err != nil {
			return fmt.Errorf("OpenAI generation failed: %w", err)
		}
		fmt.Printf("\n🔵 OpenAI: %s\n", oaiResponse)

		// Add OpenAI's response to both contexts
		oaiCtx.ModelText("OpenAI", oaiResponse)
		geminiCtx.UserText("OpenAI", oaiResponse)

		// Gemini responds
		geminiResponse, err := generate(ctx, geminiGen, "Gemini", geminiCtx)
		if err != nil {
			return fmt.Errorf("Gemini generation failed: %w", err)
		}
		fmt.Printf("\n🟢 Gemini: %s\n", geminiResponse)

		// Add Gemini's response to both contexts
		geminiCtx.ModelText("Gemini", geminiResponse)
		oaiCtx.UserText("Gemini", geminiResponse)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🎭 Conversation ended")

	return nil
}

func newOpenAIGenerator() (*genx.OpenAIGenerator, error) {
	var apiKey, baseURL string
	var cfg *ModelConfig

	// Try loading from config file first
	if *modelsDir != "" {
		var err error
		cfg, err = loadModelConfig(*modelsDir, "openai.json")
		if err != nil {
			return nil, fmt.Errorf("failed to load openai.json: %w", err)
		}
		apiKey = cfg.APIKey
		baseURL = cfg.BaseURL
	} else {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set (or use -models-dir flag)")
		}
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)

	gen := &genx.OpenAIGenerator{
		Client:           &client,
		Model:            *oaiModel,
		SupportToolCalls: true,
		UseSystemRole:    true,
		GenerateParams: &genx.ModelParams{
			MaxTokens:   256,
			Temperature: 0.7,
		},
	}

	// Apply model-specific settings from config
	if cfg != nil {
		for _, m := range cfg.Models {
			if m.Model == *oaiModel {
				gen.SupportJSONOutput = m.SupportJSONOutput
				gen.SupportToolCalls = m.SupportToolCalls
				gen.SupportTextOnly = m.SupportTextOnly
				gen.UseSystemRole = m.UseSystemRole
				if m.GenerateParams != nil {
					gen.GenerateParams = m.GenerateParams
				}
				break
			}
		}
	}

	return gen, nil
}

func newGeminiGenerator(ctx context.Context) (*genx.GeminiGenerator, error) {
	var apiKey string
	var cfg *ModelConfig

	// Try loading from config file first
	if *modelsDir != "" {
		var err error
		cfg, err = loadModelConfig(*modelsDir, "gemini.json")
		if err != nil {
			return nil, fmt.Errorf("failed to load gemini.json: %w", err)
		}
		apiKey = cfg.APIKey
	} else {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set (or use -models-dir flag)")
		}
	}

	client, err := gai.NewClient(ctx, &gai.ClientConfig{
		APIKey:  apiKey,
		Backend: gai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	gen := &genx.GeminiGenerator{
		Client: client,
		Model:  *geminiModel,
		GenerateParams: &genx.ModelParams{
			MaxTokens:   256,
			Temperature: 0.7,
		},
	}

	// Apply model-specific settings from config
	if cfg != nil {
		for _, m := range cfg.Models {
			if m.Model == *geminiModel {
				if m.GenerateParams != nil {
					gen.GenerateParams = m.GenerateParams
				}
				break
			}
		}
	}

	return gen, nil
}

func generate(ctx context.Context, gen genx.Generator, name string, mcb *genx.ModelContextBuilder) (string, error) {
	stream, err := gen.GenerateStream(ctx, name, mcb.Build())
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var sb strings.Builder

	for {
		chunk, err := stream.Next()
		if err != nil {
			if errors.Is(err, genx.ErrDone) {
				break
			}
			return "", err
		}

		if chunk.Part != nil {
			if text, ok := chunk.Part.(genx.Text); ok {
				sb.WriteString(string(text))
				if *verbose {
					fmt.Print(string(text))
				}
			}
		}
	}

	return strings.TrimSpace(sb.String()), nil
}

// readAll reads all text from a stream
func readAll(stream genx.Stream) (string, error) {
	iter := genx.Iter(stream)
	var sb strings.Builder

	for {
		el, err := iter.Next()
		if err != nil {
			if errors.Is(err, genx.ErrDone) {
				break
			}
			return "", err
		}

		if se, ok := el.(*genx.StreamElement); ok && se.MIMEType == "text/plain" {
			data, err := io.ReadAll(se)
			if err != nil {
				return "", err
			}
			sb.Write(data)
		}
	}

	return sb.String(), nil
}
