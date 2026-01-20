# DashScope Realtime + MiniMax Integration Test

This example demonstrates the integration between MiniMax SDK and DashScope Realtime API.

## ⚠️ Important: DashScope Realtime API is Audio-Only

DashScope Realtime API **does NOT support text input**. All input must be audio.
This is different from OpenAI's Realtime API which supports both text and audio input.

## Flow

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────┐
│   MiniMax LLM   │───▶│   MiniMax TTS   │───▶│  DashScope Realtime │
│  (Text Gen)     │    │  (Audio Gen)    │    │  (Audio Response)   │
└─────────────────┘    └─────────────────┘    └─────────────────────┘
       ↓                      ↓                         ↓
  "我是AI助手..."      PCM Audio (16kHz)         Audio Response
```

## Parameters

| Parameter | Options | Description |
|-----------|---------|-------------|
| `-output` | `text_only`, `text_audio` | DashScope output modalities |
| `-vad` | `server`, `client` | VAD mode: server-side or manual (recommended: client) |
| `-prompt` | string | Prompt for MiniMax LLM |
| `-v` | flag | Verbose output |

## Usage

```bash
# Build
bazel build //examples/dashscope/realtime:realtime

# Run with minimal options
bazel run //examples/dashscope/realtime:realtime -- \
  -minimax-key="YOUR_MINIMAX_KEY" \
  -dashscope-key="YOUR_DASHSCOPE_KEY"

# Run with custom prompt and client VAD
bazel run //examples/dashscope/realtime:realtime -- \
  -minimax-key="YOUR_MINIMAX_KEY" \
  -dashscope-key="YOUR_DASHSCOPE_KEY" \
  -prompt="请介绍一下北京" \
  -output=text_audio \
  -vad=client
```

## Environment Variables

You can also set API keys via environment variables:

```bash
export MINIMAX_API_KEY="your_key"
export DASHSCOPE_API_KEY="your_key"
bazel run //examples/dashscope/realtime:realtime
```

## VAD Modes

### Server VAD (`-vad=server`)
- Server automatically detects speech end
- Requires silence after audio input
- Automatic response generation

### Client VAD (`-vad=client`)
- Manual mode: client controls when to trigger response
- More reliable for file-based input
- Calls `CommitInput()` + `CreateResponse()` after audio

## Test Results

Successful integration test output:

```
=== Step 1: MiniMax LLM ===
LLM Response: 我是一个充满好奇心、情感丰富且细致入微的智能助手...

=== Step 2: MiniMax TTS ===
TTS Audio: 300188 bytes (16kHz PCM)

=== Step 3: DashScope Realtime ===
Connected to DashScope Realtime
Session created: sess_xxx
Session updated (modalities: [text audio], vad: disabled)
Sending audio (300188 bytes)...
Response started (id: resp_xxx)
[Response Audio]: 460800 bytes
[Usage]: input=308, output=271 tokens

=== Test Complete ===
```

## Notes

1. DashScope Realtime API is audio-first; text input is not directly supported
2. Use `-vad=client` for reliable file-based testing
3. Audio format: 16-bit PCM, 16kHz mono (input), 24kHz mono (output)
