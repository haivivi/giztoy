//go:build e2e

package cortex

import (
	"context"
	"os"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// E2E tests require API keys in environment variables.
// Run with: go test ./pkg/cortex/ -tags e2e -timeout 5m

func skipIfNoKey(t *testing.T, envVar string) {
	if os.Getenv(envVar) == "" {
		t.Skipf("skipping: %s not set", envVar)
	}
}

func newE2ECortex(t *testing.T) *Cortex {
	t.Helper()
	store := newTestStore(t)
	store.CtxAdd("e2e")
	store.CtxUse("e2e")
	c, err := New(context.Background(), store, WithKV(kv.NewMemory(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func applyCreds(t *testing.T, c *Cortex) {
	t.Helper()
	ctx := context.Background()

	var docs []Document

	if key := os.Getenv("QWEN_API_KEY"); key != "" {
		docs = append(docs, Document{Kind: "creds/openai", Fields: map[string]any{
			"name": "qwen", "api_key": key,
			"base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
		}})
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		docs = append(docs, Document{Kind: "creds/openai", Fields: map[string]any{
			"name": "deepseek", "api_key": key,
			"base_url": "https://api.deepseek.com/v1",
		}})
	}
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		docs = append(docs, Document{Kind: "creds/minimax", Fields: map[string]any{
			"name": "cn", "api_key": key,
		}})
	}
	if appID, token := os.Getenv("DOUBAO_APP_ID"), os.Getenv("DOUBAO_TOKEN"); appID != "" && token != "" {
		docs = append(docs, Document{Kind: "creds/doubaospeech", Fields: map[string]any{
			"name": "test", "app_id": appID, "token": token,
			"api_key": os.Getenv("DOUBAO_API_KEY"),
		}})
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		docs = append(docs, Document{Kind: "creds/genai", Fields: map[string]any{
			"name": "default", "api_key": key,
		}})
	}

	if len(docs) > 0 {
		if _, err := c.Apply(ctx, docs); err != nil {
			t.Fatalf("apply creds: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// OpenAI (Qwen) E2E
// ---------------------------------------------------------------------------

func TestE2E_OpenAI_TextChat(t *testing.T) {
	skipIfNoKey(t, "QWEN_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	result, err := c.Run(context.Background(), Document{
		Kind: "openai/text/chat",
		Fields: map[string]any{
			"cred":  "openai:qwen",
			"model": "qwen-turbo-latest",
			"messages": []any{
				map[string]any{"role": "user", "content": "Say hello in exactly one word."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
	t.Logf("Response: %s", result.Text)
}

func TestE2E_OpenAI_TextChatStream(t *testing.T) {
	skipIfNoKey(t, "QWEN_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	result, err := c.Run(context.Background(), Document{
		Kind: "openai/text/chat-stream",
		Fields: map[string]any{
			"cred":  "openai:qwen",
			"model": "qwen-turbo-latest",
			"messages": []any{
				map[string]any{"role": "user", "content": "Count from 1 to 3."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
	t.Logf("Streamed: %s", result.Text)
}

// ---------------------------------------------------------------------------
// MiniMax E2E
// ---------------------------------------------------------------------------

func TestE2E_Minimax_TextChat(t *testing.T) {
	skipIfNoKey(t, "MINIMAX_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	result, err := c.Run(context.Background(), Document{
		Kind: "minimax/text/chat",
		Fields: map[string]any{
			"cred":  "minimax:cn",
			"model": "MiniMax-M2.1",
			"messages": []any{
				map[string]any{"role": "user", "content": "Say hello."},
			},
			"max_tokens": 100,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
	t.Logf("MiniMax: %s", result.Text)
}

func TestE2E_Minimax_SpeechSynthesize(t *testing.T) {
	skipIfNoKey(t, "MINIMAX_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	output := t.TempDir() + "/test.mp3"
	result, err := c.Run(context.Background(), Document{
		Kind: "minimax/speech/synthesize",
		Fields: map[string]any{
			"cred":     "minimax:cn",
			"text":     "你好",
			"voice_id": "female-shaonv",
			"output":   output,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AudioSize == 0 {
		t.Fatal("expected non-zero audio")
	}
	t.Logf("Audio: %d bytes → %s", result.AudioSize, result.AudioFile)
}

func TestE2E_Minimax_VoiceList(t *testing.T) {
	skipIfNoKey(t, "MINIMAX_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	result, err := c.Run(context.Background(), Document{
		Kind:   "minimax/voice/list",
		Fields: map[string]any{"cred": "minimax:cn"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Voices: %v", result.Data)
}

// ---------------------------------------------------------------------------
// Doubaospeech E2E
// ---------------------------------------------------------------------------

func TestE2E_Doubao_TTSV2Stream(t *testing.T) {
	skipIfNoKey(t, "DOUBAO_APP_ID")
	c := newE2ECortex(t)
	applyCreds(t, c)

	output := t.TempDir() + "/test.mp3"
	result, err := c.Run(context.Background(), Document{
		Kind: "doubaospeech/tts/v2/stream",
		Fields: map[string]any{
			"cred":        "doubaospeech:test",
			"text":        "你好，这是E2E测试。",
			"speaker":     "zh_female_xiaohe_uranus_bigtts",
			"resource_id": "seed-tts-2.0",
			"format":      "mp3",
			"sample_rate": 24000,
			"output":      output,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AudioSize == 0 {
		t.Fatal("expected non-zero audio")
	}
	t.Logf("Doubao TTS V2: %d bytes → %s", result.AudioSize, result.AudioFile)
}

// ---------------------------------------------------------------------------
// Memory E2E (local, no API key needed)
// ---------------------------------------------------------------------------

func TestE2E_Memory_CRUD(t *testing.T) {
	c := newE2ECortex(t)
	ctx := context.Background()

	// Create persona
	result, err := c.Run(ctx, Document{Kind: "memory/create", Fields: map[string]any{"name": "e2e_test"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("create: %s", result.Status)
	}

	// Add segment
	result, err = c.Run(ctx, Document{Kind: "memory/add", Fields: map[string]any{
		"persona":  "e2e_test",
		"text":     "和小明聊了恐龙",
		"labels":   []any{"person:小明", "topic:恐龙"},
		"keywords": []any{"恐龙"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	// Entity set
	result, err = c.Run(ctx, Document{Kind: "memory/entity/set", Fields: map[string]any{
		"persona": "e2e_test",
		"label":   "person:小明",
		"attrs":   map[string]any{"age": 8},
	}})
	if err != nil {
		t.Fatal(err)
	}

	// Entity get
	result, err = c.Run(ctx, Document{Kind: "memory/entity/get", Fields: map[string]any{
		"persona": "e2e_test",
		"label":   "person:小明",
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Entity: %v", result.Data)

	// Relation add
	result, err = c.Run(ctx, Document{Kind: "memory/relation/add", Fields: map[string]any{
		"persona":  "e2e_test",
		"from":     "person:小明",
		"to":       "topic:恐龙",
		"rel_type": "likes",
	}})
	if err != nil {
		t.Fatal(err)
	}

	// Recall
	result, err = c.Run(ctx, Document{Kind: "memory/recall", Fields: map[string]any{
		"persona": "e2e_test",
		"text":    "恐龙",
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Recall: %v", result.Data)
}

// ---------------------------------------------------------------------------
// Gemini E2E
// ---------------------------------------------------------------------------

func TestE2E_Genai_TextGenerate(t *testing.T) {
	skipIfNoKey(t, "GEMINI_API_KEY")
	c := newE2ECortex(t)
	applyCreds(t, c)

	result, err := c.Run(context.Background(), Document{
		Kind: "genai/text/generate",
		Fields: map[string]any{
			"cred":  "genai:default",
			"model": "gemini-2.0-flash",
			"messages": []any{
				map[string]any{"role": "user", "content": "Say hello in one word."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
	t.Logf("Gemini: %s", result.Text)
}
