-- async_http.luau: HTTP request examples with timeout support
-- Demonstrates: rt:http() with Promise, timeout configuration

local args = rt:input() or {}
local url = args.url or "https://httpbin.org/get"

-- Example 1: Basic HTTP GET request
local function basic_get()
    local resp = rt:http({ url = url }):await()
    if resp.err then
        return nil, resp.err
    end
    return { status = resp.status, body_len = #resp.body }
end

-- Example 2: HTTP POST with headers and body
local function post_with_body()
    local resp = rt:http({
        url = url,
        method = "POST",
        headers = {
            ["Content-Type"] = "application/json",
            ["X-Custom-Header"] = "test-value"
        },
        body = '{"name":"test","value":123}'
    }):await()
    
    if resp.err then
        return nil, resp.err
    end
    return { status = resp.status }
end

-- Example 3: HTTP with custom timeout (5 seconds)
local function with_timeout()
    local resp = rt:http({
        url = url,
        timeout = 5000  -- 5 seconds in milliseconds
    }):await()
    
    if resp.err then
        return nil, resp.err
    end
    return { status = resp.status }
end

-- Example 4: Multiple concurrent HTTP requests
local function concurrent_requests()
    -- Start multiple requests concurrently
    local p1 = rt:http({ url = url, timeout = 5000 })
    local p2 = rt:http({ url = url, timeout = 5000 })
    local p3 = rt:http({ url = url, timeout = 5000 })
    
    -- Wait for all to complete
    local r1 = p1:await()
    local r2 = p2:await()
    local r3 = p3:await()
    
    local success_count = 0
    if r1.status and r1.status > 0 then success_count = success_count + 1 end
    if r2.status and r2.status > 0 then success_count = success_count + 1 end
    if r3.status and r3.status > 0 then success_count = success_count + 1 end
    
    return { success_count = success_count }
end

-- Run examples based on input
local example = args.example or "basic"

if example == "basic" then
    local result, err = basic_get()
    rt:output(result, err)
elseif example == "post" then
    local result, err = post_with_body()
    rt:output(result, err)
elseif example == "timeout" then
    local result, err = with_timeout()
    rt:output(result, err)
elseif example == "concurrent" then
    local result, err = concurrent_requests()
    rt:output(result, err)
else
    rt:output(nil, "unknown example: " .. example)
end
