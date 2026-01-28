-- History append: tests history_append and history_recent
-- Append some messages
local err = rt:history_append({ role = "user", content = "Hello" })
if err then
    rt:output(nil, "append user: " .. err)
    return
end

err = rt:history_append({ role = "assistant", content = "Hi there!" })
if err then
    rt:output(nil, "append assistant: " .. err)
    return
end

-- Get recent messages
local msgs, err = rt:history_recent(10)
if err then
    rt:output(nil, "recent: " .. err)
    return
end

rt:output({
    count = #msgs,
    first_role = msgs[1] and msgs[1].role or nil,
    first_content = msgs[1] and msgs[1].content or nil
}, nil)
