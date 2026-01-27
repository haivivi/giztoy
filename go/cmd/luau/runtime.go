package main

import (
	"github.com/haivivi/giztoy/pkg/luau"
)

// Runtime holds the state for Luau script execution.
type Runtime struct {
	state   *luau.State
	libsDir string
	kvs     map[string]any
	loaded  map[string]bool
}

// RegisterBuiltins registers all builtin functions.
func (rt *Runtime) RegisterBuiltins() error {
	// Register __builtin table
	rt.state.NewTable()

	// __builtin.http
	if err := rt.state.RegisterFunc("__builtin_http", rt.builtinHTTP); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_http")
	rt.state.SetField(-2, "http")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_http")

	// __builtin.json_encode
	if err := rt.state.RegisterFunc("__builtin_json_encode", rt.builtinJSONEncode); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_json_encode")
	rt.state.SetField(-2, "json_encode")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_json_encode")

	// __builtin.json_decode
	if err := rt.state.RegisterFunc("__builtin_json_decode", rt.builtinJSONDecode); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_json_decode")
	rt.state.SetField(-2, "json_decode")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_json_decode")

	// __builtin.kvs_get
	if err := rt.state.RegisterFunc("__builtin_kvs_get", rt.builtinKVSGet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_get")
	rt.state.SetField(-2, "kvs_get")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_get")

	// __builtin.kvs_set
	if err := rt.state.RegisterFunc("__builtin_kvs_set", rt.builtinKVSSet); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_set")
	rt.state.SetField(-2, "kvs_set")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_set")

	// __builtin.kvs_del
	if err := rt.state.RegisterFunc("__builtin_kvs_del", rt.builtinKVSDel); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_kvs_del")
	rt.state.SetField(-2, "kvs_del")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_kvs_del")

	// __builtin.log
	if err := rt.state.RegisterFunc("__builtin_log", rt.builtinLog); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_log")
	rt.state.SetField(-2, "log")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_log")

	// __builtin.env
	if err := rt.state.RegisterFunc("__builtin_env", rt.builtinEnv); err != nil {
		return err
	}
	rt.state.GetGlobal("__builtin_env")
	rt.state.SetField(-2, "env")
	rt.state.PushNil()
	rt.state.SetGlobal("__builtin_env")

	// Set __builtin global
	rt.state.SetGlobal("__builtin")

	// Initialize __loaded table for module caching
	rt.state.NewTable()
	rt.state.SetGlobal("__loaded")

	// Register require function LAST to ensure it overrides any built-in
	if err := rt.state.RegisterFunc("require", rt.builtinRequire); err != nil {
		return err
	}

	return nil
}
