// Package labelers provides query-time label selection for memory recall.
//
// A labeler takes query text and a candidate label set (usually from memory
// graph entities), then returns the best-matching labels for recall expansion.
package labelers

import "context"

// Labeler selects labels from candidates for a query text.
type Labeler interface {
	// Process selects matched labels from input candidates.
	Process(ctx context.Context, input Input) (*Result, error)

	// Model returns underlying model / implementation identifier.
	Model() string
}

// Input is the input to a [Labeler].
type Input struct {
	// Text is the query text.
	Text string `json:"text"`

	// Candidates is the candidate labels to choose from.
	Candidates []string `json:"candidates"`

	// Aliases optionally maps label -> alias words used as prompt hints.
	Aliases map[string][]string `json:"aliases,omitempty"`

	// TopK optionally limits the number of returned labels.
	TopK int `json:"top_k,omitempty"`
}

// Match is a selected candidate label.
type Match struct {
	Label string  `json:"label"`
	Score float64 `json:"score,omitempty"`
}

// Result is the output of a [Labeler].
type Result struct {
	Matches []Match `json:"matches"`
}

// Config configures a GenX labeler implementation.
type Config struct {
	// Generator is the registered generator pattern (e.g. "qwen/flash").
	Generator string `json:"generator" yaml:"generator"`

	// PromptVersion selects prompt template variant. Default "v1".
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
