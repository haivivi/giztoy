package luau

/*
#cgo CXXFLAGS: -std=c++17 -fno-exceptions -fno-rtti
#include "luau_wrapper.h"
#include <stdlib.h>

// Forward declaration for Go callback
extern int goExternalCallback(LuauState* L, uint64_t callback_id);
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Error types returned by Luau operations.
var (
	ErrCompile = errors.New("luau: compilation error")
	ErrRuntime = errors.New("luau: runtime error")
	ErrMemory  = errors.New("luau: memory error")
	ErrInvalid = errors.New("luau: invalid operation")
)

// Type represents a Luau value type.
type Type int

const (
	TypeNil      Type = C.LUAU_TYPE_NIL
	TypeBoolean  Type = C.LUAU_TYPE_BOOLEAN
	TypeNumber   Type = C.LUAU_TYPE_NUMBER
	TypeString   Type = C.LUAU_TYPE_STRING
	TypeTable    Type = C.LUAU_TYPE_TABLE
	TypeFunction Type = C.LUAU_TYPE_FUNCTION
	TypeUserdata Type = C.LUAU_TYPE_USERDATA
	TypeThread   Type = C.LUAU_TYPE_THREAD
	TypeBuffer   Type = C.LUAU_TYPE_BUFFER
	TypeVector   Type = C.LUAU_TYPE_VECTOR
)

// String returns the type name.
func (t Type) String() string {
	switch t {
	case TypeNil:
		return "nil"
	case TypeBoolean:
		return "boolean"
	case TypeNumber:
		return "number"
	case TypeString:
		return "string"
	case TypeTable:
		return "table"
	case TypeFunction:
		return "function"
	case TypeUserdata:
		return "userdata"
	case TypeThread:
		return "thread"
	case TypeBuffer:
		return "buffer"
	case TypeVector:
		return "vector"
	default:
		return "unknown"
	}
}

// OptLevel represents the compiler optimization level.
type OptLevel int

const (
	OptNone OptLevel = C.LUAU_OPT_NONE
	OptO1   OptLevel = C.LUAU_OPT_O1
	OptO2   OptLevel = C.LUAU_OPT_O2
)

// GoFunc is the signature for Go functions callable from Luau.
// The function receives the Luau state and returns the number of return values
// pushed onto the stack.
type GoFunc func(*State) int

// Global function registry for all States
var (
	globalFuncsMu    sync.RWMutex
	globalFuncs      = make(map[uint64]goFuncEntry)
	globalFuncNextID uint64
)

// goFuncEntry stores a registered function and its associated state
type goFuncEntry struct {
	fn    GoFunc
	state *State
}

// State represents a Luau virtual machine state.
type State struct {
	L           *C.LuauState
	funcIDs     []uint64 // Track registered function IDs for cleanup
	funcIDsMu   sync.Mutex
	callbackSet atomic.Bool // Whether external callback is set
}

// New creates a new Luau state.
func New() (*State, error) {
	L := C.luau_new()
	if L == nil {
		return nil, ErrMemory
	}

	s := &State{
		L:       L,
		funcIDs: make([]uint64, 0),
	}

	// Note: We intentionally don't set a finalizer here because:
	// 1. The user should explicitly call Close() when done
	// 2. Finalizers can run at unexpected times and cause issues with CGO
	return s, nil
}

// Close releases all resources associated with the state.
// This must be called explicitly to release resources.
func (s *State) Close() {
	if s.L != nil {
		C.luau_close(s.L)
		s.L = nil
	}

	// Clean up registered functions from global registry
	s.funcIDsMu.Lock()
	funcIDs := s.funcIDs
	s.funcIDs = nil
	s.funcIDsMu.Unlock()

	if len(funcIDs) > 0 {
		globalFuncsMu.Lock()
		for _, id := range funcIDs {
			delete(globalFuncs, id)
		}
		globalFuncsMu.Unlock()
	}
}

// OpenLibs opens the standard Luau libraries.
// Note: This does not automatically sandbox the environment.
func (s *State) OpenLibs() {
	C.luau_openlibs(s.L)
}

// DoString compiles and executes a Luau script string.
func (s *State) DoString(source string) error {
	return s.DoStringOpt(source, "", OptO2)
}

// DoStringOpt compiles and executes a Luau script with options.
func (s *State) DoStringOpt(source, chunkname string, opt OptLevel) error {
	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	var cchunkname *C.char
	if chunkname != "" {
		cchunkname = C.CString(chunkname)
		defer C.free(unsafe.Pointer(cchunkname))
	}

	result := C.luau_dostring(s.L, csource, C.size_t(len(source)), cchunkname, C.LuauOptLevel(opt))
	return s.checkError(result)
}

// Compile compiles Luau source to bytecode without executing.
func (s *State) Compile(source string, opt OptLevel) ([]byte, error) {
	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	var bytecode *C.char
	var bytecodeLen C.size_t

	result := C.luau_compile(csource, C.size_t(len(source)), C.LuauOptLevel(opt), &bytecode, &bytecodeLen)
	if result != C.LUAU_OK {
		return nil, ErrCompile
	}

	defer C.luau_freebytecode(bytecode)
	return C.GoBytes(unsafe.Pointer(bytecode), C.int(bytecodeLen)), nil
}

// LoadBytecode loads compiled bytecode onto the stack.
func (s *State) LoadBytecode(bytecode []byte, chunkname string) error {
	if len(bytecode) == 0 {
		return ErrInvalid
	}

	var cchunkname *C.char
	if chunkname != "" {
		cchunkname = C.CString(chunkname)
		defer C.free(unsafe.Pointer(cchunkname))
	}

	result := C.luau_loadbytecode(s.L, (*C.char)(unsafe.Pointer(&bytecode[0])),
		C.size_t(len(bytecode)), cchunkname)
	return s.checkError(result)
}

// PCall calls a function on the stack with error handling.
func (s *State) PCall(nargs, nresults int) error {
	result := C.luau_pcall(s.L, C.int(nargs), C.int(nresults))
	return s.checkError(result)
}

// checkError converts C error codes to Go errors.
func (s *State) checkError(result C.LuauError) error {
	switch result {
	case C.LUAU_OK:
		return nil
	case C.LUAU_ERR_COMPILE:
		msg := s.getError()
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrCompile, msg)
		}
		return ErrCompile
	case C.LUAU_ERR_RUNTIME:
		msg := s.getError()
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrRuntime, msg)
		}
		return ErrRuntime
	case C.LUAU_ERR_MEMORY:
		return ErrMemory
	case C.LUAU_ERR_INVALID:
		return ErrInvalid
	default:
		return fmt.Errorf("luau: unknown error %d", result)
	}
}

// getError returns the last error message.
func (s *State) getError() string {
	cmsg := C.luau_geterror(s.L)
	if cmsg == nil {
		return ""
	}
	return C.GoString(cmsg)
}

// ClearError clears the last error.
func (s *State) ClearError() {
	C.luau_clearerror(s.L)
}

// =============================================================================
// Stack Operations
// =============================================================================

// GetTop returns the index of the top element in the stack.
func (s *State) GetTop() int {
	return int(C.luau_gettop(s.L))
}

// SetTop sets the stack top to a specific index.
func (s *State) SetTop(idx int) {
	C.luau_settop(s.L, C.int(idx))
}

// Pop pops n elements from the stack.
func (s *State) Pop(n int) {
	C.luau_pop(s.L, C.int(n))
}

// PushValue pushes a copy of the value at the given index.
func (s *State) PushValue(idx int) {
	C.luau_pushvalue(s.L, C.int(idx))
}

// Remove removes the value at the given index.
func (s *State) Remove(idx int) {
	C.luau_remove(s.L, C.int(idx))
}

// Insert inserts the top value at the given index.
func (s *State) Insert(idx int) {
	C.luau_insert(s.L, C.int(idx))
}

// CheckStack ensures the stack has at least extra free slots.
func (s *State) CheckStack(extra int) bool {
	return C.luau_checkstack(s.L, C.int(extra)) != 0
}

// =============================================================================
// Type Checking
// =============================================================================

// TypeOf returns the type of the value at the given index.
func (s *State) TypeOf(idx int) Type {
	return Type(C.luau_type(s.L, C.int(idx)))
}

// TypeName returns the name of a type.
func (s *State) TypeName(t Type) string {
	return C.GoString(C.luau_typename(s.L, C.LuauType(t)))
}

// IsNil checks if the value is nil.
func (s *State) IsNil(idx int) bool {
	return C.luau_isnil(s.L, C.int(idx)) != 0
}

// IsBoolean checks if the value is a boolean.
func (s *State) IsBoolean(idx int) bool {
	return C.luau_isboolean(s.L, C.int(idx)) != 0
}

// IsNumber checks if the value is a number.
func (s *State) IsNumber(idx int) bool {
	return C.luau_isnumber(s.L, C.int(idx)) != 0
}

// IsString checks if the value is a string.
func (s *State) IsString(idx int) bool {
	return C.luau_isstring(s.L, C.int(idx)) != 0
}

// IsTable checks if the value is a table.
func (s *State) IsTable(idx int) bool {
	return C.luau_istable(s.L, C.int(idx)) != 0
}

// IsFunction checks if the value is a function.
func (s *State) IsFunction(idx int) bool {
	return C.luau_isfunction(s.L, C.int(idx)) != 0
}

// =============================================================================
// Push Values
// =============================================================================

// PushNil pushes nil onto the stack.
func (s *State) PushNil() {
	C.luau_pushnil(s.L)
}

// PushBoolean pushes a boolean onto the stack.
func (s *State) PushBoolean(b bool) {
	var v C.int
	if b {
		v = 1
	}
	C.luau_pushboolean(s.L, v)
}

// PushNumber pushes a number onto the stack.
func (s *State) PushNumber(n float64) {
	C.luau_pushnumber(s.L, C.double(n))
}

// PushInteger pushes an integer onto the stack.
func (s *State) PushInteger(n int64) {
	C.luau_pushinteger(s.L, C.int64_t(n))
}

// PushString pushes a string onto the stack.
func (s *State) PushString(str string) {
	if len(str) == 0 {
		cstr := C.CString("")
		defer C.free(unsafe.Pointer(cstr))
		C.luau_pushstring(s.L, cstr)
		return
	}
	C.luau_pushlstring(s.L, (*C.char)(unsafe.Pointer(&[]byte(str)[0])), C.size_t(len(str)))
}

// PushBytes pushes a byte slice as a string onto the stack.
func (s *State) PushBytes(data []byte) {
	if len(data) == 0 {
		cstr := C.CString("")
		defer C.free(unsafe.Pointer(cstr))
		C.luau_pushstring(s.L, cstr)
		return
	}
	C.luau_pushlstring(s.L, (*C.char)(unsafe.Pointer(&data[0])), C.size_t(len(data)))
}

// =============================================================================
// Read Values
// =============================================================================

// ToBoolean converts the value at the given index to boolean.
func (s *State) ToBoolean(idx int) bool {
	return C.luau_toboolean(s.L, C.int(idx)) != 0
}

// ToNumber converts the value at the given index to a number.
func (s *State) ToNumber(idx int) float64 {
	return float64(C.luau_tonumber(s.L, C.int(idx)))
}

// ToInteger converts the value at the given index to an integer.
func (s *State) ToInteger(idx int) int64 {
	return int64(C.luau_tointeger(s.L, C.int(idx)))
}

// ToString converts the value at the given index to a string.
func (s *State) ToString(idx int) string {
	var len C.size_t
	cstr := C.luau_tolstring(s.L, C.int(idx), &len)
	if cstr == nil {
		return ""
	}
	return C.GoStringN(cstr, C.int(len))
}

// ToBytes converts the value at the given index to a byte slice.
func (s *State) ToBytes(idx int) []byte {
	var len C.size_t
	cstr := C.luau_tolstring(s.L, C.int(idx), &len)
	if cstr == nil {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(cstr), C.int(len))
}

// =============================================================================
// Table Operations
// =============================================================================

// NewTable creates a new empty table and pushes it onto the stack.
func (s *State) NewTable() {
	C.luau_newtable(s.L)
}

// CreateTable creates a new table with pre-allocated space.
func (s *State) CreateTable(narr, nrec int) {
	C.luau_createtable(s.L, C.int(narr), C.int(nrec))
}

// GetField gets a field from a table and pushes it onto the stack.
func (s *State) GetField(idx int, key string) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	C.luau_getfield(s.L, C.int(idx), ckey)
}

// SetField sets a field in a table. Pops the value from the stack.
func (s *State) SetField(idx int, key string) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	C.luau_setfield(s.L, C.int(idx), ckey)
}

// GetTable gets a value from a table using the key on the stack.
func (s *State) GetTable(idx int) {
	C.luau_gettable(s.L, C.int(idx))
}

// SetTable sets a value in a table using key and value on the stack.
func (s *State) SetTable(idx int) {
	C.luau_settable(s.L, C.int(idx))
}

// RawGet gets a raw table value (bypasses metamethods).
func (s *State) RawGet(idx int) {
	C.luau_rawget(s.L, C.int(idx))
}

// RawSet sets a raw table value (bypasses metamethods).
func (s *State) RawSet(idx int) {
	C.luau_rawset(s.L, C.int(idx))
}

// RawGetI gets a raw table value by integer index.
func (s *State) RawGetI(idx, n int) {
	C.luau_rawgeti(s.L, C.int(idx), C.int(n))
}

// RawSetI sets a raw table value by integer index.
func (s *State) RawSetI(idx, n int) {
	C.luau_rawseti(s.L, C.int(idx), C.int(n))
}

// ObjLen returns the length of a table or string.
func (s *State) ObjLen(idx int) int {
	return int(C.luau_objlen(s.L, C.int(idx)))
}

// Next iterates to the next key-value pair in a table.
func (s *State) Next(idx int) bool {
	return C.luau_next(s.L, C.int(idx)) != 0
}

// =============================================================================
// Global Table
// =============================================================================

// GetGlobal gets a global variable and pushes it onto the stack.
func (s *State) GetGlobal(name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.luau_getglobal(s.L, cname)
}

// SetGlobal sets a global variable. Pops the value from the stack.
func (s *State) SetGlobal(name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.luau_setglobal(s.L, cname)
}

// =============================================================================
// Function Registration
// =============================================================================

// goExternalCallback is the CGO callback invoked when Luau calls a registered Go function.
// This function is called from C and looks up the actual Go function to execute.
//
//export goExternalCallback
func goExternalCallback(L *C.LuauState, callbackID C.uint64_t) C.int {
	id := uint64(callbackID)

	// Look up the function in the global registry
	globalFuncsMu.RLock()
	entry, ok := globalFuncs[id]
	globalFuncsMu.RUnlock()

	if !ok || entry.fn == nil || entry.state == nil {
		return 0
	}

	// Call the Go function with panic recovery
	var result int
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Push error message to Luau
				errMsg := fmt.Sprintf("Go function panicked: %v", r)
				cstr := C.CString(errMsg)
				C.luau_pushstring(L, cstr)
				C.free(unsafe.Pointer(cstr))
				// Note: We can't call lua_error here safely from a callback,
				// so we just return 0 and let the caller handle it
				result = 0
			}
		}()
		result = entry.fn(entry.state)
	}()

	return C.int(result)
}

// ensureCallbackSet ensures the external callback is registered for this state.
func (s *State) ensureCallbackSet() {
	if s.callbackSet.CompareAndSwap(false, true) {
		C.luau_setexternalcallback(s.L, C.LuauExternalCallback(C.goExternalCallback))
	}
}

// RegisterFunc registers a Go function as a global Luau function.
// The function can be called from Luau scripts by name.
//
// Example:
//
//	state.RegisterFunc("add", func(L *luau.State) int {
//	    a := L.ToNumber(1)
//	    b := L.ToNumber(2)
//	    L.PushNumber(a + b)
//	    return 1
//	})
//	state.DoString("print(add(1, 2))") // prints 3
func (s *State) RegisterFunc(name string, fn GoFunc) error {
	if s.L == nil {
		return ErrInvalid
	}
	if fn == nil {
		return errors.New("luau: cannot register nil function")
	}

	// Ensure the external callback is set
	s.ensureCallbackSet()

	// Generate a unique ID for this function
	id := atomic.AddUint64(&globalFuncNextID, 1)

	// Register in global map
	globalFuncsMu.Lock()
	globalFuncs[id] = goFuncEntry{fn: fn, state: s}
	globalFuncsMu.Unlock()

	// Track for cleanup
	s.funcIDsMu.Lock()
	s.funcIDs = append(s.funcIDs, id)
	s.funcIDsMu.Unlock()

	// Register in Luau
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.luau_registerexternal(s.L, cname, C.uint64_t(id))

	return nil
}

// UnregisterFunc removes a previously registered function by name.
// Note: This removes the global from Luau but doesn't free the Go function
// from memory until the State is closed.
func (s *State) UnregisterFunc(name string) {
	if s.L == nil {
		return
	}
	s.PushNil()
	s.SetGlobal(name)
}

// =============================================================================
// Memory Management
// =============================================================================

// MemoryUsage returns the current memory usage in bytes.
func (s *State) MemoryUsage() int {
	return int(C.luau_memoryusage(s.L))
}

// GC runs the garbage collector.
func (s *State) GC() {
	C.luau_gc(s.L)
}

// =============================================================================
// Utility
// =============================================================================

// Version returns the Luau version string.
func Version() string {
	return C.GoString(C.luau_version())
}

// DumpStack returns a string representation of the stack for debugging.
func (s *State) DumpStack() string {
	cstr := C.luau_dumpstack(s.L)
	if cstr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}
