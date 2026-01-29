package luau

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// ErrNoOutput is returned when GetOutput is called but Output was never called.
var ErrNoOutput = errors.New("luau tool did not call output")

// HTTPResponse represents an HTTP response for Luau scripts.
type HTTPResponse struct {
	Status  int
	Headers map[string]string
	Body    string
}

// HTTPOptions represents HTTP request options.
type HTTPOptions struct {
	Headers map[string]string
	Body    any
}

// ToolRuntime provides capabilities for Luau tool scripts.
// All async methods should be called via the scheduler's yield/resume mechanism.
type ToolRuntime interface {
	// Context returns the underlying context.Context for cancellation.
	Context() context.Context

	// --- Tool I/O (one-shot request/response) ---

	// Input returns the input value passed by the Go caller.
	// Luau usage: local args = rt:input()
	Input() (any, error)

	// Output sets the output value to be returned to the Go caller.
	// Luau usage: rt:output(result, nil) or rt:output(nil, "error message")
	Output(result any, err error)

	// --- LLM Generation ---

	// GenerateStream returns a stream of MessageChunks.
	// Luau usage: for chunk in ctx.generate_stream(model, mctx) do ... end
	GenerateStream(model string, mctx genx.ModelContext) (genx.Stream, error)

	// Generate is a convenience method that collects all chunks and returns full text.
	// Luau usage: local result = ctx.generate(model, prompt)
	Generate(model string, prompt string) (string, error)

	// GenerateWithContext generates with a ModelContext.
	GenerateWithContext(model string, mctx genx.ModelContext) (string, error)

	// Invoke calls LLM with tool for structured output.
	// Returns FuncCall with parsed arguments.
	Invoke(model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error)

	// --- HTTP ---

	// HTTPGet performs an HTTP GET request.
	HTTPGet(url string, params map[string]any) (*HTTPResponse, error)

	// HTTPPost performs an HTTP POST request.
	HTTPPost(url string, opts *HTTPOptions) (*HTTPResponse, error)

	// --- Tool ---

	// InvokeTool calls another tool by name.
	InvokeTool(toolName string, args any) (any, error)

	// --- State ---

	// StateGet retrieves a state property.
	StateGet(key string) (any, bool)

	// StateSet stores a state property.
	StateSet(key string, value any)

	// StateDelete removes a state property.
	StateDelete(key string)

	// --- History ---

	// HistoryRecent returns the most recent n messages.
	// If n <= 0, returns all recent messages (determined by state configuration).
	HistoryRecent(n int) ([]agentcfg.Message, error)

	// HistoryAppend stores a new message.
	HistoryAppend(msg agentcfg.Message) error

	// HistoryRevert undoes the last round of conversation.
	HistoryRevert() error

	// --- Memory ---

	// MemorySummary returns the long-term compressed summary.
	MemorySummary() (string, error)

	// MemorySetSummary updates the long-term summary.
	MemorySetSummary(summary string) error

	// MemoryQuery searches for relevant memory segments using RAG.
	MemoryQuery(query string) ([]agentcfg.MemorySegment, error)

	// --- Agent Info (read-only) ---

	// AgentName returns the agent definition name.
	AgentName() string

	// AgentModel returns the default model for this agent.
	AgentModel() string

	// StateID returns the unique state identifier.
	StateID() string

	// --- Logging ---

	// Log outputs a log message.
	Log(level string, args ...any)
}

// toolRuntime implements ToolRuntime.
type toolRuntime struct {
	ctx        context.Context
	runtime    agent.Runtime
	state      agent.AgentState
	logger     Logger
	agentName  string
	agentModel string

	// I/O for one-shot tool execution
	input     any
	output    any
	outputErr error
	outputSet bool
}

// Logger interface for logging.
type Logger interface {
	Log(level string, args ...any)
}

// defaultLogger is a simple logger that prints to stdout.
type defaultLogger struct{}

