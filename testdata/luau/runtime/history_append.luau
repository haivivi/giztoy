-- History Append: tests appending messages to history

-- Append some messages (history_append expects a table with role and content)
rt:history_append({ role = "user", content = "Hello" })
rt:history_append({ role = "assistant", content = "Hi there!" })

-- Get history (history_recent returns messages, err)
local history, err = rt:history_recent(10)

-- Check results
local count = 0
local first_role = nil
local first_content = nil

if history then
    count = #history
    if count > 0 then
        first_role = history[1].role
        first_content = history[1].content
    end
end

rt:output({
    count = count,
    first_role = first_role,
    first_content = first_content
}, nil)
