-- Nested coroutines test
-- Tests: coroutine calling another coroutine

local log = {}

-- Inner coroutine: simple generator
local function make_generator(prefix, count)
    return coroutine.create(function()
        for i = 1, count do
            table.insert(log, prefix .. "-yield-" .. i)
            coroutine.yield(i)
        end
        table.insert(log, prefix .. "-done")
    end)
end

-- Outer coroutine drives two inner coroutines
local outer = coroutine.create(function()
    local gen_a = make_generator("A", 3)
    local gen_b = make_generator("B", 2)
    
    table.insert(log, "outer-start")
    
    -- Interleave A and B
    while coroutine.status(gen_a) ~= "dead" or coroutine.status(gen_b) ~= "dead" do
        if coroutine.status(gen_a) ~= "dead" then
            local ok, val = coroutine.resume(gen_a)
            if ok and val then
                table.insert(log, "outer-got-A-" .. val)
            end
        end
        
        if coroutine.status(gen_b) ~= "dead" then
            local ok, val = coroutine.resume(gen_b)
            if ok and val then
                table.insert(log, "outer-got-B-" .. val)
            end
        end
        
        coroutine.yield()  -- Yield control back to main
    end
    
    table.insert(log, "outer-end")
end)

-- Drive outer coroutine
local iterations = 0
while coroutine.status(outer) ~= "dead" do
    coroutine.resume(outer)
    iterations = iterations + 1
end

-- Verify execution order
assert(log[1] == "outer-start", "Should start with outer-start")
assert(log[#log] == "outer-end", "Should end with outer-end")
assert(iterations > 1, "Should have multiple iterations")

-- Count specific events
local a_yields = 0
local b_yields = 0
for _, entry in ipairs(log) do
    if string.find(entry, "A%-yield") then a_yields = a_yields + 1 end
    if string.find(entry, "B%-yield") then b_yields = b_yields + 1 end
end

assert(a_yields == 3, "Expected 3 A yields, got " .. a_yields)
assert(b_yields == 2, "Expected 2 B yields, got " .. b_yields)

print("coroutine_nested: PASS")
