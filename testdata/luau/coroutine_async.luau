-- Coroutine async pattern test
-- Tests: async task pattern, yield/resume for I/O

-- Simulated async module (like haivivi/async)
local async = {}

-- Task type
function async.create(fn)
    return {
        co = coroutine.create(fn),
        state = "pending",
        result = nil,
        error = nil,
    }
end

function async.pending(task)
    return task.state == "pending" or task.state == "running"
end

function async.poll(task, resume_value)
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
    
    if coroutine.status(task.co) == "dead" then
        task.state = "completed"
        task.result = result
        return nil
    end
    
    return result  -- Yield value
end

function async.result(task)
    if task.state == "failed" then
        return nil, task.error
    end
    if task.state == "completed" then
        return task.result, nil
    end
    return nil, "not completed"
end

-- ============================================================================
-- Test 1: Basic async pattern
-- ============================================================================

local function test_basic_async()
    local task = async.create(function()
        local x = coroutine.yield("request_1")
        local y = coroutine.yield("request_2")
        return x + y
    end)
    
    -- First poll starts the task
    local yield1 = async.poll(task)
    assert(yield1 == "request_1", "First yield should be request_1")
    assert(async.pending(task), "Task should be pending")
    
    -- Resume with first response
    local yield2 = async.poll(task, 10)
    assert(yield2 == "request_2", "Second yield should be request_2")
    
    -- Resume with second response
    local yield3 = async.poll(task, 20)
    assert(yield3 == nil, "Should not yield after completion")
    
    local result, err = async.result(task)
    assert(result == 30, "Result should be 30")
    assert(err == nil, "No error")
end

test_basic_async()

-- ============================================================================
-- Test 2: Simulated HTTP async
-- ============================================================================

local function test_http_async()
    -- Simulate async HTTP task
    local function http_async(url)
        return async.create(function()
            -- Yield request to "host"
            local response = coroutine.yield({
                type = "http",
                url = url,
            })
            return response
        end)
    end
    
    local task = http_async("https://api.example.com/data")
    
    -- Poll to get the request
    local yield_val = async.poll(task)
    assert(yield_val.type == "http", "Should yield http request")
    assert(yield_val.url == "https://api.example.com/data", "URL should match")
    
    -- Simulate host completing HTTP and resuming
    local mock_response = { status = 200, body = '{"ok":true}' }
    async.poll(task, mock_response)
    
    local result, err = async.result(task)
    assert(result.status == 200, "Response status should be 200")
    assert(result.body == '{"ok":true}', "Response body should match")
end

test_http_async()

-- ============================================================================
-- Test 3: Auth token refresh async pattern
-- ============================================================================

local function test_auth_refresh_async()
    -- Simulate auth client with async refresh
    local auth = {
        token = nil,
        needs_refresh = function(self)
            return self.token == nil
        end,
    }
    
    function auth:get_token_async()
        local self_ref = self
        return async.create(function()
            if not self_ref:needs_refresh() then
                return self_ref.token
            end
            
            -- Yield HTTP request for token refresh
            local resp = coroutine.yield({
                type = "http",
                url = "/auth/refresh",
                method = "POST",
            })
            
            if resp.status == 200 then
                self_ref.token = resp.token
                return resp.token
            end
            error("refresh failed: " .. resp.status)
        end)
    end
    
    -- First call - should need refresh
    local task = auth:get_token_async()
    
    local yield_val = async.poll(task)
    assert(yield_val.type == "http", "Should yield http request")
    assert(yield_val.url == "/auth/refresh", "Should request refresh endpoint")
    
    -- Simulate successful token response
    async.poll(task, { status = 200, token = "new_token_123" })
    
    local result, err = async.result(task)
    assert(result == "new_token_123", "Should return new token")
    assert(auth.token == "new_token_123", "Token should be cached")
    
    -- Second call - should use cached token (no HTTP)
    local task2 = auth:get_token_async()
    local yield_val2 = async.poll(task2)
    assert(yield_val2 == nil, "Should not yield (cached)")
    
    local result2, _ = async.result(task2)
    assert(result2 == "new_token_123", "Should return cached token")
end

test_auth_refresh_async()

-- ============================================================================
-- Test 4: Multiple concurrent tasks (simulated)
-- ============================================================================

local function test_concurrent_tasks()
    local task1 = async.create(function()
        local a = coroutine.yield("task1_req")
        return "task1:" .. a
    end)
    
    local task2 = async.create(function()
        local b = coroutine.yield("task2_req")
        return "task2:" .. b
    end)
    
    -- Start both tasks
    local y1 = async.poll(task1)
    local y2 = async.poll(task2)
    
    assert(y1 == "task1_req", "Task1 should yield")
    assert(y2 == "task2_req", "Task2 should yield")
    
    -- Complete in different order (task2 first)
    async.poll(task2, "resp2")
    async.poll(task1, "resp1")
    
    local r1, _ = async.result(task1)
    local r2, _ = async.result(task2)
    
    assert(r1 == "task1:resp1", "Task1 result")
    assert(r2 == "task2:resp2", "Task2 result")
end

test_concurrent_tasks()

-- ============================================================================
-- Test 5: Task error handling
-- ============================================================================

local function test_task_error()
    local task = async.create(function()
        local x = coroutine.yield("request")
        if x < 0 then
            error("negative value")
        end
        return x * 2
    end)
    
    async.poll(task)
    async.poll(task, -5)  -- Resume with negative value
    
    local result, err = async.result(task)
    assert(result == nil, "Result should be nil on error")
    assert(err ~= nil, "Should have error")
    assert(string.find(err, "negative"), "Error should mention negative")
end

test_task_error()

print("coroutine_async: PASS")
