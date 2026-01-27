-- Coroutine wrap test
-- Tests: coroutine.wrap as iterator

-- Basic wrap usage
local iter = coroutine.wrap(function()
    coroutine.yield(1)
    coroutine.yield(2)
    coroutine.yield(3)
end)

assert(iter() == 1, "First call should return 1")
assert(iter() == 2, "Second call should return 2")
assert(iter() == 3, "Third call should return 3")
assert(iter() == nil, "Fourth call should return nil")

-- Wrap with arguments
local counter = coroutine.wrap(function(start, step)
    local current = start
    while true do
        local reset = coroutine.yield(current)
        if reset then
            current = reset
        else
            current = current + step
        end
    end
end)

assert(counter(10, 5) == 10, "Initial value should be 10")
assert(counter() == 15, "After step should be 15")
assert(counter() == 20, "After step should be 20")
assert(counter(100) == 100, "After reset should be 100")
assert(counter() == 105, "After step should be 105")

-- Using wrap as iterator in for loop
local range = function(from, to)
    return coroutine.wrap(function()
        for i = from, to do
            coroutine.yield(i)
        end
    end)
end

local sum = 0
for n in range(1, 10) do
    sum = sum + n
end

assert(sum == 55, "Sum 1..10 should be 55, got " .. sum)

-- Fibonacci generator with wrap
local fib = coroutine.wrap(function()
    local a, b = 0, 1
    while true do
        coroutine.yield(a)
        a, b = b, a + b
    end
end)

local fibs = {}
for i = 1, 10 do
    table.insert(fibs, fib())
end

-- First 10 fibonacci: 0, 1, 1, 2, 3, 5, 8, 13, 21, 34
assert(fibs[1] == 0, "fib[1] should be 0")
assert(fibs[5] == 3, "fib[5] should be 3")
assert(fibs[10] == 34, "fib[10] should be 34")

print("coroutine_wrap: PASS")
