package genx

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"

	_ "embed"
)

var (
	//go:embed inspect_model_context.gotmpl
	inspectModelContextTplContent string

	inspectModelContextTpl = template.Must(
		template.New("inspectModelContext").
			Funcs(template.FuncMap{
				"inspectMessage": InspectMessage,
				"inspectTool":    InspectTool,
				"trim":           strings.Trim,
			}).
			Parse(inspectModelContextTplContent))
)

type Stream interface {
	Next() (*MessageChunk, error)
	Close() error
	CloseWithError(error) error
}

type ModelParams struct {
	MaxTokens        int     `json:"max_tokens,omitzero"`
	FrequencyPenalty float32 `json:"frequency_penalty,omitzero"`
	N                int     `json:"n,omitzero"`
	Temperature      float32 `json:"temperature,omitzero"`
	TopP             float32 `json:"top_p,omitzero"`
	PresencePenalty  float32 `json:"presence_penalty,omitzero"`
	TopK             float32 `json:"top_k,omitzero"`
}

type Prompt struct {
	Name string
	Text string
}

type Tool interface {
	isTool()
}

type SearchWebTool struct{}

func (*SearchWebTool) isTool() {}

type ModelContext interface {
	Prompts() iter.Seq[*Prompt]
	Messages() iter.Seq[*Message]
	CoTs() iter.Seq[string]
	Tools() iter.Seq[Tool]

	Params() *ModelParams
}

func InspectTool(tool Tool) string {
	switch t := tool.(type) {
	case *FuncTool:
		name := strings.Trim(fmt.Sprintf("%q", t.Name), `"`)
		return fmt.Sprintf("### %s\n%s", name, t.Description)
	case *SearchWebTool:
		return "### SearchWebTool"
	}
	return ""
}

func InspectMessage(msg *Message) string {
	if msg == nil {
		return ""
	}
	var sb strings.Builder

	name := strings.Trim(fmt.Sprintf("%q", msg.Name), `"`)
	fmt.Fprintf(&sb, "### %s\n", msg.Role.String())
	fmt.Fprintln(&sb, name)
	switch p := msg.Payload.(type) {
	case Contents:
		for _, part := range p {
			switch pt := part.(type) {
			case Text:
				fmt.Fprintln(&sb, pt)
			case *Blob:
				if pt != nil {
					fmt.Fprintln(&sb, pt.MIMEType)
					fmt.Fprintf(&sb, "[%d]\n", len(pt.Data))
				}
			default:
				fmt.Fprintf(&sb, "[%T]\n", part)
			}
		}
	case *ToolCall:
		fmt.Fprintf(&sb, "[%s]\n", p.ID)
		if p.FuncCall != nil {
			fmt.Fprintf(&sb, "%s(%s)\n", strings.Trim(fmt.Sprintf("%q", p.FuncCall.Name), `"`), p.FuncCall.Arguments)
		}
	case *ToolResult:
		fmt.Fprintf(&sb, "[%s]\n", p.ID)
		fmt.Fprintln(&sb, p.Result)
	}
	return sb.String()
}

func InspectModelContext(mctx ModelContext) (string, error) {
	var sb strings.Builder
	if err := inspectModelContextTpl.Execute(&sb, mctx); err != nil {
		return "", err
	}
	return sb.String(), nil
}

type Generator interface {
	GenerateStream(context.Context, string, ModelContext) (Stream, error)
	Invoke(context.Context, string, ModelContext, *FuncTool) (Usage, *FuncCall, error)
}

type Usage struct {
	// Number of tokens in the prompt. When cached_content is set, this is still
	// the total effective prompt size. I.e. this includes the number of tokens
	// in the cached content.
	PromptTokenCount int64

	// Number of tokens in the cached part of the prompt, i.e. in the cached
	// content.
	CachedContentTokenCount int64

	// Number of tokens generated.
	GeneratedTokenCount int64
}

func (u Usage) String() string {
	b, _ := yaml.Marshal(map[string]map[string]any{
		"Usage": {
			"Prompt":    u.PromptTokenCount,
			"Cached":    u.CachedContentTokenCount,
			"Generated": u.GeneratedTokenCount,
		},
	})
	return string(b)
}
