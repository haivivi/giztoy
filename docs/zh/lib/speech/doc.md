# speech

## Overview
The speech module defines interfaces for voice and speech processing. It
separates pure audio streams (Voice) from speech streams that include
transcriptions (Speech). It also provides multiplexers for ASR (speech-to-text)
and TTS (text-to-speech) implementations.

## Design Goals
- Unified interfaces for ASR/TTS backends
- Stream-first APIs for long-running audio
- Clear separation between audio-only and audio+text
- Pluggable providers via multiplexer registration

## Key Concepts
- **Voice**: audio-only stream of PCM segments
- **Speech**: audio stream with text transcription per segment
- **ASR**: Opus input -> `Speech`/`SpeechStream`
- **TTS**: text input -> `Speech`
- **Sentence segmentation**: split long text into manageable chunks

## Components
- Voice/Speech interfaces
- ASR/TTS muxers
- Sentence segmentation utilities
- Speech collection and copy helpers

## Related Modules
- `docs/lib/audio/pcm` for PCM formats
- `docs/lib/audio/opusrt` for Opus streaming input
- Provider SDKs in `docs/lib/minimax`, `docs/lib/doubaospeech`
