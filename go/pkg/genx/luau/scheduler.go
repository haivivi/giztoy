package luau

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// ErrSchedulerClosed is returned when operations are attempted on a closed scheduler.
var ErrSchedulerClosed = errors.New("scheduler is closed")

// AsyncOp represents an asynchronous operation to be executed.
type AsyncOp interface {
	// Execute runs the operation and returns the result.
	// The context may be cancelled, in which case Execute should return promptly.
	Execute(ctx context.Context) (any, error)
}

// AsyncOpFunc is a function adapter for AsyncOp.
type AsyncOpFunc func(ctx context.Context) (any, error)

// Execute implements AsyncOp.
func (f AsyncOpFunc) Execute(ctx context.Context) (any, error) {
	return f(ctx)
}

// pendingOp represents an operation waiting to be executed.
type pendingOp struct {
	thread *luau.Thread
	op     AsyncOp
}

// opResult represents the result of an async operation.
type opResult struct {
	thread *luau.Thread
	value  any
	err    error
}

// Scheduler manages Luau coroutine execution with async I/O support.
// It coordinates between Luau scripts and Go goroutines for non-blocking operations.
//
// Usage pattern:
//  1. Create a scheduler with NewScheduler()
//  2. Register host functions that can yield using RegisterYieldFunc()
//  3. Run a script with Run()
//  4. Close the scheduler when done
type Scheduler struct {
	state    *luau.State
	ctx      context.Context
	cancel   context.CancelFunc
	logger   Logger
	
	mu       sync.Mutex
	pending  []pendingOp       // Operations waiting to be executed
	results  chan opResult     // Results from completed operations
	closed   bool
	
	// activeOps tracks the number of active goroutines
	activeOps sync.WaitGroup
}

// SchedulerConfig holds configuration for creating a Scheduler.
type SchedulerConfig struct {
	Logger Logger
}

// NewScheduler creates a new Scheduler with the given Luau state.
// The scheduler takes ownership of the state and will close it when done.
func NewScheduler(ctx context.Context, state *luau.State, cfg *SchedulerConfig) *Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	
	var logger Logger
	if cfg != nil && cfg.Logger != nil {
		logger = cfg.Logger
	} else {
		logger = &defaultLogger{}
	}
	
	return &Scheduler{
		state:   state,
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		results: make(chan opResult, 16),
	}
}

// State returns the underlying Luau state.
func (s *Scheduler) State() *luau.State {
	return s.state
}

// Context returns the scheduler's context.
func (s *Scheduler) Context() context.Context {
	return s.ctx
}

// SubmitAsync submits an async operation for execution.
// This should be called from a host function that wants to yield.
// The thread will be resumed with the result when the operation completes.
func (s *Scheduler) SubmitAsync(thread *luau.Thread, op AsyncOp) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.pending = append(s.pending, pendingOp{thread: thread, op: op})
	s.mu.Unlock()
}

// processPending starts goroutines for all pending operations.
func (s *Scheduler) processPending() {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	
	for _, p := range pending {
		s.activeOps.Add(1)
		go func(op pendingOp) {
			defer s.activeOps.Done()
			
			value, err := op.op.Execute(s.ctx)
			
			select {
			case s.results <- opResult{thread: op.thread, value: value, err: err}:
			case <-s.ctx.Done():
			}
		}(p)
	}
}

// RunThread runs a Luau thread until completion or yield.
// If the thread yields, pending async operations should be submitted via SubmitAsync.
// Returns the status, number of results on stack, and any error.
func (s *Scheduler) RunThread(thread *luau.Thread) (luau.CoStatus, int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return luau.CoStatusErrErr, 0, ErrSchedulerClosed
	}
	s.mu.Unlock()
	
	status, nresults := thread.Resume(0)
	
	switch status {
	case luau.CoStatusOK:
		return status, nresults, nil
	case luau.CoStatusYield:
		return status, nresults, nil
	case luau.CoStatusErrRun:
		// Get error message from stack
		errMsg := "runtime error"
		if thread.GetTop() > 0 && thread.IsString(-1) {
			errMsg = thread.ToString(-1)
		}
		return status, 0, fmt.Errorf("luau runtime error: %s", errMsg)
	default:
		return status, 0, fmt.Errorf("luau error: status %v", status)
	}
}

