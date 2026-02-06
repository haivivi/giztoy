// Package runtime provides a minimal Luau runtime with basic builtin functions.
// It includes HTTP, JSON, KVS, logging, environment, time, and module require support.
// It also supports generate (LLM), transformer (bidirectional streams), and cache.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// HTTPResult holds the result of an async HTTP request.
type HTTPResult struct {
	Status  int
	Headers map[string]string
	Body    string
	Err     error
}

// PendingOp represents a pending async operation waiting to be resumed.
type PendingOp struct {
	ID      uint64
	Thread  *luau.Thread
	Promise *Promise
}

// Runtime holds the state for Luau script execution.
type Runtime struct {
	ctx           context.Context
	state         *luau.State
	libsDir       string
	kvs           map[string]any
	loaded        map[string]bool
	bytecodeCache map[string][]byte // Pre-compiled bytecode cache

	// Async support (unified for all operations)
	pendingMu     sync.RWMutex // RWMutex for better read performance (HasPendingOps)
	pendingOps    map[uint64]*PendingOp
	completedOps  chan *PendingOp
	currentThread *luau.Thread // Currently executing thread (for yield)

	// Promise support
	promises *promiseRegistry

	// Cache support (with TTL)
	cache CacheProvider

	// Stream support
	streams *streamRegistry

	// Timeout support
	timeouts *timeoutRegistry

	// Generate support (LLM)
	generator     GeneratorFunc
	genxGenerator genx.Generator // Alternative: use genx.Generator interface directly

	// Transformer support
	transformersMu  sync.RWMutex
	transformers    map[string]TransformerFactory
	genxTransformer genx.Transformer // Alternative: use genx.Transformer interface directly

	// Context support (Agent or Tool)
	runtimeCtx Context
}

// Options configures the Runtime.
type Options struct {
	LibsDir string // Directory containing Luau library modules
}

// Option is a functional option for configuring Runtime.
type Option func(*Runtime)

// WithContext sets the context for the runtime.
func WithContext(ctx context.Context) Option {
	return func(rt *Runtime) {
		rt.ctx = ctx
	}
}

// WithLibsDir sets the libs directory.
func WithLibsDir(dir string) Option {
	return func(rt *Runtime) {
		rt.libsDir = dir
	}
}

// WithCache sets the cache provider.
func WithCache(cache CacheProvider) Option {
	return func(rt *Runtime) {
		rt.cache = cache
	}
}

// WithGenerator sets the LLM generator function.
func WithGenerator(gen GeneratorFunc) Option {
	return func(rt *Runtime) {
		rt.generator = gen
	}
}

// WithTransformer registers a transformer factory.
func WithTransformer(name string, factory TransformerFactory) Option {
	return func(rt *Runtime) {
		rt.transformersMu.Lock()
		defer rt.transformersMu.Unlock()
		if rt.transformers == nil {
			rt.transformers = make(map[string]TransformerFactory)
		}
		rt.transformers[name] = factory
	}
}

// WithGenxGenerator sets a genx.Generator interface (e.g., generators.DefaultMux).
// This allows using the genx generator mux directly without wrapping.
func WithGenxGenerator(gen genx.Generator) Option {
	return func(rt *Runtime) {
		rt.genxGenerator = gen
	}
}

// WithGenxTransformer sets a genx.Transformer interface (e.g., transformers.DefaultMux).
// This allows using the genx transformer mux directly without wrapping.
func WithGenxTransformer(t genx.Transformer) Option {
	return func(rt *Runtime) {
		rt.genxTransformer = t
	}
}

// WithRuntimeContext sets the runtime context (Agent or Tool).
func WithRuntimeContext(ctx Context) Option {
	return func(rt *Runtime) {
		rt.runtimeCtx = ctx
	}
}

// completedOpsBufferSize is the buffer size for completed operations channel.
// Larger buffer reduces contention when many operations complete simultaneously.
const completedOpsBufferSize = 256