func (l *defaultLogger) Log(level string, args ...any) {
	fmt.Printf("[%s] %s\n", level, fmt.Sprint(args...))
}

// ToolRuntimeConfig holds configuration for creating a ToolRuntime.
type ToolRuntimeConfig struct {
	AgentName  string
	AgentModel string
	Logger     Logger
}

// NewToolRuntime creates a new ToolRuntime.
func NewToolRuntime(ctx context.Context, rt agent.Runtime, state agent.AgentState, cfg *ToolRuntimeConfig) ToolRuntime {
	var logger Logger
	var agentName, agentModel string

	if cfg != nil {
		logger = cfg.Logger
		agentName = cfg.AgentName
		agentModel = cfg.AgentModel
	}

	if logger == nil {
		logger = &defaultLogger{}
	}

	return &toolRuntime{
		ctx:        ctx,
		runtime:    rt,
		state:      state,
		logger:     logger,
		agentName:  agentName,
		agentModel: agentModel,
	}
}

func (tr *toolRuntime) Context() context.Context {
	return tr.ctx
}

// --- Tool I/O Methods ---

func (tr *toolRuntime) Input() (any, error) {
	return tr.input, nil
}

func (tr *toolRuntime) Output(result any, err error) {
	tr.output = result
	tr.outputErr = err
	tr.outputSet = true
}

// SetInput sets the input value (called by Runner before execution).
func (tr *toolRuntime) SetInput(input any) {
	tr.input = input
}

// GetOutput returns the output value and error (called by Runner after execution).
// Returns (nil, ErrNoOutput) if Output was never called.
func (tr *toolRuntime) GetOutput() (any, error) {
	if !tr.outputSet {
		return nil, ErrNoOutput
	}
	return tr.output, tr.outputErr
}

// ResetIO resets the I/O state for reuse.
func (tr *toolRuntime) ResetIO() {
	tr.input = nil
	tr.output = nil
	tr.outputErr = nil
	tr.outputSet = false
}

func (tr *toolRuntime) GenerateStream(model string, mctx genx.ModelContext) (genx.Stream, error) {
	return tr.runtime.GenerateStream(tr.ctx, model, mctx)
}

func (tr *toolRuntime) Generate(model string, prompt string) (string, error) {
	// Build a simple ModelContext from the prompt
	mcb := &genx.ModelContextBuilder{}
	mcb.UserText("", prompt)
	return tr.GenerateWithContext(model, mcb.Build())
}

func (tr *toolRuntime) GenerateWithContext(model string, mctx genx.ModelContext) (string, error) {
	stream, err := tr.runtime.GenerateStream(tr.ctx, model, mctx)
	if err != nil {
		return "", fmt.Errorf("generate stream: %w", err)
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		chunk, err := stream.Next()
		if err != nil {
			if errors.Is(err, genx.ErrDone) {
				break
			}
			return "", fmt.Errorf("stream next: %w", err)
		}
		if chunk != nil && chunk.Part != nil {
			if text, ok := chunk.Part.(genx.Text); ok {
				sb.WriteString(string(text))
			}
		}
	}

	return sb.String(), nil
}

func (tr *toolRuntime) Invoke(model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return tr.runtime.Invoke(tr.ctx, model, mctx, tool)
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func (tr *toolRuntime) HTTPGet(targetURL string, params map[string]any) (*HTTPResponse, error) {
	// Build query string with proper URL encoding
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, fmt.Sprintf("%v", v))
		}
		// Handle URLs that may already have query parameters
		if strings.Contains(targetURL, "?") {
			targetURL = targetURL + "&" + values.Encode()
		} else {
			targetURL = targetURL + "?" + values.Encode()
		}
	}

	req, err := http.NewRequestWithContext(tr.ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &HTTPResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(body),
	}, nil
}

