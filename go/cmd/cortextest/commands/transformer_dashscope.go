package commands

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/cortex"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
)

func init() {
	RegisterTransformer("dashscope", newDashScopeRealtimeFactory)
}

// newDashScopeRealtimeFactory creates a DashScope realtime transformer factory.
//
// Environment variables:
//   - DASHSCOPE_API_KEY: DashScope API key (required)
//
// Flags:
//   - --model: Model name (default: qwen-omni-turbo-realtime-latest)
//   - --voice: Voice name (default: Chelsie)
//   - --instructions: System instructions
func newDashScopeRealtimeFactory(mode cortex.TransformerMode) (genx.Transformer, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DASHSCOPE_API_KEY environment variable is required")
	}

	client := dashscope.NewClient(apiKey)

	var opts []transformers.DashScopeRealtimeOption

	if flagModel != "" {
		opts = append(opts, transformers.WithDashScopeRealtimeModel(flagModel))
	}

	if flagVoice != "" {
		opts = append(opts, transformers.WithDashScopeRealtimeVoice(flagVoice))
	}

	instructions := flagInstructions
	if instructions == "" {
		instructions = "你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。"
	}
	opts = append(opts, transformers.WithDashScopeRealtimeInstructions(instructions))

	// Configure VAD mode based on cortex mode
	switch mode {
	case cortex.ModeServerVAD:
		opts = append(opts, transformers.WithDashScopeRealtimeVAD("server_vad"))
	case cortex.ModeManual:
		// Empty string means manual mode (no auto VAD)
		opts = append(opts, transformers.WithDashScopeRealtimeVAD(""))
	}

	return transformers.NewDashScopeRealtime(client, opts...), nil
}
