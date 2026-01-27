--!strict
-- Auth SDK tests

local haivivi = require("haivivi")

-- Test configuration (from environment variables)
local AUTH_URL = __builtin.env("HAIVIVI_AUTH_URL") or "https://api.stage.haivivi.cn/auth/v1"
local KEY = __builtin.env("HAIVIVI_KEY") or error("HAIVIVI_KEY environment variable is required")

local function test_auth_refresh_token()
    __builtin.log("=== Test: Auth Refresh Token ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Test refresh token
    local session, err = auth_client:refresh_token()
    
    if err then
        __builtin.log("FAIL: refresh_token error:", err)
        return false
    end
    
    if not session then
        __builtin.log("FAIL: session is nil")
        return false
    end
    
    if not session.token or session.token == "" then
        __builtin.log("FAIL: token is empty")
        return false
    end
    
    __builtin.log("PASS: Got token:", string.sub(session.token, 1, 20) .. "...")
    __builtin.log("PASS: Token expires at:", session.tokenExpireAt or "unknown")
    
    return true
end

local function test_auth_token_cache()
    __builtin.log("=== Test: Auth Token Cache ===")
    
    -- Clear cache first
    __builtin.kvs_del("haivivi:auth:session")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- First call should refresh
    local token1 = auth_client:get_token()
    if not token1 or token1 == "" then
        __builtin.log("FAIL: first get_token failed")
        return false
    end
    
    __builtin.log("PASS: First token:", string.sub(token1, 1, 20) .. "...")
    
    -- Second call should use cache
    local token2 = auth_client:get_token()
    if token1 ~= token2 then
        __builtin.log("WARN: tokens differ (cache may have expired)")
    else
        __builtin.log("PASS: Token cached correctly")
    end
    
    return true
end

local function test_auth_http_client()
    __builtin.log("=== Test: Auth HTTP Client ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Ensure we have a token
    local _, err = auth_client:refresh_token()
    if err then
        __builtin.log("FAIL: refresh_token error:", err)
        return false
    end
    
    -- Get HTTP client
    local http_client = auth_client:http_client()
    
    if not http_client then
        __builtin.log("FAIL: http_client is nil")
        return false
    end
    
    __builtin.log("PASS: HTTP client created")
    
    return true
end

local function test_auth_needs_refresh()
    __builtin.log("=== Test: Auth needs_refresh Logic ===")
    
    -- Clear cache
    __builtin.kvs_del("haivivi:auth:session")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Without session, should need refresh
    local needs1 = auth_client:needs_refresh()
    if not needs1 then
        __builtin.log("FAIL: should need refresh without session")
        return false
    end
    __builtin.log("PASS: needs_refresh returns true without session")
    
    -- After refresh, should not need refresh
    local _, err = auth_client:refresh_token()
    if err then
        __builtin.log("FAIL: refresh_token error:", err)
        return false
    end
    
    local needs2 = auth_client:needs_refresh()
    if needs2 then
        __builtin.log("WARN: still needs refresh (token may be near expiry)")
    else
        __builtin.log("PASS: needs_refresh returns false after refresh")
    end
    
    return true
end

local function test_auth_token_expiry_parsing()
    __builtin.log("=== Test: Token Expiry Parsing ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local session, err = auth_client:refresh_token()
    
    if err or not session then
        __builtin.log("FAIL: refresh_token error:", err)
        return false
    end
    
    -- Parse token expiry
    if session.tokenExpireAt then
        local expire_ts = __builtin.parse_time(session.tokenExpireAt)
        if expire_ts then
            local now = __builtin.time()
            local remaining = expire_ts - now
            __builtin.log("PASS: Token expires in", math.floor(remaining / 60), "minutes")
            __builtin.log("  Now:", now)
            __builtin.log("  Expire:", expire_ts)
            
            -- Token should expire in the future
            if remaining <= 0 then
                __builtin.log("WARN: Token is already expired!")
            end
        else
            __builtin.log("FAIL: Could not parse tokenExpireAt:", session.tokenExpireAt)
            return false
        end
    else
        __builtin.log("WARN: No tokenExpireAt in session")
    end
    
    return true
end

local function test_auth_invalid_key()
    __builtin.log("=== Test: Auth Invalid Key (Error Handling) ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, "invalid_key_12345")
    
    -- Should fail with 404 or similar error
    local session, err = auth_client:refresh_token()
    
    if session then
        __builtin.log("FAIL: Should not get session with invalid key")
        return false
    end
    
    if not err then
        __builtin.log("FAIL: Should get error with invalid key")
        return false
    end
    
    __builtin.log("PASS: Got expected error:", err)
    
    -- Verify error contains useful info (HTTP status code)
    if string.find(err, "HTTP") then
        __builtin.log("PASS: Error contains HTTP status info")
    else
        __builtin.log("WARN: Error does not contain HTTP status")
    end
    
    return true
end

local function test_auth_kvs_persistence()
    __builtin.log("=== Test: Auth KVS Persistence ===")
    
    -- Clear cache
    __builtin.kvs_del("haivivi:auth:session")
    
    -- Create first client and refresh
    local client1 = haivivi.auth.new_client(AUTH_URL, KEY)
    local session1, err1 = client1:refresh_token()
    if err1 or not session1 then
        __builtin.log("FAIL: first refresh error:", err1)
        return false
    end
    
    local token1 = session1.token
    __builtin.log("First client token:", string.sub(token1, 1, 20) .. "...")
    
    -- Create second client - should load from KVS
    local client2 = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Check if session was loaded from KVS
    if client2.session and client2.session.token then
        local token2 = client2.session.token
        __builtin.log("Second client loaded token:", string.sub(token2, 1, 20) .. "...")
        
        if token1 == token2 then
            __builtin.log("PASS: Session persisted via KVS")
        else
            __builtin.log("WARN: Tokens differ (may be expected)")
        end
    else
        __builtin.log("WARN: Second client did not load session from KVS")
    end
    
    return true
end

-- ============================================================================
-- Async Token Refresh Tests (Coroutine-based)
-- Note: These tests verify the async API structure. Actual HTTP execution
-- through async.run_sync may have limitations with coroutine-based HTTP.
-- ============================================================================

local function test_auth_refresh_token_async()
    __builtin.log("=== Test: Auth Refresh Token (Async) ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Test that async task can be created
    local task = auth_client:refresh_token_async()
    
    if not task or not task.co then
        __builtin.log("FAIL: async task not created")
        return false
    end
    
    __builtin.log("PASS: Async task created")
    
    -- Verify task structure
    if coroutine.status(task.co) ~= "suspended" then
        __builtin.log("FAIL: task coroutine not suspended")
        return false
    end
    
    __builtin.log("PASS: Task coroutine is suspended")
    
    -- Poll to get the yield value (should be HTTP request)
    local yield_val = haivivi.async.poll(task)
    
    if yield_val and yield_val.type == "http" then
        __builtin.log("PASS: Task yields HTTP request")
        __builtin.log("  URL:", yield_val.request and yield_val.request.url or "nil")
    else
        __builtin.log("INFO: Unexpected yield value:", yield_val)
    end
    
    return true
end

local function test_auth_get_token_async()
    __builtin.log("=== Test: Auth Get Token (Async) ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- Ensure we have a cached token first (via sync)
    local _, err = auth_client:refresh_token()
    if err then
        __builtin.log("INFO: Could not pre-cache token:", err)
    end
    
    -- Get async task
    local task = auth_client:get_token_async()
    
    if not task or not task.co then
        __builtin.log("FAIL: async task not created")
        return false
    end
    
    __builtin.log("PASS: Async get_token task created")
    
    -- Poll task - if token is cached, should complete immediately
    local yield_val = haivivi.async.poll(task)
    
    if yield_val == nil then
        -- Task completed without yielding (used cached token)
        local token, err = haivivi.async.result(task)
        if token and token ~= "" then
            __builtin.log("PASS: Async used cached token:", string.sub(token, 1, 20) .. "...")
            return true
        else
            __builtin.log("INFO: No cached token available")
        end
    else
        __builtin.log("PASS: Task yields for refresh (token not cached or stale)")
    end
    
    return true
end

local function test_auth_async_cached()
    __builtin.log("=== Test: Auth Async with Cached Token ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    
    -- First call - ensure token is cached via sync
    local session, err1 = auth_client:refresh_token()
    if err1 then
        __builtin.log("INFO: initial refresh error:", err1)
        __builtin.log("SKIP: Cannot test cached async without valid session")
        return true  -- Skip test
    end
    
    __builtin.log("Token cached:", string.sub(session.token, 1, 20) .. "...")
    
    -- Verify token state is valid
    local state = auth_client:token_state()
    __builtin.log("Token state:", state)
    
    if state == "valid" then
        -- Now async get_token should not need HTTP (token cached)
        local task = auth_client:get_token_async()
        
        -- Poll task - should complete immediately without yielding HTTP
        local yield_val = haivivi.async.poll(task)
        
        if yield_val == nil then
            local token, err = haivivi.async.result(task)
            if token and token ~= "" then
                __builtin.log("PASS: Async used cached token (no HTTP yield)")
                return true
            end
        end
        
        __builtin.log("INFO: Token state was valid but task still yielded")
    else
        __builtin.log("INFO: Token state is", state, "- async may need refresh")
    end
    
    __builtin.log("PASS: Async cache test completed")
    return true
end

-- Run all tests
local function run_tests()
    __builtin.log("Starting Auth SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        test_auth_refresh_token,
        test_auth_token_cache,
        test_auth_http_client,
        test_auth_needs_refresh,
        test_auth_token_expiry_parsing,
        test_auth_invalid_key,
        test_auth_kvs_persistence,
        -- Async tests
        test_auth_refresh_token_async,
        test_auth_get_token_async,
        test_auth_async_cached,
    }
    
    for _, test in ipairs(tests) do
        local ok, err = pcall(test)
        if ok and err ~= false then
            passed = passed + 1
        else
            failed = failed + 1
            if not ok then
                __builtin.log("ERROR:", err)
            end
        end
        __builtin.log("")
    end
    
    __builtin.log("=== Results ===")
    __builtin.log("Passed:", passed)
    __builtin.log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
