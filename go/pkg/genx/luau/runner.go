package luau

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// ErrScriptError is returned when a Luau script execution fails.
var ErrScriptError = errors.New("luau script error")

// Runner executes Luau tool scripts with a ToolRuntime.
type Runner struct {
	mu     sync.Mutex
	state  *luau.State
	logger Logger

	// Compiled bytecode cache
	scripts map[string][]byte
}

// RunnerConfig holds configuration for creating a Runner.
type RunnerConfig struct {
	Logger Logger
}

// NewRunner creates a new Runner.
func NewRunner(cfg *RunnerConfig) (*Runner, error) {
	state, err := luau.New()
	if err != nil {
		return nil, fmt.Errorf("create luau state: %w", err)
	}

	state.OpenLibs()

	var logger Logger
	if cfg != nil && cfg.Logger != nil {
		logger = cfg.Logger
	} else {
		logger = &defaultLogger{}
	}

	return &Runner{
		state:   state,
		logger:  logger,
		scripts: make(map[string][]byte),
	}, nil
}

// Close releases resources.
func (r *Runner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
	return nil
}

// Compile compiles a Luau script and caches the bytecode.
func (r *Runner) Compile(name, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	bytecode, err := r.state.Compile(source, luau.OptO2)
	if err != nil {
		return fmt.Errorf("compile %s: %w", name, err)
	}

	r.scripts[name] = bytecode
	return nil
}

// Run executes a compiled script with the given input.
// The script should call rt:output(result, err) to return a value.
func (r *Runner) Run(ctx context.Context, name string, rt agent.Runtime, state agent.AgentState, input any) (any, error) {
	r.mu.Lock()
	bytecode, ok := r.scripts[name]
	if !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("script %q not found", name)
	}

	// Create a fresh state for this execution to avoid conflicts
	execState, err := luau.New()
	if err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("create execution state: %w", err)
	}
	r.mu.Unlock()

	defer execState.Close()
	execState.OpenLibs()

	// Create ToolRuntime
	tr := &toolRuntime{
		ctx:     ctx,
		runtime: rt,
		state:   state,
		logger:  r.logger,
	}
	tr.SetInput(input)

	// Register host functions
	r.registerHostFunctions(execState, tr)

	// Load and execute the script
	if err := execState.LoadBytecode(bytecode, name); err != nil {
		return nil, fmt.Errorf("load bytecode: %w", err)
	}

	if err := execState.PCall(0, 0); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScriptError, err)
	}

	// Get the output
	return tr.GetOutput()
}

// RunSource compiles and executes a script directly (for testing).
func (r *Runner) RunSource(ctx context.Context, source string, rt agent.Runtime, state agent.AgentState, input any) (any, error) {
	r.mu.Lock()
	// Create a fresh state for this execution
	execState, err := luau.New()
	if err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("create execution state: %w", err)
	}
	r.mu.Unlock()

	defer execState.Close()
	execState.OpenLibs()

	// Create ToolRuntime
	tr := &toolRuntime{
		ctx:     ctx,
		runtime: rt,
		state:   state,
		logger:  r.logger,
	}
	tr.SetInput(input)

	// Register host functions
	r.registerHostFunctions(execState, tr)

	// Execute the script
	if err := execState.DoString(source); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScriptError, err)
	}

	// Get the output
	return tr.GetOutput()
}

