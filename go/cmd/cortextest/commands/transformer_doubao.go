package commands

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/cortex"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
)

func init() {
	RegisterTransformer("doubao", newDoubaoRealtimeFactory)
}

// newDoubaoRealtimeFactory creates a Doubao realtime transformer factory.
//
// Environment variables:
//   - DOUBAO_APP_ID: Doubao application ID (required)
//   - DOUBAO_ACCESS_KEY: Doubao access key (required)
//   - DOUBAO_APP_KEY: Doubao app key (optional, defaults to APP_ID)
//
// Flags:
//   - --voice: Speaker voice name
//   - --instructions: System role instructions
//
// Note: Doubao currently does not support VAD mode switching.
// The mode parameter is accepted for interface compatibility but ignored.
func newDoubaoRealtimeFactory(mode cortex.TransformerMode) (genx.Transformer, error) {
	_ = mode // Doubao doesn't support VAD mode switching yet
	appID := os.Getenv("DOUBAO_APP_ID")
	if appID == "" {
		return nil, fmt.Errorf("DOUBAO_APP_ID environment variable is required")
	}

	accessKey := os.Getenv("DOUBAO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("DOUBAO_TOKEN") // Fallback to DOUBAO_TOKEN
	}
	if accessKey == "" {
		return nil, fmt.Errorf("DOUBAO_ACCESS_KEY or DOUBAO_TOKEN environment variable is required")
	}

	appKey := os.Getenv("DOUBAO_APP_KEY")
	if appKey == "" {
		appKey = appID // Default to APP_ID
	}

	client := doubaospeech.NewClient(appID,
		doubaospeech.WithV2APIKey(accessKey, appKey),
	)

	var opts []transformers.DoubaoRealtimeOption

	if flagVoice != "" {
		opts = append(opts, transformers.WithDoubaoRealtimeSpeaker(flagVoice))
	}

	if flagInstructions != "" {
		opts = append(opts, transformers.WithDoubaoRealtimeSystemRole(flagInstructions))
	} else {
		opts = append(opts, transformers.WithDoubaoRealtimeSystemRole(
			"你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。",
		))
	}

	return transformers.NewDoubaoRealtime(client, opts...), nil
}
