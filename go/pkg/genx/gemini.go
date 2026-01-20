package genx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/genai"
)

var _ Generator = (*GeminiGenerator)(nil)

// GeminiGenerator implements Generator using Google Gemini API.
type GeminiGenerator struct {
	Client *genai.Client `json:"-"`

	InvokeParams   *ModelParams `json:"invoke_params,omitzero"`
	GenerateParams *ModelParams `json:"generate_params,omitzero"`

	// Model should not start with "models/"
	Model string `json:"model"`
}

func (g *GeminiGenerator) Invoke(ctx context.Context, _ string, mctx ModelContext, fn *FuncTool) (Usage, *FuncCall, error) {
	cfg, contents, err := g.convModelContext(mctx)
	if err != nil {
		return Usage{}, nil, err
	}
	cfg.ResponseMIMEType = "application/json"
	cfg.ResponseSchema = geminiConvSchema(fn.Argument)
	resp, err := g.Client.Models.GenerateContent(ctx, g.Model, contents, cfg)
	if err != nil {
		if e, ok := err.(*apierror.APIError); ok {
			err = e.Unwrap()
		}
		return Usage{}, nil, err
	}
	if len(resp.Candidates) == 0 {
		return Usage{}, nil, fmt.Errorf("no candidates")
	}
	t := resp.Candidates[0]
	if t.FinishReason != genai.FinishReasonStop {
		if t.FinishReason == genai.FinishReasonMaxTokens {
			return geminiConvUsage(resp.UsageMetadata), nil, errors.New("max tokens")
		}
		return Usage{}, nil, fmt.Errorf("unexpected finish reason: %s", t.FinishReason)
	}
	var sb strings.Builder
	for _, p := range t.Content.Parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
	}
	return Usage{}, fn.NewFuncCall(sb.String()), nil
}

func (g *GeminiGenerator) GenerateStream(ctx context.Context, _ string, mctx ModelContext) (Stream, error) {
	cfg, contents, err := g.convModelContext(mctx)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("no contents")
	}
	sb := NewStreamBuilder(mctx, 32)
	go func() {
		if err := geminiPull(sb, g.Client.Models.GenerateContentStream(ctx, g.Model, contents, cfg)); err != nil {
			sb.Abort(err)
		}
	}()
	return sb.Stream(), nil
}

func geminiPull(builder *StreamBuilder, itr iter.Seq2[*genai.GenerateContentResponse, error]) error {
	var selIdx int32
	for chunk, err := range itr {
		if err != nil {
			return err
		}
		if len(chunk.Candidates) == 0 {
			continue
		}
		var sel *genai.Candidate
		if selIdx == 0 {
			selIdx = chunk.Candidates[0].Index
			sel = chunk.Candidates[0]
		} else {
			for _, c := range chunk.Candidates {
				if c.Index == selIdx {
					sel = c
					break
				}
			}
			if sel == nil {
				continue
			}
		}

		var (
			sb     strings.Builder
			blobs  = make(map[string]*bytes.Buffer)
			chunks = []*MessageChunk{}
		)
		for _, p := range sel.Content.Parts {
			switch {
			case p.Text != "":
				sb.WriteString(p.Text)
			case p.InlineData != nil:
				if _, ok := blobs[p.InlineData.MIMEType]; !ok {
					blobs[p.InlineData.MIMEType] = &bytes.Buffer{}
				}
				blobs[p.InlineData.MIMEType].Write(p.InlineData.Data)
			case p.FunctionCall != nil:
				b, _ := json.Marshal(p.FunctionCall.Args)
				chunks = append(chunks, &MessageChunk{
					Role: RoleModel,
					ToolCall: &ToolCall{
						ID: p.FunctionCall.Name,
						FuncCall: &FuncCall{
							Name:      p.FunctionCall.Name,
							Arguments: string(b),
						},
					},
				})
			default:
				return fmt.Errorf("unexpected part type: %T", p)
			}
		}
		if sb.Len() > 0 {
			chunks = append(chunks, &MessageChunk{
				Role: RoleModel,
				Part: Text(sb.String()),
			})
		}
		if len(blobs) > 0 {
			for mime, blob := range blobs {
				chunks = append(chunks, &MessageChunk{
					Role: RoleModel,
					Part: &Blob{
						MIMEType: mime,
						Data:     blob.Bytes(),
					},
				})
			}
		}
		if err := builder.Add(chunks...); err != nil {
			return err
		}
		switch sel.FinishReason {
		default:
			return builder.Unexpected(
				geminiConvUsage(chunk.UsageMetadata),
				fmt.Errorf("unexpected finish reason: %s", sel.FinishReason),
			)
		case genai.FinishReasonUnspecified, "":
			// continue
		case genai.FinishReasonStop:
			return builder.Done(geminiConvUsage(chunk.UsageMetadata))
		case genai.FinishReasonMaxTokens:
			return builder.Truncated(geminiConvUsage(chunk.UsageMetadata))
		case genai.FinishReasonSafety:
			var cats []string
			for _, sr := range sel.SafetyRatings {
				if sr.Blocked {
					cats = append(cats, string(sr.Category))
				}
			}
			return builder.Blocked(
				geminiConvUsage(chunk.UsageMetadata),
				"blocked by "+strings.Join(cats, ", "),
			)
		}
	}
	return errors.New("unexpected end of stream: no finish reason")
}

