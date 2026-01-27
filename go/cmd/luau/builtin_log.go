package main

import (
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/pkg/luau"
)

// builtinLog implements __builtin.log(...)
func (rt *Runtime) builtinLog(state *luau.State) int {
	n := state.GetTop()
	parts := make([]string, 0, n)

	for i := 1; i <= n; i++ {
		switch state.TypeOf(i) {
		case luau.TypeNil:
			parts = append(parts, "nil")
		case luau.TypeBoolean:
			if state.ToBoolean(i) {
				parts = append(parts, "true")
			} else {
				parts = append(parts, "false")
			}
		case luau.TypeNumber:
			parts = append(parts, fmt.Sprintf("%g", state.ToNumber(i)))
		case luau.TypeString:
			parts = append(parts, state.ToString(i))
		case luau.TypeTable:
			parts = append(parts, "[table]")
		case luau.TypeFunction:
			parts = append(parts, "[function]")
		default:
			parts = append(parts, fmt.Sprintf("[%s]", state.TypeName(state.TypeOf(i))))
		}
	}

	fmt.Println(strings.Join(parts, "\t"))
	return 0
}
