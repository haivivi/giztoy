//! Speaker identification via audio embeddings and locality-sensitive hashing (LSH).
//!
//! # Architecture
//!
//! The pipeline processes audio in three stages:
//!
//! 1. [`VoiceprintModel::extract`]: PCM16 16kHz mono audio -> embedding vector
//! 2. [`Hasher::hash`]: embedding -> 16-bit hex hash (e.g., "A3F8")
//! 3. [`Detector::feed`]: sliding window of hashes -> [`SpeakerStatus`]
//!
//! # Multi-Level Precision
//!
//! Voice hashes support multi-level precision via prefix truncation,
//! similar to geohash:
//!
//! ```text
//! 16 bit: A3F8  <- exact match
//! 12 bit: A3F   <- fuzzy match
//!  8 bit: A3    <- group level
//!  4 bit: A     <- coarse partition
//!  0 bit: *     <- no filter
//! ```
//!
//! # Feature Extraction
//!
//! The [`fbank`] module provides Kaldi-compatible log mel filterbank extraction:
//! - Povey window (hamming^0.85)
//! - Pre-emphasis 0.97
//! - Cooley-Tukey FFT
//! - Mel triangular filterbank
//! - CMVN normalization

mod detector;
mod error;
pub mod fbank;
mod hasher;
mod model;
mod voiceprint;

pub use detector::{Detector, DetectorConfig};
pub use error::VoiceprintError;
pub use fbank::{cmvn, compute_fbank, l2_normalize, FbankConfig};
pub use hasher::Hasher;
pub use model::VoiceprintModel;
pub use voiceprint::{voice_label, SpeakerChunk, SpeakerStatus};
