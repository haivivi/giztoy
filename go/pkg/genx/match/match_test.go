package match

import (
	"encoding/json"
	"testing"
)

func TestParseKVToArgs_TypeConversion(t *testing.T) {
	m := &Matcher{
		specs: map[string]map[string]Var{
			"test_rule": {
				"str_var":   {Label: "string var", Type: "string"},
				"int_var":   {Label: "int var", Type: "int"},
				"float_var": {Label: "float var", Type: "float"},
				"bool_var":  {Label: "bool var", Type: "bool"},
				"empty_var": {Label: "empty type", Type: ""},
			},
		},
	}

	vars := m.specs["test_rule"]

	tests := []struct {
		name     string
		kv       string
		varName  string
		wantVal  any
		wantType string
	}{
		{
			name:     "string type",
			kv:       "str_var=hello",
			varName:  "str_var",
			wantVal:  "hello",
			wantType: "string",
		},
		{
			name:     "int type",
			kv:       "int_var=42",
			varName:  "int_var",
			wantVal:  int64(42),
			wantType: "int64",
		},
		{
			name:     "negative int",
			kv:       "int_var=-100",
			varName:  "int_var",
			wantVal:  int64(-100),
			wantType: "int64",
		},
		{
			name:     "float type",
			kv:       "float_var=3.14",
			varName:  "float_var",
			wantVal:  float64(3.14),
			wantType: "float64",
		},
		{
			name:     "bool true",
			kv:       "bool_var=true",
			varName:  "bool_var",
			wantVal:  true,
			wantType: "bool",
		},
		{
			name:     "bool false",
			kv:       "bool_var=false",
			varName:  "bool_var",
			wantVal:  false,
			wantType: "bool",
		},
		{
			name:     "bool 1",
			kv:       "bool_var=1",
			varName:  "bool_var",
			wantVal:  true,
			wantType: "bool",
		},
		{
			name:     "empty type defaults to string",
			kv:       "empty_var=test",
			varName:  "empty_var",
			wantVal:  "test",
			wantType: "string",
		},
		{
			name:     "int parse error fallback to string",
			kv:       "int_var=not_a_number",
			varName:  "int_var",
			wantVal:  "not_a_number",
			wantType: "string",
		},
		{
			name:     "float parse error fallback to string",
			kv:       "float_var=not_a_float",
			varName:  "float_var",
			wantVal:  "not_a_float",
			wantType: "string",
		},
		{
			name:     "bool parse error fallback to string",
			kv:       "bool_var=maybe",
			varName:  "bool_var",
			wantVal:  "maybe",
			wantType: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := m.parseKVToArgs(tt.kv, vars)
			arg, ok := args[tt.varName]
			if !ok {
				t.Fatalf("expected arg %q to exist", tt.varName)
			}
			if !arg.HasValue {
				t.Fatalf("expected arg %q to have value", tt.varName)
			}
			if arg.Value != tt.wantVal {
				t.Errorf("Value = %v (%T), want %v (%T)", arg.Value, arg.Value, tt.wantVal, tt.wantVal)
			}
		})
	}
}

func TestParseKVToArgs_MultipleVars(t *testing.T) {
	m := &Matcher{
		specs: map[string]map[string]Var{
			"play_song": {
				"title":  {Label: "song title", Type: "string"},
				"volume": {Label: "volume level", Type: "int"},
			},
		},
	}

	vars := m.specs["play_song"]
	args := m.parseKVToArgs("title=My Song, volume=80", vars)

	if args["title"].Value != "My Song" {
		t.Errorf("title = %v, want %v", args["title"].Value, "My Song")
	}
	if args["volume"].Value != int64(80) {
		t.Errorf("volume = %v (%T), want %v", args["volume"].Value, args["volume"].Value, int64(80))
	}
}

func TestParseKVToArgs_EmptyKV(t *testing.T) {
	m := &Matcher{
		specs: map[string]map[string]Var{
			"test": {
				"var1": {Label: "var1", Type: "string"},
			},
		},
	}

	vars := m.specs["test"]
	args := m.parseKVToArgs("", vars)

	if args["var1"].HasValue {
		t.Error("expected var1 to not have value for empty kv")
	}
}

func TestParseLine(t *testing.T) {
	m := &Matcher{
		specs: map[string]map[string]Var{
			"play_song": {
				"title": {Label: "song title", Type: "string"},
			},
			"stop": {},
		},
	}

	tests := []struct {
		name     string
		line     string
		wantRule string
		wantRaw  string
	}{
		{
			name:     "rule with args",
			line:     "play_song: title=Hello",
			wantRule: "play_song",
		},
		{
			name:     "rule without args",
			line:     "stop",
			wantRule: "stop",
		},
		{
			name:    "unknown rule",
			line:    "unknown_rule: x=1",
			wantRaw: "unknown_rule: x=1",
		},
		{
			name:    "empty line",
			line:    "",
			wantRaw: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := m.parseLine(tt.line)
			if result.Rule != tt.wantRule {
				t.Errorf("Rule = %q, want %q", result.Rule, tt.wantRule)
			}
			if result.RawText != tt.wantRaw {
				t.Errorf("RawText = %q, want %q", result.RawText, tt.wantRaw)
			}
		})
	}
}