// New creates a new Runtime with the given Luau state.
func New(state *luau.State, opts *Options) *Runtime {
	libsDir := ""
	if opts != nil {
		libsDir = opts.LibsDir
	}

	return &Runtime{
		ctx:           context.Background(),
		state:         state,
		libsDir:       libsDir,
		kvs:           make(map[string]any),
		loaded:        make(map[string]bool),
		bytecodeCache: make(map[string][]byte),
		streams:       newStreamRegistry(),
		promises:      newPromiseRegistry(),
		timeouts:      newTimeoutRegistry(),
		pendingOps:    make(map[uint64]*PendingOp),
		completedOps:  make(chan *PendingOp, completedOpsBufferSize),
		transformers:  make(map[string]TransformerFactory),
	}
}

// NewWithOptions creates a new Runtime with functional options.
func NewWithOptions(state *luau.State, options ...Option) *Runtime {
	rt := &Runtime{
		ctx:           context.Background(),
		state:         state,
		kvs:           make(map[string]any),
		loaded:        make(map[string]bool),
		bytecodeCache: make(map[string][]byte),
		streams:       newStreamRegistry(),
		promises:      newPromiseRegistry(),
		timeouts:      newTimeoutRegistry(),
		pendingOps:    make(map[uint64]*PendingOp),
		completedOps:  make(chan *PendingOp, completedOpsBufferSize),
		transformers:  make(map[string]TransformerFactory),
	}

	for _, opt := range options {
		opt(rt)
	}

	return rt
}

// RegisterTransformer adds a transformer factory at runtime.
func (rt *Runtime) RegisterTransformer(name string, factory TransformerFactory) {
	rt.transformersMu.Lock()
	defer rt.transformersMu.Unlock()
	rt.transformers[name] = factory
}

// SetGenerator sets the LLM generator function.
func (rt *Runtime) SetGenerator(gen GeneratorFunc) {
	rt.generator = gen
}

// SetCache sets the cache provider.
func (rt *Runtime) SetCache(cache CacheProvider) {
	rt.cache = cache
}

// SetContext sets the runtime context.
func (rt *Runtime) SetContext(ctx context.Context) {
	rt.ctx = ctx
}

// SetRuntimeContext sets the Agent/Tool context.
func (rt *Runtime) SetRuntimeContext(rctx Context) {
	rt.runtimeCtx = rctx
}

// GetRuntimeContext returns the current runtime context.
func (rt *Runtime) GetRuntimeContext() Context {
	return rt.runtimeCtx
}

// CreateAgentContext creates and sets an AgentContext for this runtime.
// Returns the AgentContext for external interaction.
func (rt *Runtime) CreateAgentContext(cfg *AgentContextConfig) *AgentContext {
	ac := NewAgentContext(cfg)
	rt.runtimeCtx = ac
	return ac
}

// CreateToolContext creates and sets a ToolContext for this runtime.
// Returns the ToolContext for external interaction.
func (rt *Runtime) CreateToolContext() *ToolContext {
	tc := NewToolContext()
	rt.runtimeCtx = tc
	return tc
}

// State returns the underlying Luau state.
func (rt *Runtime) State() *luau.State {
	return rt.state
}

// SetLibsDir sets the libs directory path.
func (rt *Runtime) SetLibsDir(dir string) {
	rt.libsDir = dir
}

// InitAsync initializes async support (kept for backward compatibility).
// Note: New() and NewWithOptions() now initialize async support automatically.
func (rt *Runtime) InitAsync() {
	if rt.pendingOps == nil {
		rt.pendingOps = make(map[uint64]*PendingOp)
	}
	if rt.completedOps == nil {
		rt.completedOps = make(chan *PendingOp, 100)
	}
	if rt.promises == nil {
		rt.promises = newPromiseRegistry()
	}
}

