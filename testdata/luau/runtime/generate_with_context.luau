-- Generate with model context: tests building a full model context for LLM
-- This script builds a model context with system prompt and messages,
-- then calls generate_with_context and returns the result
-- rt:generate_with_context(model, mctx) returns string, err

-- Build model context with messages
local mctx = {
    system = "You are a helpful math tutor.",
    messages = {
        { role = "user", content = "What is 3+3?" },
        { role = "assistant", content = "3+3 equals 6." },
        { role = "user", content = "Can you explain that?" }
    }
}

-- Call generate_with_context - returns the full response string directly
local response, err = rt:generate_with_context("test-model", mctx)
if err then
    rt:output(nil, err)
    return
end

rt:output({ response = response }, nil)
