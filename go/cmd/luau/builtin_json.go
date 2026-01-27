package main

import (
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/pkg/luau"
)

// builtinJSONEncode implements __builtin.json_encode(value) -> string
func (rt *Runtime) builtinJSONEncode(state *luau.State) int {
	value := luaToGo(state, 1)
	data, err := json.Marshal(value)
	if err != nil {
		state.PushNil()
		return 1
	}
	state.PushString(string(data))
	return 1
}

// builtinJSONDecode implements __builtin.json_decode(str) -> value
func (rt *Runtime) builtinJSONDecode(state *luau.State) int {
	str := state.ToString(1)
	if str == "" {
		state.PushNil()
		return 1
	}

	var value any
	if err := json.Unmarshal([]byte(str), &value); err != nil {
		state.PushNil()
		return 1
	}

	goToLua(state, value)
	return 1
}

// luaToGo converts a Lua value at the given stack index to a Go value.
func luaToGo(state *luau.State, idx int) any {
	switch state.TypeOf(idx) {
	case luau.TypeNil:
		return nil
	case luau.TypeBoolean:
		return state.ToBoolean(idx)
	case luau.TypeNumber:
		return state.ToNumber(idx)
	case luau.TypeString:
		return state.ToString(idx)
	case luau.TypeTable:
		return luaTableToGo(state, idx)
	default:
		return nil
	}
}

// luaTableToGo converts a Lua table to Go map or slice.
func luaTableToGo(state *luau.State, idx int) any {
	// Convert negative index to absolute
	if idx < 0 {
		idx = state.GetTop() + idx + 1
	}

	// Check if it's an array (consecutive integer keys starting from 1)
	isArray := true
	maxIdx := 0

	state.PushNil()
	for state.Next(idx) {
		if state.TypeOf(-2) != luau.TypeNumber {
			isArray = false
			state.Pop(2)
			break
		}
		i := int(state.ToNumber(-2))
		if i > maxIdx {
			maxIdx = i
		}
		state.Pop(1)
	}

	if isArray && maxIdx > 0 {
		// Convert to slice
		arr := make([]any, maxIdx)
		for i := 1; i <= maxIdx; i++ {
			state.PushNumber(float64(i))
			state.GetTable(idx)
			arr[i-1] = luaToGo(state, -1)
			state.Pop(1)
		}
		return arr
	}

	// Convert to map
	m := make(map[string]any)
	state.PushNil()
	for state.Next(idx) {
		var key string
		switch state.TypeOf(-2) {
		case luau.TypeString:
			key = state.ToString(-2)
		case luau.TypeNumber:
			num := state.ToNumber(-2)
			// Use integer format if it's a whole number, otherwise use float
			if num == float64(int64(num)) {
				key = fmt.Sprintf("%d", int64(num))
			} else {
				key = fmt.Sprintf("%f", num)
			}
		default:
			state.Pop(1)
			continue
		}
		m[key] = luaToGo(state, -1)
		state.Pop(1)
	}
	return m
}

// goToLua pushes a Go value onto the Lua stack.
func goToLua(state *luau.State, value any) {
	switch v := value.(type) {
	case nil:
		state.PushNil()
	case bool:
		state.PushBoolean(v)
	case float64:
		state.PushNumber(v)
	case float32:
		state.PushNumber(float64(v))
	case int:
		state.PushNumber(float64(v))
	case int64:
		state.PushNumber(float64(v))
	case string:
		state.PushString(v)
	case []any:
		state.NewTable()
		for i, item := range v {
			state.PushNumber(float64(i + 1))
			goToLua(state, item)
			state.SetTable(-3)
		}
	case map[string]any:
		state.NewTable()
		for k, item := range v {
			state.PushString(k)
			goToLua(state, item)
			state.SetTable(-3)
		}
	default:
		state.PushNil()
	}
}
