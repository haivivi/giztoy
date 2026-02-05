package runtime

import (
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// builtinSleep implements rt:sleep(ms) -> Promise
// Returns a Promise that resolves after the specified milliseconds.
// This is an async operation that yields the coroutine.
func (rt *Runtime) builtinSleep(state *luau.State) int {
	ms := state.ToNumber(1)
	if ms < 0 {
		ms = 0
	}

	promise := rt.promises.newPromise()

	go func() {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		promise.Resolve(nil)
	}()

	rt.pushPromiseObject(state, promise)
	return 1
}
