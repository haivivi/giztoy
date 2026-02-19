package cortex

import (
	"context"
	"fmt"
	"time"

	"github.com/haivivi/giztoy/go/pkg/dashscope"
)

func init() {
	RegisterRunHandler("dashscope/omni/chat", runDashscopeOmniChat)
}

func runDashscopeOmniChat(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	credRef := task.GetString("cred")
	if credRef == "" {
		return nil, fmt.Errorf("dashscope/omni/chat: missing 'cred' field")
	}
	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}

	apiKey, _ := cred["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("dashscope cred missing api_key")
	}

	var opts []dashscope.Option
	if workspace, _ := cred["workspace"].(string); workspace != "" {
		opts = append(opts, dashscope.WithWorkspace(workspace))
	}
	if baseURL, _ := cred["base_url"].(string); baseURL != "" {
		opts = append(opts, dashscope.WithBaseURL(baseURL))
	}
	client := dashscope.NewClient(apiKey, opts...)

	model := task.GetString("model")
	if model == "" {
		model = dashscope.ModelQwenOmniTurboRealtimeLatest
	}

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(connectCtx, &dashscope.RealtimeConfig{Model: model})
	if err != nil {
		return nil, fmt.Errorf("dashscope connect: %w", err)
	}
	defer session.Close()

	return &RunResult{
		Kind:   task.Kind,
		Status: "ok",
		Text:   "Connected to " + model,
		Data:   map[string]any{"model": model},
	}, nil
}
