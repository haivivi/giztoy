package genx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
)

var _ Generator = (*OpenAIGenerator)(nil)

const (
	oaiFinishReasonStop          string = "stop"
	oaiFinishReasonToolCalls     string = "tool_calls"
	oaiFinishReasonLength        string = "length"
	oaiFinishReasonFunctionCall  string = "function_call"
	oaiFinishReasonContentFilter string = "content_filter"

	oaiMaxTextContentLength = 1048576
)

// OpenAISchemaFormatter formats a JSON schema for OpenAI structured outputs.
type OpenAISchemaFormatter func(m *jsonschema.Schema) *jsonschema.Schema

// OpenAIGenerator implements Generator using OpenAI API.
type OpenAIGenerator struct {
	Client *openai.Client `json:"-"`

	Model string `json:"model"`

	GenerateParams *ModelParams `json:"generate_params,omitzero"`
	InvokeParams   *ModelParams `json:"invoke_params,omitzero"`

	SupportJSONOutput  bool `json:"support_json_output,omitzero"`
	SupportToolCalls   bool `json:"support_tool_calls,omitzero"`
	SupportTextOnly    bool `json:"support_text_only,omitzero"`
	UseSystemRole      bool `json:"use_system_role,omitzero"`
	InvokeWithToolName bool `json:"invoke_with_tool_name,omitzero"`

	ExtraFields map[string]any `json:"extra_fields,omitzero"`

	SchemaFormatter OpenAISchemaFormatter `json:"-"`
}

func (g *OpenAIGenerator) Invoke(ctx context.Context, _ string, mctx ModelContext, fn *FuncTool) (Usage, *FuncCall, error) {
	switch {
	case g.SupportJSONOutput:
		return g.invokeJSONOutput(ctx, mctx, fn)
	case g.SupportToolCalls:
		return g.invokeToolCalls(ctx, mctx, fn)
	default:
		return Usage{}, nil, errors.New("json output or tool calls are required")
	}
}

func (g *OpenAIGenerator) GenerateStream(ctx context.Context, _ string, mctx ModelContext) (Stream, error) {
	params, err := g.chatCompletion(mctx, g.GenerateParams)
	if err != nil {
		return nil, err
	}
	sb := NewStreamBuilder(mctx, 32)
	go func() {
		if err := (&oaiPuller{}).pull(sb, g.Client.Chat.Completions.NewStreaming(ctx, params)); err != nil {
			sb.Abort(err)
		}
	}()
	return sb.Stream(), nil
}

func (g *OpenAIGenerator) invokeJSONOutput(ctx context.Context, mctx ModelContext, fn *FuncTool) (Usage, *FuncCall, error) {
	params, err := g.chatCompletion(mctx, g.InvokeParams)
	if err != nil {
		return Usage{}, nil, err
	}
	// When using json_schema response format, tools must not be sent
	// alongside it — they would conflict and cause validation errors.
	params.Tools = nil
	params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        fn.Name,
				Description: param.NewOpt(fn.Description),
				Schema:      g.convSchemaForOutput(fn.Argument),
				Strict:      param.NewOpt(true),
			},
		},
	}
	resp, err := g.Client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Usage{}, nil, err
	}
	if len(resp.Choices) == 0 {
		return Usage{}, nil, fmt.Errorf("no choices")
	}
	choice := resp.Choices[0]
	if choice.Message.Refusal != "" {
		return Usage{}, nil, fmt.Errorf("blocked: %s", choice.Message.Refusal)
	}
	if choice.FinishReason != oaiFinishReasonStop {
		return Usage{}, nil, fmt.Errorf("want stop, got unexpected finish reason: %s", choice.FinishReason)
	}
	if len(choice.Message.Content) == 0 {
		return Usage{}, nil, fmt.Errorf("no content")
	}
	return Usage{}, fn.NewFuncCall(choice.Message.Content), nil
}