// RegisterBuiltins registers all builtin functions.
func (rt *Runtime) RegisterBuiltins() error {
	// Initialize async support
	rt.InitAsync()

	// Register __builtin table
	rt.state.NewTable()

	// __builtin.http
	if err := rt.state.RegisterFunc("__builtin_http", rt.builtinHTTP); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_http")
	rt.state.SetField(-2, "http")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_http")

	// __builtin.json_encode
	if err := rt.state.RegisterFunc("__builtin_json_encode", rt.builtinJSONEncode); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_json_encode")
	rt.state.SetField(-2, "json_encode")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_json_encode")

	// __builtin.json_decode
	if err := rt.state.RegisterFunc("__builtin_json_decode", rt.builtinJSONDecode); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_json_decode")
	rt.state.SetField(-2, "json_decode")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_json_decode")

	// __builtin.kvs_get
	if err := rt.state.RegisterFunc("__builtin_kvs_get", rt.builtinKVSGet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_get")
	rt.state.SetField(-2, "kvs_get")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_get")

	// __builtin.kvs_set
	if err := rt.state.RegisterFunc("__builtin_kvs_set", rt.builtinKVSSet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_set")
	rt.state.SetField(-2, "kvs_set")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_set")

	// __builtin.kvs_del
	if err := rt.state.RegisterFunc("__builtin_kvs_del", rt.builtinKVSDel); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_del")
	rt.state.SetField(-2, "kvs_del")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_del")

	// __builtin.cache_get
	if err := rt.state.RegisterFunc("__builtin_cache_get", rt.builtinCacheGet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_cache_get")
	rt.state.SetField(-2, "cache_get")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_cache_get")

	// __builtin.cache_set
	if err := rt.state.RegisterFunc("__builtin_cache_set", rt.builtinCacheSet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_cache_set")
	rt.state.SetField(-2, "cache_set")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_cache_set")

	// __builtin.cache_del
	if err := rt.state.RegisterFunc("__builtin_cache_del", rt.builtinCacheDelete); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_cache_del")
	rt.state.SetField(-2, "cache_del")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_cache_del")

	// __builtin.log
	if err := rt.state.RegisterFunc("__builtin_log", rt.builtinLog); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_log")
	rt.state.SetField(-2, "log")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_log")

	// __builtin.env
	if err := rt.state.RegisterFunc("__builtin_env", rt.builtinEnv); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_env")
	rt.state.SetField(-2, "env")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_env")

	// __builtin.time
	if err := rt.state.RegisterFunc("__builtin_time", rt.builtinTime); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_time")
	rt.state.SetField(-2, "time")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_time")

	// __builtin.parse_time
	if err := rt.state.RegisterFunc("__builtin_parse_time", rt.builtinParseTime); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_parse_time")
	rt.state.SetField(-2, "parse_time")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_parse_time")

	// __builtin.uuid
	if err := rt.state.RegisterFunc("__builtin_uuid", rt.builtinUUID); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_uuid")
	rt.state.SetField(-2, "uuid")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_uuid")

	// __builtin.generate (LLM)
	if err := rt.state.RegisterFunc("__builtin_generate", rt.builtinGenerate); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_generate")
	rt.state.SetField(-2, "generate")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_generate")

	// __builtin.transformer
	if err := rt.state.RegisterFunc("__builtin_transformer", rt.builtinTransformer); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_transformer")
	rt.state.SetField(-2, "transformer")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_transformer")

	// __builtin.sleep
	if err := rt.state.RegisterFunc("__builtin_sleep", rt.builtinSleep); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_sleep")
	rt.state.SetField(-2, "sleep")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_sleep")

	// __builtin.timeout
	if err := rt.state.RegisterFunc("__builtin_timeout", rt.builtinTimeout); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_timeout")
	rt.state.SetField(-2, "timeout")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_timeout")

	// __builtin.await_all
	if err := rt.state.RegisterFunc("__builtin_await_all", rt.builtinAwaitAll); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_await_all")
	rt.state.SetField(-2, "await_all")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_await_all")

	// __builtin.await_any
	if err := rt.state.RegisterFunc("__builtin_await_any", rt.builtinAwaitAny); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_await_any")
	rt.state.SetField(-2, "await_any")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_await_any")

	// Set __builtin global
	rt.state.SetGlobal("__builtin")

	// Register Promise methods as globals (used by pushPromiseObject)
	if err := rt.state.RegisterFunc("__promise_await", rt.builtinPromiseAwait); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__promise_is_ready", rt.builtinPromiseIsReady); err != nil {
		return err
	}

	// Register Timeout methods as globals (used by pushTimeoutHandle)
	if err := rt.state.RegisterFunc("__timeout_await", rt.builtinTimeoutAwait); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__timeout_is_ready", rt.builtinTimeoutIsReady); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__timeout_cancel", rt.builtinTimeoutCancel); err != nil {
		return err
	}

	// Register Stream methods as globals (used by pushStreamObject)
	if err := rt.state.RegisterFunc("__stream_recv", rt.builtinStreamRecv); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__stream_close", rt.builtinStreamClose); err != nil {
		return err
	}

	// Register BiStream methods as globals (used by pushBiStreamObject)
	if err := rt.state.RegisterFunc("__bistream_send", rt.builtinBiStreamSend); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__bistream_close_send", rt.builtinBiStreamCloseSend); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__bistream_recv", rt.builtinBiStreamRecv); err != nil {
		return err
	}
	if err := rt.state.RegisterFunc("__bistream_close", rt.builtinBiStreamClose); err != nil {
		return err
	}

	// Initialize __loaded table for module caching
	rt.state.NewTable()
	rt.state.SetGlobal("__loaded")

	// Register require function LAST to ensure it overrides any built-in
	if err := rt.state.RegisterFunc("require", rt.builtinRequire); err != nil {
		return err
	}

	return nil
}