func TestCompile(t *testing.T) {
	rules := []*Rule{
		{
			Name: "play_song",
			Vars: map[string]Var{
				"title": {Label: "song title", Type: "string"},
			},
			Patterns: []Pattern{
				{Input: "play [title]"},
			},
		},
		{
			Name: "stop",
			Patterns: []Pattern{
				{Input: "stop"},
			},
		},
	}

	matcher, err := Compile(rules)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if matcher.SystemPrompt() == "" {
		t.Error("SystemPrompt() is empty")
	}

	// Check specs
	if _, ok := matcher.specs["play_song"]; !ok {
		t.Error("specs missing play_song")
	}
	if _, ok := matcher.specs["stop"]; !ok {
		t.Error("specs missing stop")
	}
}

func TestCompile_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		rules   []*Rule
		wantErr bool
	}{
		{
			name: "invalid var type",
			rules: []*Rule{
				{
					Name: "test",
					Vars: map[string]Var{
						"x": {Type: "invalid"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "label contains bracket",
			rules: []*Rule{
				{
					Name: "test",
					Vars: map[string]Var{
						"x": {Label: "test[bracket]"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "pattern with undefined placeholder",
			rules: []*Rule{
				{
					Name: "test",
					Patterns: []Pattern{
						{Input: "test [undefined]"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "pattern with newline",
			rules: []*Rule{
				{
					Name: "test",
					Patterns: []Pattern{
						{Input: "test\nwith\nnewline"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.rules)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandPattern(t *testing.T) {
	vars := map[string]Var{
		"title":  {Label: "song title"},
		"volume": {Label: "volume level"},
	}

	tests := []struct {
		name       string
		ruleName   string
		input      string
		wantInput  string
		wantOutput string
	}{
		{
			name:       "single var",
			ruleName:   "play_song",
			input:      "play [title]",
			wantInput:  "play [song title]",
			wantOutput: "play_song: title=[song title]",
		},
		{
			name:       "multiple vars",
			ruleName:   "set_volume",
			input:      "play [title] at [volume]",
			wantInput:  "play [song title] at [volume level]",
			wantOutput: "set_volume: title=[song title], volume=[volume level]",
		},
		{
			name:       "no vars",
			ruleName:   "stop",
			input:      "stop playback",
			wantInput:  "stop playback",
			wantOutput: "stop",
		},
		{
			name:       "empty input",
			ruleName:   "empty",
			input:      "",
			wantInput:  "",
			wantOutput: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInput, gotOutput := expandPattern(tt.ruleName, tt.input, vars)
			if gotInput != tt.wantInput {
				t.Errorf("input = %q, want %q", gotInput, tt.wantInput)
			}
			if gotOutput != tt.wantOutput {
				t.Errorf("output = %q, want %q", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestExample_JSON(t *testing.T) {
	tests := []struct {
		name    string
		example Example
		want    string
	}{
		{
			name:    "subject only",
			example: Example{Subject: "test"},
			want:    `["test"]`,
		},
		{
			name:    "subject and user text",
			example: Example{Subject: "test", UserText: "hello"},
			want:    `["test","hello"]`,
		},
		{
			name:    "all fields",
			example: Example{Subject: "test", UserText: "hello", FormattedTo: "output"},
			want:    `["test","hello","output"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.example)
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("Marshal = %s, want %s", string(b), tt.want)
			}

			var got Example
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("Unmarshal error = %v", err)
			}
			if got.Subject != tt.example.Subject {
				t.Errorf("Subject = %q, want %q", got.Subject, tt.example.Subject)
			}
		})
	}
}

func TestPattern_JSON(t *testing.T) {
	tests := []struct {
		name    string
		pattern Pattern
		want    string
	}{
		{
			name:    "input only",
			pattern: Pattern{Input: "play songs"},
			want:    `"play songs"`,
		},
		{
			name:    "input and output",
			pattern: Pattern{Input: "play [title]", Output: "play_song: title=[title]"},
			want:    `["play [title]","play_song: title=[title]"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.pattern)
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("Marshal = %s, want %s", string(b), tt.want)
			}

			var got Pattern
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("Unmarshal error = %v", err)
			}
			if got.Input != tt.pattern.Input {
				t.Errorf("Input = %q, want %q", got.Input, tt.pattern.Input)
			}
			if got.Output != tt.pattern.Output {
				t.Errorf("Output = %q, want %q", got.Output, tt.pattern.Output)
			}
		})
	}
}

func TestCollect(t *testing.T) {
	results := []Result{
		{Rule: "rule1"},
		{Rule: "rule2"},
	}

	seq := func(yield func(Result, error) bool) {
		for _, r := range results {
			if !yield(r, nil) {
				return
			}
		}
	}

	collected, err := Collect(seq)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(collected) != len(results) {
		t.Errorf("len(collected) = %d, want %d", len(collected), len(results))
	}
}

func TestWithTpl(t *testing.T) {
	customTpl := "Custom template: {{range .Rules}}{{.Name}}{{end}}"
	rules := []*Rule{
		{Name: "test_rule"},
	}

	matcher, err := Compile(rules, WithTpl(customTpl))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if matcher.SystemPrompt() != "Custom template: test_rule" {
		t.Errorf("SystemPrompt() = %q, want custom template output", matcher.SystemPrompt())
	}
}