// registerHostFunctions registers all ToolRuntime methods as Luau globals.
func (r *Runner) registerHostFunctions(state *luau.State, tr *toolRuntime) {
	// Create 'rt' table
	state.NewTable()

	// rt:input() -> value
	state.RegisterFunc("__rt_input", func(s *luau.State) int {
		value, _ := tr.Input()
		pushLuaValue(s, value)
		return 1
	})
	state.GetGlobal("__rt_input")
	state.SetField(-2, "input")

	// rt:output(value, err) - Note: method call passes self as arg 1
	state.RegisterFunc("__rt_output", func(s *luau.State) int {
		var result any
		var errMsg error

		// Get result (arg 2, since arg 1 is self)
		if !s.IsNil(2) {
			result = toLuaValue(s, 2)
		}

		// Get error (arg 3)
		if !s.IsNil(3) {
			errMsg = errors.New(s.ToString(3))
		}

		tr.Output(result, errMsg)
		return 0
	})
	state.GetGlobal("__rt_output")
	state.SetField(-2, "output")

	// rt:generate(model, prompt) -> string, err
	state.RegisterFunc("__rt_generate", func(s *luau.State) int {
		// arg 1 is self, arg 2 is model, arg 3 is prompt
		model := s.ToString(2)
		prompt := s.ToString(3)

		result, err := tr.Generate(model, prompt)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}

		s.PushString(result)
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_generate")
	state.SetField(-2, "generate")

	// rt:generate_with_context(model, mctx) -> string, err
	// mctx is a table with:
	//   - system: string (optional, system prompt)
	//   - messages: array of {role, content, name?}
	state.RegisterFunc("__rt_generate_with_context", func(s *luau.State) int {
		// arg 1 is self
		model := s.ToString(2)

		if !s.IsTable(3) {
			s.PushNil()
			s.PushString("generate_with_context: expected table for context")
			return 2
		}

		mctxTable := toLuaTable(s, 3)
		mctx := buildModelContextFromTable(mctxTable)

		result, err := tr.GenerateWithContext(model, mctx)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}

		s.PushString(result)
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_generate_with_context")
	state.SetField(-2, "generate_with_context")

	// rt:http_get(url, params) -> response, err
	state.RegisterFunc("__rt_http_get", func(s *luau.State) int {
		// arg 1 is self
		url := s.ToString(2)
		var params map[string]any
		if s.IsTable(3) {
			params = toLuaTable(s, 3)
		}

		resp, err := tr.HTTPGet(url, params)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}

		// Push response as table
		s.NewTable()
		s.PushInteger(int64(resp.Status))
		s.SetField(-2, "status")
		s.PushString(resp.Body)
		s.SetField(-2, "body")

		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_http_get")
	state.SetField(-2, "http_get")

	// rt:http_post(url, opts) -> response, err
	state.RegisterFunc("__rt_http_post", func(s *luau.State) int {
		// arg 1 is self
		url := s.ToString(2)
		var opts *HTTPOptions
		if s.IsTable(3) {
			optsMap := toLuaTable(s, 3)
			opts = &HTTPOptions{}
			if body, ok := optsMap["body"]; ok {
				opts.Body = body
			}
			if headers, ok := optsMap["headers"].(map[string]any); ok {
				opts.Headers = make(map[string]string)
				for k, v := range headers {
					opts.Headers[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		resp, err := tr.HTTPPost(url, opts)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}

		s.NewTable()
		s.PushInteger(int64(resp.Status))
		s.SetField(-2, "status")
		s.PushString(resp.Body)
		s.SetField(-2, "body")

		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_http_post")
	state.SetField(-2, "http_post")

	// rt:state_get(key) -> value
	state.RegisterFunc("__rt_state_get", func(s *luau.State) int {
		// arg 1 is self
		key := s.ToString(2)
		value, ok := tr.StateGet(key)
		if !ok {
			s.PushNil()
			return 1
		}
		pushLuaValue(s, value)
		return 1
	})
	state.GetGlobal("__rt_state_get")
	state.SetField(-2, "state_get")

	// rt:state_set(key, value)
	state.RegisterFunc("__rt_state_set", func(s *luau.State) int {
		// arg 1 is self
		key := s.ToString(2)
		value := toLuaValue(s, 3)
		tr.StateSet(key, value)
		return 0
	})
	state.GetGlobal("__rt_state_set")
	state.SetField(-2, "state_set")

	// rt:state_delete(key)
	state.RegisterFunc("__rt_state_delete", func(s *luau.State) int {
		// arg 1 is self
		key := s.ToString(2)
		tr.StateDelete(key)
		return 0
	})
	state.GetGlobal("__rt_state_delete")
	state.SetField(-2, "state_delete")

	// rt:history_recent(n) -> messages, err
	state.RegisterFunc("__rt_history_recent", func(s *luau.State) int {
		// arg 1 is self
		n := int(s.ToInteger(2))
		msgs, err := tr.HistoryRecent(n)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}
		// Convert messages to Lua table
		s.CreateTable(len(msgs), 0)
		for i, msg := range msgs {
			s.NewTable()
			s.PushString(string(msg.Role))
			s.SetField(-2, "role")
			s.PushString(msg.Content)
			s.SetField(-2, "content")
			if msg.Name != "" {
				s.PushString(msg.Name)
				s.SetField(-2, "name")
			}
			s.RawSetI(-2, i+1)
		}
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_history_recent")
	state.SetField(-2, "history_recent")

	// rt:history_append(msg) -> err
	state.RegisterFunc("__rt_history_append", func(s *luau.State) int {
		// arg 1 is self, arg 2 is message table
		if !s.IsTable(2) {
			s.PushString("history_append: expected table")
			return 1
		}
		msgMap := toLuaTable(s, 2)
		msg := agentcfg.Message{
			Role:    agentcfg.MessageRole(fmt.Sprintf("%v", msgMap["role"])),
			Content: fmt.Sprintf("%v", msgMap["content"]),
		}
		if name, ok := msgMap["name"]; ok {
			msg.Name = fmt.Sprintf("%v", name)
		}
		err := tr.HistoryAppend(msg)
		if err != nil {
			s.PushString(err.Error())
			return 1
		}
		s.PushNil()
		return 1
	})
	state.GetGlobal("__rt_history_append")
	state.SetField(-2, "history_append")

	// rt:history_revert() -> err
	state.RegisterFunc("__rt_history_revert", func(s *luau.State) int {
		// arg 1 is self
		err := tr.HistoryRevert()
		if err != nil {
			s.PushString(err.Error())
			return 1
		}
		s.PushNil()
		return 1
	})
	state.GetGlobal("__rt_history_revert")
	state.SetField(-2, "history_revert")

	// rt:memory_summary() -> summary, err
	state.RegisterFunc("__rt_memory_summary", func(s *luau.State) int {
		// arg 1 is self
		summary, err := tr.MemorySummary()
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}
		s.PushString(summary)
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_memory_summary")
	state.SetField(-2, "memory_summary")

	// rt:memory_set_summary(summary) -> err
	state.RegisterFunc("__rt_memory_set_summary", func(s *luau.State) int {
		// arg 1 is self
		summary := s.ToString(2)
		err := tr.MemorySetSummary(summary)
		if err != nil {
			s.PushString(err.Error())
			return 1
		}
		s.PushNil()
		return 1
	})
	state.GetGlobal("__rt_memory_set_summary")
	state.SetField(-2, "memory_set_summary")

	// rt:memory_query(query) -> segments, err
	state.RegisterFunc("__rt_memory_query", func(s *luau.State) int {
		// arg 1 is self
		query := s.ToString(2)
		segments, err := tr.MemoryQuery(query)
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}
		// Convert segments to Lua table
		s.CreateTable(len(segments), 0)
		for i, seg := range segments {
			s.NewTable()
			s.PushString(seg.ID)
			s.SetField(-2, "id")
			s.PushString(seg.Summary)
			s.SetField(-2, "summary")
			if len(seg.Keywords) > 0 {
				s.CreateTable(len(seg.Keywords), 0)
				for j, kw := range seg.Keywords {
					s.PushString(kw)
					s.RawSetI(-2, j+1)
				}
				s.SetField(-2, "keywords")
			}
			s.RawSetI(-2, i+1)
		}
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_memory_query")
	state.SetField(-2, "memory_query")

	// rt:log(level, ...)
	state.RegisterFunc("__rt_log", func(s *luau.State) int {
		// arg 1 is self
		level := s.ToString(2)
		nargs := s.GetTop()
		args := make([]any, 0, nargs-2)
		for i := 3; i <= nargs; i++ {
			args = append(args, toLuaValue(s, i))
		}
		tr.Log(level, args...)
		return 0
	})
	state.GetGlobal("__rt_log")
	state.SetField(-2, "log")

	// Set 'rt' as global
	state.SetGlobal("rt")
}

// pushLuaValue pushes a Go value onto the Luau stack.
func pushLuaValue(s *luau.State, value any) {
	if value == nil {
		s.PushNil()
		return
	}

	switch v := value.(type) {
	case bool:
		s.PushBoolean(v)
	case int:
		s.PushInteger(int64(v))
	case int64:
		s.PushInteger(v)
	case float64:
		s.PushNumber(v)
	case string:
		s.PushString(v)
	case []byte:
		s.PushBytes(v)
	case map[string]any:
		s.NewTable()
		for k, val := range v {
			s.PushString(k)
			pushLuaValue(s, val)
			s.SetTable(-3)
		}
	case []any:
		s.CreateTable(len(v), 0)
		for i, val := range v {
			pushLuaValue(s, val)
			s.RawSetI(-2, i+1)
		}
	default:
		// Try JSON encoding for complex types
		if data, err := json.Marshal(v); err == nil {
			s.PushString(string(data))
		} else {
			s.PushString(fmt.Sprintf("%v", v))
		}
	}
}

// toLuaValue converts a Luau stack value to a Go value.
func toLuaValue(s *luau.State, idx int) any {
	switch s.TypeOf(idx) {
	case luau.TypeNil:
		return nil
	case luau.TypeBoolean:
		return s.ToBoolean(idx)
	case luau.TypeNumber:
		n := s.ToNumber(idx)
		// Check if it's an integer
		if n == float64(int64(n)) {
			return int64(n)
		}
		return n
	case luau.TypeString:
		return s.ToString(idx)
	case luau.TypeTable:
		return toLuaTable(s, idx)
	default:
		return nil
	}
}

// toLuaTable converts a Luau table to a Go map or slice.
func toLuaTable(s *luau.State, idx int) map[string]any {
	result := make(map[string]any)

	// Normalize index
	if idx < 0 {
		idx = s.GetTop() + idx + 1
	}

	s.PushNil() // First key
	for s.Next(idx) {
		// Key is at -2, value is at -1
		var key string
		switch s.TypeOf(-2) {
		case luau.TypeString:
			key = s.ToString(-2)
		case luau.TypeNumber:
			key = fmt.Sprintf("%v", s.ToNumber(-2))
		default:
			s.Pop(1) // Remove value, keep key for next iteration
			continue
		}

		value := toLuaValue(s, -1)
		result[key] = value

		s.Pop(1) // Remove value, keep key for next iteration
	}

	return result
}

// buildModelContextFromTable converts a Luau table to a genx.ModelContext.
// The table can have:
//   - system: string (system prompt)
//   - messages: array of {role, content, name?}
func buildModelContextFromTable(table map[string]any) genx.ModelContext {
	mcb := &genx.ModelContextBuilder{}

	// Add system prompt if present
	if system, ok := table["system"].(string); ok && system != "" {
		mcb.PromptText("system", system)
	}

	// Add messages if present
	if msgs, ok := table["messages"].(map[string]any); ok {
		// Messages come as a map with numeric keys from Lua array
		// Convert to ordered slice
		for i := 1; ; i++ {
			key := fmt.Sprintf("%d", i)
			msgAny, exists := msgs[key]
			if !exists {
				break
			}
			msgMap, ok := msgAny.(map[string]any)
			if !ok {
				continue
			}

			role := fmt.Sprintf("%v", msgMap["role"])
			content := fmt.Sprintf("%v", msgMap["content"])

			switch role {
			case "user":
				name := ""
				if n, ok := msgMap["name"].(string); ok {
					name = n
				}
				mcb.UserText(name, content)
			case "model", "assistant":
				name := ""
				if n, ok := msgMap["name"].(string); ok {
					name = n
				}
				mcb.ModelText(name, content)
			}
		}
	}

	return mcb.Build()
}
