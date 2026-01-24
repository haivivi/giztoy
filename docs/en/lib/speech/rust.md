# speech (Rust)

## Crate Layout
- `voice.rs`: Voice/VoiceSegment interfaces
- `speech.rs`: Speech/SpeechSegment interfaces
- `asr.rs`: ASR multiplexer and traits
- `tts.rs`: TTS multiplexer and traits
- `segment.rs`: sentence segmentation utilities
- `util.rs`: speech collector and iterator helpers

## Public Interfaces
- **Voice**: `Voice`, `VoiceSegment`, `VoiceStream`
- **Speech**: `Speech`, `SpeechSegment`, `SpeechStream`
- **ASR**: `StreamTranscriber`, `Transcriber` (async), `ASR`
- **TTS**: `Synthesizer`, `TTS` (async)
- **Segmentation**: `SentenceSegmenter`, `SentenceIterator`

## Design Notes
- Async traits are used across ASR/TTS and stream interfaces.
- `ASR` and `TTS` use a trie-based mux with async read/write locks.
- `SpeechCollector` composes a `SpeechStream` into a single `Speech`.

## Differences vs Go
- No global mux singletons; callers construct `ASR`/`TTS` explicitly.
- Async `AsyncRead` is used for text input in TTS.