// makeMethodWrapper creates a wrapper function that removes 'self' (arg 1)
// and shifts remaining arguments for method-style calls (rt:method(arg) -> rt.method(rt, arg)).
func (rt *Runtime) makeMethodWrapper(fn func(*luau.State) int) func(*luau.State) int {
	return func(state *luau.State) int {
		// Remove the first argument (self) by shifting stack
		nargs := state.GetTop()
		if nargs > 1 {
			state.Remove(1) // Remove 'self' at position 1
		}
		return fn(state)
	}
}

// RegisterContextFunctions registers context-specific functions (recv/emit or input/output).
// This creates an 'rt' table with the context methods and also includes all builtins.
// Should be called after RegisterBuiltins.
func (rt *Runtime) RegisterContextFunctions() error {
	// Create 'rt' table if it doesn't exist
	rt.state.GetGlobal("rt")
	if rt.state.IsNil(-1) {
		rt.state.Pop(1)
		rt.state.NewTable()
	}

	// Register builtin functions on the 'rt' table as methods
	// These are wrapped to skip the 'self' argument when called as rt:method(arg)
	builtinMethods := []struct {
		name string
		fn   func(*luau.State) int
	}{
		{"log", rt.builtinLog},
		{"time", rt.builtinTime},
		{"parse_time", rt.builtinParseTime},
		{"env", rt.builtinEnv},
		{"json_encode", rt.builtinJSONEncode},
		{"json_decode", rt.builtinJSONDecode},
		{"kvs_get", rt.builtinKVSGet},
		{"kvs_set", rt.builtinKVSSet},
		{"kvs_del", rt.builtinKVSDel},
		{"cache_get", rt.builtinCacheGet},
		{"cache_set", rt.builtinCacheSet},
		{"cache_del", rt.builtinCacheDelete},
		{"uuid", rt.builtinUUID},
		{"http", rt.builtinHTTP},
		{"generate", rt.builtinGenerate},
		{"transformer", rt.builtinTransformer},
		{"sleep", rt.builtinSleep},
		{"timeout", rt.builtinTimeout},
		{"await_all", rt.builtinAwaitAll},
		{"await_any", rt.builtinAwaitAny},
	}

	for _, m := range builtinMethods {
		globalName := "__rt_" + m.name
		// Wrap function to skip 'self' argument
		if err := rt.state.RegisterFunc(globalName, rt.makeMethodWrapper(m.fn)); err != nil {
			return err
		}
		rt.state.GetGlobal(globalName)
		rt.state.SetField(-2, m.name)
		rt.state.PushNil()
		rt.state.SetGlobal(globalName)
	}

	// Register context-specific functions (recv/emit or input/output)
	if rt.runtimeCtx != nil {
		rt.runtimeCtx.RegisterFunctions(rt.state)
	}

	// Set 'rt' as global
	rt.state.SetGlobal("rt")

	return nil
}

