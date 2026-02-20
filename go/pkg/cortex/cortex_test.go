package cortex

import (
	"context"
	"fmt"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

func newTestCortex(t *testing.T) *Cortex {
	t.Helper()
	store := newTestStore(t)
	store.CtxAdd("test")
	store.CtxUse("test")
	c, err := New(context.Background(), store, WithKV(kv.NewMemory(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// ---------------------------------------------------------------------------
// Apply tests
// ---------------------------------------------------------------------------

func TestApplyCredsOpenAI(t *testing.T) {
	c := newTestCortex(t)
	results, err := c.Apply(context.Background(), []Document{{
		Kind: "creds/openai",
		Fields: map[string]any{
			"name":     "qwen",
			"api_key":  "sk-test",
			"base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("unexpected: %+v", results)
	}
}

func TestApplyCredsMinimax(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "creds/minimax",
		Fields: map[string]any{"name": "cn", "api_key": "eyJ...", "base_url": "https://api.minimaxi.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyCredsDoubaospeech(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "creds/doubaospeech",
		Fields: map[string]any{"name": "test", "app_id": "111", "token": "tok"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyCredsDashscope(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "creds/dashscope",
		Fields: map[string]any{"name": "default", "api_key": "sk-xxx"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyCredsGenai(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "creds/genai",
		Fields: map[string]any{"name": "default", "api_key": "AIza..."},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyGenxGenerator(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator",
		Fields: map[string]any{
			"name": "qwen/turbo", "cred": "openai:qwen",
			"model": "qwen-turbo-latest", "max_tokens": 2048,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyGenxTTS(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "genx/tts",
		Fields: map[string]any{"name": "minimax/shaonv", "cred": "minimax:cn", "voice_id": "female-shaonv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyGenxASR(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind:   "genx/asr",
		Fields: map[string]any{"name": "doubao/sauc", "cred": "doubaospeech:test"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyBatch(t *testing.T) {
	c := newTestCortex(t)
	results, err := c.Apply(context.Background(), []Document{
		{Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "k1"}},
		{Kind: "creds/minimax", Fields: map[string]any{"name": "cn", "api_key": "k2"}},
		{Kind: "genx/generator", Fields: map[string]any{"name": "qwen/turbo", "cred": "openai:qwen", "model": "m"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestApplyUpdate(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "old"},
	}})
	results, err := c.Apply(ctx, []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "new"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "updated" {
		t.Fatalf("expected 'updated', got %q", results[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Validation error tests
// ---------------------------------------------------------------------------

func TestApplyUnknownKind(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "foo/bar", Fields: map[string]any{"name": "x"},
	}})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestApplyMissingRequiredField(t *testing.T) {
	c := newTestCortex(t)
	// creds/openai requires name and api_key
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen"},
	}})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

func TestApplyEmptyRequiredField(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": ""},
	}})
	if err == nil {
		t.Fatal("expected error for empty api_key")
	}
}

func TestApplyDoubaospeechMissingToken(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "creds/doubaospeech", Fields: map[string]any{"name": "test", "app_id": "111"},
	}})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestApplyGenxMissingCred(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator", Fields: map[string]any{"name": "x", "model": "m"},
	}})
	if err == nil {
		t.Fatal("expected error for missing cred")
	}
}

func TestApplyGenxMissingModel(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator", Fields: map[string]any{"name": "x", "cred": "openai:qwen"},
	}})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestApplyGenxBadCredFormat(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator", Fields: map[string]any{"name": "x", "cred": "no-colon", "model": "m"},
	}})
	if err == nil {
		t.Fatal("expected error for bad cred format")
	}
}

func TestApplyGenxBadTemperature(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator", Fields: map[string]any{
			"name": "x", "cred": "openai:qwen", "model": "m", "temperature": 5.0,
		},
	}})
	if err == nil {
		t.Fatal("expected error for temperature=5")
	}
}

func TestApplyGenxBadMaxTokens(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/generator", Fields: map[string]any{
			"name": "x", "cred": "openai:qwen", "model": "m", "max_tokens": -1,
		},
	}})
	if err == nil {
		t.Fatal("expected error for max_tokens=-1")
	}
}

func TestApplyTTSMissingVoiceID(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Apply(context.Background(), []Document{{
		Kind: "genx/tts", Fields: map[string]any{"name": "x", "cred": "minimax:cn"},
	}})
	if err == nil {
		t.Fatal("expected error for missing voice_id")
	}
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestGetAfterApply(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "sk-test"},
	}})

	doc, err := c.Get(ctx, "creds:openai:qwen")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "creds/openai" {
		t.Fatalf("expected kind=creds/openai, got %q", doc.Kind)
	}
	if doc.GetString("api_key") != "sk-test" {
		t.Fatalf("expected api_key=sk-test, got %q", doc.GetString("api_key"))
	}
}

func TestGetNotFound(t *testing.T) {
	c := newTestCortex(t)
	_, err := c.Get(context.Background(), "creds:openai:nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetGenx(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{{
		Kind: "genx/generator", Fields: map[string]any{
			"name": "qwen/turbo", "cred": "openai:qwen", "model": "qwen-turbo-latest",
		},
	}})

	doc, err := c.Get(ctx, "genx:generator:qwen/turbo")
	if err != nil {
		t.Fatal(err)
	}
	if doc.GetString("model") != "qwen-turbo-latest" {
		t.Fatalf("model mismatch: %q", doc.GetString("model"))
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestListAll(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{
		{Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "k1"}},
		{Kind: "creds/openai", Fields: map[string]any{"name": "deepseek", "api_key": "k2"}},
		{Kind: "creds/minimax", Fields: map[string]any{"name": "cn", "api_key": "k3"}},
	})

	docs, err := c.List(ctx, "creds:*", ListOpts{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3, got %d", len(docs))
	}
}

