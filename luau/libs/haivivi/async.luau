--!strict
-- Async module for coroutine-based asynchronous operations.
--
-- This module provides coroutine wrappers for blocking operations like HTTP,
-- allowing host runtimes to implement true async I/O via event loops.
--
-- Usage:
--   local task = async.http(request)
--   -- In an event loop:
--   while async.pending(task) do
--       async.poll(task)  -- Drives the coroutine
--   end
--   local resp = async.result(task)

local async = {}

-- Task state
export type TaskState = "pending" | "running" | "completed" | "failed"

-- Generic async task
export type Task<T> = {
    co: thread,
    state: TaskState,
    result: T?,
    error: string?,
}

-- Yield types for host to handle
export type YieldType = "http" | "sleep" | "kvs_get" | "kvs_set" | "background_task"

export type YieldValue = {
    type: YieldType,
    -- HTTP request
    request: {
        url: string,
        method: string?,
        headers: {[string]: string}?,
        body: string?,
    }?,
    -- Sleep duration in seconds
    duration: number?,
    -- KVS key
    key: string?,
    -- KVS value
    value: any?,
    -- Background task reference (for background_task type)
    task: Task<any>?,
}

-- Create an async HTTP task
function async.http(request: {
    url: string,
    method: string?,
    headers: {[string]: string}?,
    body: string?,
}): Task<any>
    local task: Task<any> = {
        co = coroutine.create(function()
            -- Yield to host with HTTP request
            local resp = coroutine.yield({
                type = "http",
                request = request,
            } :: YieldValue)
            return resp
        end),
        state = "pending",
        result = nil,
        error = nil,
    }
    return task
end

-- Create an async sleep task
function async.sleep(seconds: number): Task<nil>
    local task: Task<nil> = {
        co = coroutine.create(function()
            coroutine.yield({
                type = "sleep",
                duration = seconds,
            } :: YieldValue)
            return nil
        end),
        state = "pending",
        result = nil,
        error = nil,
    }
    return task
end

-- Check if task is pending
function async.pending(task: Task<any>): boolean
    return task.state == "pending" or task.state == "running"
end

-- Check if task is completed (success or failure)
function async.done(task: Task<any>): boolean
    return task.state == "completed" or task.state == "failed"
end

-- Poll/drive the task (called by event loop)
-- Returns the yield value if task yielded, nil otherwise
function async.poll(task: Task<any>, resume_value: any?): YieldValue?
    if task.state == "completed" or task.state == "failed" then
        return nil
    end
    
    task.state = "running"
    
    local ok, result = coroutine.resume(task.co, resume_value)
    
    if not ok then
        task.state = "failed"
        task.error = tostring(result)
        return nil
    end
    
    local status = coroutine.status(task.co)
    if status == "dead" then
        task.state = "completed"
        task.result = result
        return nil
    end
    
    -- Task yielded, return the yield value
    return result :: YieldValue
end

-- Get task result (only valid after completion)
function async.result(task: Task<any>): (any?, string?)
    if task.state == "failed" then
        return nil, task.error
    end
    if task.state == "completed" then
        return task.result, nil
    end
    return nil, "task not completed"
end

-- ============================================================================
-- Task combinators
-- ============================================================================

-- Wait for all tasks to complete
function async.all(tasks: {Task<any>}): Task<{any}>
    local task: Task<{any}> = {
        co = coroutine.create(function()
            local results = {}
            
            -- Poll all tasks until done
            while true do
                local all_done = true
                for i, t in ipairs(tasks) do
                    if async.pending(t) then
                        all_done = false
                        local yield_val = async.poll(t)
                        if yield_val then
                            -- Re-yield to host
                            local resp = coroutine.yield(yield_val)
                            async.poll(t, resp)
                        end
                    end
                    if async.done(t) then
                        results[i] = t.result
                    end
                end
                if all_done then break end
            end
            
            return results
        end),
        state = "pending",
        result = nil,
        error = nil,
    }
    return task
end

-- Wait for first task to complete
function async.race(tasks: {Task<any>}): Task<any>
    local task: Task<any> = {
        co = coroutine.create(function()
            while true do
                for _, t in ipairs(tasks) do
                    if async.pending(t) then
                        local yield_val = async.poll(t)
                        if yield_val then
                            local resp = coroutine.yield(yield_val)
                            async.poll(t, resp)
                        end
                    end
                    if async.done(t) then
                        return t.result
                    end
                end
            end
        end),
        state = "pending",
        result = nil,
        error = nil,
    }
    return task
end

-- ============================================================================
-- Synchronous wrappers (for compatibility)
-- ============================================================================

-- Run an async function synchronously (blocking)
-- This is for compatibility when host doesn't support async
function async.run_sync<T>(task: Task<T>): (T?, string?)
    while async.pending(task) do
        local yield_val = async.poll(task)
        if yield_val then
            -- Handle yield synchronously
            local result: any = nil
            if yield_val.type == "http" and yield_val.request then
                result = __builtin.http(yield_val.request)
            elseif yield_val.type == "sleep" and yield_val.duration then
                -- No-op for sync mode (can't actually sleep in pure Luau)
                result = nil
            elseif yield_val.type == "kvs_get" and yield_val.key then
                result = __builtin.kvs_get(yield_val.key)
            elseif yield_val.type == "kvs_set" and yield_val.key then
                __builtin.kvs_set(yield_val.key, yield_val.value)
                result = true
            elseif yield_val.type == "background_task" and yield_val.task then
                -- Run background task synchronously
                async.run_sync(yield_val.task)
                result = nil
            end
            async.poll(task, result)
        end
    end
    return async.result(task)
end

return async
