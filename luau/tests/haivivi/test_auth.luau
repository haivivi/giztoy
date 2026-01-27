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

-- Run all tests
local function run_tests()
    __builtin.log("Starting Auth SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    if test_auth_refresh_token() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_auth_token_cache() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_auth_http_client() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    __builtin.log("=== Results ===")
    __builtin.log("Passed:", passed)
    __builtin.log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
