# speech (Go)

## Package Layout
- `voice.go`: Voice/VoiceSegment interfaces
- `speech.go`: Speech/SpeechSegment interfaces
- `asr.go`: ASR multiplexer and interfaces
- `tts.go`: TTS multiplexer and interfaces
- `segment.go`: default sentence segmentation
- `util.go`: collectors and copy helpers
- Provider implementations: `asr_doubao_sauc.go`, `tts_doubao_v1.go`,
  `tts_doubao_v2.go`, `tts_minimax.go`

## Public Interfaces
- **Voice**: `Voice`, `VoiceSegment`, `VoiceStream`
- **Speech**: `Speech`, `SpeechSegment`, `SpeechStream`
- **ASR**: `StreamTranscriber`, `Transcriber`, `ASR` mux + helpers
- **TTS**: `Synthesizer`, `TTS` mux + helpers
- **Segmentation**: `SentenceSegmenter`, `SentenceIterator`

## Design Notes
- Global muxes `ASRMux` and `TTSMux` provide default routing.
- ASR uses Opus frame streams (`opusrt.FrameReader`).
- `DefaultSentenceSegmenter` splits by punctuation with a rune cap.
- `CollectSpeech` and `CopySpeech` help aggregate or export streams.

## Usage Notes
- `Transcribe` falls back to streaming when the backend does not implement
  the `Transcriber` interface.
- `Revoice` streams existing speech into a TTS backend via a pipe.
