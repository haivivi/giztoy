package genx

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/goccy/go-yaml"
)

var _ ModelContext = (*modelContext)(nil)

type ModelContextBuilder struct {
	Prompts  []*Prompt
	Messages []*Message
	CoTs     []string

	Tools []Tool

	Params *ModelParams
}

func (mcb *ModelContextBuilder) Build() ModelContext {
	return &modelContext{
		prompts:  mcb.Prompts,
		messages: mcb.Messages,
		cots:     mcb.CoTs,
		tools:    mcb.Tools,
		params:   mcb.Params,
	}
}

func (mcb *ModelContextBuilder) lastPrompt() (*Prompt, bool) {
	if len(mcb.Prompts) == 0 {
		return nil, false
	}
	return mcb.Prompts[len(mcb.Prompts)-1], true
}

func (mcb *ModelContextBuilder) SetCoT(cot ...any) {
	texts := make([]string, 0, len(cot))
	for _, c := range cot {
		s, ok := c.(string)
		if ok {
			texts = append(texts, s)
		} else {
			b, err := yaml.Marshal(c)
			if err != nil {
				panic(err)
			}
			texts = append(texts, string(b))
		}
	}
	mcb.CoTs = texts
}

func (mcb *ModelContextBuilder) AddPrompt(prompt *Prompt) {
	if p, ok := mcb.lastPrompt(); ok && p.Name == prompt.Name {
		if p.Text != "" {
			p.Text += "\n" + prompt.Text
		} else {
			p.Text = prompt.Text
		}
		return
	}
	mcb.Prompts = append(mcb.Prompts, prompt)
}

func (mcb *ModelContextBuilder) lastMessage() (*Message, bool) {
	if len(mcb.Messages) == 0 {
		return nil, false
	}
	return mcb.Messages[len(mcb.Messages)-1], true
}

func (mcb *ModelContextBuilder) AddMessage(msg *Message) {
	if m, ok := mcb.lastMessage(); ok {
		switch p := m.Payload.(type) {
		default:
			break
		case Contents:
			if msg.Role != m.Role || msg.Name != m.Name {
				break
			}
			new, ok := msg.Payload.(Contents)
			if !ok {
				break
			}
			m.Payload = append(p, new...)
			return
		}
	}
	mcb.Messages = append(mcb.Messages, msg)
}

func (mcb *ModelContextBuilder) AddTool(tool Tool) {
	mcb.Tools = append(mcb.Tools, tool)
}

func (mcb *ModelContextBuilder) Prompt(name, key string, value any) error {
	b, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return err
	}
	mcb.AddPrompt(&Prompt{
		Name: name,
		Text: string(b),
	})
	return nil
}

func (mcb *ModelContextBuilder) PromptText(name, text string) {
	mcb.AddPrompt(&Prompt{
		Name: name,
		Text: text,
	})
}

func (mcb *ModelContextBuilder) UserText(name, text string) {
	mcb.AddMessage(&Message{
		Role:    RoleUser,
		Name:    name,
		Payload: Contents{Text(text)},
	})
}

func (mcb *ModelContextBuilder) UserBlob(name string, mimeType string, data []byte) {
	mcb.AddMessage(&Message{
		Role:    RoleUser,
		Name:    name,
		Payload: Contents{&Blob{MIMEType: mimeType, Data: data}},
	})
}

func (mcb *ModelContextBuilder) ModelText(name, text string) {
	mcb.AddMessage(&Message{
		Role:    RoleModel,
		Name:    name,
		Payload: Contents{Text(text)},
	})
}

func (mcb *ModelContextBuilder) ModelBlob(name string, mimeType string, data []byte) {
	mcb.AddMessage(&Message{
		Role:    RoleModel,
		Name:    name,
		Payload: Contents{&Blob{MIMEType: mimeType, Data: data}},
	})
}

func (mcb *ModelContextBuilder) toolCall(name string, id, fn, argument string) {
	mcb.Messages = append(mcb.Messages, &Message{
		Role:    RoleModel,
		Name:    name,
		Payload: &ToolCall{ID: id, FuncCall: &FuncCall{Name: fn, Arguments: argument}},
	})
}

func (mcb *ModelContextBuilder) toolResult(name, id, result string) {
	mcb.Messages = append(mcb.Messages, &Message{
		Role:    RoleTool,
		Name:    name,
		Payload: &ToolResult{ID: id, Result: result},
	})
}

func (mcb *ModelContextBuilder) toolCallResult(name, argument, result string) error {
	id := "call_" + hexString()
	mcb.toolCall("", id, name, argument)
	mcb.toolResult("", id, result)
	return nil
}

func (mcb *ModelContextBuilder) InvokeTool(ctx context.Context, tool *ToolCall) error {
	if tool.FuncCall == nil {
		return fmt.Errorf("invoke can only be called on function call: id=%s", tool.ID)
	}
	res, err := tool.FuncCall.Invoke(ctx)
	if err != nil {
		return err
	}
	return mcb.AddToolCallResult(tool.FuncCall.Name, tool.FuncCall.Arguments, res)
}

func (mcb *ModelContextBuilder) AddToolCallResult(toolName string, callArg, callResult any) error {
	argstr, ok := callArg.(string)
	if !ok {
		b, err := json.Marshal(callArg)
		if err != nil {
			return fmt.Errorf("failed to marshal tool call argument to json string: %w", err)
		}
		argstr = string(b)
	}

	resstr, ok := callResult.(string)
	if !ok {
		b, err := json.Marshal(callResult)
		if err != nil {
			return fmt.Errorf("failed to marshal tool call result to json string: %w", err)
		}
		resstr = string(b)
	}
	return mcb.toolCallResult(toolName, argstr, resstr)
}

type modelContext struct {
	prompts  []*Prompt
	messages []*Message
	cots     []string

	tools []Tool

	params *ModelParams
}

func (mctx *modelContext) Prompts() iter.Seq[*Prompt] {
	return func(yield func(*Prompt) bool) {
		for _, prompt := range mctx.prompts {
			if !yield(prompt) {
				return
			}
		}
	}
}

func (mctx *modelContext) Messages() iter.Seq[*Message] {
	return func(yield func(*Message) bool) {
		for _, message := range mctx.messages {
			if !yield(message) {
				return
			}
		}
	}
}

func (mctx *modelContext) CoTs() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, cot := range mctx.cots {
			if !yield(cot) {
				return
			}
		}
	}
}

func (mctx *modelContext) Tools() iter.Seq[Tool] {
	return func(yield func(Tool) bool) {
		for _, tool := range mctx.tools {
			if !yield(tool) {
				return
			}
		}
	}
}

func (mctx *modelContext) Params() *ModelParams {
	return mctx.params
}
