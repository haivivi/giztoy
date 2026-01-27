-- Coroutine scheduler simulation test
-- Tests: round-robin scheduling, task management

-- Simple task scheduler
local Scheduler = {}
Scheduler.__index = Scheduler

function Scheduler.new()
    return setmetatable({
        tasks = {},
        results = {},
    }, Scheduler)
end

function Scheduler:spawn(name, fn)
    local task = {
        name = name,
        co = coroutine.create(fn),
    }
    table.insert(self.tasks, task)
end

function Scheduler:run()
    while #self.tasks > 0 do
        local remaining = {}
        
        for _, task in ipairs(self.tasks) do
            if coroutine.status(task.co) ~= "dead" then
                local ok, result = coroutine.resume(task.co)
                if ok and result then
                    table.insert(self.results, task.name .. ": " .. tostring(result))
                end
                
                if coroutine.status(task.co) ~= "dead" then
                    table.insert(remaining, task)
                end
            end
        end
        
        self.tasks = remaining
    end
    
    return self.results
end

-- Test scheduler with multiple tasks
local sched = Scheduler.new()

sched:spawn("task-A", function()
    for i = 1, 3 do
        coroutine.yield("A" .. i)
    end
end)

sched:spawn("task-B", function()
    for i = 1, 2 do
        coroutine.yield("B" .. i)
    end
end)

sched:spawn("task-C", function()
    coroutine.yield("C1")
end)

local results = sched:run()

-- Should interleave: A1, B1, C1, A2, B2, A3
assert(#results == 6, "Expected 6 results, got " .. #results)

-- Verify all tasks completed
local a_count, b_count, c_count = 0, 0, 0
for _, r in ipairs(results) do
    if string.find(r, "task%-A") then a_count = a_count + 1 end
    if string.find(r, "task%-B") then b_count = b_count + 1 end
    if string.find(r, "task%-C") then c_count = c_count + 1 end
end

assert(a_count == 3, "Task A should have 3 yields")
assert(b_count == 2, "Task B should have 2 yields")
assert(c_count == 1, "Task C should have 1 yield")

-- Test priority-like behavior (add more tasks dynamically)
local sched2 = Scheduler.new()
local dynamic_spawned = false

sched2:spawn("main", function()
    coroutine.yield("main-1")
    -- In real scenario, could spawn new tasks here
    coroutine.yield("main-2")
end)

sched2:spawn("worker", function()
    for i = 1, 4 do
        coroutine.yield("work-" .. i)
    end
end)

local results2 = sched2:run()
assert(#results2 == 6, "Expected 6 results from sched2")

print("coroutine_scheduler: PASS")
