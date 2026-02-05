package registry

import (
	"fmt"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// requireStateSimple implements require() using __loaded global for caching.
type requireStateSimple struct {
	registry Registry
	state    *luau.State

	// Module cache
	mu     sync.RWMutex
	loaded map[string]bool

	// Bytecode cache (for re-execution)
	bytecode map[string][]byte

	// Cycle detection
	loadingMu sync.Mutex
	loading   map[string]bool
}

// newRequireFuncSimple creates a simpler require function.
func newRequireFuncSimple(reg Registry, state *luau.State) luau.GoFunc {
	rs := &requireStateSimple{
		registry: reg,
		state:    state,
		loaded:   make(map[string]bool),
		bytecode: make(map[string][]byte),
		loading:  make(map[string]bool),
	}
	return rs.require
}

func (rs *requireStateSimple) require(state *luau.State) int {
	moduleName := state.ToString(1)
	if moduleName == "" {
		state.PushNil()
		state.PushString("require: module name is required")
		return 2
	}

	name, constraint := ParseRequireName(moduleName)

	// Check __loaded global table first
	state.GetGlobal("__loaded")
	if state.IsTable(-1) {
		state.GetField(-1, name)
		if !state.IsNil(-1) {
			// Already loaded, return cached value
			state.Remove(-2) // Remove __loaded table
			return 1
		}
		state.Pop(1) // Pop nil
	}
	state.Pop(1) // Pop __loaded

	// Check for cyclic dependency
	rs.loadingMu.Lock()
	if rs.loading[name] {
		rs.loadingMu.Unlock()
		state.PushNil()
		state.PushString(fmt.Sprintf("%s: %s", ErrCyclicDependency.Error(), name))
		return 2
	}
	rs.loading[name] = true
	rs.loadingMu.Unlock()

	defer func() {
		rs.loadingMu.Lock()
		delete(rs.loading, name)
		rs.loadingMu.Unlock()
	}()

	// Check bytecode cache
	rs.mu.RLock()
	bytecode, hasBytecode := rs.bytecode[name]
	rs.mu.RUnlock()

	if !hasBytecode {
		// Resolve the package
		pkg, err := rs.registry.Resolve(name, constraint)
		if err != nil {
			state.PushNil()
			state.PushString(fmt.Sprintf("require: %s: %v", name, err))
			return 2
		}

		// Compile
		bytecode, err = state.Compile(string(pkg.Entry), luau.OptO2)
		if err != nil {
			state.PushNil()
			state.PushString(fmt.Sprintf("require: compile %s: %v", name, err))
			return 2
		}

		// Cache bytecode
		rs.mu.Lock()
		rs.bytecode[name] = bytecode
		rs.mu.Unlock()
	}

	// Load bytecode
	if err := state.LoadBytecode(bytecode, name); err != nil {
		state.PushNil()
		state.PushString(fmt.Sprintf("require: load %s: %v", name, err))
		return 2
	}

	// Execute the module
	if err := state.PCall(0, 1); err != nil {
		state.PushNil()
		state.PushString(fmt.Sprintf("require: execute %s: %v", name, err))
		return 2
	}

	// If module returned nil, push an empty table
	if state.IsNil(-1) {
		state.Pop(1)
		state.NewTable()
	}

	// Cache in __loaded
	state.GetGlobal("__loaded")
	if state.IsTable(-1) {
		state.PushValue(-2) // Push module result
		state.SetField(-2, name)
	}
	state.Pop(1) // Pop __loaded

	return 1
}

// RegisterRequireFunc registers the require function for a registry.
func RegisterRequireFunc(state *luau.State, reg Registry) error {
	// Ensure __loaded table exists
	state.GetGlobal("__loaded")
	if state.IsNil(-1) {
		state.Pop(1)
		state.NewTable()
		state.SetGlobal("__loaded")
	} else {
		state.Pop(1)
	}

	// Register require function
	fn := newRequireFuncSimple(reg, state)
	return state.RegisterFunc("require", fn)
}
