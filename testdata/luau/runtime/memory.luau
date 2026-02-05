-- Memory: tests memory get/set operations

-- Set memory summary
rt:memory_set_summary("User likes coding in Go and Luau")

-- Get memory summary
local summary, err = rt:memory_summary()

rt:output({
    summary = summary
}, nil)
