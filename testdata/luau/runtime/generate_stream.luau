-- Generate Stream: calls LLM generate with streaming API
-- For luau/runtime (CLI) which uses stream-based API
-- Usage: bazel run //go/cmd/luau -- -runtime tool -models $PWD/testdata/models testdata/luau/runtime/generate_stream.luau <<< '{"model":"deepseek/chat","prompt":"Hello"}'

local args = rt:input()
if not args then
    rt:output(nil, "no input provided")
    return
end

if not args.model then
    rt:output(nil, "model is required")
    return
end

rt:log("info", "Using model: " .. args.model)

-- Build model context
local mctx = {
    system = args.system or "You are a helpful assistant.",
    messages = args.messages or {{ role = "user", content = args.prompt or "Hello" }}
}

rt:log("info", "Calling generate...")

-- Generate returns a stream
local stream, err = rt:generate(args.model, mctx)
if err then
    rt:log("error", "Generate error: " .. tostring(err))
    rt:output(nil, err)
    return
end

rt:log("info", "Got stream, receiving chunks...")

-- Collect text from stream chunks
local text_parts = {}
local chunk_count = 0
while true do
    local chunk, recv_err = stream:recv()
    if recv_err then
        rt:log("info", "Recv done: " .. tostring(recv_err))
        -- "stream EOF" means done, other errors are real errors
        if recv_err ~= "stream EOF" then
            rt:output(nil, recv_err)
            return
        end
        break
    end
    if chunk == nil then
        rt:log("info", "Got nil chunk")
        break
    end
    chunk_count = chunk_count + 1
    rt:log("debug", "Chunk " .. chunk_count .. ": " .. tostring(chunk.part))
    -- Extract text from chunk.part
    -- Part can be: string, or table { type = "text", value = "..." }
    if chunk.part then
        if type(chunk.part) == "string" then
            table.insert(text_parts, chunk.part)
        elseif type(chunk.part) == "table" then
            if chunk.part.type == "text" and chunk.part.value then
                table.insert(text_parts, chunk.part.value)
            end
        end
    end
end

stream:close()

local result = table.concat(text_parts, "")
rt:log("info", "Total chunks: " .. chunk_count .. ", text length: " .. #result)

rt:output({ text = result }, nil)