func (tr *toolRuntime) HTTPPost(url string, opts *HTTPOptions) (*HTTPResponse, error) {
	var body io.Reader
	if opts != nil && opts.Body != nil {
		switch b := opts.Body.(type) {
		case string:
			body = strings.NewReader(b)
		case []byte:
			body = strings.NewReader(string(b))
		case map[string]any, []any:
			// JSON encode complex types
			data, err := json.Marshal(b)
			if err != nil {
				return nil, fmt.Errorf("marshal body to JSON: %w", err)
			}
			body = strings.NewReader(string(data))
		default:
			return nil, fmt.Errorf("unsupported body type: %T (expected string, []byte, or table)", opts.Body)
		}
	}

	req, err := http.NewRequestWithContext(tr.ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}

	if opts != nil && opts.Headers != nil {
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &HTTPResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(respBody),
	}, nil
}

func (tr *toolRuntime) InvokeTool(toolName string, args any) (any, error) {
	tool, err := tr.runtime.GetTool(tr.ctx, toolName)
	if err != nil {
		return nil, fmt.Errorf("get tool %s: %w", toolName, err)
	}

	// Convert args to JSON string
	var argsStr string
	switch v := args.(type) {
	case string:
		argsStr = v
	case map[string]any, []any:
		// Luau tables are converted to map or slice, serialize to JSON
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal tool args to JSON: %w", err)
		}
		argsStr = string(b)
	default:
		return nil, fmt.Errorf("args must be a JSON string or table, got %T", args)
	}

	call := tool.NewFuncCall(argsStr)
	return call.Invoke(tr.ctx)
}

func (tr *toolRuntime) StateGet(key string) (any, bool) {
	if tr.state == nil {
		return nil, false
	}
	return tr.state.Get(key)
}

func (tr *toolRuntime) StateSet(key string, value any) {
	if tr.state == nil {
		return
	}
	tr.state.Set(key, value)
}

func (tr *toolRuntime) StateDelete(key string) {
	if tr.state == nil {
		return
	}
	tr.state.Delete(key)
}

func (tr *toolRuntime) Log(level string, args ...any) {
	tr.logger.Log(level, args...)
}

// --- History Methods ---

func (tr *toolRuntime) HistoryRecent(n int) ([]agentcfg.Message, error) {
	if tr.state == nil {
		return nil, errors.New("no state available")
	}
	// LoadRecent returns messages based on state's configuration
	// The n parameter is used to limit, but if n <= 0, return all
	messages, err := tr.state.LoadRecent(tr.ctx)
	if err != nil {
		return nil, fmt.Errorf("load recent messages: %w", err)
	}
	if n > 0 && len(messages) > n {
		// Return only the last n messages
		messages = messages[len(messages)-n:]
	}
	return messages, nil
}

func (tr *toolRuntime) HistoryAppend(msg agentcfg.Message) error {
	if tr.state == nil {
		return errors.New("no state available")
	}
	return tr.state.StoreMessage(tr.ctx, msg)
}

func (tr *toolRuntime) HistoryRevert() error {
	if tr.state == nil {
		return errors.New("no state available")
	}
	return tr.state.Revert(tr.ctx)
}

// --- Memory Methods ---

func (tr *toolRuntime) MemorySummary() (string, error) {
	if tr.state == nil {
		return "", errors.New("no state available")
	}
	return tr.state.Summary(tr.ctx)
}

func (tr *toolRuntime) MemorySetSummary(summary string) error {
	if tr.state == nil {
		return errors.New("no state available")
	}
	return tr.state.SetSummary(tr.ctx, summary)
}

func (tr *toolRuntime) MemoryQuery(query string) ([]agentcfg.MemorySegment, error) {
	if tr.state == nil {
		return nil, errors.New("no state available")
	}
	return tr.state.Query(tr.ctx, agentcfg.MemoryQuery{Text: query})
}

// --- Agent Info Methods ---

func (tr *toolRuntime) AgentName() string {
	return tr.agentName
}

func (tr *toolRuntime) AgentModel() string {
	return tr.agentModel
}

func (tr *toolRuntime) StateID() string {
	if tr.state == nil {
		return ""
	}
	return tr.state.ID()
}
