-- Generate: calls LLM generate with model and prompt from input
local args = rt:input()
local result, err = rt:generate(args.model, args.prompt)
if err then
    rt:output(nil, err)
    return
end
rt:output({ text = result }, nil)
