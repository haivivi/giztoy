-- Realtime Transformer Test Script
-- Usage: bazel run //go/cmd/luau -- -runtime tool -models $PWD/testdata/models testdata/luau/runtime/realtime.luau <<< '{"text":"你好"}'
-- Note: This script demonstrates the Realtime API for voice-to-voice interaction.
-- Note: rt is a global provided by the tool runtime

-- Get input
local input = rt:input()
if not input then
    input = { text = "你好" }
end

local model = input.model or "doubao-realtime"  -- default model
local inputText = input.text or "你好"

rt:log("info", "Realtime test starting with model: " .. model)
rt:log("info", "Input: " .. inputText)

-- Create Realtime transformer
local realtime, err = rt:transformer("realtime", {
    model = model,
})

if err then
    rt:log("error", "Failed to create Realtime transformer: " .. tostring(err))
    rt:output({ success = false, error = tostring(err) }, nil)
    return
end

-- Send text input (realtime can handle both text and audio)
local sendErr = realtime:send({
    part = inputText,
    is_bos = true,
    is_eos = true,  -- Single message, mark as complete
})

if sendErr then
    rt:log("error", "Failed to send input: " .. tostring(sendErr))
    rt:output({ success = false, error = tostring(sendErr) }, nil)
    return
end

-- Close send to signal end of input
realtime:close_send()

-- Receive responses (can be text and/or audio)
local responses = {
    text = {},
    audio = {},
}
local totalAudioBytes = 0

while true do
    local chunk, recvErr = realtime:recv()
    if recvErr then
        if recvErr == "EOF" or recvErr == "stream EOF" then
            rt:log("info", "Realtime stream completed")
            break
        end
        rt:log("error", "Recv error: " .. tostring(recvErr))
        break
    end
    
    if chunk and chunk.part then
        if type(chunk.part) == "string" then
            -- Text response
            if #chunk.part > 0 then
                table.insert(responses.text, chunk.part)
                rt:log("debug", "Text chunk: " .. chunk.part)
            end
        elseif type(chunk.part) == "table" and chunk.part.data then
            -- Audio response
            local dataLen = #chunk.part.data
            totalAudioBytes = totalAudioBytes + dataLen
            table.insert(responses.audio, {
                type = chunk.part.type,
                mime_type = chunk.part.mime_type,
                size = dataLen,
            })
            rt:log("debug", string.format("Audio chunk: %d bytes", dataLen))
        end
    end
    
    if chunk and chunk.is_eos then
        rt:log("info", "End of stream received")
        break
    end
end

-- Close the transformer
realtime:close()

local fullText = table.concat(responses.text, "")
rt:log("info", "Realtime completed")
rt:log("info", "Response text: " .. fullText)
rt:log("info", string.format("Audio: %d chunks, %d bytes", #responses.audio, totalAudioBytes))

rt:output({
    success = true,
    model = model,
    input = inputText,
    response_text = fullText,
    audio_chunk_count = #responses.audio,
    audio_total_bytes = totalAudioBytes,
}, nil)
