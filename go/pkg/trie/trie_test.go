package trie

import (
	"fmt"
	"testing"
)

func TestTrieBasic(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("a/b/c", "value1"); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}
	
	val, ok := tr.GetValue("a/b/c")
	if !ok || val != "value1" {
		t.Errorf("GetValue failed: got %v, %v", val, ok)
	}
}

func TestTrieWildcard(t *testing.T) {
	tr := New[string]()

	if err := tr.SetValue("device/+/state", "single_wildcard"); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}
	if err := tr.SetValue("logs/#", "multi_wildcard"); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}
	
	val, ok := tr.GetValue("device/gear-001/state")
	if !ok || val != "single_wildcard" {
		t.Errorf("Single wildcard match failed: got %v, %v", val, ok)
	}
	
	val, ok = tr.GetValue("logs/app/debug/line1")
	if !ok || val != "multi_wildcard" {
		t.Errorf("Multi wildcard match failed: got %v, %v", val, ok)
	}
}

// generatePaths generates test paths for benchmarking
func generatePaths(count int) []string {
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		a := i % 10
		b := (i / 10) % 10
		c := (i / 100) % 10
		paths[i] = fmt.Sprintf("device/gear-%03d/sensor/%d/data/%d", i, a, b*10+c)
	}
	return paths
}

// BenchmarkTrieSet benchmarks inserting paths into trie
func BenchmarkTrieSet(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		paths := generatePaths(size)
		b.Run(fmt.Sprintf("exact_paths/%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr := New[int]()
				for j, path := range paths {
					tr.SetValue(path, j)
				}
			}
		})
	}
}

// BenchmarkTrieGetExact benchmarks exact path lookups
func BenchmarkTrieGetExact(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		paths := generatePaths(size)
		tr := New[int]()
		for j, path := range paths {
			tr.SetValue(path, j)
		}
		
		b.Run(fmt.Sprintf("lookup/%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, path := range paths {
					tr.Get(path)
				}
			}
		})
	}
}

// BenchmarkTrieGetWildcard benchmarks wildcard pattern matching
func BenchmarkTrieGetWildcard(b *testing.B) {
	tr := New[string]()
	patterns := []string{
		"device/+/sensor/+/data/+",
		"device/gear-001/+/+/data/+",
		"device/#",
		"device/+/#",
		"logs/#",
	}
	for _, pattern := range patterns {
		tr.SetValue(pattern, pattern)
	}
	
	testPaths := []string{
		"device/gear-001/sensor/0/data/1",
		"device/gear-999/sensor/5/data/99",
		"device/gear-001/state/online",
		"logs/app/debug/line1",
		"logs/system/error",
	}
	
	b.Run("wildcard_match", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				tr.Get(path)
			}
		}
	})
}

// BenchmarkTrieMatchPath benchmarks match_path with mixed patterns
func BenchmarkTrieMatchPath(b *testing.B) {
	tr := New[int]()

	// Add exact paths
	exactPaths := generatePaths(1000)
	for j, path := range exactPaths {
		tr.SetValue(path, j)
	}
	
	// Add wildcard patterns
	tr.SetValue("device/+/sensor/+/data/+", -1)
	tr.SetValue("device/#", -2)
	tr.SetValue("logs/#", -3)
	
	testPaths := []string{
		"device/gear-500/sensor/5/data/50",  // exact match exists
		"device/gear-9999/sensor/0/data/0",  // wildcard match only
		"device/unknown/state",               // # wildcard
		"logs/anything/here",                 // # wildcard
	}
	
	b.Run("mixed_match", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				tr.Match(path)
			}
		}
	})
}

// BenchmarkTrieWalk benchmarks walking all nodes
func BenchmarkTrieWalk(b *testing.B) {
	for _, size := range []int{100, 1000} {
		paths := generatePaths(size)
		tr := New[int]()
		for j, path := range paths {
			tr.SetValue(path, j)
		}
		
		b.Run(fmt.Sprintf("walk_all/%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
	count := 0
				tr.Walk(func(_ string, _ int, set bool) {
		if set {
			count++
		}
	})
				_ = count
			}
		})
	}
}

// BenchmarkTrieDeepPaths benchmarks deep path operations
func BenchmarkTrieDeepPaths(b *testing.B) {
	// Create very deep paths
	deepPaths := make([]string, 100)
	for i := 0; i < 100; i++ {
		deepPaths[i] = fmt.Sprintf("a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/%d", i)
	}
	
	b.Run("deep_set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := New[int]()
			for j, path := range deepPaths {
				tr.SetValue(path, j)
			}
		}
	})
	
	tr := New[int]()
	for j, path := range deepPaths {
		tr.SetValue(path, j)
	}
	
	b.Run("deep_get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range deepPaths {
				tr.Get(path)
			}
		}
	})
}

// BenchmarkTrieMemory shows memory allocation patterns
func BenchmarkTrieMemory(b *testing.B) {
	paths := generatePaths(1000)
	
	b.Run("set_allocs", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := New[int]()
			for j, path := range paths {
				tr.SetValue(path, j)
			}
		}
	})
	
	tr := New[int]()
	for j, path := range paths {
		tr.SetValue(path, j)
	}
	
	b.Run("get_allocs", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range paths {
				tr.Get(path)
			}
		}
	})
}
