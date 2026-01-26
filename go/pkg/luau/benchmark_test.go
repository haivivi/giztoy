package luau

import (
	"runtime"
	"testing"
)

// =============================================================================
// Script Execution Benchmarks
// =============================================================================

func BenchmarkDoStringSimple(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(`x = 1 + 2`)
	}
}

func BenchmarkDoStringArithmetic(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local a = 0
		for i = 1, 100 do
			a = a + i * 2 - 1
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkDoStringFibonacci(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local function fib(n)
			if n <= 1 then return n end
			return fib(n - 1) + fib(n - 2)
		end
		local result = fib(15)
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkDoStringLoop10k(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local sum = 0
		for i = 1, 10000 do
			sum = sum + i
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkDoStringStringOps(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local s = ""
		for i = 1, 100 do
			s = s .. tostring(i)
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkDoStringTableOps(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local t = {}
		for i = 1, 1000 do
			t[i] = i * 2
		end
		local sum = 0
		for i = 1, 1000 do
			sum = sum + t[i]
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

// =============================================================================
// Compilation Benchmarks
// =============================================================================

func BenchmarkCompileSimple(b *testing.B) {
	state, _ := New()
	defer state.Close()

	source := `local x = 1 + 2`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.Compile(source, OptO2)
	}
}

func BenchmarkCompileComplex(b *testing.B) {
	state, _ := New()
	defer state.Close()

	source := `
		local function factorial(n)
			if n <= 1 then return 1 end
			return n * factorial(n - 1)
		end
		
		local function fibonacci(n)
			if n <= 1 then return n end
			return fibonacci(n - 1) + fibonacci(n - 2)
		end
		
		local function quicksort(arr, lo, hi)
			if lo < hi then
				local pivot = arr[hi]
				local i = lo - 1
				for j = lo, hi - 1 do
					if arr[j] <= pivot then
						i = i + 1
						arr[i], arr[j] = arr[j], arr[i]
					end
				end
				arr[i + 1], arr[hi] = arr[hi], arr[i + 1]
				local p = i + 1
				quicksort(arr, lo, p - 1)
				quicksort(arr, p + 1, hi)
			end
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.Compile(source, OptO2)
	}
}

func BenchmarkLoadBytecode(b *testing.B) {
	state, _ := New()
	defer state.Close()

	bytecode, _ := state.Compile(`local x = 1 + 2; return x`, OptO2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.LoadBytecode(bytecode, "test")
		state.Pop(1)
	}
}

func BenchmarkCompileAndRun(b *testing.B) {
	state, _ := New()
	defer state.Close()

	source := `
		local sum = 0
		for i = 1, 100 do
			sum = sum + i
		end
		return sum
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytecode, _ := state.Compile(source, OptO2)
		state.LoadBytecode(bytecode, "test")
		state.PCall(0, 1)
		state.Pop(1)
	}
}

// =============================================================================
// Stack Operation Benchmarks
// =============================================================================

func BenchmarkPushNumber(b *testing.B) {
	state, _ := New()
	defer state.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.PushNumber(3.14159)
		state.Pop(1)
	}
}

func BenchmarkPushString(b *testing.B) {
	state, _ := New()
	defer state.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.PushString("hello world")
		state.Pop(1)
	}
}

func BenchmarkPushStringLong(b *testing.B) {
	state, _ := New()
	defer state.Close()

	longString := string(make([]byte, 1024))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.PushString(longString)
		state.Pop(1)
	}
}

func BenchmarkToNumber(b *testing.B) {
	state, _ := New()
	defer state.Close()

	state.PushNumber(3.14159)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = state.ToNumber(-1)
	}
}

func BenchmarkToString(b *testing.B) {
	state, _ := New()
	defer state.Close()

	state.PushString("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = state.ToString(-1)
	}
}

// =============================================================================
// Table Operation Benchmarks
// =============================================================================

func BenchmarkNewTable(b *testing.B) {
	state, _ := New()
	defer state.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.NewTable()
		state.Pop(1)
	}
}

func BenchmarkSetField(b *testing.B) {
	state, _ := New()
	defer state.Close()

	state.NewTable()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.PushNumber(42)
		state.SetField(-2, "key")
	}
}

func BenchmarkGetField(b *testing.B) {
	state, _ := New()
	defer state.Close()

	state.NewTable()
	state.PushNumber(42)
	state.SetField(-2, "key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetField(-1, "key")
		state.Pop(1)
	}
}

