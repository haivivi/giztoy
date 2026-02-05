-- TTS Transformer Test Script
-- Usage: bazel run //go/cmd/luau -- -runtime tool -models $PWD/testdata/models testdata/luau/runtime/tts.luau <<< '{"text":"你好世界"}'
-- Note: rt is a global provided by the tool runtime

-- Get input text
local input = rt:input()
if not input or not input.text then
    rt:log("error", "no input text provided, using default")
    input = { text = "你好，这是一个 TTS 测试。" }
end

local text = input.text
local model = input.model or "doubao-tts"  -- default model

rt:log("info", "TTS test starting with model: " .. model)
rt:log("info", "Input text: " .. text)

-- Create TTS transformer
local tts, err = rt:transformer("tts", {
    model = model,
})

if err then
    rt:log("error", "Failed to create TTS transformer: " .. tostring(err))
    rt:output({ success = false, error = tostring(err) }, nil)
    return
end

-- Send text to TTS
rt:log("debug", "Sending text to TTS...")
local sendErr = tts:send({
    part = text,
    is_bos = true,
})

if sendErr then
    rt:log("error", "Failed to send text: " .. tostring(sendErr))
    rt:output({ success = false, error = tostring(sendErr) }, nil)
    return
end

rt:log("debug", "Text sent, closing send...")
-- Close send to signal end of input
tts:close_send()
rt:log("debug", "Send closed, receiving audio...")

-- Receive audio chunks
local chunks = {}
local totalBytes = 0

local chunkNum = 0
while true do
    local chunk, recvErr = tts:recv()
    chunkNum = chunkNum + 1
    
    rt:log("debug", string.format("Recv iteration %d: chunk=%s, err=%s", 
        chunkNum, chunk and "table" or "nil", tostring(recvErr)))
    
    if recvErr then
        if recvErr == "EOF" or recvErr == "stream EOF" then
            rt:log("info", "TTS stream completed (EOF)")
            break
        end
        rt:log("error", "Recv error: " .. tostring(recvErr))
        break
    end
    
    if chunk == nil then
        rt:log("info", "TTS stream completed (nil chunk)")
        break
    end
    
    rt:log("debug", string.format("Chunk %d: part=%s, is_bos=%s, is_eos=%s", 
        chunkNum, type(chunk.part), tostring(chunk.is_bos), tostring(chunk.is_eos)))
    
    if chunk.part then
        -- chunk.part should be a table with type, mime_type, data
        if type(chunk.part) == "table" then
            rt:log("debug", string.format("Part table: type=%s, data=%s", 
                tostring(chunk.part.type), chunk.part.data and "present" or "nil"))
            if chunk.part.data then
                local dataLen = #chunk.part.data
                totalBytes = totalBytes + dataLen
                table.insert(chunks, {
                    type = chunk.part.type,
                    mime_type = chunk.part.mime_type,
                    size = dataLen,
                })
                rt:log("debug", string.format("Received audio chunk: %d bytes, mime: %s", 
                    dataLen, chunk.part.mime_type or "unknown"))
            end
        else
            rt:log("debug", "Part is not a table: " .. type(chunk.part))
        end
    end
    
    if chunk.is_eos then
        rt:log("info", "End of stream received")
        break
    end
end

-- Close the transformer
tts:close()

rt:log("info", string.format("TTS completed: %d chunks, %d total bytes", #chunks, totalBytes))

rt:output({
    success = true,
    model = model,
    input_text = text,
    chunk_count = #chunks,
    total_bytes = totalBytes,
}, nil)
