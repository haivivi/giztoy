-- Coroutine producer-consumer pattern
-- Tests: coroutines, yield, resume, tables

function producer()
    for i = 1, 5 do
        coroutine.yield(i * 10)
    end
end

co = coroutine.create(producer)
results = {}

while true do
    local status, value = coroutine.resume(co)
    if not status or value == nil then break end
    table.insert(results, value)
end

total = 0
for _, v in ipairs(results) do
    total = total + v
end
-- Expected: total = 150 (10 + 20 + 30 + 40 + 50)
