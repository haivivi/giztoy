-- async_combinators.luau: Promise combinator examples
-- Demonstrates: rt:await_all(), rt:await_any()

local args = rt:input() or {}

-- Example 1: await_all - wait for all promises to complete
local function await_all_example()
    -- Create multiple timeout promises (they return {cancelled=false} which is not nil)
    local p1 = rt:timeout(30)
    local p2 = rt:timeout(20)
    local p3 = rt:timeout(10)
    
    -- Wait for all to complete
    local results = rt:await_all(p1, p2, p3):await()
    
    -- Count results
    local count = 0
    for k, v in pairs(results) do
        if v ~= nil then
            count = count + 1
        end
    end
    
    return { result_count = count }
end

-- Example 2: await_any - wait for first promise to complete
local function await_any_example()
    -- Create timeout promises with different durations
    local p1 = rt:timeout(100)  -- slowest
    local p2 = rt:timeout(50)   -- medium
    local p3 = rt:timeout(10)   -- fastest (should win)
    
    -- Wait for first to complete
    local result, err = rt:await_any(p1, p2, p3):await()
    
    return { has_result = true, has_error = err ~= nil }
end

-- Example 3: await_all with HTTP requests
local function await_all_http()
    local url = args.url
    if not url then
        return { skipped = true }
    end
    
    -- Start multiple HTTP requests
    local p1 = rt:http({ url = url, timeout = 5000 })
    local p2 = rt:http({ url = url, timeout = 5000 })
    
    -- Wait for all
    local results = rt:await_all(p1, p2):await()
    
    local success_count = 0
    for _, r in pairs(results) do
        if r and r.status and r.status == 200 then
            success_count = success_count + 1
        end
    end
    
    return { success_count = success_count }
end

-- Example 4: await_any as race condition
local function race_example()
    -- Timeout acts as a deadline
    local timeout_promise = rt:timeout(100)
    local work_promise = rt:sleep(50)  -- Simulated work
    
    -- Race between work and timeout
    local result, err = rt:await_any(work_promise, timeout_promise):await()
    
    -- If work finished first, timeout result will have cancelled = false
    -- since it wasn't the winner
    return { completed = true }
end

-- Example 5: Empty combinators
local function empty_combinators()
    -- await_all with no arguments returns empty array
    local all_result = rt:await_all():await()
    
    -- await_any with no arguments returns nil
    local any_result = rt:await_any():await()
    
    local all_count = 0
    for _ in pairs(all_result) do
        all_count = all_count + 1
    end
    
    return { 
        all_empty = all_count == 0, 
        any_nil = any_result == nil 
    }
end

-- Run examples based on input
local example = args.example or "all"

if example == "all" then
    rt:output(await_all_example(), nil)
elseif example == "any" then
    rt:output(await_any_example(), nil)
elseif example == "http" then
    rt:output(await_all_http(), nil)
elseif example == "race" then
    rt:output(race_example(), nil)
elseif example == "empty" then
    rt:output(empty_combinators(), nil)
else
    rt:output(nil, "unknown example: " .. example)
end
