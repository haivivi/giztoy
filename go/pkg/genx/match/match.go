package match

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"text/template"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/generators"

	_ "embed"
)

//go:embed default.gotmpl
var defaultPromptTpl string

// Option configures Compile behavior.
type Option func(*compileConfig)

type compileConfig struct {
	tpl string
}

// WithTpl sets a custom prompt template (overrides the default embedded template).
func WithTpl(tpl string) Option {
	return func(c *compileConfig) {
		c.tpl = tpl
	}
}

// Matcher is a compiled matcher built from rules.
// It holds the rendered system prompt and var schemas for parsing results.
type Matcher struct {
	systemPrompt string
	specs        map[string]map[string]Var // rule name -> var name -> Var
}

// SystemPrompt returns the rendered system prompt for debugging.
func (m *Matcher) SystemPrompt() string {
	return m.systemPrompt
}

// Arg holds a matched argument's value along with its definition.
type Arg struct {
	// Value is the extracted value, typed according to Var.Type.
	Value any

	// Var is the variable definition from the rule.
	Var Var

	// HasValue indicates whether a value was successfully extracted.
	HasValue bool
}

// Result is the structured output from a single match.
type Result struct {
	// Rule is the matched rule name. Empty if no rule matched.
	Rule string

	// Args holds the extracted arguments, keyed by variable name.
	Args map[string]Arg

	// RawText holds the original line when no rule matched.
	RawText string
}

// MatchOption configures Match behavior.
type MatchOption func(*matchConfig)

type matchConfig struct {
	gen genx.Generator
}

// WithGenerator sets a custom generator for Match.
// If not provided, Match uses generators.GenerateStream.
func WithGenerator(gen genx.Generator) MatchOption {
	return func(c *matchConfig) {
		c.gen = gen
	}
}

// Match executes the matcher against user input and returns streaming results.
// It combines the matcher's system prompt with the user's ModelContext,
// generates a stream from the model, and parses output lines into Results.
// If no WithGenerator option is provided, it uses generators.GenerateStream.
func (m *Matcher) Match(ctx context.Context, pattern string, mc genx.ModelContext, opts ...MatchOption) iter.Seq2[Result, error] {
	cfg := &matchConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build internal context with system prompt
	mcb := &genx.ModelContextBuilder{}
	mcb.PromptText("", m.systemPrompt)
	internal := mcb.Build()

	// Combine: user context first, then internal prompt
	combined := genx.ModelContexts(mc, internal)

	return func(yield func(Result, error) bool) {
		var stream genx.Stream
		var err error

		if cfg.gen != nil {
			stream, err = cfg.gen.GenerateStream(ctx, pattern, combined)
		} else {
			stream, err = generators.GenerateStream(ctx, pattern, combined)
		}
		if err != nil {
			yield(Result{}, fmt.Errorf("generate: %w", err))
			return
		}
		defer stream.Close()

		var sb strings.Builder
		pending := ""
		stopped := false

		flush := func(line string) bool {
			if stopped {
				return false
			}
			line = strings.TrimSpace(line)
			if line == "" {
				return true
			}
			r, ok := m.parseLine(line)
			if ok {
				if !yield(r, nil) {
					stopped = true
					return false
				}
			}
			return true
		}

		for {
			chunk, err := stream.Next()
			if err != nil {
				if !errors.Is(err, genx.ErrDone) && !stopped {
					if !yield(Result{}, err) {
						stopped = true
					}
				}
				break
			}
			if chunk != nil && chunk.Part != nil {
				if text, ok := chunk.Part.(genx.Text); ok {
					sb.WriteString(string(text))
				}
			}

			s := pending + sb.String()
			sb.Reset()

			for {
				i := strings.IndexByte(s, '\n')
				if i < 0 {
					pending = s
					break
				}
				line := s[:i]
				s = s[i+1:]
				if !flush(line) {
					return
				}
			}
		}

		if !stopped {
			flush(pending)
		}
	}
}

// parseLine parses a single output line into a Result.
// Format: "rule_name: key1=value1, key2=value2" or just "rule_name"
// If the line doesn't match any known rule, returns Result with empty Rule/Args and RawText set.
func (m *Matcher) parseLine(line string) (Result, bool) {
	name, kv, hasColon := strings.Cut(line, ":")
	name = strings.TrimSpace(name)

	if name == "" {
		return Result{RawText: line}, true
	}

	// Check if it's a known rule (with or without colon)
	vars, ok := m.specs[name]
	if !ok {
		// Not a known rule - return as raw text
		return Result{RawText: line}, true
	}

	// Known rule - parse arguments if there's a colon
	var args map[string]Arg
	if hasColon {
		args = m.parseKVToArgs(strings.TrimSpace(kv), vars)
	} else {
		// No colon means no arguments, but still a valid rule
		args = m.parseKVToArgs("", vars)
	}
	return Result{Rule: name, Args: args}, true
}

// parseKVToArgs parses "key1=value1, key2=value2" into Args using var definitions.
func (m *Matcher) parseKVToArgs(kv string, vars map[string]Var) map[string]Arg {
	args := make(map[string]Arg)

	// Pre-fill all known vars with HasValue=false
	for name, v := range vars {
		args[name] = Arg{
			Value:    nil,
			Var:      v,
			HasValue: false,
		}
	}

	if strings.TrimSpace(kv) == "" {
		return args
	}

	for part := range strings.SplitSeq(kv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}

		varDef, exists := vars[k]
		if !exists {
			continue
		}

		// Convert value based on Var.Type
		var typedValue any = v
		switch varDef.Type {
		case "int":
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				typedValue = parsed
			}
		case "float":
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				typedValue = parsed
			}
		case "bool":
			if parsed, err := strconv.ParseBool(v); err == nil {
				typedValue = parsed
			}
			// "string" or empty: keep as string
		}

		args[k] = Arg{
			Value:    typedValue,
			Var:      varDef,
			HasValue: true,
		}
	}

	return args
}

// Collect consumes a streaming sequence into a slice.
func Collect(seq iter.Seq2[Result, error]) ([]Result, error) {
	var out []Result
	for r, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Compile compiles rules into a reusable Matcher.
func Compile(rules []*Rule, opts ...Option) (*Matcher, error) {
	cfg := &compileConfig{tpl: defaultPromptTpl}
	for _, opt := range opts {
		opt(cfg)
	}

	data, err := buildPromptData(rules)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("prompt").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).Parse(cfg.tpl)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	specs := make(map[string]map[string]Var, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		if _, exists := specs[r.Name]; exists {
			slog.Warn("match: duplicate rule name, skipping", "name", r.Name)
			continue
		}
		specs[r.Name] = r.Vars
	}

	return &Matcher{
		systemPrompt: buf.String(),
		specs:        specs,
	}, nil
}

// promptData is the data structure passed to the prompt template.
type promptData struct {
	References map[string]string
	Rules      []ruleData
}

type ruleData struct {
	Name     string
	Patterns []patternData
	Examples []Example
}

type patternData struct {
	Input  string
	Output string
}

func buildPromptData(rules []*Rule) (*promptData, error) {
	data := &promptData{
		References: make(map[string]string),
	}
	for _, r := range rules {
		if r != nil {
			if err := r.compileTo(data); err != nil {
				return nil, err
			}
		}
	}
	return data, nil
}
