package genx

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

var _ Tool = (*FuncTool)(nil)

type FuncToolOption[ArgType any] interface {
	applyToFuncTool(*FuncTool)
}

func WithSchema[T any](s *jsonschema.Schema) FuncToolOption[any] {
	return &typeSchemaOption{t: reflect.TypeFor[T](), s: s}
}

type typeSchemaOption struct {
	t reflect.Type
	s *jsonschema.Schema
}

func (o *typeSchemaOption) applyToFuncTool(t *FuncTool) {
	t.typeSchemas[o.t] = o.s
}

type InvokeFunc[T any] func(ctx context.Context, call *FuncCall, arg T) (any, error)

func (fn InvokeFunc[T]) applyToFuncTool(t *FuncTool) {
	t.Invoke = func(ctx context.Context, call *FuncCall, arg string) (any, error) {
		var v T
		if err := unmarshalJSON([]byte(arg), &v); err != nil {
			return nil, fmt.Errorf("unmarshal %q error: %w", arg, err)
		}
		return fn(ctx, call, v)
	}
}

type FuncTool struct {
	Name        string
	Description string
	Argument    *jsonschema.Schema

	typeSchemas map[reflect.Type]*jsonschema.Schema

	Invoke InvokeFunc[string]
}

func (tool *FuncTool) NewFuncCall(args string) *FuncCall {
	return &FuncCall{
		Name:      tool.Name,
		Arguments: args,

		tool: tool,
	}
}

func (*FuncTool) isTool() {}

func NewFuncTool[ArgType any](name, descrption string, opts ...FuncToolOption[ArgType]) (*FuncTool, error) {
	tool := &FuncTool{
		Name:        name,
		Description: descrption,
		typeSchemas: make(map[reflect.Type]*jsonschema.Schema),
	}
	for _, opt := range opts {
		opt.applyToFuncTool(tool)
	}
	arg, err := jsonschema.For[ArgType](&jsonschema.ForOptions{
		TypeSchemas: tool.typeSchemas,
	})
	if err != nil {
		return nil, err
	}
	tool.Argument = arg

	if tool.Invoke == nil {
		tool.Invoke = func(ctx context.Context, _ *FuncCall, arg string) (any, error) {
			var v ArgType
			if err := unmarshalJSON([]byte(arg), &v); err != nil {
				return nil, fmt.Errorf("unmarshal %q error: %w", arg, err)
			}
			return &v, nil
		}
	}
	return tool, nil
}

func MustNewFuncTool[ArgType any](name, descrption string, opts ...FuncToolOption[ArgType]) *FuncTool {
	tool, err := NewFuncTool(name, descrption, opts...)
	if err != nil {
		panic(err)
	}
	return tool
}
