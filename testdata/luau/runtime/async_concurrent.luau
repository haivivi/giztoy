-- async_concurrent.luau: Concurrent operation patterns
-- Demonstrates: Parallel execution, fan-out/fan-in, pipelines

local args = rt:input() or {}

-- Example 1: Fan-out pattern - start many operations in parallel
local function fan_out()
    local count = args.count or 5
    local promises = {}
    
    -- Start all operations concurrently
    for i = 1, count do
        promises[i] = rt:sleep(10)  -- Each sleeps 10ms
    end
    
    -- Wait for all to complete
    for i = 1, count do
        promises[i]:await()
    end
    
    return { completed = count }
end

-- Example 2: Staggered start - start operations with delays
local function staggered_start()
    local results = {}
    
    -- Start first operation
    local p1 = rt:sleep(30)
    
    -- Wait a bit, then start second
    rt:sleep(10):await()
    local p2 = rt:sleep(30)
    
    -- Wait a bit more, then start third
    rt:sleep(10):await()
    local p3 = rt:sleep(30)
    
    -- All should complete around the same time
    p1:await()
    p2:await()
    p3:await()
    
    return { pattern = "staggered" }
end

-- Example 3: Timeout with work - deadline pattern
local function deadline_with_work()
    -- Set a 100ms deadline
    local deadline = rt:timeout(100)
    local work_done = 0
    
    -- Do work until deadline
    while not deadline:is_ready() do
        rt:sleep(10):await()
        work_done = work_done + 1
        
        -- Safety limit
        if work_done >= 20 then
            break
        end
    end
    
    -- Cancel or await deadline to clean up
    deadline:cancel()
    deadline:await()
    
    return { work_iterations = work_done }
end

-- Example 4: Parallel HTTP with timeout
local function parallel_http_with_deadline()
    local url = args.url
    if not url then
        return { skipped = true }
    end
    
    -- Overall deadline for all requests
    local deadline = rt:timeout(3000)  -- 3 second deadline
    
    -- Start requests
    local requests = {}
    for i = 1, 3 do
        requests[i] = rt:http({ url = url, timeout = 2000 })
    end
    
    -- Collect results
    local success = 0
    for i = 1, 3 do
        if deadline:is_ready() then
            break  -- Deadline exceeded
        end
        local resp = requests[i]:await()
        if resp.status == 200 then
            success = success + 1
        end
    end
    
    -- Clean up deadline
    deadline:cancel()
    deadline:await()
    
    return { success_count = success }
end

-- Example 5: Sequential vs Parallel comparison
local function sequential_vs_parallel()
    -- Sequential: 3 x 20ms = ~60ms
    local seq_start = os.clock()
    rt:sleep(20):await()
    rt:sleep(20):await()
    rt:sleep(20):await()
    local seq_time = (os.clock() - seq_start) * 1000
    
    -- Parallel: all 3 x 20ms run concurrently = ~20ms
    local par_start = os.clock()
    local p1 = rt:sleep(20)
    local p2 = rt:sleep(20)
    local p3 = rt:sleep(20)
    p1:await()
    p2:await()
    p3:await()
    local par_time = (os.clock() - par_start) * 1000
    
    -- Parallel should be significantly faster
    return { 
        sequential_ms = math.floor(seq_time),
        parallel_ms = math.floor(par_time),
        parallel_faster = par_time < seq_time
    }
end

-- Example 6: Mixed operations
local function mixed_operations()
    -- Start different types of async operations (all return non-nil results)
    local t1 = rt:timeout(30)
    local t2 = rt:timeout(50)
    local t3 = rt:timeout(20)
    
    -- Wait for all using await_all
    local results = rt:await_all(t1, t2, t3):await()
    
    local count = 0
    for k, v in pairs(results) do
        if v ~= nil then
            count = count + 1
        end
    end
    
    return { result_count = count }
end

-- Run examples based on input
local example = args.example or "fanout"

if example == "fanout" then
    rt:output(fan_out(), nil)
elseif example == "staggered" then
    rt:output(staggered_start(), nil)
elseif example == "deadline" then
    rt:output(deadline_with_work(), nil)
elseif example == "parallel_http" then
    rt:output(parallel_http_with_deadline(), nil)
elseif example == "compare" then
    rt:output(sequential_vs_parallel(), nil)
elseif example == "mixed" then
    rt:output(mixed_operations(), nil)
else
    rt:output(nil, "unknown example: " .. example)
end
