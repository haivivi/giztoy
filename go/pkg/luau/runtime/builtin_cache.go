package runtime

import (
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// cacheEntry represents a cached value with optional TTL.
type cacheEntry struct {
	value     any
	expiresAt time.Time // zero means no expiration
}

// CacheProvider is the interface for cache storage.
// This allows different implementations (memory, badger, etc.)
type CacheProvider interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
	Delete(key string)
}

// memoryCache is an in-memory cache implementation.
type memoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

// newMemoryCache creates a new in-memory cache.
func newMemoryCache() *memoryCache {
	return &memoryCache{
		items: make(map[string]cacheEntry),
	}
}

func (c *memoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiration
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.Delete(key)
		return nil, false
	}

	return entry.value, true
}

func (c *memoryCache) Set(key string, value any, ttl time.Duration) {
	entry := cacheEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.items[key] = entry
	c.mu.Unlock()
}

func (c *memoryCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// builtinCacheGet implements __builtin.cache_get(key) -> value
func (rt *Runtime) builtinCacheGet(state *luau.State) int {
	key := state.ToString(1)
	if key == "" {
		state.PushNil()
		return 1
	}

	if rt.cache == nil {
		state.PushNil()
		return 1
	}

	value, ok := rt.cache.Get(key)
	if !ok {
		state.PushNil()
		return 1
	}

	goToLua(state, value)
	return 1
}

// builtinCacheSet implements __builtin.cache_set(key, value, ttl?)
// ttl is optional, in seconds
func (rt *Runtime) builtinCacheSet(state *luau.State) int {
	key := state.ToString(1)
	if key == "" {
		return 0
	}

	if rt.cache == nil {
		rt.cache = newMemoryCache()
	}

	value := luaToGo(state, 2)

	var ttl time.Duration
	if state.GetTop() >= 3 && !state.IsNil(3) {
		ttlSec := state.ToNumber(3)
		if ttlSec > 0 {
			ttl = time.Duration(ttlSec) * time.Second
		}
	}

	rt.cache.Set(key, value, ttl)
	return 0
}

// builtinCacheDelete implements __builtin.cache_del(key)
func (rt *Runtime) builtinCacheDelete(state *luau.State) int {
	key := state.ToString(1)
	if key != "" && rt.cache != nil {
		rt.cache.Delete(key)
	}
	return 0
}
