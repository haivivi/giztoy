-- Generate with context: builds ModelContext from history and calls generate
-- First, add some history
rt:history_append({ role = "user", content = "What is 2+2?" })
rt:history_append({ role = "assistant", content = "2+2 equals 4." })
rt:history_append({ role = "user", content = "And 3+3?" })

-- Get history messages
local msgs, err = rt:history_recent(10)
if err then
    rt:output(nil, "history_recent: " .. err)
    return
end

-- Build model context with system prompt and history
local mctx = {
    system = "You are a helpful math tutor.",
    messages = msgs
}

-- Generate with context
local result, err = rt:generate_with_context("qwen-turbo", mctx)
if err then
    rt:output(nil, "generate: " .. err)
    return
end

rt:output({
    response = result,
    msg_count = #msgs
}, nil)