// ResumeWithResult resumes a yielded thread with a result value.
func (s *Scheduler) ResumeWithResult(thread *luau.Thread, value any, err error) (luau.CoStatus, int, error) {
	// Push the result onto the thread's stack
	if err != nil {
		thread.PushNil()
		thread.PushString(err.Error())
		status, nresults := thread.Resume(2)
		return s.handleResumeStatus(thread, status, nresults)
	}
	
	s.pushValue(thread.State, value)
	thread.PushNil() // No error
	status, nresults := thread.Resume(2)
	return s.handleResumeStatus(thread, status, nresults)
}

// handleResumeStatus processes the status after resuming a thread.
func (s *Scheduler) handleResumeStatus(thread *luau.Thread, status luau.CoStatus, nresults int) (luau.CoStatus, int, error) {
	switch status {
	case luau.CoStatusOK:
		return status, nresults, nil
	case luau.CoStatusYield:
		return status, nresults, nil
	case luau.CoStatusErrRun:
		errMsg := "runtime error"
		if thread.GetTop() > 0 && thread.IsString(-1) {
			errMsg = thread.ToString(-1)
		}
		return status, 0, fmt.Errorf("luau runtime error: %s", errMsg)
	default:
		return status, 0, fmt.Errorf("luau error: status %v", status)
	}
}

// pushValue pushes a Go value onto the Luau stack.
func (s *Scheduler) pushValue(state *luau.State, value any) {
	if value == nil {
		state.PushNil()
		return
	}
	
	switch v := value.(type) {
	case bool:
		state.PushBoolean(v)
	case int:
		state.PushInteger(int64(v))
	case int64:
		state.PushInteger(v)
	case float64:
		state.PushNumber(v)
	case string:
		state.PushString(v)
	case []byte:
		state.PushBytes(v)
	case map[string]any:
		state.NewTable()
		for k, val := range v {
			state.PushString(k)
			s.pushValue(state, val)
			state.SetTable(-3)
		}
	case []any:
		state.CreateTable(len(v), 0)
		for i, val := range v {
			s.pushValue(state, val)
			state.RawSetI(-2, i+1)
		}
	default:
		// For unsupported types, push as string representation
		state.PushString(fmt.Sprintf("%v", v))
	}
}

// Run executes the main loop, processing async operations until all threads complete.
// It processes pending operations and resumes threads with results.
func (s *Scheduler) Run() error {
	for {
		// Process any pending operations
		s.processPending()
		
		// Check if we have any active operations
		s.mu.Lock()
		hasPending := len(s.pending) > 0
		s.mu.Unlock()
		
		if !hasPending {
			// No pending operations, check if we're waiting for results
			select {
			case result := <-s.results:
				// Resume the thread with the result
				status, _, err := s.ResumeWithResult(result.thread, result.value, result.err)
				if err != nil {
					s.logger.Log("error", "thread resume error:", err)
				}
				if status == luau.CoStatusOK {
					// Thread completed
					result.thread.Close()
				}
			case <-s.ctx.Done():
				return s.ctx.Err()
			default:
				// No results waiting, check if we're done
				// Wait a bit for any results
				select {
				case result := <-s.results:
					status, _, err := s.ResumeWithResult(result.thread, result.value, result.err)
					if err != nil {
						s.logger.Log("error", "thread resume error:", err)
					}
					if status == luau.CoStatusOK {
						result.thread.Close()
					}
				case <-s.ctx.Done():
					return s.ctx.Err()
				default:
					// Truly nothing to do
					return nil
				}
			}
		}
	}
}

// Close shuts down the scheduler and releases resources.
func (s *Scheduler) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	
	// Cancel context to stop pending operations
	s.cancel()
	
	// Wait for active operations to complete
	s.activeOps.Wait()
	
	// Close the results channel
	close(s.results)
	
	return nil
}

// YieldForAsync is a helper that yields the current thread for an async operation.
// It should be called from a host function registered with RegisterFunc.
// Returns the number of values to yield (always 0, as results come via resume).
func YieldForAsync(state *luau.State) int {
	return state.Yield(0)
}
