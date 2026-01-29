// Package luau provides ToolRuntime for running Luau scripts with access to
// GenX capabilities like LLM generation, HTTP requests, and state management.
//
// The package enables Luau scripts to call async operations via coroutine
// yield/resume, allowing non-blocking concurrent execution.
//
// # Architecture
//
// Luau scripts interact with the system through an rt (runtime) object:
//
//	-- Tool script example
//	local input = rt:input()
//	local result, err = rt:generate("gpt-4o", "Write a poem about " .. input)
//	if err then
//	    rt:output(nil, err)
//	else
//	    rt:output({ text = result }, nil)
//	end
//
// # Async Operations
//
// Async operations (generate, http, invoke) use coroutine yield/resume:
//   - When a Luau script calls an async method, it yields
//   - The scheduler runs the operation in a goroutine
//   - When complete, the scheduler resumes the coroutine with the result
//
// # Available Runtime Methods
//
// The rt object provides these methods:
//   - rt:input() - Get input passed to the script
//   - rt:output(result, err) - Set output to return to caller
//   - rt:generate(model, prompt) - Generate text with LLM
//   - rt:generate_with_context(model, mctx) - Generate with full context
//   - rt:http_get(url, params) - HTTP GET request
//   - rt:http_post(url, opts) - HTTP POST request
//   - rt:state_get(key), rt:state_set(key, value) - State management
//   - rt:history_recent(n), rt:history_append(msg) - History management
//   - rt:log(level, ...) - Logging
package luau
