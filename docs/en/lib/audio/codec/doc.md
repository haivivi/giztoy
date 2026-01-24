# Audio Codec Module

Audio encoding and decoding for Opus, MP3, and OGG formats.

## Design Goals

1. **Native Performance**: FFI bindings to proven C libraries
2. **Streaming Support**: Process audio in chunks, not full files
3. **VoIP Optimized**: Low-latency Opus encoding for voice chat

## Codec Support Matrix

| Codec | Encode | Decode | Library | Use Case |
|-------|--------|--------|---------|----------|
| Opus | ✅ | ✅ | libopus | Voice chat, streaming |
| MP3 | ✅ | ✅ | LAME / minimp3 | File storage, compatibility |
| OGG | N/A | N/A | libogg | Container format |

## Sub-modules

### opus/

Opus codec implementation for voice and audio.

**Features:**
- Encoder with VoIP/Audio/LowDelay modes
- Decoder with PLC (Packet Loss Concealment)
- TOC (Table of Contents) parsing
- Frame duration detection

**Key Types:**
- `Encoder`, `Decoder`
- `Frame`, `TOC`, `FrameDuration`

### mp3/

MP3 codec for compatibility with legacy systems.

**Features:**
- LAME-based encoding with quality presets
- minimp3-based decoding (header-only library)

**Key Types:**
- `Encoder`, `Decoder`

### ogg/

OGG container format for packaging Opus/Vorbis streams.

**Features:**
- Page-based streaming
- Bitstream management
- Synchronization recovery

**Key Types:**
- `Encoder`, `Stream`, `Sync`, `Page`

## Opus Frame Durations

| Duration | Samples@16K | Samples@48K |
|----------|-------------|-------------|
| 2.5ms | 40 | 120 |
| 5ms | 80 | 240 |
| 10ms | 160 | 480 |
| 20ms | 320 | 960 |
| 40ms | 640 | 1920 |
| 60ms | 960 | 2880 |

**Recommended:** 20ms frames balance latency and compression.

## Common Opus Bitrates

| Application | Bitrate | Quality |
|-------------|---------|---------|
| Voice (narrow) | 8-12 kbps | Intelligible |
| Voice (wide) | 16-24 kbps | Good |
| Voice (HD) | 32-48 kbps | Excellent |
| Music | 64-128 kbps | Hi-Fi |

## Native Library Versions

| Library | Minimum Version | Notes |
|---------|-----------------|-------|
| libopus | 1.3.0 | Opus encoder/decoder |
| libogg | 1.3.0 | OGG container |
| LAME | 3.100 | MP3 encoder |
| minimp3 | - | Header-only decoder |

## Examples

See parent `audio/` documentation for usage examples.

## Related Modules

- `opusrt/` - Realtime Opus streaming with OGG container
- `resampler/` - Sample rate conversion before encoding
