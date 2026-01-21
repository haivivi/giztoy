package match

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"
)

// Var defines a variable to be extracted from user input.
type Var struct {
	// Label is a short label for the variable, used in pattern expansion.
	// Must not contain '[' or ']' characters.
	Label string `json:"label" yaml:"label"`
	// Type is the variable type: string|int|float|bool.
	Type string `json:"type" yaml:"type"`
}

// Example is a structured grounding example for the prompt.
//
// JSON supports arrays of 1-3 elements:
// - ["subject"]                         (1 element)
// - ["subject", "user_text"]            (2 elements)
// - ["subject", "user_text", "output"]  (3 elements)
type Example struct {
	Subject     string `json:"-"`
	UserText    string `json:"-"`
	FormattedTo string `json:"-"`
}

func (e Example) MarshalJSON() ([]byte, error) {
	// Emit minimal array:
	// - 1 element if only Subject
	// - 2 elements if Subject + UserText
	// - 3 elements if all present
	if e.FormattedTo != "" {
		if e.UserText == "" {
			return nil, fmt.Errorf("invalid example: FormattedTo is set but UserText is empty")
		}
		return json.Marshal([]string{e.Subject, e.UserText, e.FormattedTo})
	}
	if e.UserText != "" {
		return json.Marshal([]string{e.Subject, e.UserText})
	}
	return json.Marshal([]string{e.Subject})
}

func (e *Example) UnmarshalJSON(b []byte) error {
	if e == nil {
		return fmt.Errorf("example is nil")
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err != nil {
		return fmt.Errorf("example must be an array: %w", err)
	}
	if len(arr) < 1 || len(arr) > 3 {
		return fmt.Errorf("invalid example array length: %d (expected 1-3)", len(arr))
	}
	e.Subject = arr[0]
	if len(arr) >= 2 {
		e.UserText = arr[1]
	}
	if len(arr) == 3 {
		e.FormattedTo = arr[2]
	}
	return nil
}

// Pattern is a single input/output pattern example for one rule.
//
// JSON supports:
// - "play songs"          (string)
// - ["play [title]", "k=v"] (array)
type Pattern struct {
	Input  string `json:"-"`
	Output string `json:"-"`
}

func (p Pattern) MarshalJSON() ([]byte, error) {
	// Keep output compact:
	// - if Output is empty, encode as scalar string
	// - else encode as [input, output]
	if p.Output == "" {
		return json.Marshal(p.Input)
	}
	return json.Marshal([]string{p.Input, p.Output})
}

func (p *Pattern) UnmarshalJSON(b []byte) error {
	if p == nil {
		return fmt.Errorf("pattern is nil")
	}
	// string
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		p.Input = s
		p.Output = ""
		return nil
	}
	// [input, output?]
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		if len(arr) < 1 || len(arr) > 2 {
			return fmt.Errorf("invalid pattern array length: %d", len(arr))
		}
		p.Input = arr[0]
		if len(arr) == 2 {
			p.Output = arr[1]
		} else {
			p.Output = ""
		}
		return nil
	}
	return fmt.Errorf("unsupported pattern json")
}

// Rule is a schema-driven rule definition that corresponds to JSON/YAML data on disk.
type Rule struct {
	Name string `json:"name" yaml:"name"`

	// References is a map of unique name -> description.
	// When merging multiple rules, entries with the same key are deduplicated.
	References map[string]string `json:"references,omitempty" yaml:"references,omitempty"`

	// Vars is a map of unique name -> variable definition.
	// The key is the variable name used in patterns (e.g. "title" for [title]).
	Vars map[string]Var `json:"vars,omitempty" yaml:"vars,omitempty"`

	Patterns []Pattern `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	Examples []Example `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// Valid var types.
var validVarTypes = map[string]struct{}{
	"string": {},
	"int":    {},
	"float":  {},
	"bool":   {},
}

// placeholderRe matches [varName] placeholders (varName must be word characters).
var placeholderRe = regexp.MustCompile(`\[(\w+)\]`)

// compileTo compiles this rule into the prompt data.
// Returns error if data is nil or validation fails.
func (r *Rule) compileTo(data *promptData) error {
	if r == nil {
		return nil
	}
	if data == nil {
		return fmt.Errorf("rule %q: prompt data is nil", r.Name)
	}

	// Validate var types and labels
	for name, v := range r.Vars {
		if v.Type != "" {
			if _, ok := validVarTypes[v.Type]; !ok {
				return fmt.Errorf("rule %q: var %q has invalid type %q (expected string|int|float|bool)", r.Name, name, v.Type)
			}
		}
		if strings.ContainsAny(v.Label, "[]") {
			return fmt.Errorf("rule %q: var %q label must not contain '[' or ']'", r.Name, name)
		}
	}

	// Validate patterns
	for i, p := range r.Patterns {
		// Input/Output must not contain newlines
		if strings.ContainsAny(p.Input, "\r\n") {
			return fmt.Errorf("rule %q: pattern[%d] input contains newline", r.Name, i)
		}
		if strings.ContainsAny(p.Output, "\r\n") {
			return fmt.Errorf("rule %q: pattern[%d] output contains newline", r.Name, i)
		}
		// Placeholders must exist in vars
		matches := placeholderRe.FindAllStringSubmatch(p.Input, -1)
		for _, m := range matches {
			varName := m[1]
			if _, ok := r.Vars[varName]; !ok {
				return fmt.Errorf("rule %q: pattern[%d] has placeholder [%s] not defined in vars", r.Name, i, varName)
			}
		}
	}

	// Merge references (deduplicate by key)
	maps.Copy(data.References, r.References)

	rd := ruleData{
		Name:     r.Name,
		Examples: r.Examples,
	}

	for _, p := range r.Patterns {
		pd := patternData{
			Input:  p.Input,
			Output: p.Output,
		}
		// If Output is empty, auto-generate from vars
		if pd.Output == "" {
			pd.Input, pd.Output = expandPattern(r.Name, p.Input, r.Vars)
		}
		rd.Patterns = append(rd.Patterns, pd)
	}

	data.Rules = append(data.Rules, rd)
	return nil
}

// expandPattern expands [varName] placeholders using var labels and generates output format.
// e.g. "play [title]" with ruleName="play_song" and vars{title: {Label: "song title"}} =>
//
//	input:  "play [song title]"
//	output: "play_song: title=[song title]"
func expandPattern(ruleName, input string, vars map[string]Var) (string, string) {
	if input == "" {
		return "", ruleName
	}

	var outParts []string

	expanded := placeholderRe.ReplaceAllStringFunc(input, func(match string) string {
		// match is "[varName]", extract varName
		varName := match[1 : len(match)-1]
		v, ok := vars[varName]
		if !ok || v.Label == "" {
			// No label defined, keep original
			return match
		}
		label := "[" + v.Label + "]"
		outParts = append(outParts, varName+"="+label)
		return label
	})

	if len(outParts) == 0 {
		return expanded, ruleName
	}
	return expanded, ruleName + ": " + strings.Join(outParts, ", ")
}
