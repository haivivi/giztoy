-- ASR Transformer Test Script
-- Usage: bazel run //go/cmd/luau -- -runtime tool -models $PWD/testdata/models testdata/luau/runtime/asr.luau <<< '{}'
-- Note: This script demonstrates the ASR API. In practice, you would stream real audio data.
-- Note: rt is a global provided by the tool runtime

-- Get input
local input = rt:input()
if not input then
    input = {}
end

local model = input.model or "doubao-asr"  -- default model

rt:log("info", "ASR test starting with model: " .. model)

-- Create ASR transformer
local asr, err = rt:transformer("asr", {
    model = model,
})

if err then
    rt:log("error", "Failed to create ASR transformer: " .. tostring(err))
    rt:output({ success = false, error = tostring(err) }, nil)
    return
end

-- For testing, we'll send a small synthetic audio chunk
-- In real usage, you would stream actual audio data
local testAudioData = string.rep("\0", 1600)  -- 100ms of silence at 16kHz 16-bit mono

rt:log("info", "Sending test audio data...")

-- Send audio data
local sendErr = asr:send({
    part = {
        data = testAudioData,
        mime_type = "audio/pcm;rate=16000;bits=16;channels=1",
    },
    is_bos = true,
})

if sendErr then
    rt:log("error", "Failed to send audio: " .. tostring(sendErr))
    rt:output({ success = false, error = tostring(sendErr) }, nil)
    return
end

-- Send end of stream
asr:send({
    part = "",
    is_eos = true,
})

-- Close send to signal end of input
asr:close_send()

-- Receive transcription results
local transcriptions = {}

while true do
    local chunk, recvErr = asr:recv()
    if recvErr then
        if recvErr == "EOF" or recvErr == "stream EOF" then
            rt:log("info", "ASR stream completed")
            break
        end
        rt:log("error", "Recv error: " .. tostring(recvErr))
        break
    end
    
    if chunk and chunk.part then
        -- chunk.part should be the transcribed text
        if type(chunk.part) == "string" and #chunk.part > 0 then
            table.insert(transcriptions, chunk.part)
            rt:log("info", "Transcription: " .. chunk.part)
        end
    end
    
    if chunk and chunk.is_eos then
        rt:log("info", "End of stream received")
        break
    end
end

-- Close the transformer
asr:close()

local fullText = table.concat(transcriptions, " ")
rt:log("info", "ASR completed. Full transcription: " .. fullText)

rt:output({
    success = true,
    model = model,
    transcription = fullText,
    segment_count = #transcriptions,
}, nil)
