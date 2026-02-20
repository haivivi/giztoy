package cortex

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

func init() {
	RegisterRunHandler("genai/text/generate", runGenaiTextGenerate)
}

func runGenaiTextGenerate(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	credRef := task.GetString("cred")
	if credRef == "" {
		return nil, fmt.Errorf("genai/text/generate: missing 'cred' field")
	}
	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}

	apiKey, _ := cred["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("genai cred missing api_key")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("genai client: %w", err)
	}

	model := task.GetString("model")
	if model == "" {
		return nil, fmt.Errorf("genai/text/generate: missing 'model' field")
	}

	msgs, _ := task.Fields["messages"].([]any)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("genai/text/generate: missing 'messages' field")
	}

	var textParts []*genai.Part
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		content, _ := mm["content"].(string)
		textParts = append(textParts, &genai.Part{Text: content})
	}

	resp, err := client.Models.GenerateContent(ctx, model, []*genai.Content{
		{Parts: textParts, Role: "user"},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("genai generate: %w", err)
	}

	var sb strings.Builder
	if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			sb.WriteString(part.Text)
		}
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Text: sb.String()}, nil
}
