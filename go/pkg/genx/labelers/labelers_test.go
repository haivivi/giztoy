package labelers

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProcessBasicMatch(t *testing.T) {
	mux := NewMux()
	err := mux.Handle("labeler/test", &stubLabeler{model: "mock", result: &Result{Matches: []Match{
		{Label: "person:小明", Score: 0.99},
		{Label: "topic:恐龙", Score: 0.93},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := mux.Process(context.Background(), "labeler/test", Input{
		Text:       "我昨天和小明聊恐龙",
		Candidates: []string{"person:小明", "person:小红", "topic:恐龙"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(res.Matches))
	}
}

func TestProcessEmptyCandidates(t *testing.T) {
	g := NewGenX(Config{Generator: "unused"})
	res, err := g.Process(context.Background(), Input{Text: "聊恐龙", Candidates: nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0", len(res.Matches))
	}
}

func TestProcessUnregisteredPattern(t *testing.T) {
	mux := NewMux()
	_, err := mux.Process(context.Background(), "labeler/not-found", Input{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLabelerErrorPropagation(t *testing.T) {
	mux := NewMux()
	expected := errors.New("boom")
	if err := mux.Handle("labeler/error", &stubLabeler{model: "mock", err: expected}); err != nil {
		t.Fatal(err)
	}
	_, err := mux.Process(context.Background(), "labeler/error", Input{})
	if !errors.Is(err, expected) {
		t.Fatalf("Process() error = %v, want %v", err, expected)
	}
}

func TestProcessTopKLimit(t *testing.T) {
	call := structCall(`{"matches":[{"label":"person:小明","score":0.9},{"label":"topic:恐龙","score":0.8}]}`)
	res, err := parseAndValidate(&call, Input{
		Candidates: []string{"person:小明", "topic:恐龙"},
		TopK:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(res.Matches))
	}
}

func TestProcessEmptyText(t *testing.T) {
	g := NewGenX(Config{Generator: "unused"})
	res, err := g.Process(context.Background(), Input{Text: "", Candidates: []string{"topic:恐龙"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0", len(res.Matches))
	}
}

func TestProcessAliasMatching(t *testing.T) {
	prompt := buildPrompt(Input{
		Text:       "明明今天在上海",
		Candidates: []string{"person:小明", "place:上海"},
		Aliases: map[string][]string{
			"person:小明": {"明明", "小明同学"},
		},
	})
	if !strings.Contains(prompt, "aliases: 明明, 小明同学") {
		t.Fatalf("prompt does not contain aliases, got: %s", prompt)
	}
}

func TestProcessResultValidation(t *testing.T) {
	t.Run("label out of candidates", func(t *testing.T) {
		call := structCall(`{"matches":[{"label":"person:小红","score":0.6}]}`)
		_, err := parseAndValidate(&call, Input{Candidates: []string{"person:小明"}})
		if err == nil {
			t.Fatal("expected candidate validation error")
		}
	})

	t.Run("score out of range", func(t *testing.T) {
		call := structCall(`{"matches":[{"label":"person:小明","score":1.2}]}`)
		_, err := parseAndValidate(&call, Input{Candidates: []string{"person:小明"}})
		if err == nil {
			t.Fatal("expected score range validation error")
		}
	})
}
