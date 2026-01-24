# Audio Package

Audio processing framework for speech and multimedia applications.

## Design Goals

1. **Real-time Processing**: Low-latency audio mixing, encoding, and streaming
2. **Format Flexibility**: Support common audio formats (PCM, Opus, MP3, OGG)
3. **Cross-platform**: FFI bindings to native libraries (libopus, libsoxr, lame)
4. **Streaming-first**: Designed for continuous audio streams, not just files

## Architecture

```mermaid
graph TB
    subgraph audio["audio/"]
        subgraph row1[" "]
            pcm["pcm/<br/>- Format<br/>- Chunk<br/>- Mixer"]
            codec["codec/<br/>- opus/<br/>- mp3/<br/>- ogg/"]
            resampler["resampler/<br/>- soxr<br/>- Format<br/>- Convert"]
        end
        subgraph row2[" "]
            opusrt["opusrt/<br/>- Buffer<br/>- Realtime<br/>- OGG R/W"]
            songs["songs/<br/>- Catalog<br/>- Notes<br/>- PCM gen"]
            portaudio["portaudio/<br/>(Go only)<br/>- Stream<br/>- Device"]
        end
    end
```

## Submodules

| Module | Description | Go | Rust |
|--------|-------------|:--:|:----:|
| [pcm/](./pcm/doc.md) | PCM format, chunks, mixing | ✅ | ✅ |
| [codec/](./codec/doc.md) | Audio codecs (Opus, MP3, OGG) | ✅ | ✅ |
| [resampler/](./resampler/doc.md) | Sample rate conversion (soxr) | ✅ | ✅ |
| [opusrt/](./opusrt/doc.md) | Realtime Opus streaming | ✅ | ⚠️ |
| [songs/](./songs/doc.md) | Built-in melodies | ✅ | ✅ |
| [portaudio/](./portaudio/doc.md) | Audio I/O devices | ✅ | ❌ |

## Audio Formats

### PCM Formats (Predefined)

| Format | Sample Rate | Channels | Bit Depth |
|--------|-------------|----------|-----------|
| `L16Mono16K` | 16000 Hz | 1 | 16-bit |
| `L16Mono24K` | 24000 Hz | 1 | 16-bit |
| `L16Mono48K` | 48000 Hz | 1 | 16-bit |

### Codec Support

| Codec | Encode | Decode | Container |
|-------|--------|--------|-----------|
| Opus | ✅ | ✅ | Raw, OGG |
| MP3 | ✅ | ✅ | Raw |
| OGG | N/A | N/A | Container only |

## Common Workflows

### Voice Chat (Low Latency)

```mermaid
flowchart LR
    A[Microphone] --> B[PCM 16kHz]
    B --> C[Opus Encode]
    C --> D[Network]
    D --> E[Opus Decode]
    E --> F[Mixer]
    F --> G[Speaker]
```

### Speech Synthesis Playback

```mermaid
flowchart LR
    A[API Response<br/>Base64 MP3] --> B[MP3 Decode]
    B --> C[Resample<br/>24K→16K]
    C --> D[Mixer]
    D --> E[Speaker]
```

### Audio Recording

```mermaid
flowchart LR
    A[PCM Stream] --> B[Opus Encode]
    B --> C[OGG Writer]
    C --> D[File]
```

## Native Dependencies

| Library | Purpose | Build System |
|---------|---------|--------------|
| libopus | Opus codec | pkg-config / Bazel |
| libsoxr | Resampling | pkg-config / Bazel |
| lame | MP3 encoding | Bazel (bundled) |
| minimp3 | MP3 decoding | Bazel (bundled) |
| libogg | OGG container | pkg-config / Bazel |
| portaudio | Audio I/O | pkg-config / Bazel |

## Examples Directory

- `examples/go/audio/` - Go audio examples
- `examples/rust/audio/` - Rust audio examples

## Related Packages

- `buffer` - Used for audio data buffering
- `speech` - High-level speech synthesis/recognition
- `minimax`, `doubaospeech` - TTS/ASR APIs returning audio
