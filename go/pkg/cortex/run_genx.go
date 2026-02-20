package cortex

import (
	"context"
	"fmt"
)

func init() {
	RegisterRunHandler("genx/generator", runGenxGenerator)
	RegisterRunHandler("genx/tts", runGenxTTS)
	RegisterRunHandler("genx/asr", runGenxASR)
}

func runGenxGenerator(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	name := task.GetString("name")
	if name == "" {
		return nil, fmt.Errorf("genx/generator: missing 'name'")
	}
	msgs := task.Fields["messages"]
	if msgs == nil {
		return nil, fmt.Errorf("genx/generator: missing 'messages'")
	}

	genxDoc, err := c.Get(ctx, "genx:generator:"+name)
	if err != nil {
		return nil, fmt.Errorf("genx/generator: lookup %q: %w", name, err)
	}

	credRef := genxDoc.GetString("cred")
	model := genxDoc.GetString("model")

	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}

	credService := ""
	parts := splitCred(credRef)
	if len(parts) == 2 {
		credService = parts[0]
	}

	switch credService {
	case "openai":
		return runOpenAITextChat(ctx, c, Document{
			Kind: "openai/text/chat",
			Fields: map[string]any{
				"cred":     credRef,
				"model":    model,
				"messages": msgs,
			},
		})
	case "genai":
		return runGenaiTextGenerate(ctx, c, Document{
			Kind: "genai/text/generate",
			Fields: map[string]any{
				"cred":     credRef,
				"model":    model,
				"messages": msgs,
			},
		})
	default:
		_ = cred
		return nil, fmt.Errorf("genx/generator: unsupported cred service %q for generator", credService)
	}
}

func runGenxTTS(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	name := task.GetString("name")
	if name == "" {
		return nil, fmt.Errorf("genx/tts: missing 'name'")
	}
	text := task.GetString("text")
	if text == "" {
		return nil, fmt.Errorf("genx/tts: missing 'text'")
	}

	genxDoc, err := c.Get(ctx, "genx:tts:"+name)
	if err != nil {
		return nil, fmt.Errorf("genx/tts: lookup %q: %w", name, err)
	}

	credRef := genxDoc.GetString("cred")
	voiceID := genxDoc.GetString("voice_id")

	credService := ""
	parts := splitCred(credRef)
	if len(parts) == 2 {
		credService = parts[0]
	}

	switch credService {
	case "minimax":
		return runMinimaxSpeechSynthesize(ctx, c, Document{
			Kind: "minimax/speech/synthesize",
			Fields: map[string]any{
				"cred":     credRef,
				"text":     text,
				"voice_id": voiceID,
				"output":   task.GetString("output"),
			},
		})
	case "doubaospeech":
		return runDoubaoTTSV2Stream(ctx, c, Document{
			Kind: "doubaospeech/tts/v2/stream",
			Fields: map[string]any{
				"cred":        credRef,
				"text":        text,
				"speaker":     voiceID,
				"resource_id": "seed-tts-2.0",
				"format":      "mp3",
				"sample_rate": 24000,
				"output":      task.GetString("output"),
			},
		})
	default:
		return nil, fmt.Errorf("genx/tts: unsupported cred service %q for tts", credService)
	}
}

func runGenxASR(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	name := task.GetString("name")
	if name == "" {
		return nil, fmt.Errorf("genx/asr: missing 'name'")
	}
	audio := task.GetString("audio")
	if audio == "" {
		return nil, fmt.Errorf("genx/asr: missing 'audio'")
	}

	genxDoc, err := c.Get(ctx, "genx:asr:"+name)
	if err != nil {
		return nil, fmt.Errorf("genx/asr: lookup %q: %w", name, err)
	}

	credRef := genxDoc.GetString("cred")
	return runDoubaoASRV1Recognize(ctx, c, Document{
		Kind: "doubaospeech/asr/v1/recognize",
		Fields: map[string]any{
			"cred":   credRef,
			"audio":  audio,
			"format": genxDoc.GetString("format"),
		},
	})
}

func splitCred(cred string) []string {
	for i, c := range cred {
		if c == ':' {
			return []string{cred[:i], cred[i+1:]}
		}
	}
	return []string{cred}
}
