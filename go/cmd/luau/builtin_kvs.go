package main

import (
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// builtinKVSGet implements __builtin.kvs_get(key) -> value
func (rt *Runtime) builtinKVSGet(state *luau.State) int {
	key := state.ToString(1)
	if key == "" {
		state.PushNil()
		return 1
	}

	value, ok := rt.kvs[key]
	if !ok {
		state.PushNil()
		return 1
	}

	goToLua(state, value)
	return 1
}

// builtinKVSSet implements __builtin.kvs_set(key, value)
func (rt *Runtime) builtinKVSSet(state *luau.State) int {
	key := state.ToString(1)
	if key == "" {
		return 0
	}

	value := luaToGo(state, 2)
	rt.kvs[key] = value
	return 0
}

// builtinKVSDel implements __builtin.kvs_del(key)
func (rt *Runtime) builtinKVSDel(state *luau.State) int {
	key := state.ToString(1)
	if key != "" {
		delete(rt.kvs, key)
	}
	return 0
}
