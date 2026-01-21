//! Built-in melodies for testing audio playback.
//!
//! All songs are defined using the beat-based notation system with tempo and time signature.
//!
//! # Example
//!
//! ```rust
//! use giztoy_audio::songs::{Song, RenderOptions, ALL_SONGS};
//! use giztoy_audio::pcm::Format;
//!
//! // Get a song by ID
//! if let Some(song) = Song::by_id("twinkle_star") {
//!     // Render to PCM data
//!     let opts = RenderOptions::default();
//!     let data = song.render_bytes(opts);
//! }
//! ```

mod notes;
mod types;
mod pcm;
mod catalog;

pub use notes::*;
pub use types::*;
pub use pcm::*;
pub use catalog::*;
