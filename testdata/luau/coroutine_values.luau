-- Coroutine value passing test
-- Tests: yield/resume value passing, function arguments

-- Test passing values through yield/resume
local co = coroutine.create(function(a, b)
    local sum = a + b
    local multiplier = coroutine.yield(sum)  -- yield sum, receive multiplier
    return sum * multiplier
end)

-- First resume: pass arguments (10, 20), get yield value (30)
local ok1, result1 = coroutine.resume(co, 10, 20)
assert(ok1, "First resume should succeed")
assert(result1 == 30, "Expected yield value 30, got " .. tostring(result1))

-- Second resume: pass multiplier (3), get return value (90)
local ok2, result2 = coroutine.resume(co, 3)
assert(ok2, "Second resume should succeed")
assert(result2 == 90, "Expected return value 90, got " .. tostring(result2))

-- Test multiple yields with values
local values = {}
local co2 = coroutine.create(function()
    for i = 1, 4 do
        local input = coroutine.yield(i * 100)
        table.insert(values, input)
    end
    return "done"
end)

local outputs = {}
local input = 0
while true do
    local ok, val = coroutine.resume(co2, input)
    if not ok or val == "done" or val == nil then break end
    table.insert(outputs, val)
    input = input + 1
end

-- Expected outputs: 100, 200, 300, 400
assert(#outputs == 4, "Expected 4 outputs, got " .. #outputs)
assert(outputs[1] == 100, "First output should be 100")
assert(outputs[4] == 400, "Fourth output should be 400")

-- Expected values (inputs received after each yield): 1, 2, 3, 4
-- Loop runs 4 times, each yield receives the next input value
assert(#values == 4, "Expected 4 input values, got " .. #values)
assert(values[1] == 1, "First input should be 1")
assert(values[4] == 4, "Fourth input should be 4")

print("coroutine_values: PASS")