func (g *OpenAIGenerator) invokeToolCalls(ctx context.Context, mctx ModelContext, fn *FuncTool) (Usage, *FuncCall, error) {
	params, err := g.chatCompletion(mctx, g.InvokeParams)
	if err != nil {
		return Usage{}, nil, err
	}
	tools := append(params.Tools, openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        fn.Name,
			Description: param.NewOpt(fn.Description),
			Parameters:  g.convSchemaForFunc(fn.Argument),
			Strict:      param.NewOpt(true),
		},
	})
	params.Tools = tools
	if g.InvokeWithToolName {
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
			OfChatCompletionNamedToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
				Function: openai.ChatCompletionNamedToolChoiceFunctionParam{
					Name: fn.Name,
				},
			},
		}
	} else {
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.NewOpt("auto"),
		}
	}

	resp, err := g.Client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Usage{}, nil, err
	}

	if len(resp.Choices) == 0 {
		return Usage{}, nil, fmt.Errorf("no choices")
	}
	choice := resp.Choices[0]
	if choice.Message.Refusal != "" {
		return Usage{}, nil, Blocked(oaiConvUsage(&resp.Usage), choice.Message.Refusal)
	}
	if choice.FinishReason != oaiFinishReasonToolCalls {
		return Usage{}, nil, fmt.Errorf("want tool calls, got unexpected finish reason: %s, %v", choice.FinishReason, choice)
	}
	if len(choice.Message.ToolCalls) == 0 {
		return Usage{}, nil, fmt.Errorf("no tool calls")
	}
	toolCall := choice.Message.ToolCalls[0]
	return Usage{}, fn.NewFuncCall(toolCall.Function.Arguments), nil
}

func (g *OpenAIGenerator) chatCompletion(mctx ModelContext, mp *ModelParams) (openai.ChatCompletionNewParams, error) {
	msgs, err := g.convModelContext(mctx)
	if err != nil {
		return openai.ChatCompletionNewParams{}, err
	}
	params := openai.ChatCompletionNewParams{
		Messages: msgs,
		Model:    g.Model,
	}
	if mp != nil {
		if mp.FrequencyPenalty > 0 {
			params.FrequencyPenalty = param.NewOpt(float64(mp.FrequencyPenalty))
		}
		if mp.MaxTokens > 0 {
			params.MaxCompletionTokens = param.NewOpt(int64(mp.MaxTokens))
		}
		if mp.N > 0 {
			params.N = param.NewOpt(int64(mp.N))
		}
		if mp.Temperature > 0 {
			params.Temperature = param.NewOpt(float64(mp.Temperature))
		}
		if mp.TopP > 0 {
			params.TopP = param.NewOpt(float64(mp.TopP))
		}
		if mp.PresencePenalty > 0 {
			params.PresencePenalty = param.NewOpt(float64(mp.PresencePenalty))
		}
	}
	if g.SupportToolCalls {
		for tool := range mctx.Tools() {
			switch tool := tool.(type) {
			case *FuncTool:
				params.Tools = append(params.Tools, openai.ChatCompletionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name:        tool.Name,
						Description: param.NewOpt(tool.Description),
						Parameters:  g.convSchemaForFunc(tool.Argument),
					},
				})
			default:
				return openai.ChatCompletionNewParams{}, fmt.Errorf("unexpected tool type: %T", tool)
			}
		}
	}
	if len(g.ExtraFields) > 0 {
		params.SetExtraFields(g.ExtraFields)
	}
	return params, nil
}

type oaiPuller struct {
	runningTool *openai.ChatCompletionChunkChoiceDeltaToolCall
}

func (p *oaiPuller) commitTool(sb *StreamBuilder) error {
	if p.runningTool == nil {
		return nil
	}

	defer func() { p.runningTool = nil }()

	return sb.Add(&MessageChunk{
		Role: RoleModel,
		ToolCall: &ToolCall{
			ID: p.runningTool.ID,
			FuncCall: &FuncCall{
				Name:      p.runningTool.Function.Name,
				Arguments: p.runningTool.Function.Arguments,
			},
		},
	})
}

func (p *oaiPuller) pull(sb *StreamBuilder, stream *ssestream.Stream[openai.ChatCompletionChunk]) (re error) {
	var index int64

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		var sel *openai.ChatCompletionChunkChoice
		if index == 0 {
			index = chunk.Choices[0].Index
			sel = &chunk.Choices[0]
		} else {
			for _, c := range chunk.Choices {
				if c.Index == index {
					sel = &c
					break
				}
			}
			if sel == nil {
				continue
			}
		}
		if s := sel.Delta.Content; s != "" {
			if err := sb.Add(&MessageChunk{
				Role: RoleModel,
				Part: Text(s),
			}); err != nil {
				return err
			}
		}
		for _, t := range sel.Delta.ToolCalls {
			switch p.runningTool {
			default:
				if t.ID == "" || t.ID == p.runningTool.ID {
					p.runningTool.Function.Name += t.Function.Name
					p.runningTool.Function.Arguments += t.Function.Arguments
				} else {
					if err := p.commitTool(sb); err != nil {
						return err
					}
					p.runningTool = &t
				}
			case nil:
				if t.ID != "" {
					p.runningTool = &t
				}
			}
		}
		switch sel.FinishReason {
		case oaiFinishReasonFunctionCall,
			oaiFinishReasonToolCalls:
			if err := p.commitTool(sb); err != nil {
				return err
			}
			return sb.Done(oaiConvUsage(&chunk.Usage))
		case oaiFinishReasonStop:
			return sb.Done(oaiConvUsage(&chunk.Usage))
		case oaiFinishReasonLength:
			return sb.Truncated(oaiConvUsage(&chunk.Usage))
		case oaiFinishReasonContentFilter:
			return sb.Blocked(oaiConvUsage(&chunk.Usage), sel.Delta.Refusal)
		}
		if s := sel.Delta.Refusal; s != "" {
			return sb.Blocked(oaiConvUsage(&chunk.Usage), s)
		}
	}
	return stream.Err()
}

