package runtime

import (
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestMemoryCache_GetSetDelete(t *testing.T) {
	cache := newMemoryCache()

	t.Run("set and get", func(t *testing.T) {
		cache.Set("key1", "value1", 0)
		v, ok := cache.Get("key1")
		if !ok {
			t.Error("expected to find key1")
		}
		if v != "value1" {
			t.Errorf("expected 'value1', got %v", v)
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		_, ok := cache.Get("nonexistent")
		if ok {
			t.Error("expected not to find nonexistent key")
		}
	})

	t.Run("delete", func(t *testing.T) {
		cache.Set("key2", "value2", 0)
		cache.Delete("key2")
		_, ok := cache.Get("key2")
		if ok {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		cache.Set("key3", "original", 0)
		cache.Set("key3", "updated", 0)
		v, ok := cache.Get("key3")
		if !ok {
			t.Error("expected to find key3")
		}
		if v != "updated" {
			t.Errorf("expected 'updated', got %v", v)
		}
	})

	t.Run("different value types", func(t *testing.T) {
		cache.Set("int", 42, 0)
		cache.Set("float", 3.14, 0)
		cache.Set("bool", true, 0)
		cache.Set("map", map[string]any{"nested": "value"}, 0)

		if v, _ := cache.Get("int"); v != 42 {
			t.Errorf("int: expected 42, got %v", v)
		}
		if v, _ := cache.Get("float"); v != 3.14 {
			t.Errorf("float: expected 3.14, got %v", v)
		}
		if v, _ := cache.Get("bool"); v != true {
			t.Errorf("bool: expected true, got %v", v)
		}
	})
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := newMemoryCache()

	t.Run("item expires after TTL", func(t *testing.T) {
		cache.Set("expiring", "value", 50*time.Millisecond)

		// Should exist immediately
		v, ok := cache.Get("expiring")
		if !ok {
			t.Error("expected to find key immediately after set")
		}
		if v != "value" {
			t.Errorf("expected 'value', got %v", v)
		}

		// Wait for expiration
		time.Sleep(60 * time.Millisecond)

		// Should be expired now
		_, ok = cache.Get("expiring")
		if ok {
			t.Error("expected key to be expired")
		}
	})

	t.Run("zero TTL means no expiration", func(t *testing.T) {
		cache.Set("permanent", "value", 0)

		// Should exist
		_, ok := cache.Get("permanent")
		if !ok {
			t.Error("expected permanent key to exist")
		}
	})

	t.Run("negative TTL treated as zero", func(t *testing.T) {
		cache.Set("negative-ttl", "value", -1*time.Second)

		// Should exist (no expiration set)
		_, ok := cache.Get("negative-ttl")
		if !ok {
			t.Error("expected key with negative TTL to exist")
		}
	})
}

func TestMemoryCache_Concurrent(t *testing.T) {
	cache := newMemoryCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			cache.Set(key, n, 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			cache.Get(key)
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + n%26))
			cache.Delete(key)
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestBuiltinCacheGet(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	cache := newMemoryCache()
	cache.Set("testkey", "testvalue", 0)
	rt.cache = cache

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	t.Run("get existing key", func(t *testing.T) {
		err := state.DoString(`
			local v = rt:cache_get("testkey")
			_G.result = v
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		state.GetGlobal("result")
		if state.ToString(-1) != "testvalue" {
			t.Errorf("expected 'testvalue', got %s", state.ToString(-1))
		}
		state.Pop(1)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		err := state.DoString(`
			local v = rt:cache_get("nonexistent")
			_G.result = v
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		state.GetGlobal("result")
		if !state.IsNil(-1) {
			t.Error("expected nil for non-existent key")
		}
		state.Pop(1)
	})

	t.Run("get with empty key", func(t *testing.T) {
		err := state.DoString(`
			local v = rt:cache_get("")
			_G.result = v
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		state.GetGlobal("result")
		if !state.IsNil(-1) {
			t.Error("expected nil for empty key")
		}
		state.Pop(1)
	})
}

func TestBuiltinCacheGetNoCache(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	// Don't set rt.cache - leave it nil

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = state.DoString(`
		local v = rt:cache_get("anykey")
		_G.result = v
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	if !state.IsNil(-1) {
		t.Error("expected nil when cache is not set")
	}
}

func TestBuiltinCacheSet(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	t.Run("set creates cache if nil", func(t *testing.T) {
		if rt.cache != nil {
			t.Skip("cache already exists")
		}

		err := state.DoString(`
			rt:cache_set("key1", "value1")
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		if rt.cache == nil {
			t.Error("expected cache to be created")
		}

		v, ok := rt.cache.Get("key1")
		if !ok {
			t.Error("expected to find key1")
		}
		if v != "value1" {
			t.Errorf("expected 'value1', got %v", v)
		}
	})

	t.Run("set with TTL via direct cache", func(t *testing.T) {
		// Test TTL through direct cache API (more reliable than timing through Lua)
		rt.cache.Set("expiring_direct", "value", 50*time.Millisecond)

		// Should exist
		_, ok := rt.cache.Get("expiring_direct")
		if !ok {
			t.Error("expected key to exist immediately")
		}

		// Wait and check expiration
		time.Sleep(100 * time.Millisecond)
		_, ok = rt.cache.Get("expiring_direct")
		if ok {
			t.Error("expected key to be expired")
		}
	})

	t.Run("set with TTL via Lua", func(t *testing.T) {
		// Test that TTL parameter is passed correctly through Lua
		err := state.DoString(`
			rt:cache_set("expiring_lua", "value", 1)  -- 1 second TTL
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		// Should exist immediately
		_, ok := rt.cache.Get("expiring_lua")
		if !ok {
			t.Error("expected key to exist immediately after set")
		}
		// Note: we don't wait for expiration here to avoid flaky timing tests
	})

	t.Run("set with nil TTL", func(t *testing.T) {
		err := state.DoString(`
			rt:cache_set("nilttl", "value", nil)
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		_, ok := rt.cache.Get("nilttl")
		if !ok {
			t.Error("expected key to exist")
		}
	})

	t.Run("set with empty key does nothing", func(t *testing.T) {
		err := state.DoString(`
			rt:cache_set("", "value")
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}
		// No error expected, just a no-op
	})

	t.Run("set complex value", func(t *testing.T) {
		err := state.DoString(`
			rt:cache_set("complex", {name = "test", count = 42})
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		v, ok := rt.cache.Get("complex")
		if !ok {
			t.Error("expected to find complex key")
		}

		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", v)
		}
		if m["name"] != "test" {
			t.Errorf("expected name='test', got %v", m["name"])
		}
	})
}

func TestBuiltinCacheDelete(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	cache := newMemoryCache()
	cache.Set("todelete", "value", 0)
	rt.cache = cache

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	t.Run("delete existing key", func(t *testing.T) {
		err := state.DoString(`
			rt:cache_del("todelete")
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}

		_, ok := cache.Get("todelete")
		if ok {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		// Should not error
		err := state.DoString(`
			rt:cache_del("nonexistent")
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}
	})

	t.Run("delete with empty key", func(t *testing.T) {
		// Should not error
		err := state.DoString(`
			rt:cache_del("")
		`)
		if err != nil {
			t.Fatalf("DoString failed: %v", err)
		}
	})
}

func TestBuiltinCacheDeleteNoCache(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	// Don't set rt.cache

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Should not error even without cache
	err = state.DoString(`
		rt:cache_del("anykey")
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}
}
