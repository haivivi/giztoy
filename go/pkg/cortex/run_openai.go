package cortex

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func init() {
	RegisterRunHandler("openai/text/chat", runOpenAITextChat)
	RegisterRunHandler("openai/text/chat-stream", runOpenAITextChatStream)
}

func newOpenAIClient(cred map[string]any) (*openai.Client, error) {
	apiKey, _ := cred["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("openai cred missing api_key")
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL, _ := cred["base_url"].(string); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &client, nil
}

func buildOpenAIMessages(task Document) []openai.ChatCompletionMessageParamUnion {
	msgs, _ := task.Fields["messages"].([]any)
	var params []openai.ChatCompletionMessageParamUnion
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		role, _ := mm["role"].(string)
		content, _ := mm["content"].(string)
		switch role {
		case "system":
			params = append(params, openai.SystemMessage(content))
		case "user":
			params = append(params, openai.UserMessage(content))
		case "assistant":
			params = append(params, openai.AssistantMessage(content))
		}
	}
	return params
}

func runOpenAITextChat(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	credRef := task.GetString("cred")
	if credRef == "" {
		return nil, fmt.Errorf("openai/text/chat: missing 'cred' field")
	}
	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}
	client, err := newOpenAIClient(cred)
	if err != nil {
		return nil, err
	}

	model := task.GetString("model")
	if model == "" {
		return nil, fmt.Errorf("openai/text/chat: missing 'model' field")
	}

	messages := buildOpenAIMessages(task)
	if len(messages) == 0 {
		return nil, fmt.Errorf("openai/text/chat: missing 'messages' field")
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	}
	if mt := task.GetInt("max_tokens"); mt > 0 {
		params.MaxTokens = openai.Int(int64(mt))
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	text := ""
	if len(resp.Choices) > 0 {
		text = resp.Choices[0].Message.Content
	}

	return &RunResult{
		Kind:   task.Kind,
		Status: "ok",
		Text:   text,
		Data: map[string]any{
			"model":          resp.Model,
			"usage_prompt":   resp.Usage.PromptTokens,
			"usage_complete": resp.Usage.CompletionTokens,
			"finish_reason":  string(resp.Choices[0].FinishReason),
		},
	}, nil
}

func runOpenAITextChatStream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	credRef := task.GetString("cred")
	if credRef == "" {
		return nil, fmt.Errorf("openai/text/chat-stream: missing 'cred' field")
	}
	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}
	client, err := newOpenAIClient(cred)
	if err != nil {
		return nil, err
	}

	model := task.GetString("model")
	if model == "" {
		return nil, fmt.Errorf("openai/text/chat-stream: missing 'model' field")
	}

	messages := buildOpenAIMessages(task)
	if len(messages) == 0 {
		return nil, fmt.Errorf("openai/text/chat-stream: missing 'messages' field")
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	}

	stream := client.Chat.Completions.NewStreaming(ctx, params)

	var sb strings.Builder
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			sb.WriteString(delta)
		}
	}
	if err := stream.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	return &RunResult{
		Kind:   task.Kind,
		Status: "ok",
		Text:   sb.String(),
	}, nil
}
