package runtime

import (
	"sync"
	"sync/atomic"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// PromiseResult holds the result of an async operation.
type PromiseResult struct {
	Values []any // Return values to push onto Lua stack
	Err    error // Error if any
}

// Promise represents an async operation that will complete in the future.
// In Lua, it exposes :await() and :is_ready() methods.
type Promise struct {
	id       uint64
	resultCh chan PromiseResult
	done     atomic.Bool // Atomic for fast lock-free reads
	mu       sync.Mutex  // Only for protecting result write
	result   PromiseResult
}

// promisePool reduces allocation pressure by reusing Promise objects.
var promisePool = sync.Pool{
	New: func() any {
		return &Promise{
			resultCh: make(chan PromiseResult, 1),
		}
	},
}

// promiseRegistry manages active promises in the runtime.
type promiseRegistry struct {
	mu       sync.RWMutex // RWMutex for better read performance
	promises map[uint64]*Promise
	nextID   uint64
}

func newPromiseRegistry() *promiseRegistry {
	return &promiseRegistry{
		promises: make(map[uint64]*Promise),
	}
}

// newPromise creates a new Promise and registers it.
// Uses sync.Pool to reduce allocation pressure.
func (r *promiseRegistry) newPromise() *Promise {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++

	// Get from pool
	p := promisePool.Get().(*Promise)

	// Reset state
	p.id = r.nextID
	p.done.Store(false)
	p.result = PromiseResult{}

	// Drain and recreate channel if needed (could have leftover data)
	select {
	case <-p.resultCh:
	default:
	}

	r.promises[p.id] = p
	return p
}

// getPromise retrieves a promise by ID (read-only, uses RLock).
func (r *promiseRegistry) getPromise(id uint64) (*Promise, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.promises[id]
	return p, ok
}

// removePromise removes a promise from the registry and returns it to the pool.
func (r *promiseRegistry) removePromise(id uint64) {
	r.mu.Lock()
	p, ok := r.promises[id]
	if ok {
		delete(r.promises, id)
	}
	r.mu.Unlock()

	// Return to pool for reuse
	if ok && p != nil {
		promisePool.Put(p)
	}
}

// Resolve completes the promise with the given result.
func (p *Promise) Resolve(values ...any) {
	// Fast path: check if already done without lock
	if p.done.Load() {
		return
	}

	p.mu.Lock()
	// Double-check under lock
	if p.done.Load() {
		p.mu.Unlock()
		return
	}
	p.result = PromiseResult{Values: values}
	p.done.Store(true)
	p.mu.Unlock()

	// Non-blocking send (channel has buffer of 1)
	select {
	case p.resultCh <- p.result:
	default:
	}
}

// Reject completes the promise with an error.
func (p *Promise) Reject(err error) {
	// Fast path: check if already done without lock
	if p.done.Load() {
		return
	}

	p.mu.Lock()
	// Double-check under lock
	if p.done.Load() {
		p.mu.Unlock()
		return
	}
	p.result = PromiseResult{Err: err}
	p.done.Store(true)
	p.mu.Unlock()

	select {
	case p.resultCh <- p.result:
	default:
	}
}

// IsReady returns true if the promise has been resolved or rejected.
// Uses atomic load for lock-free fast path.
func (p *Promise) IsReady() bool {
	return p.done.Load()
}

// Result returns the result if ready, otherwise returns false.
func (p *Promise) Result() (PromiseResult, bool) {
	// Fast path: check done flag atomically
	if !p.done.Load() {
		return PromiseResult{}, false
	}

	// Done, so result is stable - can read without lock
	return p.result, true
}

// ResultChan returns the channel that will receive the result.
func (p *Promise) ResultChan() <-chan PromiseResult {
	return p.resultCh
}

// builtinPromiseAwait implements promise:await() -> values...
// This yields the coroutine until the promise is resolved.
func (rt *Runtime) builtinPromiseAwait(state *luau.State) int {
	// Get promise ID from self._id
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	promise, ok := rt.promises.getPromise(id)
	if !ok {
		state.PushNil()
		state.PushString("promise not found")
		return 2
	}

	// Check if already resolved
	if result, ready := promise.Result(); ready {
		rt.promises.removePromise(id)
		return rt.pushPromiseResult(state, result)
	}

	// Not ready - register as pending and yield
	rt.registerPendingPromise(promise)

	// Yield - will be resumed when promise resolves
	return state.Yield(0)
}

// builtinPromiseIsReady implements promise:is_ready() -> bool
func (rt *Runtime) builtinPromiseIsReady(state *luau.State) int {
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	promise, ok := rt.promises.getPromise(id)
	if !ok {
		state.PushBoolean(true) // Treat missing promise as "done"
		return 1
	}

	state.PushBoolean(promise.IsReady())
	return 1
}

// pushPromiseResult pushes the promise result values onto the Lua stack.
func (rt *Runtime) pushPromiseResult(state *luau.State, result PromiseResult) int {
	if result.Err != nil {
		state.PushNil()
		state.PushString(result.Err.Error())
		return 2
	}

	// Push all values
	for _, v := range result.Values {
		goToLua(state, v)
	}

	// If no values, push nil
	if len(result.Values) == 0 {
		state.PushNil()
		return 1
	}

	return len(result.Values)
}

// pushPromiseObject creates a Lua promise object with :await() and :is_ready() methods.
func (rt *Runtime) pushPromiseObject(state *luau.State, promise *Promise) {
	state.NewTable()

	// _id field
	state.PushInteger(int64(promise.id))
	state.SetField(-2, "_id")

	// Get pre-registered await method from globals and set on table
	state.GetGlobal("__promise_await")
	state.SetField(-2, "await")

	// Get pre-registered is_ready method from globals and set on table
	state.GetGlobal("__promise_is_ready")
	state.SetField(-2, "is_ready")
}

// builtinAwaitAll implements rt:await_all(promises...) -> Promise
// Returns a Promise that resolves with an array of all results when all input promises resolve.
func (rt *Runtime) builtinAwaitAll(state *luau.State) int {
	nargs := state.GetTop()
	if nargs == 0 {
		// No promises - return immediately resolved promise with empty array
		promise := rt.promises.newPromise()
		promise.Resolve([]any{})
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Collect promise IDs
	// Check both _id (regular promises) and _promise_id (timeout handles)
	promiseIDs := make([]uint64, 0, nargs)
	for i := 1; i <= nargs; i++ {
		if !state.IsTable(i) {
			// Not a promise table, skip
			continue
		}
		// First try _promise_id (for TimeoutHandle which has both _id and _promise_id)
		state.GetField(i, "_promise_id")
		if !state.IsNil(-1) {
			id := uint64(state.ToInteger(-1))
			promiseIDs = append(promiseIDs, id)
			state.Pop(1)
			continue
		}
		state.Pop(1)

		// Fall back to _id (for regular Promise objects)
		state.GetField(i, "_id")
		if !state.IsNil(-1) {
			id := uint64(state.ToInteger(-1))
			promiseIDs = append(promiseIDs, id)
		}
		state.Pop(1)
	}

	if len(promiseIDs) == 0 {
		// No valid promises - return immediately resolved promise with empty array
		promise := rt.promises.newPromise()
		promise.Resolve([]any{})
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Create result promise
	resultPromise := rt.promises.newPromise()

	// Start goroutine to wait for all promises
	go func() {
		results := make([]any, len(promiseIDs))
		var hasError bool
		var firstError string

		for i, id := range promiseIDs {
			p, ok := rt.promises.getPromise(id)
			if !ok {
				results[i] = nil
				continue
			}

			// Wait for this promise
			result := <-p.ResultChan()
			if result.Err != nil {
				if !hasError {
					hasError = true
					firstError = result.Err.Error()
				}
				results[i] = map[string]any{"err": result.Err.Error()}
			} else if len(result.Values) > 0 {
				results[i] = result.Values[0]
			} else {
				results[i] = nil
			}
		}

		if hasError {
			resultPromise.Resolve(results, firstError)
		} else {
			resultPromise.Resolve(results, nil)
		}
	}()

	rt.pushPromiseObject(state, resultPromise)
	return 1
}

// builtinAwaitAny implements rt:await_any(promises...) -> Promise
// Returns a Promise that resolves with the first result from any input promise.
func (rt *Runtime) builtinAwaitAny(state *luau.State) int {
	nargs := state.GetTop()
	if nargs == 0 {
		// No promises - return immediately resolved promise with nil
		promise := rt.promises.newPromise()
		promise.Resolve(nil)
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Collect promise IDs and their channels
	// Check both _promise_id (for TimeoutHandle) and _id (for regular Promise)
	type promiseInfo struct {
		id uint64
		ch <-chan PromiseResult
	}
	promises := make([]promiseInfo, 0, nargs)

	for i := 1; i <= nargs; i++ {
		if !state.IsTable(i) {
			continue
		}

		var id uint64
		var found bool

		// First try _promise_id (for TimeoutHandle)
		state.GetField(i, "_promise_id")
		if !state.IsNil(-1) {
			id = uint64(state.ToInteger(-1))
			found = true
		}
		state.Pop(1)

		// Fall back to _id (for regular Promise)
		if !found {
			state.GetField(i, "_id")
			if !state.IsNil(-1) {
				id = uint64(state.ToInteger(-1))
				found = true
			}
			state.Pop(1)
		}

		if found {
			if p, ok := rt.promises.getPromise(id); ok {
				promises = append(promises, promiseInfo{id: id, ch: p.ResultChan()})
			}
		}
	}

	if len(promises) == 0 {
		// No valid promises - return immediately resolved promise with nil
		promise := rt.promises.newPromise()
		promise.Resolve(nil)
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Create result promise
	resultPromise := rt.promises.newPromise()

	// Start goroutine to wait for first promise
	go func() {
		// Use select with reflect for dynamic number of channels
		// For simplicity, we'll use a goroutine per promise approach
		done := make(chan struct{})
		var once sync.Once

		for _, pi := range promises {
			go func(info promiseInfo) {
				select {
				case result := <-info.ch:
					once.Do(func() {
						if result.Err != nil {
							resultPromise.Resolve(nil, result.Err.Error())
						} else if len(result.Values) > 0 {
							resultPromise.Resolve(result.Values[0], nil)
						} else {
							resultPromise.Resolve(nil, nil)
						}
						close(done)
					})
				case <-done:
					// Another promise won
				}
			}(pi)
		}
	}()

	rt.pushPromiseObject(state, resultPromise)
	return 1
}