func (g *OpenAIGenerator) convModelContext(mctx ModelContext) ([]openai.ChatCompletionMessageParamUnion, error) {
	out := []openai.ChatCompletionMessageParamUnion{}
	for p := range mctx.Prompts() {
		out = append(out, g.convPrompt(p)...)
	}
	for msg := range mctx.Messages() {
		param, err := g.convMessage(msg)
		if err != nil {
			return nil, err
		}
		out = append(out, param)
	}
	return out, nil
}

func (g *OpenAIGenerator) convPrompt(p *Prompt) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(p.Text)/oaiMaxTextContentLength+1)
	t := p.Text
	for len(t) > 0 {
		v := t
		if len(v) > oaiMaxTextContentLength {
			v, t = t[:oaiMaxTextContentLength], t[oaiMaxTextContentLength:]
		} else {
			t = ""
		}
		if g.UseSystemRole {
			mp := openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.NewOpt(v),
					},
				},
			}
			if p.Name != "" {
				mp.OfSystem.Name = param.NewOpt(p.Name)
			}
			out = append(out, mp)
		} else {
			mp := openai.ChatCompletionMessageParamUnion{
				OfDeveloper: &openai.ChatCompletionDeveloperMessageParam{
					Content: openai.ChatCompletionDeveloperMessageParamContentUnion{
						OfString: param.NewOpt(v),
					},
				},
			}
			if p.Name != "" {
				mp.OfDeveloper.Name = param.NewOpt(p.Name)
			}
			out = append(out, mp)
		}
	}
	return out
}

func (g *OpenAIGenerator) convMessage(msg *Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch t := msg.Payload.(type) {
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf(
			"unexpected message type: %T, message must be a content, tool call, or tool result",
			t,
		)
	case Contents:
		switch msg.Role {
		default:
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf(
				"unexpected content message role: %s, a content message must be a user or model message",
				msg.Role,
			)
		case RoleUser:
			return g.convUserMessage(msg)
		case RoleModel:
			return g.convModelMessage(msg)
		}
	case *ToolCall:
		mp := openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: []openai.ChatCompletionMessageToolCallParam{
					{
						ID: t.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      t.FuncCall.Name,
							Arguments: t.FuncCall.Arguments,
						},
					},
				},
			},
		}
		if msg.Name != "" {
			mp.OfAssistant.Name = param.NewOpt(msg.Name)
		}
		return mp, nil
	case *ToolResult:
		return openai.ToolMessage(t.Result, t.ID), nil
	}
}

func (g *OpenAIGenerator) convModelMessage(msg *Message) (openai.ChatCompletionMessageParamUnion, error) {
	var text bytes.Buffer
	for _, c := range msg.Payload.(Contents) {
		switch v := c.(type) {
		case Text:
			text.WriteString(string(v))
		case *Blob:
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("model message must contain text only")
		}
	}
	if text.Len() == 0 {
		return openai.ChatCompletionMessageParamUnion{}, errors.New("model message must contain text")
	}
	mp := openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: param.NewOpt(text.String()),
			},
		},
	}
	if msg.Name != "" {
		mp.OfAssistant.Name = param.NewOpt(msg.Name)
	}
	return mp, nil
}

