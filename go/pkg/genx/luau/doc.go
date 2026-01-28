// Package luau provides ToolContext for running Luau scripts with access to
// GenX capabilities like LLM generation, HTTP requests, and state management.
//
// The package enables Luau scripts to call async operations via coroutine
// yield/resume, allowing non-blocking concurrent execution.
//
// # Architecture
//
// Luau scripts interact with the system through a ctx object:
//
//	function invoke(ctx, args)
//	    local result = ctx.generate("gpt-4o", "Write a poem")
//	    return { text = result }
//	end
//
// # Async Operations
//
// Async operations (generate, http, invoke) use coroutine yield/resume:
//   - When a Luau script calls an async method, it yields
//   - The scheduler runs the operation in a goroutine
//   - When complete, the scheduler resumes the coroutine with the result
//
// # ModelContext Builder
//
// Luau scripts can build ModelContext using the builder pattern:
//
//	local mctx = ctx.model_context()
//	    :system("You are a helpful assistant")
//	    :user("Hello")
//	    :build()
//
//	for chunk in ctx.generate_stream("gpt-4o", mctx) do
//	    ctx.emit({ text = chunk.text })
//	end
package luau