// RegisterAll registers both builtins and context functions.
// Convenience method that calls RegisterBuiltins and RegisterContextFunctions.
func (rt *Runtime) RegisterAll() error {
	if err := rt.RegisterBuiltins(); err != nil {
		return err
	}
	return rt.RegisterContextFunctions()
}

// PrecompileModules walks the libs directory and pre-compiles all .luau files.
// This catches compile errors early and speeds up require() calls.
func (rt *Runtime) PrecompileModules() error {
	rt.bytecodeCache = make(map[string][]byte)

	// Check if libs directory exists
	if rt.libsDir == "" {
		return nil
	}
	if _, err := os.Stat(rt.libsDir); os.IsNotExist(err) {
		return nil // No libs directory, nothing to precompile
	}

	var compileErrors []string

	err := filepath.Walk(rt.libsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, non-luau files, and type declaration files (.d.luau)
		if info.IsDir() || !strings.HasSuffix(path, ".luau") || strings.HasSuffix(path, ".d.luau") {
			return nil
		}

		// Calculate module name from path
		relPath, err := filepath.Rel(rt.libsDir, path)
		if err != nil {
			return err
		}

		// Convert path to module name:
		// haivivi/http.luau -> haivivi/http
		// haivivi/init.luau -> haivivi
		moduleName := strings.TrimSuffix(relPath, ".luau")
		if strings.HasSuffix(moduleName, "/init") {
			moduleName = strings.TrimSuffix(moduleName, "/init")
		}

		// Read source
		source, err := os.ReadFile(path)
		if err != nil {
			compileErrors = append(compileErrors, fmt.Sprintf("%s: read error: %v", path, err))
			return nil
		}

		// Compile to bytecode
		bytecode, err := rt.state.Compile(string(source), luau.OptO2)
		if err != nil {
			compileErrors = append(compileErrors, fmt.Sprintf("%s: compile error: %v", path, err))
			return nil
		}

		// Cache bytecode
		rt.bytecodeCache[moduleName] = bytecode

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk libs directory: %w", err)
	}

	if len(compileErrors) > 0 {
		return fmt.Errorf("compile errors:\n  %s", strings.Join(compileErrors, "\n  "))
	}

	return nil
}

// GetBytecode returns pre-compiled bytecode for a module, or nil if not found.
func (rt *Runtime) GetBytecode(name string) []byte {
	return rt.bytecodeCache[name]
}

// RunSync executes a Luau script synchronously (blocking, no async support).
// Deprecated: Use Run() for async support.
func (rt *Runtime) RunSync(source, chunkname string) error {
	return rt.state.DoStringOpt(source, chunkname, luau.OptO2)
}

// Polling intervals for the event loop.
const (
	minPollInterval = 100 * time.Microsecond // Minimum poll interval (100us)
	maxPollInterval = 5 * time.Millisecond   // Maximum poll interval (5ms)
)

// Run runs a script in a coroutine with event loop for async operations.
// All async operations (HTTP, stream recv/send) will yield and be resumed
// when their results are ready.
func (rt *Runtime) Run(source, chunkname string) error {
	// Compile script to bytecode
	bytecode, err := rt.state.Compile(source, luau.OptO2)
	if err != nil {
		return fmt.Errorf("compile error: %w", err)
	}

	// Create a new thread to run the script
	thread, err := rt.state.NewThread()
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}
	defer thread.Close()

	// Load bytecode onto thread's stack
	if err := thread.LoadBytecode(bytecode, chunkname); err != nil {
		return fmt.Errorf("failed to load bytecode: %w", err)
	}

	// Set current thread for async operations
	rt.currentThread = thread

	// Initial resume (start the script)
	status, _ := thread.Resume(0)

	// Event loop with adaptive polling
	pollInterval := minPollInterval
	consecutiveEmpty := 0

	for status == luau.CoStatusYield || rt.HasPendingOps() {
		if rt.HasPendingOps() {
			// Try to poll without blocking first
			processed := rt.PollCompletedNonBlocking()

			if processed {
				// Got work done - reset to fast polling
				pollInterval = minPollInterval
				consecutiveEmpty = 0
			} else {
				// No work available - back off gradually
				consecutiveEmpty++
				if consecutiveEmpty > 3 {
					// Increase poll interval (exponential backoff with cap)
					pollInterval = pollInterval * 2
					if pollInterval > maxPollInterval {
						pollInterval = maxPollInterval
					}
				}
				time.Sleep(pollInterval)
			}
		}

		// Check thread status
		status = thread.Status()
		if status == luau.CoStatusOK {
			break
		}
		if status != luau.CoStatusYield {
			// Error
			errMsg := thread.ToString(-1)
			return fmt.Errorf("runtime error: %s", errMsg)
		}
	}

	rt.currentThread = nil

	// Check final status
	if status != luau.CoStatusOK {
		errMsg := thread.ToString(-1)
		return fmt.Errorf("runtime error: %s", errMsg)
	}

	return nil
}

