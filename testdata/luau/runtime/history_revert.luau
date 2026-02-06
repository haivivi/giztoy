-- History Revert: tests reverting history to remove recent messages
-- MockState.Revert removes everything from the last user message (inclusive)

-- Add some messages (history_append expects a table)
rt:history_append({ role = "user", content = "Message 1" })
rt:history_append({ role = "assistant", content = "Response 1" })
rt:history_append({ role = "user", content = "Message 2" })

-- Revert (removes from last user message onwards, leaving only Message 1 + Response 1)
rt:history_revert()

-- Get updated history
local history, err = rt:history_recent(10)
local count = 0
if history then
    count = #history
end

rt:output({
    count = count
}, nil)
