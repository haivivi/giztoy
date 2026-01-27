-- Token refresh with coroutine test
-- Tests: stale/expired token handling, refresh task deduplication

-- ============================================================================
-- Mock async module (simplified)
-- ============================================================================

local async = {}

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
    return result
end

function async.result(task)
    if task.state == "failed" then return nil, task.error end
    if task.state == "completed" then return task.result, nil end
    return nil, "not completed"
end

-- ============================================================================
-- Mock Auth Client
-- ============================================================================

local function create_mock_auth()
    local auth = {
        token = nil,
        token_expire_at = 0,
        _refresh_task = nil,
        _refresh_count = 0,  -- Track how many refreshes were performed
    }
    
    -- Token states
    local STALE_THRESHOLD = 900  -- 15 minutes
    
    function auth:token_state()
        if not self.token then return "none" end
        local now = os.time()
        local remaining = self.token_expire_at - now
        if remaining <= 0 then return "expired" end
        if remaining <= STALE_THRESHOLD then return "stale" end
        return "valid"
    end
    
    function auth:is_refreshing()
        return self._refresh_task ~= nil and async.pending(self._refresh_task)
    end
    
    -- Create refresh task
    function auth:_create_refresh_task()
        local self_ref = self
        return {
            co = coroutine.create(function()
                -- Yield HTTP request
                local resp = coroutine.yield({ type = "http", url = "/refresh" })
                
                self_ref._refresh_count = self_ref._refresh_count + 1
                self_ref._refresh_task = nil
                
                if resp.error then
                    error(resp.error)
                end
                
                self_ref.token = resp.token
                self_ref.token_expire_at = resp.expire_at
                return { token = resp.token, expire_at = resp.expire_at }
            end),
            state = "pending",
            result = nil,
            error = nil,
        }
    end
    
    -- Async get token with deduplication
    function auth:get_token_async()
        local self_ref = self
        return {
            co = coroutine.create(function()
                local state = self_ref:token_state()
                
                if state == "valid" then
                    return self_ref.token
                end
                
                if state == "stale" then
                    -- Start background refresh if not running
                    if not self_ref:is_refreshing() then
                        self_ref._refresh_task = self_ref:_create_refresh_task()
                    end
                    -- Return current stale token (don't wait)
                    return self_ref.token
                end
                
                -- expired or none: must wait for refresh
                local refresh_task
                if self_ref:is_refreshing() then
                    refresh_task = self_ref._refresh_task
                else
                    refresh_task = self_ref:_create_refresh_task()
                    self_ref._refresh_task = refresh_task
                end
                
                -- Wait for refresh
                while async.pending(refresh_task) do
                    local yield_val = async.poll(refresh_task)
                    if yield_val then
                        local resp = coroutine.yield(yield_val)
                        async.poll(refresh_task, resp)
                    end
                end
                
                local session, err = async.result(refresh_task)
                if session then
                    return session.token
                end
                error(err or "refresh failed")
            end),
            state = "pending",
            result = nil,
            error = nil,
        }
    end
    
    return auth
end

-- ============================================================================
-- Test 1: Valid token - no refresh
-- ============================================================================

local function test_valid_token()
    local auth = create_mock_auth()
    auth.token = "valid_token"
    auth.token_expire_at = os.time() + 3600  -- 1 hour from now
    
    local task = auth:get_token_async()
    local yield_val = async.poll(task)
    
    -- Should complete immediately without yielding
    assert(yield_val == nil, "Valid token should not yield")
    
    local result, err = async.result(task)
    assert(result == "valid_token", "Should return valid token")
    assert(auth._refresh_count == 0, "Should not refresh")
end

test_valid_token()

-- ============================================================================
-- Test 2: Stale token - background refresh, return immediately
-- ============================================================================

local function test_stale_token()
    local auth = create_mock_auth()
    auth.token = "stale_token"
    auth.token_expire_at = os.time() + 600  -- 10 minutes (within 15 min threshold)
    
    local task = auth:get_token_async()
    local yield_val = async.poll(task)
    
    -- Should complete immediately (return stale token)
    assert(yield_val == nil, "Stale token should return immediately")
    
    local result, err = async.result(task)
    assert(result == "stale_token", "Should return stale token")
    
    -- Background refresh task should have been started
    assert(auth:is_refreshing(), "Should have started background refresh")
    
    -- Complete the background refresh
    local bg_yield = async.poll(auth._refresh_task)
    assert(bg_yield.type == "http", "Background should yield HTTP")
    
    async.poll(auth._refresh_task, { token = "new_token", expire_at = os.time() + 3600 })
    
    assert(auth.token == "new_token", "Token should be updated")
    assert(auth._refresh_count == 1, "Should have refreshed once")
end

test_stale_token()

-- ============================================================================
-- Test 3: Expired token - must wait for refresh
-- ============================================================================

local function test_expired_token()
    local auth = create_mock_auth()
    auth.token = "expired_token"
    auth.token_expire_at = os.time() - 100  -- Already expired
    
    local task = auth:get_token_async()
    local yield_val = async.poll(task)
    
    -- Should yield HTTP request (waiting for refresh)
    assert(yield_val ~= nil, "Expired token should yield")
    assert(yield_val.type == "http", "Should yield HTTP request")
    
    -- Simulate HTTP response
    async.poll(task, { token = "fresh_token", expire_at = os.time() + 3600 })
    
    local result, err = async.result(task)
    assert(result == "fresh_token", "Should return fresh token")
    assert(auth._refresh_count == 1, "Should have refreshed once")
end

test_expired_token()

-- ============================================================================
-- Test 4: No token - must refresh
-- ============================================================================

local function test_no_token()
    local auth = create_mock_auth()
    -- No token set
    
    local task = auth:get_token_async()
    local yield_val = async.poll(task)
    
    assert(yield_val ~= nil, "No token should yield")
    assert(yield_val.type == "http", "Should yield HTTP request")
    
    async.poll(task, { token = "new_token", expire_at = os.time() + 3600 })
    
    local result, err = async.result(task)
    assert(result == "new_token", "Should return new token")
end

test_no_token()

-- ============================================================================
-- Test 5: Multiple callers - refresh task is created once
-- ============================================================================

local function test_refresh_task_sharing()
    local auth = create_mock_auth()
    auth.token = "expired_token"
    auth.token_expire_at = os.time() - 100
    
    -- First caller
    local task1 = auth:get_token_async()
    local yield1 = async.poll(task1)
    assert(yield1 and yield1.type == "http", "Task1 should yield HTTP")
    
    -- At this point, auth._refresh_task is set
    assert(auth:is_refreshing(), "Should have refresh in progress")
    
    -- Remember refresh count before second call
    local count_before = auth._refresh_count
    
    -- Complete first refresh
    async.poll(task1, { token = "fresh_token", expire_at = os.time() + 3600 })
    
    local result1, err1 = async.result(task1)
    assert(result1 == "fresh_token", "Task1 should get token")
    assert(auth._refresh_count == count_before + 1, "Should have refreshed once")
    
    -- Now token is valid, second caller should not trigger refresh
    local task2 = auth:get_token_async()
    local yield2 = async.poll(task2)
    
    -- Should complete immediately (token is now valid)
    assert(yield2 == nil, "Task2 should not yield (token is valid)")
    
    local result2, err2 = async.result(task2)
    assert(result2 == "fresh_token", "Task2 should get same token")
    
    -- No additional refresh
    assert(auth._refresh_count == count_before + 1, "Should not refresh again")
end

test_refresh_task_sharing()

-- ============================================================================
-- Test 6: Refresh error handling
-- ============================================================================

local function test_refresh_error()
    local auth = create_mock_auth()
    auth.token = nil
    
    local task = auth:get_token_async()
    local yield_val = async.poll(task)
    
    -- Simulate HTTP error
    async.poll(task, { error = "network error" })
    
    local result, err = async.result(task)
    assert(result == nil, "Should not return token on error")
    assert(err ~= nil, "Should have error")
end

test_refresh_error()

print("coroutine_token_refresh: PASS")
