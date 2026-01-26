// Package luau provides Go bindings for the Luau scripting language.
//
// Luau is a fast, small, safe, gradually typed scripting language derived
// from Lua. It is designed for embedding in applications and provides
// built-in sandboxing for security.
//
// # Basic Usage
//
//	state, err := luau.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer state.Close()
//
//	// Open standard libraries (sandboxed)
//	state.OpenLibs()
//
//	// Execute a script
//	if err := state.DoString(`print("Hello from Luau!")`); err != nil {
//	    log.Fatal(err)
//	}
//
// # Working with Values
//
//	// Set a global variable
//	state.PushNumber(42)
//	state.SetGlobal("answer")
//
//	// Get a global variable
//	state.GetGlobal("answer")
//	fmt.Println(state.ToNumber(-1)) // 42
//	state.Pop(1)
//
// # Calling Lua Functions from Go
//
//	state.DoString(`function add(a, b) return a + b end`)
//	state.GetGlobal("add")
//	state.PushNumber(10)
//	state.PushNumber(20)
//	state.PCall(2, 1)
//	result := state.ToNumber(-1)
//	state.Pop(1)
//
// # Registering Go Functions
//
//	state.RegisterFunc("greet", func(L *luau.State) int {
//	    name := L.ToString(1)
//	    L.PushString(fmt.Sprintf("Hello, %s!", name))
//	    return 1
//	})
//
//	state.DoString(`print(greet("World"))`) // Hello, World!
//
// # Tables
//
//	state.NewTable()
//	state.PushString("value")
//	state.SetField(-2, "key")
//	state.SetGlobal("myTable")
//
//	state.DoString(`print(myTable.key)`) // value
//
// # Error Handling
//
//	err := state.DoString(`invalid syntax here !!!`)
//	if err != nil {
//	    fmt.Println("Script error:", err)
//	}
//
// # Security
//
// OpenLibs() opens the standard Luau libraries (base, math, string, table,
// coroutine, utf8, debug, os, io). It does NOT automatically sandbox the
// environment or restrict dangerous functions.
//
// If you need a sandboxed environment for untrusted code, you are responsible
// for controlling which libraries and globals are exposed. Consider:
//   - Removing os/io globals after OpenLibs() if not needed
//   - Using a restricted environment table
//   - Not exposing loadfile/dofile to untrusted scripts
package luau