func (g *OpenAIGenerator) convUserMessage(msg *Message) (openai.ChatCompletionMessageParamUnion, error) {
	var (
		mp3  bytes.Buffer
		wav  bytes.Buffer
		text bytes.Buffer
	)
	for _, c := range msg.Payload.(Contents) {
		switch v := c.(type) {
		case Text:
			text.WriteString(string(v))
		case *Blob:
			if g.SupportTextOnly {
				return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("model %v support text message only", g.Model)
			}
			switch v.MIMEType {
			case "audio/mp3", "audio/mpeg":
				mp3.Write(v.Data)
			case "audio/wav":
				wav.Write(v.Data)
			default:
				return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unsupported message type: %T", v)
			}
		}
	}

	var contents []openai.ChatCompletionContentPartUnionParam
	switch {
	case g.SupportTextOnly, mp3.Len() == 0 && wav.Len() == 0:
		if text.Len() == 0 {
			return openai.ChatCompletionMessageParamUnion{}, errors.New("user message must contain text")
		}
		mp := openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: param.NewOpt(text.String()),
			},
		}
		if msg.Name != "" {
			mp.Name = param.NewOpt(msg.Name)
		}
		return openai.ChatCompletionMessageParamUnion{OfUser: &mp}, nil
	case text.Len() > 0:
		contents = append(contents, openai.TextContentPart(text.String()))
	}

	if mp3.Len() > 0 {
		contents = append(contents, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
			Data:   base64.StdEncoding.EncodeToString(mp3.Bytes()),
			Format: "mp3",
		}))
	}
	if wav.Len() > 0 {
		contents = append(contents, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
			Data:   base64.StdEncoding.EncodeToString(wav.Bytes()),
			Format: "wav",
		}))
	}
	if len(contents) == 0 {
		return openai.ChatCompletionMessageParamUnion{}, errors.New("user message must contain text or audio")
	}
	mp := openai.ChatCompletionUserMessageParam{
		Content: openai.ChatCompletionUserMessageParamContentUnion{
			OfArrayOfContentParts: contents,
		},
	}
	if msg.Name != "" {
		mp.Name = param.NewOpt(msg.Name)
	}
	return openai.ChatCompletionMessageParamUnion{
		OfUser: &mp,
	}, nil
}

func (g *OpenAIGenerator) convSchemaForOutput(s *jsonschema.Schema) any {
	if s == nil {
		return nil
	}
	return (any)(g.patchSchema(s))
}

func (g *OpenAIGenerator) convSchemaForFunc(s *jsonschema.Schema) openai.FunctionParameters {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(g.patchSchema(s))
	if err != nil {
		return nil
	}
	var m openai.FunctionParameters
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// FormatOpenAISchema formats a schema for OpenAI structured outputs.
//
// OpenAI strict mode requires:
//   - All objects must have additionalProperties: false
//   - All properties must be listed in required
//
// See https://platform.openai.com/docs/guides/structured-outputs
func FormatOpenAISchema(m *jsonschema.Schema) *jsonschema.Schema {
	if m == nil {
		return nil
	}

	// Merge Type into Types if both are set (jsonschema library may set
	// Types: ["null", "array"] with Type: "" for nullable fields). We need
	// to consolidate into a single representation for OpenAI.
	if m.Type != "" && len(m.Types) > 0 {
		m.Types = append(m.Types, m.Type)
		m.Type = ""
	}

	typ := m.Type
	if typ == "" && len(m.Types) > 0 {
		// Determine effective type for switch dispatch.
		for _, t := range m.Types {
			if t != "null" && t != "" {
				typ = t
				break
			}
		}
	}

	switch typ {
	case "array":
		m.Items = FormatOpenAISchema(m.Items)
	case "object":
		// additionalProperties: false must always be set in objects
		// https://platform.openai.com/docs/guides/structured-outputs#additionalproperties-false-must-always-be-set-in-objects
		m.AdditionalProperties = &jsonschema.Schema{Not: &jsonschema.Schema{}} // false schema

		requires := make(map[string]struct{})
		for _, v := range m.Required {
			requires[v] = struct{}{}
		}
		for k, v := range m.Properties {
			if _, ok := requires[k]; !ok {
				requires[k] = struct{}{}
				// Add "null" only if not already present.
				if !slices.Contains(v.Types, "null") {
					v.Types = append(v.Types, "null")
				}
			}
			m.Properties[k] = FormatOpenAISchema(v)
		}

		// All fields must be required
		// https://platform.openai.com/docs/guides/structured-outputs#all-fields-must-be-required
		m.Required = slices.Collect(maps.Keys(requires))
	}
	return m
}

func (g *OpenAIGenerator) patchSchema(m *jsonschema.Schema) *jsonschema.Schema {
	if m == nil {
		return nil
	}
	s := m.CloneSchemas()
	if g.SchemaFormatter != nil {
		return g.SchemaFormatter(s)
	}
	return FormatOpenAISchema(s)
}

func oaiConvUsage(usage *openai.CompletionUsage) Usage {
	return Usage{
		PromptTokenCount:    usage.PromptTokens,
		GeneratedTokenCount: usage.CompletionTokens,
	}
}
