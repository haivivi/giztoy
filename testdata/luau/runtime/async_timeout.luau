-- async_timeout.luau: Timeout and cancellation examples
-- Demonstrates: rt:timeout(), :cancel(), :await()

local args = rt:input() or {}

-- Example 1: Basic timeout that fires
local function basic_timeout()
    local handle = rt:timeout(100)  -- 100ms timeout
    local result = handle:await()
    
    -- result.cancelled will be false since timeout fired normally
    return { cancelled = result.cancelled, fired = not result.cancelled }
end

-- Example 2: Cancel a timeout before it fires
local function cancel_timeout()
    local handle = rt:timeout(1000)  -- 1 second timeout
    
    -- Cancel immediately
    local was_cancelled = handle:cancel()
    
    -- Await will return immediately with cancelled = true
    local result = handle:await()
    
    return { 
        was_cancelled = was_cancelled, 
        result_cancelled = result.cancelled 
    }
end

-- Example 3: Check if timeout is ready without blocking
local function check_ready()
    local handle = rt:timeout(50)  -- 50ms timeout
    
    -- Check immediately - should not be ready
    local ready_before = handle:is_ready()
    
    -- Wait for it
    rt:sleep(100):await()
    
    -- Now it should be ready
    local ready_after = handle:is_ready()
    
    -- Await to clean up
    handle:await()
    
    return { ready_before = ready_before, ready_after = ready_after }
end

-- Example 4: Multiple timeouts with different durations
local function multiple_timeouts()
    local t1 = rt:timeout(30)
    local t2 = rt:timeout(20)
    local t3 = rt:timeout(10)
    
    -- Wait for all (they should complete in order: t3, t2, t1)
    local r1 = t1:await()
    local r2 = t2:await()
    local r3 = t3:await()
    
    local all_fired = (not r1.cancelled) and (not r2.cancelled) and (not r3.cancelled)
    return { all_fired = all_fired }
end

-- Example 5: Timeout as a deadline pattern
local function deadline_pattern()
    -- Create a 200ms deadline
    local deadline = rt:timeout(200)
    
    -- Simulate some work with multiple short sleeps
    for i = 1, 3 do
        if deadline:is_ready() then
            return { completed = false, iterations = i - 1 }
        end
        rt:sleep(50):await()
    end
    
    -- Cancel the deadline since we finished
    deadline:cancel()
    deadline:await()
    
    return { completed = true, iterations = 3 }
end

-- Run examples based on input
local example = args.example or "basic"

if example == "basic" then
    rt:output(basic_timeout(), nil)
elseif example == "cancel" then
    rt:output(cancel_timeout(), nil)
elseif example == "ready" then
    rt:output(check_ready(), nil)
elseif example == "multiple" then
    rt:output(multiple_timeouts(), nil)
elseif example == "deadline" then
    rt:output(deadline_pattern(), nil)
else
    rt:output(nil, "unknown example: " .. example)
end
