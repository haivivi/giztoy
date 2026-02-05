package runtime

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// timeoutHandle represents a scheduled timeout that can be cancelled.
type timeoutHandle struct {
	id        uint64
	cancelled atomic.Bool
	fired     atomic.Bool
	done      chan struct{}
	promise   *Promise
}

// timeoutRegistry manages active timeouts.
type timeoutRegistry struct {
	mu       sync.Mutex
	timeouts map[uint64]*timeoutHandle
	nextID   uint64
}

func newTimeoutRegistry() *timeoutRegistry {
	return &timeoutRegistry{
		timeouts: make(map[uint64]*timeoutHandle),
	}
}

func (r *timeoutRegistry) newTimeout(promise *Promise) *timeoutHandle {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	h := &timeoutHandle{
		id:      r.nextID,
		done:    make(chan struct{}),
		promise: promise,
	}
	r.timeouts[h.id] = h
	return h
}

func (r *timeoutRegistry) getTimeout(id uint64) (*timeoutHandle, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.timeouts[id]
	return h, ok
}

func (r *timeoutRegistry) removeTimeout(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.timeouts, id)
}

// Cancel cancels the timeout. Returns true if cancelled before firing.
func (h *timeoutHandle) Cancel() bool {
	if h.cancelled.Swap(true) {
		return false // Already cancelled
	}
	if h.fired.Load() {
		return false // Already fired
	}
	close(h.done)
	// Resolve the promise with cancelled=true
	h.promise.Resolve(map[string]any{"cancelled": true})
	return true
}

// builtinTimeout implements rt:timeout(ms) -> TimeoutHandle
// Returns a handle with :await() and :cancel() methods.
// The handle's :await() returns { cancelled = true/false } when the timeout fires or is cancelled.
func (rt *Runtime) builtinTimeout(state *luau.State) int {
	ms := state.ToNumber(1)
	if ms < 0 {
		ms = 0
	}

	// Initialize timeout registry if needed
	if rt.timeouts == nil {
		rt.timeouts = newTimeoutRegistry()
	}

	// Create promise for the timeout result
	promise := rt.promises.newPromise()

	// Create timeout handle
	handle := rt.timeouts.newTimeout(promise)

	// Start timeout goroutine
	go func() {
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
			if !handle.cancelled.Load() {
				handle.fired.Store(true)
				handle.promise.Resolve(map[string]any{"cancelled": false})
			}
		case <-handle.done:
			// Cancelled - promise already resolved in Cancel()
		}
		rt.timeouts.removeTimeout(handle.id)
	}()

	// Push timeout handle object (combines Promise and cancel functionality)
	rt.pushTimeoutHandle(state, handle, promise)
	return 1
}

// pushTimeoutHandle creates a Lua timeout handle object with :await(), :is_ready(), and :cancel() methods.
func (rt *Runtime) pushTimeoutHandle(state *luau.State, handle *timeoutHandle, promise *Promise) {
	state.NewTable()

	// _id field (for timeout handle)
	state.PushInteger(int64(handle.id))
	state.SetField(-2, "_id")

	// _promise_id field (for promise methods)
	state.PushInteger(int64(promise.id))
	state.SetField(-2, "_promise_id")

	// Get pre-registered methods from globals
	state.GetGlobal("__timeout_await")
	state.SetField(-2, "await")

	state.GetGlobal("__timeout_is_ready")
	state.SetField(-2, "is_ready")

	state.GetGlobal("__timeout_cancel")
	state.SetField(-2, "cancel")
}

// builtinTimeoutAwait implements handle:await() -> { cancelled = bool }
// This yields the coroutine until the timeout fires or is cancelled.
func (rt *Runtime) builtinTimeoutAwait(state *luau.State) int {
	// Get promise ID from self._promise_id
	state.GetField(1, "_promise_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	promise, ok := rt.promises.getPromise(id)
	if !ok {
		state.PushNil()
		state.PushString("timeout promise not found")
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

// builtinTimeoutIsReady implements handle:is_ready() -> bool
func (rt *Runtime) builtinTimeoutIsReady(state *luau.State) int {
	state.GetField(1, "_promise_id")
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

// builtinTimeoutCancel implements handle:cancel() -> bool
// Returns true if cancelled before firing.
func (rt *Runtime) builtinTimeoutCancel(state *luau.State) int {
	// Get handle ID from self._id
	state.GetField(1, "_id")
	id := uint64(state.ToInteger(-1))
	state.Pop(1)

	if rt.timeouts == nil {
		state.PushBoolean(false)
		return 1
	}

	handle, ok := rt.timeouts.getTimeout(id)
	if !ok {
		state.PushBoolean(false)
		return 1
	}

	cancelled := handle.Cancel()
	state.PushBoolean(cancelled)
	return 1
}