func TestListPrefix(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{
		{Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "k1"}},
		{Kind: "creds/minimax", Fields: map[string]any{"name": "cn", "api_key": "k2"}},
	})

	docs, err := c.List(ctx, "creds:openai:*", ListOpts{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1, got %d", len(docs))
	}
}

func TestListLimit(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		c.Apply(ctx, []Document{{
			Kind:   "creds/openai",
			Fields: map[string]any{"name": fmt.Sprintf("app%02d", i), "api_key": "k"},
		}})
	}

	docs, err := c.List(ctx, "creds:*", ListOpts{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 5 {
		t.Fatalf("expected 5, got %d", len(docs))
	}
}

func TestListFrom(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{
		{Kind: "creds/openai", Fields: map[string]any{"name": "a", "api_key": "k1"}},
		{Kind: "creds/openai", Fields: map[string]any{"name": "b", "api_key": "k2"}},
		{Kind: "creds/openai", Fields: map[string]any{"name": "c", "api_key": "k3"}},
	})

	docs, err := c.List(ctx, "creds:*", ListOpts{From: "creds:openai:a", All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 (after 'a'), got %d", len(docs))
	}
}

func TestListEmpty(t *testing.T) {
	c := newTestCortex(t)
	docs, err := c.List(context.Background(), "creds:*", ListOpts{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0, got %d", len(docs))
	}
}

func TestListGenx(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{
		{Kind: "genx/generator", Fields: map[string]any{"name": "qwen/turbo", "cred": "openai:qwen", "model": "m1"}},
		{Kind: "genx/tts", Fields: map[string]any{"name": "minimax/shaonv", "cred": "minimax:cn", "voice_id": "v1"}},
	})

	gens, _ := c.List(ctx, "genx:generator:*", ListOpts{All: true})
	if len(gens) != 1 {
		t.Fatalf("expected 1 generator, got %d", len(gens))
	}

	tts, _ := c.List(ctx, "genx:tts:*", ListOpts{All: true})
	if len(tts) != 1 {
		t.Fatalf("expected 1 tts, got %d", len(tts))
	}

	all, _ := c.List(ctx, "genx:*", ListOpts{All: true})
	if len(all) != 2 {
		t.Fatalf("expected 2 genx total, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDeleteCreds(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{{
		Kind: "creds/openai", Fields: map[string]any{"name": "qwen", "api_key": "k"},
	}})

	if err := c.Delete(ctx, "creds:openai:qwen"); err != nil {
		t.Fatal(err)
	}

	_, err := c.Get(ctx, "creds:openai:qwen")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	c := newTestCortex(t)
	err := c.Delete(context.Background(), "creds:openai:nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent delete")
	}
}

func TestDeleteGenx(t *testing.T) {
	c := newTestCortex(t)
	ctx := context.Background()

	c.Apply(ctx, []Document{{
		Kind: "genx/generator", Fields: map[string]any{"name": "qwen/turbo", "cred": "openai:qwen", "model": "m"},
	}})

	if err := c.Delete(ctx, "genx:generator:qwen/turbo"); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Document parsing tests
// ---------------------------------------------------------------------------

func TestParseDocumentsSingle(t *testing.T) {
	data := []byte(`kind: creds/openai
name: qwen
api_key: sk-test
`)
	docs, err := ParseDocuments(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].Kind != "creds/openai" {
		t.Fatalf("kind mismatch: %q", docs[0].Kind)
	}
	if docs[0].GetString("api_key") != "sk-test" {
		t.Fatalf("api_key mismatch: %q", docs[0].GetString("api_key"))
	}
}

func TestParseDocumentsMulti(t *testing.T) {
	data := []byte(`---
kind: creds/openai
name: qwen
api_key: k1
---
kind: creds/minimax
name: cn
api_key: k2
`)
	docs, err := ParseDocuments(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
}

func TestParseDocumentsMissingKind(t *testing.T) {
	data := []byte(`name: qwen
api_key: sk-test
`)
	_, err := ParseDocuments(data)
	if err == nil {
		t.Fatal("expected error for missing kind")
	}
}

func TestParseDocumentsInvalidYAML(t *testing.T) {
	data := []byte(`kind: creds/openai
  bad indent: here
`)
	_, err := ParseDocuments(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// ---------------------------------------------------------------------------
// Schema tests
// ---------------------------------------------------------------------------

func TestSchemaRegistryHas12Kinds(t *testing.T) {
	r := NewSchemaRegistry()
	kinds := r.Kinds()
	if len(kinds) != 12 {
		t.Fatalf("expected 12 kinds, got %d: %v", len(kinds), kinds)
	}
}

func TestSchemaValidateCredFormatOK(t *testing.T) {
	err := validateCredFormat(map[string]any{"cred": "openai:qwen"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSchemaValidateCredFormatBad(t *testing.T) {
	err := validateCredFormat(map[string]any{"cred": "nocolon"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSchemaValidateTemperatureOK(t *testing.T) {
	err := validateTemperature(map[string]any{"temperature": 1.0})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSchemaValidateTemperatureBad(t *testing.T) {
	err := validateTemperature(map[string]any{"temperature": 3.0})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSchemaValidateMaxTokensOK(t *testing.T) {
	err := validateMaxTokens(map[string]any{"max_tokens": 1024})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSchemaValidateMaxTokensBad(t *testing.T) {
	err := validateMaxTokens(map[string]any{"max_tokens": 0})
	if err == nil {
		t.Fatal("expected error")
	}
}
