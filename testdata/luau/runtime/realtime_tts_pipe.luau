-- Realtime TTS Pipeline Test Script
-- Usage: bazel run //go/cmd/luau -- -runtime tool -models $PWD/testdata/models testdata/luau/runtime/realtime_tts_pipe.luau <<< '{"text":"你好，今天天气怎么样？"}'
-- 
-- This script demonstrates streaming pipeline:
-- Text → TTS → Audio chunks → Realtime → Text/Audio response
--
-- Note: rt is a global provided by the tool runtime

-- Get input
local input = rt:input()
if not input then
    input = { text = "你好，请问今天天气怎么样？" }
end

local ttsModel = input.tts_model or "minimax/shaonv"
local realtimeModel = input.realtime_model or "doubao/vv"
local inputText = input.text or "你好"

rt:log("info", "Pipeline test starting")
rt:log("info", "TTS model: " .. ttsModel)
rt:log("info", "Realtime model: " .. realtimeModel)
rt:log("info", "Input text: " .. inputText)

-- Create TTS transformer
local tts, ttsErr = rt:transformer("tts", {
    model = ttsModel,
})

if ttsErr then
    rt:log("error", "Failed to create TTS transformer: " .. tostring(ttsErr))
    rt:output({ success = false, error = "TTS: " .. tostring(ttsErr) }, nil)
    return
end

-- Create Realtime transformer
local realtime, rtErr = rt:transformer("realtime", {
    model = realtimeModel,
})

if rtErr then
    rt:log("error", "Failed to create Realtime transformer: " .. tostring(rtErr))
    tts:close()
    rt:output({ success = false, error = "Realtime: " .. tostring(rtErr) }, nil)
    return
end

-- Send text to TTS
rt:log("info", "Sending text to TTS...")
local sendErr = tts:send({
    part = inputText,
    is_bos = true,
})

if sendErr then
    rt:log("error", "Failed to send text to TTS: " .. tostring(sendErr))
    tts:close()
    realtime:close()
    rt:output({ success = false, error = "TTS send: " .. tostring(sendErr) }, nil)
    return
end

-- Close TTS send to trigger synthesis
tts:close_send()

-- Stream TTS output to Realtime input (pipeline pattern)
rt:log("info", "Starting TTS → Realtime pipeline...")
local audioChunksSent = 0
local totalBytesSent = 0

while true do
    local chunk, recvErr = tts:recv()
    
    -- Check for end of stream
    if recvErr then
        if recvErr ~= "EOF" and recvErr ~= "stream EOF" then
            rt:log("error", "TTS recv error: " .. tostring(recvErr))
        end
        break
    end
    
    if chunk == nil then
        break
    end
    
    -- Forward audio chunk to realtime (streaming pipeline)
    if chunk.part and type(chunk.part) == "table" and chunk.part.data then
        local dataLen = #chunk.part.data
        if dataLen > 0 then
            -- Send audio to realtime immediately (no buffering)
            local pipeErr = realtime:send({
                part = chunk.part,  -- Keep blob format: {type, mime_type, data}
            })
            
            if pipeErr then
                rt:log("error", "Realtime send error: " .. tostring(pipeErr))
                break
            end
            
            audioChunksSent = audioChunksSent + 1
            totalBytesSent = totalBytesSent + dataLen
            rt:log("debug", string.format("Piped chunk %d: %d bytes", audioChunksSent, dataLen))
        end
    end
    
    -- Check for TTS end of stream
    if chunk.is_eos then
        rt:log("info", "TTS stream completed")
        break
    end
end

-- Close TTS
tts:close()

rt:log("info", string.format("Pipeline complete: sent %d chunks, %d bytes to Realtime", 
    audioChunksSent, totalBytesSent))

-- Send explicit EOS to trigger VAD processing
rt:log("info", "Sending EOS to trigger VAD...")
realtime:send({ is_eos = true })

-- Close send channel
realtime:close_send()

-- Receive Realtime responses
rt:log("info", "Receiving Realtime responses...")
local responses = {
    text = {},
    audio = {},
}
local totalAudioBytesReceived = 0

while true do
    local chunk, recvErr = realtime:recv()
    
    if recvErr then
        if recvErr ~= "EOF" and recvErr ~= "stream EOF" then
            rt:log("error", "Realtime recv error: " .. tostring(recvErr))
        else
            rt:log("info", "Realtime stream completed")
        end
        break
    end
    
    if chunk == nil then
        rt:log("info", "Realtime stream ended (nil chunk)")
        break
    end
    
    -- Process response chunk
    if chunk.part then
        if type(chunk.part) == "string" then
            -- Text response
            if #chunk.part > 0 then
                table.insert(responses.text, chunk.part)
                rt:log("debug", "Text: " .. chunk.part)
            end
        elseif type(chunk.part) == "table" then
            -- Check for text in table format
            if chunk.part.type == "text" and chunk.part.value then
                table.insert(responses.text, chunk.part.value)
                rt:log("debug", "Text: " .. chunk.part.value)
            -- Audio response
            elseif chunk.part.data then
                local dataLen = #chunk.part.data
                totalAudioBytesReceived = totalAudioBytesReceived + dataLen
                table.insert(responses.audio, {
                    type = chunk.part.type,
                    mime_type = chunk.part.mime_type,
                    size = dataLen,
                })
                rt:log("debug", string.format("Audio response: %d bytes", dataLen))
            end
        end
    end
    
    if chunk.is_eos then
        rt:log("info", "Realtime end of stream")
        break
    end
end

-- Close realtime
realtime:close()

-- Compile results
local responseText = table.concat(responses.text, "")

rt:log("info", "=== Pipeline Results ===")
rt:log("info", "Input: " .. inputText)
rt:log("info", "Audio sent: " .. audioChunksSent .. " chunks, " .. totalBytesSent .. " bytes")
rt:log("info", "Response text: " .. responseText)
rt:log("info", "Response audio: " .. #responses.audio .. " chunks, " .. totalAudioBytesReceived .. " bytes")

rt:output({
    success = true,
    input_text = inputText,
    tts_model = ttsModel,
    realtime_model = realtimeModel,
    audio_chunks_sent = audioChunksSent,
    audio_bytes_sent = totalBytesSent,
    response_text = responseText,
    response_audio_chunks = #responses.audio,
    response_audio_bytes = totalAudioBytesReceived,
}, nil)