// RunAsync is an alias for Run (kept for backward compatibility).
func (rt *Runtime) RunAsync(source, chunkname string) error {
	return rt.Run(source, chunkname)
}

// registerPendingPromise registers a promise as pending, waiting to be resolved.
func (rt *Runtime) registerPendingPromise(promise *Promise) {
	rt.pendingMu.Lock()
	defer rt.pendingMu.Unlock()

	op := &PendingOp{
		ID:      promise.id,
		Thread:  rt.currentThread,
		Promise: promise,
	}
	rt.pendingOps[promise.id] = op

	// Start a goroutine to watch for promise completion
	go func() {
		result := <-promise.ResultChan()
		_ = result // Result is stored in promise
		rt.completedOps <- op
	}()
}

// HasPendingOps returns true if there are pending async operations.
// Uses RLock for better read performance.
func (rt *Runtime) HasPendingOps() bool {
	rt.pendingMu.RLock()
	defer rt.pendingMu.RUnlock()
	return len(rt.pendingOps) > 0
}

// PollCompletedNonBlocking checks for completed operations without blocking.
// Returns true if an operation was processed.
func (rt *Runtime) PollCompletedNonBlocking() bool {
	select {
	case op := <-rt.completedOps:
		rt.processCompletedOp(op)
		return true
	default:
		return false
	}
}

// PollCompleted checks for completed operations and resumes their threads.
// This may block briefly if no operations are ready.
func (rt *Runtime) PollCompleted() bool {
	select {
	case op := <-rt.completedOps:
		rt.processCompletedOp(op)

		rt.pendingMu.RLock()
		hasPending := len(rt.pendingOps) > 0
		rt.pendingMu.RUnlock()

		return hasPending || op.Thread.Status() == luau.CoStatusYield

	default:
		rt.pendingMu.RLock()
		hasPending := len(rt.pendingOps) > 0
		rt.pendingMu.RUnlock()
		return hasPending
	}
}

// processCompletedOp handles a completed operation by resuming its thread.
func (rt *Runtime) processCompletedOp(op *PendingOp) {
	// Remove from pending map
	rt.pendingMu.Lock()
	delete(rt.pendingOps, op.ID)
	rt.pendingMu.Unlock()

	// Get the result from the promise
	result, _ := op.Promise.Result()

	// Push result onto thread's stack
	nresults := rt.pushPromiseResult(op.Thread.State, result)

	// Clean up promise (returns to pool)
	rt.promises.removePromise(op.Promise.id)

	// Resume the thread
	rt.currentThread = op.Thread
	status, _ := op.Thread.Resume(nresults)
	rt.currentThread = nil

	if status != luau.CoStatusOK && status != luau.CoStatusYield {
		errMsg := op.Thread.ToString(-1)
		slog.Error("luau thread error", "error", errMsg)
	}
}

// KVSGet returns a value from the key-value store.
func (rt *Runtime) KVSGet(key string) (any, bool) {
	v, ok := rt.kvs[key]
	return v, ok
}

// KVSSet stores a value in the key-value store.
func (rt *Runtime) KVSSet(key string, value any) {
	rt.kvs[key] = value
}

// KVSDel deletes a value from the key-value store.
func (rt *Runtime) KVSDel(key string) {
	delete(rt.kvs, key)
}
