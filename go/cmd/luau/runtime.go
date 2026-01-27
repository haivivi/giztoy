package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/haivivi/giztoy/pkg/luau"
)

// HTTPResult holds the result of an async HTTP request.
type HTTPResult struct {
	Status  int
	Headers map[string]string
	Body    string
	Err     error
}

// PendingRequest represents an in-flight HTTP request.
type PendingRequest struct {
	ID       uint64
	Thread   *luau.Thread
	ResultCh chan HTTPResult
}

// Runtime holds the state for Luau script execution.
type Runtime struct {
	state         *luau.State
	libsDir       string
	kvs           map[string]any
	loaded        map[string]bool
	bytecodeCache map[string][]byte // Pre-compiled bytecode cache

	// Async HTTP support
	pendingMu      sync.Mutex
	pendingReqs    map[uint64]*PendingRequest
	nextRequestID  uint64
	completedReqs  chan *PendingRequest // Channel for completed requests
	currentThread  *luau.Thread         // Currently executing thread (for yield)
}

// InitAsync initializes the async HTTP support.
func (rt *Runtime) InitAsync() {
	rt.pendingReqs = make(map[uint64]*PendingRequest)
	rt.completedReqs = make(chan *PendingRequest, 100)
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

	// Set __builtin global
	rt.state.SetGlobal("__builtin")

	// Initialize __loaded table for module caching
	rt.state.NewTable()
	rt.state.SetGlobal("__loaded")

	// Register require function LAST to ensure it overrides any built-in
	if err := rt.state.RegisterFunc("require", rt.builtinRequire); err != nil {
		return err
	}

	return nil
}

// PrecompileModules walks the libs directory and pre-compiles all .luau files.
// This catches compile errors early and speeds up require() calls.
func (rt *Runtime) PrecompileModules() error {
	rt.bytecodeCache = make(map[string][]byte)

	// Check if libs directory exists
	if _, err := os.Stat(rt.libsDir); os.IsNotExist(err) {
		return nil // No libs directory, nothing to precompile
	}

	var compileErrors []string

	err := filepath.Walk(rt.libsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-luau files
		if info.IsDir() || !strings.HasSuffix(path, ".luau") {
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
		fmt.Printf("[luau] precompiled: %s (%d bytes)\n", moduleName, len(bytecode))

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk libs directory: %w", err)
	}

	if len(compileErrors) > 0 {
		return fmt.Errorf("compile errors:\n  %s", strings.Join(compileErrors, "\n  "))
	}

	fmt.Printf("[luau] precompiled %d modules\n", len(rt.bytecodeCache))
	return nil
}

// GetBytecode returns pre-compiled bytecode for a module, or nil if not found.
func (rt *Runtime) GetBytecode(name string) []byte {
	return rt.bytecodeCache[name]
}
