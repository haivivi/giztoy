-- History revert: tests history_revert operation
-- Append messages
rt:history_append({ role = "user", content = "First" })
rt:history_append({ role = "assistant", content = "Response" })
rt:history_append({ role = "user", content = "Second" })

-- Revert last round
local err = rt:history_revert()
if err then
    rt:output(nil, "revert: " .. err)
    return
end

local msgs, _ = rt:history_recent(10)
rt:output({ count = #msgs }, nil)