func geminiConvMessage(last *genai.Content, msg *Message) (new *genai.Content, err error) {
	var (
		role  string
		parts []*genai.Part
	)
	switch t := msg.Payload.(type) {
	default:
		return nil, fmt.Errorf("unexpected message type: %T", t)
	case Contents:
		switch msg.Role {
		default:
			return nil, fmt.Errorf("mismatched role and type: role=%s, type=%T", msg.Role, msg.Payload)
		case RoleUser:
			role = "user"
		case RoleModel:
			role = "model"
		}

		for _, c := range msg.Payload.(Contents) {
			switch v := c.(type) {
			case Text:
				parts = append(parts, genai.NewPartFromText(string(v)))
			case *Blob:
				parts = append(parts, genai.NewPartFromBytes(v.Data, v.MIMEType))
			}
		}
	case *ToolCall:
		role = "model"
		var args map[string]any
		if err := json.Unmarshal([]byte(t.FuncCall.Arguments), &args); err != nil {
			args = map[string]any{
				"text": t.FuncCall.Arguments,
			}
		}
		parts = append(parts, genai.NewPartFromFunctionCall(t.ID, args))
	case *ToolResult:
		role = "user"
		var result map[string]any
		if err := json.Unmarshal([]byte(t.Result), &result); err != nil {
			result = map[string]any{
				"text": t.Result,
			}
		}
		parts = append(parts, genai.NewPartFromFunctionResponse(t.ID, result))
	}
	if last == nil || last.Role != role {
		return &genai.Content{
			Role:  role,
			Parts: parts,
		}, nil
	}
	last.Parts = append(last.Parts, parts...)
	return nil, nil
}

func (g *GeminiGenerator) convModelContext(mctx ModelContext) (*genai.GenerateContentConfig, []*genai.Content, error) {
	cfg := genai.GenerateContentConfig{
		SafetySettings: []*genai.SafetySetting{
			{
				Category:  genai.HarmCategoryHateSpeech,
				Threshold: genai.HarmBlockThresholdOff,
			},
			{
				Category:  genai.HarmCategoryHarassment,
				Threshold: genai.HarmBlockThresholdOff,
			},
			{
				Category:  genai.HarmCategoryDangerousContent,
				Threshold: genai.HarmBlockThresholdOff,
			},
		},
	}
	prompts := []*genai.Part{}
	for p := range mctx.Prompts() {
		prompts = append(prompts, genai.NewPartFromText(p.Text))
	}
	if len(prompts) > 0 {
		cfg.SystemInstruction = &genai.Content{Parts: prompts}
	}
	mp := g.InvokeParams
	if p := mctx.Params(); p != nil {
		mp = p
	}
	if mp != nil {
		cfg.MaxOutputTokens = int32(mp.MaxTokens)
		cfg.Temperature = &mp.Temperature
		cfg.TopP = &mp.TopP
		cfg.TopK = &mp.TopK
	}

	tools := []*genai.Tool{}
	for t := range mctx.Tools() {
		switch t := t.(type) {
		case *FuncTool:
			tools = append(tools, &genai.Tool{
				FunctionDeclarations: []*genai.FunctionDeclaration{
					{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  geminiConvSchema(t.Argument),
					},
				},
			})
		default:
			return nil, nil, fmt.Errorf("unexpected tool type: %T", t)
		}
	}
	if len(tools) > 0 {
		cfg.Tools = tools
	}

	var (
		contents []*genai.Content
		last     *genai.Content
	)
	for msg := range mctx.Messages() {
		new, err := geminiConvMessage(last, msg)
		if err != nil {
			return nil, nil, err
		}
		if new != nil {
			contents = append(contents, new)
			last = new
		}
	}
	if len(contents) == 0 {
		return nil, nil, fmt.Errorf("no contents")
	}

	return &cfg, contents, nil
}

func geminiConvSchema(schema *jsonschema.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}

	enums := make([]string, 0, len(schema.Enum))
	for _, v := range schema.Enum {
		enums = append(enums, fmt.Sprintf("%v", v))
	}

	gs := genai.Schema{
		Format:      schema.Format,
		Description: schema.Description,
		Enum:        enums,
		Items:       geminiConvSchema(schema.Items),
		Required:    schema.Required,
	}

	requires := map[string]struct{}{}
	for _, v := range schema.Required {
		requires[v] = struct{}{}
	}

	if n := len(schema.Properties); n > 0 {
		gs.Properties = make(map[string]*genai.Schema, n)
		for k, prop := range schema.Properties {
			gs.Properties[k] = geminiConvSchema(prop)
		}
	}
	switch schema.Type {
	case "object":
		gs.Type = genai.TypeObject
	case "array":
		gs.Type = genai.TypeArray
	case "string":
		gs.Type = genai.TypeString
	case "number":
		gs.Type = genai.TypeNumber
	case "integer":
		gs.Type = genai.TypeInteger
	case "boolean":
		gs.Type = genai.TypeBoolean
	}
	return &gs
}

func geminiConvUsage(usage *genai.GenerateContentResponseUsageMetadata) Usage {
	return Usage{
		PromptTokenCount:        int64(usage.PromptTokenCount),
		CachedContentTokenCount: int64(usage.CachedContentTokenCount),
		GeneratedTokenCount:     int64(usage.CandidatesTokenCount),
	}
}
