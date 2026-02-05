-- Generate: calls LLM generate with model and prompt from input
-- rt:generate(model, prompt) returns string, err
local args = rt:input()
if not args then
    rt:output(nil, "no input provided")
    return
end

if not args.model then
    rt:output(nil, "model is required")
    return
end

local prompt = args.prompt or "Hello"

-- Generate returns the full text directly
local text, err = rt:generate(args.model, prompt)
if err then
    rt:output(nil, err)
    return
end

rt:output({ text = text }, nil)
