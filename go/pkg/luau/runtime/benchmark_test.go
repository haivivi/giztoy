package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// BenchmarkPromiseCreate measures Promise allocation overhead.
func BenchmarkPromiseCreate(b *testing.B) {
	registry := newPromiseRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.newPromise()
	}
}

// BenchmarkPromiseResolve measures resolution and channel signaling.
func BenchmarkPromiseResolve(b *testing.B) {
	registry := newPromiseRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := registry.newPromise()
		p.Resolve("result", nil)
		<-p.ResultChan()
	}
}

// BenchmarkPromiseIsReady measures checking promise readiness.
func BenchmarkPromiseIsReady(b *testing.B) {
	registry := newPromiseRegistry()
	p := registry.newPromise()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.IsReady()
	}
}

// BenchmarkEventLoop_SinglePromise measures a single async op cycle.
func BenchmarkEventLoop_SinglePromise(b *testing.B) {
	for i := 0; i < b.N; i++ {
		state, err := luau.New()
		if err != nil {
			b.Fatal(err)
		}
		state.OpenLibs()

		rt := New(state, nil)
		if err := rt.RegisterAll(); err != nil {
			state.Close()
			b.Fatal(err)
		}

		// Run a simple sleep that resolves immediately (0ms)
		err = rt.Run(`rt:sleep(0):await()`, "bench")
		if err != nil {
			state.Close()
			b.Fatal(err)
		}
		state.Close()
	}
}

// BenchmarkEventLoop_ConcurrentPromises measures handling multiple concurrent Promises.
func BenchmarkEventLoop_ConcurrentPromises(b *testing.B) {
	benchmarks := []struct {
		name  string
		count int
	}{
		{"10", 10},
		{"50", 50},
		{"100", 100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			script := fmt.Sprintf(`
				local promises = {}
				for i = 1, %d do
					promises[i] = rt:sleep(0)
				end
				for i = 1, %d do
					promises[i]:await()
				end
			`, bm.count, bm.count)

			for i := 0; i < b.N; i++ {
				state, err := luau.New()
				if err != nil {
					b.Fatal(err)
				}
				state.OpenLibs()

				rt := New(state, nil)
				if err := rt.RegisterAll(); err != nil {
					state.Close()
					b.Fatal(err)
				}

				err = rt.Run(script, "bench")
				if err != nil {
					state.Close()
					b.Fatal(err)
				}
				state.Close()
			}
		})
	}
}

// BenchmarkSleep_Sequential measures sequential sleep operations.
func BenchmarkSleep_Sequential(b *testing.B) {
	state, err := luau.New()
	if err != nil {
		b.Fatal(err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use 0ms sleep to measure overhead without actual waiting
		err = rt.Run(`rt:sleep(0):await()`, "bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTTP_Async measures async HTTP requests via Promise.
func BenchmarkHTTP_Async(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	state, err := luau.New()
	if err != nil {
		b.Fatal(err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		b.Fatal(err)
	}

	state.PushString(server.URL)
	state.SetGlobal("testURL")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = rt.Run(`rt:http({ url = testURL, method = "GET" }):await()`, "bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTTP_ConcurrentRequests measures concurrent HTTP requests.
func BenchmarkHTTP_ConcurrentRequests(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	benchmarks := []struct {
		name  string
		count int
	}{
		{"5", 5},
		{"10", 10},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			script := fmt.Sprintf(`
				local promises = {}
				for i = 1, %d do
					promises[i] = rt:http({ url = testURL, method = "GET" })
				end
				for i = 1, %d do
					promises[i]:await()
				end
			`, bm.count, bm.count)

			for i := 0; i < b.N; i++ {
				state, err := luau.New()
				if err != nil {
					b.Fatal(err)
				}
				state.OpenLibs()

				rt := New(state, nil)
				if err := rt.RegisterAll(); err != nil {
					state.Close()
					b.Fatal(err)
				}

				state.PushString(server.URL)
				state.SetGlobal("testURL")

				err = rt.Run(script, "bench")
				if err != nil {
					state.Close()
					b.Fatal(err)
				}
				state.Close()
			}
		})
	}
}

// BenchmarkRuntimeCreate measures runtime initialization overhead.
func BenchmarkRuntimeCreate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		state, err := luau.New()
		if err != nil {
			b.Fatal(err)
		}
		state.OpenLibs()

		rt := New(state, nil)
		if err := rt.RegisterAll(); err != nil {
			state.Close()
			b.Fatal(err)
		}
		state.Close()
	}
}

// BenchmarkRunScript measures script execution overhead (no async).
func BenchmarkRunScript(b *testing.B) {
	state, err := luau.New()
	if err != nil {
		b.Fatal(err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = rt.Run(`local x = 1 + 1`, "bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}