func BenchmarkTableIteration(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	// Create a table with 100 entries
	state.DoString(`
		t = {}
		for i = 1, 100 do
			t["key" .. i] = i
		end
	`)

	state.GetGlobal("t")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.PushNil()
		for state.Next(-2) {
			state.Pop(1)
		}
	}
}

// =============================================================================
// Memory Benchmarks
// =============================================================================

func BenchmarkMemoryAllocation(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local t = {}
		for i = 1, 1000 do
			t[i] = {x = i, y = i * 2, z = tostring(i)}
		end
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
		state.GC()
	}
}

func BenchmarkGC(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	// Allocate some memory
	state.DoString(`
		for _ = 1, 100 do
			local t = {}
			for i = 1, 100 do
				t[i] = tostring(i)
			end
		end
	`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GC()
	}
}

// =============================================================================
// Optimization Level Comparison
// =============================================================================

func benchmarkOptLevel(b *testing.B, opt OptLevel) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local function fib(n)
			if n <= 1 then return n end
			return fib(n - 1) + fib(n - 2)
		end
		local result = fib(20)
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoStringOpt(script, "fib", opt)
	}
}

func BenchmarkOptNone(b *testing.B) {
	benchmarkOptLevel(b, OptNone)
}

func BenchmarkOptO1(b *testing.B) {
	benchmarkOptLevel(b, OptO1)
}

func BenchmarkOptO2(b *testing.B) {
	benchmarkOptLevel(b, OptO2)
}

// =============================================================================
// Real-World Scenario Benchmarks
// =============================================================================

func BenchmarkJSONLikeProcessing(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local data = {
			users = {},
			total = 0
		}
		
		for i = 1, 100 do
			data.users[i] = {
				id = i,
				name = "user" .. i,
				email = "user" .. i .. "@example.com",
				active = i % 2 == 0
			}
			if data.users[i].active then
				data.total = data.total + 1
			end
		end
		
		return data.total
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkConfigParsing(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	script := `
		local config = {
			server = {
				host = "localhost",
				port = 8080,
				timeout = 30
			},
			database = {
				driver = "postgres",
				host = "db.example.com",
				port = 5432,
				name = "mydb"
			},
			features = {
				auth = true,
				cache = true,
				logging = {
					level = "info",
					format = "json"
				}
			}
		}
		
		-- Access nested values
		local result = config.server.host .. ":" .. tostring(config.server.port)
		if config.features.auth then
			result = result .. " (auth enabled)"
		end
		return result
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

func BenchmarkAgentToolSimulation(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	// Simulate a tool that processes arguments and returns results
	script := `
		local function invoke(args)
			-- Validate input
			if not args.query then
				return {error = "missing query"}
			end
			
			-- Process
			local results = {}
			for i = 1, 10 do
				results[i] = {
					id = i,
					score = math.random(),
					match = string.match(args.query, "^%w+")
				}
			end
			
			-- Sort by score
			table.sort(results, function(a, b) return a.score > b.score end)
			
			return {
				query = args.query,
				count = #results,
				results = results
			}
		end
		
		local result = invoke({query = "test query string"})
		return result.count
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.DoString(script)
	}
}

// =============================================================================
// Memory Usage Reporting
// =============================================================================

func BenchmarkMemoryUsageReporting(b *testing.B) {
	state, _ := New()
	defer state.Close()
	state.OpenLibs()

	var m runtime.MemStats

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Allocate in Lua
		state.DoString(`
			local t = {}
			for i = 1, 1000 do
				t[i] = "string" .. i
			end
		`)

		// Measure
		luauMem := state.MemoryUsage()
		runtime.ReadMemStats(&m)

		// Clear
		state.SetTop(0)
		state.GC()

		// Report (only on last iteration to avoid noise)
		if i == b.N-1 {
			b.ReportMetric(float64(luauMem), "luau_bytes")
			b.ReportMetric(float64(m.HeapAlloc), "go_heap_bytes")
		}
	}
}

// =============================================================================
// State Creation/Destruction Benchmarks
// =============================================================================

func BenchmarkNewClose(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, _ := New()
		state.Close()
	}
}

func BenchmarkNewOpenLibsClose(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, _ := New()
		state.OpenLibs()
		state.Close()
	}
}

// =============================================================================
// Parallel Execution Benchmarks
// =============================================================================

func BenchmarkParallelExecution(b *testing.B) {
	script := `
		local sum = 0
		for i = 1, 100 do
			sum = sum + i
		end
		return sum
	`

	b.RunParallel(func(pb *testing.PB) {
		state, _ := New()
		defer state.Close()
		state.OpenLibs()

		for pb.Next() {
			state.DoString(script)
		}
	})
}
