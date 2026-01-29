-- Memory: tests memory_summary, memory_set_summary, memory_query
-- Set summary
local err = rt:memory_set_summary("User likes coding in Go and Luau")
if err then
    rt:output(nil, "set summary: " .. err)
    return
end

-- Get summary
local summary, err = rt:memory_summary()
if err then
    rt:output(nil, "get summary: " .. err)
    return
end

-- Query memory (mock will return empty)
local segments, err = rt:memory_query("coding preferences")
if err then
    rt:output(nil, "query: " .. err)
    return
end

rt:output({
    summary = summary,
    segment_count = #segments
}, nil)
