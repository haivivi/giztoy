//! Interfaces for voice and speech processing.
//!
//! This crate provides interfaces for:
//! - [`Voice`] and [`VoiceSegment`]: Pure audio data streams
//! - [`Speech`] and [`SpeechSegment`]: Audio with text transcription
//! - [`Synthesizer`] and [`TTS`]: Text-to-speech synthesis
//! - [`StreamTranscriber`] and [`ASR`]: Automatic speech recognition
//!
//! # Example
//!
//! ```rust,ignore
//! use giztoy_speech::{TTS, Synthesizer};
//!
//! // Register a TTS synthesizer
//! let mut tts = TTS::new();
//! tts.handle("voice/en-US", my_synthesizer)?;
//!
//! // Synthesize speech
//! let speech = tts.synthesize("voice/en-US", text_reader, format).await?;
//! ```

mod voice;
mod speech;
mod tts;
mod asr;
mod segment;
mod util;

pub use voice::*;
pub use speech::*;
pub use tts::*;
pub use asr::*;
pub use segment::*;
pub use util::*;

#[cfg(test)]
mod tests;
