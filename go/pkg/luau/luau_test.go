package luau

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// Basic State Tests
// =============================================================================

func TestNew(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	if state.L == nil {
		t.Fatal("State.L is nil")
	}
}

func TestClose(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	state.Close()

	if state.L != nil {
		t.Fatal("State.L should be nil after Close()")
	}

	// Close should be safe to call multiple times
	state.Close()
}

func TestOpenLibs(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Test that math library is available
	err = state.DoString(`result = math.abs(-5)`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 5 {
		t.Errorf("Expected 5, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestSandbox(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Standard libraries should be available
	err = state.DoString(`assert(type(string) == "table", "string lib should be available")`)
	if err != nil {
		t.Fatalf("Standard lib test failed: %v", err)
	}

	// math library should be available
	err = state.DoString(`assert(type(math) == "table", "math lib should be available")`)
	if err != nil {
		t.Fatalf("Standard lib test failed: %v", err)
	}

	// os library should be available (Luau's os is safe, only has time/date functions)
	err = state.DoString(`assert(type(os) == "table", "os lib should be available")`)
	if err != nil {
		t.Fatalf("Standard lib test failed: %v", err)
	}
}

// =============================================================================
// Script Execution Tests
// =============================================================================

func TestDoString(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	tests := []struct {
		name   string
		script string
		check  func(*State) error
	}{
		{
			name:   "simple assignment",
			script: `x = 42`,
			check: func(s *State) error {
				s.GetGlobal("x")
				if s.ToNumber(-1) != 42 {
					t.Errorf("Expected 42, got %v", s.ToNumber(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name:   "string concatenation",
			script: `greeting = "Hello" .. " " .. "World"`,
			check: func(s *State) error {
				s.GetGlobal("greeting")
				if s.ToString(-1) != "Hello World" {
					t.Errorf("Expected 'Hello World', got '%s'", s.ToString(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name:   "arithmetic",
			script: `result = (10 + 5) * 2 - 3`,
			check: func(s *State) error {
				s.GetGlobal("result")
				if s.ToNumber(-1) != 27 {
					t.Errorf("Expected 27, got %v", s.ToNumber(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name:   "function definition and call",
			script: `function add(a, b) return a + b end; sum = add(3, 4)`,
			check: func(s *State) error {
				s.GetGlobal("sum")
				if s.ToNumber(-1) != 7 {
					t.Errorf("Expected 7, got %v", s.ToNumber(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name:   "table creation",
			script: `t = {a = 1, b = 2, c = 3}`,
			check: func(s *State) error {
				s.GetGlobal("t")
				if !s.IsTable(-1) {
					t.Error("Expected table")
				}
				s.GetField(-1, "b")
				if s.ToNumber(-1) != 2 {
					t.Errorf("Expected 2, got %v", s.ToNumber(-1))
				}
				s.Pop(2)
				return nil
			},
		},
		{
			name: "loops",
			script: `
				total = 0
				for i = 1, 10 do
					total = total + i
				end
			`,
			check: func(s *State) error {
				s.GetGlobal("total")
				if s.ToNumber(-1) != 55 {
					t.Errorf("Expected 55, got %v", s.ToNumber(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name: "conditionals",
			script: `
				x = 10
				if x > 5 then
					result = "big"
				else
					result = "small"
				end
			`,
			check: func(s *State) error {
				s.GetGlobal("result")
				if s.ToString(-1) != "big" {
					t.Errorf("Expected 'big', got '%s'", s.ToString(-1))
				}
				s.Pop(1)
				return nil
			},
		},
		{
			name: "closures",
			script: `
				function makeCounter()
					local count = 0
					return function()
						count = count + 1
						return count
					end
				end
				counter = makeCounter()
				a = counter()
				b = counter()
				c = counter()
			`,
			check: func(s *State) error {
				s.GetGlobal("c")
				if s.ToNumber(-1) != 3 {
					t.Errorf("Expected 3, got %v", s.ToNumber(-1))
				}
				s.Pop(1)
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := state.DoString(tt.script)
			if err != nil {
				t.Fatalf("DoString failed: %v", err)
			}
			if tt.check != nil {
				tt.check(state)
			}
		})
	}
}

func TestDoStringError(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Syntax error
	err = state.DoString(`invalid syntax here !!!`)
	if err == nil {
		t.Fatal("Expected compilation error")
	}

	// Runtime error
	state.OpenLibs()
	err = state.DoString(`error("test error")`)
	if err == nil {
		t.Fatal("Expected runtime error")
	}
}

// =============================================================================
// Compile and LoadBytecode Tests
// =============================================================================

func TestCompile(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	bytecode, err := state.Compile(`return 1 + 2`, OptO2)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(bytecode) == 0 {
		t.Fatal("Bytecode is empty")
	}
}

func TestCompileError(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	_, err = state.Compile(`invalid syntax !!!`, OptO2)
	if err == nil {
		t.Fatal("Expected compilation error")
	}
}

func TestLoadBytecode(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Compile
	bytecode, err := state.Compile(`return 42`, OptO2)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Load
	err = state.LoadBytecode(bytecode, "test")
	if err != nil {
		t.Fatalf("LoadBytecode failed: %v", err)
	}

	// Execute
	err = state.PCall(0, 1)
	if err != nil {
		t.Fatalf("PCall failed: %v", err)
	}

	if state.ToNumber(-1) != 42 {
		t.Errorf("Expected 42, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

// =============================================================================
// Stack Operations Tests
// =============================================================================

func TestStackOperations(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Test GetTop on empty stack
	if state.GetTop() != 0 {
		t.Errorf("Expected 0, got %d", state.GetTop())
	}

	// Push values
	state.PushNumber(1)
	state.PushNumber(2)
	state.PushNumber(3)

	if state.GetTop() != 3 {
		t.Errorf("Expected 3, got %d", state.GetTop())
	}

	// Pop
	state.Pop(1)
	if state.GetTop() != 2 {
		t.Errorf("Expected 2, got %d", state.GetTop())
	}

	// SetTop
	state.SetTop(1)
	if state.GetTop() != 1 {
		t.Errorf("Expected 1, got %d", state.GetTop())
	}

	// PushValue
	state.PushValue(1)
	if state.GetTop() != 2 {
		t.Errorf("Expected 2, got %d", state.GetTop())
	}

	// Both values should be 1
	if state.ToNumber(1) != 1 || state.ToNumber(2) != 1 {
		t.Error("PushValue failed")
	}
}

func TestInsertRemove(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.PushNumber(1)
	state.PushNumber(2)
	state.PushNumber(3)

	// Insert 3 at position 1
	state.Insert(1)

	// Stack should now be: 3, 1, 2
	if state.ToNumber(1) != 3 {
		t.Errorf("Expected 3 at position 1, got %v", state.ToNumber(1))
	}

	// Remove position 2
	state.Remove(2)

	// Stack should now be: 3, 2
	if state.GetTop() != 2 {
		t.Errorf("Expected 2 items, got %d", state.GetTop())
	}
}

func TestCheckStack(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	if !state.CheckStack(100) {
		t.Error("CheckStack(100) should return true")
	}
}

// =============================================================================
// Type Checking Tests
// =============================================================================

func TestTypeChecking(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Test nil
	state.PushNil()
	if !state.IsNil(-1) {
		t.Error("Expected nil")
	}
	if state.TypeOf(-1) != TypeNil {
		t.Error("Expected TypeNil")
	}
	state.Pop(1)

	// Test boolean
	state.PushBoolean(true)
	if !state.IsBoolean(-1) {
		t.Error("Expected boolean")
	}
	if state.TypeOf(-1) != TypeBoolean {
		t.Error("Expected TypeBoolean")
	}
	state.Pop(1)

	// Test number
	state.PushNumber(3.14)
	if !state.IsNumber(-1) {
		t.Error("Expected number")
	}
	if state.TypeOf(-1) != TypeNumber {
		t.Error("Expected TypeNumber")
	}
	state.Pop(1)

	// Test string
	state.PushString("hello")
	if !state.IsString(-1) {
		t.Error("Expected string")
	}
	if state.TypeOf(-1) != TypeString {
		t.Error("Expected TypeString")
	}
	state.Pop(1)

	// Test table
	state.NewTable()
	if !state.IsTable(-1) {
		t.Error("Expected table")
	}
	if state.TypeOf(-1) != TypeTable {
		t.Error("Expected TypeTable")
	}
	state.Pop(1)
}

func TestTypeName(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	tests := []struct {
		typ  Type
		name string
	}{
		{TypeNil, "nil"},
		{TypeBoolean, "boolean"},
		{TypeNumber, "number"},
		{TypeString, "string"},
		{TypeTable, "table"},
		{TypeFunction, "function"},
	}

	for _, tt := range tests {
		if tt.typ.String() != tt.name {
			t.Errorf("Type.String() for %d: expected %s, got %s", tt.typ, tt.name, tt.typ.String())
		}
	}
}

// =============================================================================
// Value Push/Read Tests
// =============================================================================

func TestPushAndReadValues(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Boolean true
	state.PushBoolean(true)
	if !state.ToBoolean(-1) {
		t.Error("Expected true")
	}
	state.Pop(1)

	// Boolean false
	state.PushBoolean(false)
	if state.ToBoolean(-1) {
		t.Error("Expected false")
	}
	state.Pop(1)

	// Number
	state.PushNumber(3.14159)
	if state.ToNumber(-1) != 3.14159 {
		t.Errorf("Expected 3.14159, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	// Integer
	state.PushInteger(123456789)
	if state.ToInteger(-1) != 123456789 {
		t.Errorf("Expected 123456789, got %v", state.ToInteger(-1))
	}
	state.Pop(1)

	// String
	state.PushString("Hello, Luau!")
	if state.ToString(-1) != "Hello, Luau!" {
		t.Errorf("Expected 'Hello, Luau!', got '%s'", state.ToString(-1))
	}
	state.Pop(1)

	// Empty string
	state.PushString("")
	if state.ToString(-1) != "" {
		t.Errorf("Expected empty string, got '%s'", state.ToString(-1))
	}
	state.Pop(1)

	// Bytes
	data := []byte{0x00, 0x01, 0x02, 0xFF}
	state.PushBytes(data)
	result := state.ToBytes(-1)
	if len(result) != len(data) {
		t.Errorf("Expected %d bytes, got %d", len(data), len(result))
	}
	for i, b := range result {
		if b != data[i] {
			t.Errorf("Byte %d: expected %d, got %d", i, data[i], b)
		}
	}
	state.Pop(1)
}

// =============================================================================
// Table Tests
// =============================================================================

func TestTableOperations(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Create table
	state.NewTable()

	// Set field
	state.PushNumber(42)
	state.SetField(-2, "answer")

	// Get field
	state.GetField(-1, "answer")
	if state.ToNumber(-1) != 42 {
		t.Errorf("Expected 42, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	// Set with key on stack
	state.PushString("key")
	state.PushString("value")
	state.SetTable(-3)

	// Get with key on stack
	state.PushString("key")
	state.GetTable(-2)
	if state.ToString(-1) != "value" {
		t.Errorf("Expected 'value', got '%s'", state.ToString(-1))
	}
	state.Pop(2)
}

func TestTableIteration(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Create table with values
	err = state.DoString(`t = {a = 1, b = 2, c = 3}`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("t")

	// Iterate
	count := 0
	state.PushNil()
	for state.Next(-2) {
		count++
		state.Pop(1) // pop value, keep key for next iteration
	}
	state.Pop(1) // pop table

	if count != 3 {
		t.Errorf("Expected 3 iterations, got %d", count)
	}
}

func TestRawGetSet(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.NewTable()

	// RawSet
	state.PushString("key")
	state.PushNumber(100)
	state.RawSet(-3)

	// RawGet
	state.PushString("key")
	state.RawGet(-2)
	if state.ToNumber(-1) != 100 {
		t.Errorf("Expected 100, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	// RawSetI
	state.PushString("first")
	state.RawSetI(-2, 1)

	// RawGetI
	state.RawGetI(-1, 1)
	if state.ToString(-1) != "first" {
		t.Errorf("Expected 'first', got '%s'", state.ToString(-1))
	}
	state.Pop(2)
}

func TestObjLen(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// String length
	state.PushString("hello")
	if state.ObjLen(-1) != 5 {
		t.Errorf("Expected length 5, got %d", state.ObjLen(-1))
	}
	state.Pop(1)

	// Table length (array part)
	state.OpenLibs()
	state.DoString(`t = {1, 2, 3, 4, 5}`)
	state.GetGlobal("t")
	if state.ObjLen(-1) != 5 {
		t.Errorf("Expected length 5, got %d", state.ObjLen(-1))
	}
	state.Pop(1)
}

func TestCreateTable(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Create table with hints
	state.CreateTable(10, 5)
	if !state.IsTable(-1) {
		t.Error("Expected table")
	}
	state.Pop(1)
}

// =============================================================================
// Global Tests
// =============================================================================

func TestGlobals(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Set global
	state.PushString("test value")
	state.SetGlobal("myGlobal")

	// Get global
	state.GetGlobal("myGlobal")
	if state.ToString(-1) != "test value" {
		t.Errorf("Expected 'test value', got '%s'", state.ToString(-1))
	}
	state.Pop(1)
}

// =============================================================================
// Memory and GC Tests
// =============================================================================

func TestMemoryUsage(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	initial := state.MemoryUsage()
	if initial <= 0 {
		t.Error("Memory usage should be positive")
	}

	// Allocate some tables (use checkstack to ensure we have room)
	state.CheckStack(100)
	for i := 0; i < 100; i++ {
		state.NewTable()
	}

	afterAlloc := state.MemoryUsage()
	if afterAlloc <= initial {
		t.Error("Memory usage should increase after allocations")
	}

	// Pop all tables
	state.SetTop(0)

	// Run GC
	state.GC()

	afterGC := state.MemoryUsage()
	if afterGC >= afterAlloc {
		t.Error("Memory usage should decrease after GC")
	}
}

func TestGC(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Just verify GC doesn't crash
	state.GC()
}

// =============================================================================
// Utility Tests
// =============================================================================

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Error("Version should not be empty")
	}
	if !strings.Contains(v, "Lua") {
		t.Errorf("Version should contain 'Lua', got '%s'", v)
	}
}

func TestDumpStack(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.PushNumber(42)
	state.PushString("hello")
	state.PushBoolean(true)

	dump := state.DumpStack()
	if dump == "" {
		t.Error("DumpStack should not be empty")
	}

	if !strings.Contains(dump, "42") {
		t.Error("DumpStack should contain '42'")
	}
	if !strings.Contains(dump, "hello") {
		t.Error("DumpStack should contain 'hello'")
	}
	if !strings.Contains(dump, "true") {
		t.Error("DumpStack should contain 'true'")
	}
}

func TestClearError(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Cause an error
	state.DoString(`error("test")`)

	// Clear it
	state.ClearError()

	// This should not crash
}

// =============================================================================
// Complex Script Tests
// =============================================================================

func TestRecursiveFibonacci(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	script := `
		function fib(n)
			if n <= 1 then
				return n
			end
			return fib(n - 1) + fib(n - 2)
		end
		result = fib(10)
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 55 {
		t.Errorf("Expected fib(10) = 55, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestTableAsObject(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	script := `
		local Point = {}
		Point.__index = Point

		function Point.new(x, y)
			local self = setmetatable({}, Point)
			self.x = x
			self.y = y
			return self
		end

		function Point:distance(other)
			local dx = self.x - other.x
			local dy = self.y - other.y
			return math.sqrt(dx * dx + dy * dy)
		end

		local p1 = Point.new(0, 0)
		local p2 = Point.new(3, 4)
		result = p1:distance(p2)
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 5 {
		t.Errorf("Expected distance = 5, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestCoroutines(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	script := `
		function producer()
			for i = 1, 5 do
				coroutine.yield(i * 10)
			end
		end

		co = coroutine.create(producer)
		results = {}
		while true do
			local status, value = coroutine.resume(co)
			if not status or value == nil then break end
			table.insert(results, value)
		end
		
		total = 0
		for _, v in ipairs(results) do
			total = total + v
		end
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("total")
	// 10 + 20 + 30 + 40 + 50 = 150
	if state.ToNumber(-1) != 150 {
		t.Errorf("Expected total = 150, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestLuauTypeAnnotations(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Luau type annotations should be accepted
	script := `
		type Point = {x: number, y: number}
		
		local function createPoint(x: number, y: number): Point
			return {x = x, y = y}
		end
		
		local p: Point = createPoint(10, 20)
		result = p.x + p.y
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("DoString with type annotations failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 30 {
		t.Errorf("Expected 30, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestStringLibrary(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	tests := []struct {
		script   string
		expected string
	}{
		{`result = string.upper("hello")`, "HELLO"},
		{`result = string.lower("WORLD")`, "world"},
		{`result = string.sub("hello world", 1, 5)`, "hello"},
		{`result = string.rep("ab", 3)`, "ababab"},
		{`result = string.reverse("hello")`, "olleh"},
	}

	for _, tt := range tests {
		err := state.DoString(tt.script)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		state.GetGlobal("result")
		if state.ToString(-1) != tt.expected {
			t.Errorf("For '%s': expected '%s', got '%s'", tt.script, tt.expected, state.ToString(-1))
		}
		state.Pop(1)
	}
}

func TestMathLibrary(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	tests := []struct {
		script   string
		expected float64
	}{
		{`result = math.abs(-5)`, 5},
		{`result = math.floor(3.7)`, 3},
		{`result = math.ceil(3.2)`, 4},
		{`result = math.max(1, 2, 3)`, 3},
		{`result = math.min(1, 2, 3)`, 1},
		{`result = math.sqrt(16)`, 4},
		{`result = math.pow(2, 10)`, 1024},
	}

	for _, tt := range tests {
		err := state.DoString(tt.script)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		state.GetGlobal("result")
		if state.ToNumber(-1) != tt.expected {
			t.Errorf("For '%s': expected %v, got %v", tt.script, tt.expected, state.ToNumber(-1))
		}
		state.Pop(1)
	}
}

func TestTableLibrary(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	script := `
		t = {3, 1, 4, 1, 5, 9, 2, 6}
		table.sort(t)
		result = table.concat(t, ",")
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToString(-1) != "1,1,2,3,4,5,6,9" {
		t.Errorf("Expected '1,1,2,3,4,5,6,9', got '%s'", state.ToString(-1))
	}
	state.Pop(1)
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestEmptyScript(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	err = state.DoString("")
	if err != nil {
		t.Fatalf("Empty script should not fail: %v", err)
	}
}

func TestWhitespaceScript(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	err = state.DoString("   \n\t\n   ")
	if err != nil {
		t.Fatalf("Whitespace-only script should not fail: %v", err)
	}
}

func TestCommentOnlyScript(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	err = state.DoString("-- this is a comment")
	if err != nil {
		t.Fatalf("Comment-only script should not fail: %v", err)
	}
}

func TestUnicodeStrings(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Test Unicode in Go -> Lua
	state.PushString("你好世界 🌍")
	state.SetGlobal("greeting")

	state.GetGlobal("greeting")
	result := state.ToString(-1)
	if result != "你好世界 🌍" {
		t.Errorf("Expected '你好世界 🌍', got '%s'", result)
	}
	state.Pop(1)

	// Test Unicode in Lua script
	err = state.DoString(`unicode = "こんにちは"`)
	if err != nil {
		t.Fatalf("DoString with Unicode failed: %v", err)
	}

	state.GetGlobal("unicode")
	result = state.ToString(-1)
	if result != "こんにちは" {
		t.Errorf("Expected 'こんにちは', got '%s'", result)
	}
	state.Pop(1)
}

func TestLargeScript(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Generate a large script
	var builder strings.Builder
	builder.WriteString("total = 0\n")
	for i := 0; i < 1000; i++ {
		builder.WriteString("total = total + 1\n")
	}

	err = state.DoString(builder.String())
	if err != nil {
		t.Fatalf("Large script failed: %v", err)
	}

	state.GetGlobal("total")
	if state.ToNumber(-1) != 1000 {
		t.Errorf("Expected 1000, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestDeepNesting(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	script := `
		t = {}
		current = t
		for i = 1, 100 do
			current.nested = {}
			current = current.nested
		end
		current.value = 42
		
		-- Traverse back
		current = t
		depth = 0
		while current.nested do
			current = current.nested
			depth = depth + 1
		end
		result = current.value
	`

	err = state.DoString(script)
	if err != nil {
		t.Fatalf("Deep nesting script failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 42 {
		t.Errorf("Expected 42, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

// =============================================================================
// Optimization Level Tests
// =============================================================================

func TestOptimizationLevels(t *testing.T) {
	script := `
		local sum = 0
		for i = 1, 100 do
			sum = sum + i
		end
		result = sum
	`

	levels := []OptLevel{OptNone, OptO1, OptO2}

	for _, opt := range levels {
		t.Run(opt.String(), func(t *testing.T) {
			state, err := New()
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			defer state.Close()

			err = state.DoStringOpt(script, "test", opt)
			if err != nil {
				t.Fatalf("DoStringOpt failed: %v", err)
			}

			state.GetGlobal("result")
			if state.ToNumber(-1) != 5050 {
				t.Errorf("Expected 5050, got %v", state.ToNumber(-1))
			}
			state.Pop(1)
		})
	}
}

func (o OptLevel) String() string {
	switch o {
	case OptNone:
		return "OptNone"
	case OptO1:
		return "OptO1"
	case OptO2:
		return "OptO2"
	default:
		return "Unknown"
	}
}

// =============================================================================
// RegisterFunc Tests
// =============================================================================

func TestRegisterFuncSimple(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a simple function that returns a constant
	err = state.RegisterFunc("getAnswer", func(L *State) int {
		L.PushNumber(42)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau
	err = state.DoString(`result = getAnswer()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 42 {
		t.Errorf("Expected 42, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncWithArgs(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register an add function
	err = state.RegisterFunc("add", func(L *State) int {
		a := L.ToNumber(1)
		b := L.ToNumber(2)
		L.PushNumber(a + b)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau
	err = state.DoString(`result = add(10, 20)`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 30 {
		t.Errorf("Expected 30, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncMultiReturn(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function that returns multiple values
	err = state.RegisterFunc("minmax", func(L *State) int {
		a := L.ToNumber(1)
		b := L.ToNumber(2)
		if a < b {
			L.PushNumber(a)
			L.PushNumber(b)
		} else {
			L.PushNumber(b)
			L.PushNumber(a)
		}
		return 2
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau
	err = state.DoString(`min, max = minmax(30, 10)`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("min")
	if state.ToNumber(-1) != 10 {
		t.Errorf("Expected min=10, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	state.GetGlobal("max")
	if state.ToNumber(-1) != 30 {
		t.Errorf("Expected max=30, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncStringArgs(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a greeting function
	err = state.RegisterFunc("greet", func(L *State) int {
		name := L.ToString(1)
		L.PushString("Hello, " + name + "!")
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau
	err = state.DoString(`result = greet("World")`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToString(-1) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", state.ToString(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncOverwrite(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register first version
	err = state.RegisterFunc("getValue", func(L *State) int {
		L.PushNumber(1)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Register second version (overwrite)
	err = state.RegisterFunc("getValue", func(L *State) int {
		L.PushNumber(2)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc (overwrite) failed: %v", err)
	}

	// Call should return new value
	err = state.DoString(`result = getValue()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 2 {
		t.Errorf("Expected 2, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncNestedCall(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function that calls back into Luau
	err = state.RegisterFunc("callLuau", func(L *State) int {
		// Get the Luau function passed as argument
		// and call it with a value
		L.PushValue(1)        // Push the function
		L.PushNumber(100)     // Push argument
		err := L.PCall(1, 1)  // Call with 1 arg, 1 result
		if err != nil {
			L.PushNil()
			return 1
		}
		// Result is already on stack
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau with a Luau function
	err = state.DoString(`
		function double(x)
			return x * 2
		end
		result = callLuau(double)
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 200 {
		t.Errorf("Expected 200, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncMultipleFuncs(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register multiple functions
	err = state.RegisterFunc("inc", func(L *State) int {
		L.PushNumber(L.ToNumber(1) + 1)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc(inc) failed: %v", err)
	}

	err = state.RegisterFunc("dec", func(L *State) int {
		L.PushNumber(L.ToNumber(1) - 1)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc(dec) failed: %v", err)
	}

	err = state.RegisterFunc("double", func(L *State) int {
		L.PushNumber(L.ToNumber(1) * 2)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc(double) failed: %v", err)
	}

	// Call all from Luau
	err = state.DoString(`result = double(inc(dec(10)))`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	// dec(10) = 9, inc(9) = 10, double(10) = 20
	state.GetGlobal("result")
	if state.ToNumber(-1) != 20 {
		t.Errorf("Expected 20, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncCleanupOnClose(t *testing.T) {
	// Test that registered functions are cleaned up when State is closed
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Register some functions
	for i := 0; i < 100; i++ {
		err = state.RegisterFunc("func"+string(rune('0'+i%10)), func(L *State) int {
			return 0
		})
		if err != nil {
			t.Fatalf("RegisterFunc failed: %v", err)
		}
	}

	// Close should clean up
	state.Close()

	// Verify global registry doesn't grow unboundedly by creating another state
	state2, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state2.Close()

	err = state2.RegisterFunc("test", func(L *State) int {
		return 0
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}
}

func TestUnregisterFunc(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function
	err = state.RegisterFunc("myFunc", func(L *State) int {
		L.PushNumber(42)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Verify it works
	err = state.DoString(`result = myFunc()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	// Unregister
	state.UnregisterFunc("myFunc")

	// Verify it's nil now
	state.GetGlobal("myFunc")
	if !state.IsNil(-1) {
		t.Error("Expected myFunc to be nil after unregister")
	}
	state.Pop(1)
}

func TestRegisterFuncTableArg(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function that receives and processes a table
	err = state.RegisterFunc("sumTable", func(L *State) int {
		if !L.IsTable(1) {
			L.PushNumber(0)
			return 1
		}

		sum := 0.0
		L.PushNil() // First key
		for L.Next(1) {
			if L.IsNumber(-1) {
				sum += L.ToNumber(-1)
			}
			L.Pop(1) // Pop value, keep key
		}
		L.PushNumber(sum)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call from Luau with a table
	err = state.DoString(`result = sumTable({10, 20, 30, 40})`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 100 {
		t.Errorf("Expected 100, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncErrorHandling(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function that returns an error via Luau's error mechanism
	err = state.RegisterFunc("failFunc", func(L *State) int {
		L.PushString("intentional error from Go")
		// Return -1 or use lua_error equivalent
		// For now, we just push the error and return 0
		return 0
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Test that it doesn't crash
	err = state.DoString(`result = failFunc()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}

func TestRegisterFuncNilCallback(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Should return error for nil callback
	err = state.RegisterFunc("nilFunc", nil)
	if err == nil {
		t.Error("Expected error for nil callback")
	}
}

// =============================================================================
// Memory Management Tests
// =============================================================================

func TestMemoryRegisterMany(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register 10000 functions
	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("func_%d", i)
		idx := i // Capture for closure
		err = state.RegisterFunc(name, func(L *State) int {
			L.PushNumber(float64(idx))
			return 1
		})
		if err != nil {
			t.Fatalf("RegisterFunc %d failed: %v", i, err)
		}
	}

	// Verify some functions work
	err = state.DoString(`result = func_9999()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 9999 {
		t.Errorf("Expected 9999, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestMemoryCallLoop(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	callCount := 0
	err = state.RegisterFunc("increment", func(L *State) int {
		callCount++
		L.PushNumber(float64(callCount))
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Call 100000 times
	err = state.DoString(`
		for i = 1, 100000 do
			increment()
		end
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	if callCount != 100000 {
		t.Errorf("Expected 100000 calls, got %d", callCount)
	}
}

func TestMemoryStringArgs(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	totalLen := 0
	err = state.RegisterFunc("processString", func(L *State) int {
		s := L.ToString(1)
		totalLen += len(s)
		L.PushNumber(float64(len(s)))
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Pass large strings many times
	err = state.DoString(`
		local bigStr = string.rep("x", 10000)
		for i = 1, 100 do
			processString(bigStr)
		end
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	expectedLen := 10000 * 100
	if totalLen != expectedLen {
		t.Errorf("Expected total length %d, got %d", expectedLen, totalLen)
	}
}

func TestMemoryStateClose(t *testing.T) {
	// Create many states, register functions, close them
	for i := 0; i < 100; i++ {
		state, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		for j := 0; j < 100; j++ {
			name := fmt.Sprintf("func_%d", j)
			err = state.RegisterFunc(name, func(L *State) int {
				return 0
			})
			if err != nil {
				t.Fatalf("RegisterFunc failed: %v", err)
			}
		}

		state.Close()
	}

	// Verify we can still create new states
	state, err := New()
	if err != nil {
		t.Fatalf("New() after cleanup failed: %v", err)
	}
	defer state.Close()

	err = state.RegisterFunc("test", func(L *State) int {
		L.PushNumber(42)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc after cleanup failed: %v", err)
	}
}

func TestMemoryGCInteraction(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a function that creates tables
	err = state.RegisterFunc("createTables", func(L *State) int {
		L.CheckStack(100)
		for i := 0; i < 100; i++ {
			L.NewTable()
			L.Pop(1)
		}
		return 0
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	initialMem := state.MemoryUsage()

	// Call many times with GC
	for i := 0; i < 100; i++ {
		err = state.DoString(`createTables()`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}
		state.GC()
	}

	finalMem := state.MemoryUsage()

	// Memory should not grow unboundedly
	// Allow some variance but should be roughly similar
	if finalMem > initialMem*3 {
		t.Errorf("Memory grew too much: initial=%d, final=%d", initialMem, finalMem)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestConcurrentRegisterMultiState(t *testing.T) {
	// Test that multiple goroutines can register functions on their own States
	// without interfering with each other (tests global registry thread-safety)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// 10 goroutines, each with its own State
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			state, err := New()
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: New() failed: %v", idx, err)
				return
			}
			defer state.Close()

			state.OpenLibs()

			// Register 100 functions
			for j := 0; j < 100; j++ {
				name := fmt.Sprintf("concurrent_%d_%d", idx, j)
				val := idx*100 + j
				err := state.RegisterFunc(name, func(L *State) int {
					L.PushNumber(float64(val))
					return 1
				})
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: RegisterFunc failed: %v", idx, err)
					return
				}
			}

			// Verify a function works
			err = state.DoString(fmt.Sprintf(`result = concurrent_%d_50()`, idx))
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: DoString failed: %v", idx, err)
				return
			}

			state.GetGlobal("result")
			expected := float64(idx*100 + 50)
			if state.ToNumber(-1) != expected {
				errors <- fmt.Errorf("goroutine %d: expected %v, got %v", idx, expected, state.ToNumber(-1))
			}
			state.Pop(1)
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentCallDifferentStates(t *testing.T) {
	// Test that multiple goroutines can call registered functions
	// on different States concurrently

	var wg sync.WaitGroup
	var totalCalls int64
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			state, err := New()
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: New() failed: %v", idx, err)
				return
			}
			defer state.Close()

			state.OpenLibs()

			err = state.RegisterFunc("increment", func(L *State) int {
				atomic.AddInt64(&totalCalls, 1)
				L.PushNumber(1)
				return 1
			})
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: RegisterFunc failed: %v", idx, err)
				return
			}

			// Call 1000 times
			for j := 0; j < 1000; j++ {
				err = state.DoString(`increment()`)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iter %d: DoString failed: %v", idx, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// 10 goroutines * 1000 calls = 10000
	if totalCalls != 10000 {
		t.Errorf("Expected 10000 total calls, got %d", totalCalls)
	}
}

func TestConcurrentStateCreationAndDestruction(t *testing.T) {
	// Test rapid state creation and destruction with registered functions
	// to verify no memory leaks or race conditions in cleanup

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				state, err := New()
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iter %d: New() failed: %v", idx, j, err)
					return
				}

				// Register a function
				err = state.RegisterFunc("test", func(L *State) int {
					L.PushNumber(42)
					return 1
				})
				if err != nil {
					state.Close()
					errors <- fmt.Errorf("goroutine %d iter %d: RegisterFunc failed: %v", idx, j, err)
					return
				}

				// Close immediately
				state.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentRegistryAccess(t *testing.T) {
	// Test that the global registry handles concurrent access correctly
	// by having multiple goroutines register/unregister functions rapidly

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			state, err := New()
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: New() failed: %v", idx, err)
				return
			}
			defer state.Close()

			// Rapidly register and use functions
			for j := 0; j < 100; j++ {
				name := fmt.Sprintf("rapid_%d_%d", idx, j)
				err := state.RegisterFunc(name, func(L *State) int {
					L.PushNumber(1)
					return 1
				})
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iter %d: RegisterFunc failed: %v", idx, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestRegisterFuncEmptyName(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	// Empty name should work (Lua allows it)
	err = state.RegisterFunc("", func(L *State) int {
		L.PushNumber(42)
		return 1
	})
	// This might succeed or fail depending on implementation
	// Just verify it doesn't crash
	_ = err
}

func TestRegisterFuncSpecialCharsName(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Underscore is valid
	err = state.RegisterFunc("_my_func", func(L *State) int {
		L.PushNumber(1)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc with underscore failed: %v", err)
	}

	err = state.DoString(`result = _my_func()`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 1 {
		t.Errorf("Expected 1, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncUnicodeName(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Luau doesn't support Unicode identifiers, but we can register with Unicode
	// and access via _G table
	err = state.RegisterFunc("计算", func(L *State) int {
		a := L.ToNumber(1)
		b := L.ToNumber(2)
		L.PushNumber(a + b)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc with Chinese name failed: %v", err)
	}

	// Access via _G["计算"] since Luau doesn't allow Unicode identifiers
	err = state.DoString(`result = _G["计算"](10, 20)`)
	if err != nil {
		t.Fatalf("DoString with Chinese function name failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 30 {
		t.Errorf("Expected 30, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

func TestRegisterFuncCallUnregistered(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Calling unregistered function should error
	err = state.DoString(`result = nonExistentFunc()`)
	if err == nil {
		t.Error("Expected error when calling unregistered function")
	}
}

func TestRegisterFuncDeepRecursion(t *testing.T) {
	state, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Register a recursive counter
	err = state.RegisterFunc("countdown", func(L *State) int {
		n := int(L.ToNumber(1))
		if n <= 0 {
			L.PushNumber(0)
			return 1
		}
		// Call Luau function recursively
		L.GetGlobal("countdown")
		L.PushNumber(float64(n - 1))
		L.PCall(1, 1)
		result := L.ToNumber(-1)
		L.Pop(1)
		L.PushNumber(result + 1)
		return 1
	})
	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	// Test with moderate depth (not too deep to avoid stack overflow)
	err = state.DoString(`result = countdown(100)`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if state.ToNumber(-1) != 100 {
		t.Errorf("Expected 100, got %v", state.ToNumber(-1))
	}
	state.Pop(1)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkRegisterFunc(b *testing.B) {
	state, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("bench_func_%d", i)
		state.RegisterFunc(name, func(L *State) int {
			return 0
		})
	}
}

func BenchmarkCallNoArgs(b *testing.B) {
	state, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()
	state.RegisterFunc("noArgs", func(L *State) int {
		L.PushNumber(42)
		return 1
	})

	// Compile the call once
	bytecode, _ := state.Compile("return noArgs()", OptO2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.LoadBytecode(bytecode, "bench")
		state.PCall(0, 1)
		state.Pop(1)
	}
}

func BenchmarkCallWithArgs(b *testing.B) {
	state, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()
	state.RegisterFunc("withArgs", func(L *State) int {
		a := L.ToNumber(1)
		b := L.ToNumber(2)
		L.PushNumber(a + b)
		return 1
	})

	bytecode, _ := state.Compile("return withArgs(10, 20)", OptO2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.LoadBytecode(bytecode, "bench")
		state.PCall(0, 1)
		state.Pop(1)
	}
}

func BenchmarkCallReturnString(b *testing.B) {
	state, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()
	state.RegisterFunc("returnString", func(L *State) int {
		L.PushString("hello world from Go!")
		return 1
	})

	bytecode, _ := state.Compile("return returnString()", OptO2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.LoadBytecode(bytecode, "bench")
		state.PCall(0, 1)
		state.Pop(1)
	}
}

func BenchmarkRegister1000Funcs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, _ := New()
		for j := 0; j < 1000; j++ {
			name := fmt.Sprintf("func_%d", j)
			state.RegisterFunc(name, func(L *State) int {
				return 0
			})
		}
		state.Close()
	}
}

func BenchmarkMemoryPressure(b *testing.B) {
	state, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	defer state.Close()

	state.OpenLibs()
	state.RegisterFunc("allocate", func(L *State) int {
		L.NewTable()
		for i := 0; i < 100; i++ {
			L.PushNumber(float64(i))
			L.RawSetI(-2, i+1)
		}
		return 1
	})

	bytecode, _ := state.Compile("local t = allocate(); t = nil", OptO2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.LoadBytecode(bytecode, "bench")
		state.PCall(0, 0)
		if i%100 == 0 {
			state.GC()
		}
	}
}
