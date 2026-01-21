package match

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// UnmarshalYAML implements yaml.Unmarshaler for Pattern.
// Supports:
//   - scalar string: "play songs"
//   - array: ["play [title]", "rule: k=v"]
func (p *Pattern) UnmarshalYAML(b []byte) error {
	if p == nil {
		return fmt.Errorf("pattern is nil")
	}

	// Try scalar string first
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		p.Input = s
		p.Output = ""
		return nil
	}

	// Try array [input, output?]
	var arr []string
	if err := yaml.Unmarshal(b, &arr); err == nil {
		if len(arr) < 1 || len(arr) > 2 {
			return fmt.Errorf("invalid pattern array length: %d (expected 1-2)", len(arr))
		}
		p.Input = arr[0]
		if len(arr) == 2 {
			p.Output = arr[1]
		} else {
			p.Output = ""
		}
		return nil
	}

	return fmt.Errorf("pattern must be string or [input, output] array")
}

// UnmarshalYAML implements yaml.Unmarshaler for Example.
// Supports arrays of 1-3 elements:
//   - ["subject"]
//   - ["subject", "user_text"]
//   - ["subject", "user_text", "output"]
func (e *Example) UnmarshalYAML(b []byte) error {
	if e == nil {
		return fmt.Errorf("example is nil")
	}

	var arr []string
	if err := yaml.Unmarshal(b, &arr); err != nil {
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

// ParseRuleYAML parses a Rule from YAML bytes.
func ParseRuleYAML(data []byte) (*Rule, error) {
	var r Rule
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return &r, nil
}
