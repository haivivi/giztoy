use std::collections::HashMap;

use crate::voiceprint::{voice_label, SpeakerChunk, SpeakerStatus};

/// Uses a sliding window of voice hashes to determine speaker status.
///
/// Tracks the most recent hashes and classifies the current state as
/// Single, Overlap, or Unknown based on hash distribution within the window.
///
/// # Algorithm
///
/// The detector maintains a circular buffer of the last N hashes.
/// On each [`Detector::feed`] call it counts how many distinct hashes appear:
///
/// - 1 dominant hash with high ratio -> StatusSingle
/// - 2 dominant hashes -> StatusOverlap
/// - 3+ hashes or insufficient data -> StatusUnknown
pub struct Detector {
    window: Vec<String>,
    pos: usize,
    filled: usize,
    min_ratio: f32,
}

/// Configuration for [`Detector`].
pub struct DetectorConfig {
    /// Sliding window size (default: 5).
    pub window_size: usize,
    /// Minimum fraction of the window that the dominant hash must occupy
    /// to be considered "stable" (default: 0.6).
    pub min_ratio: f32,
}

impl Default for DetectorConfig {
    fn default() -> Self {
        Self {
            window_size: 5,
            min_ratio: 0.6,
        }
    }
}

impl Detector {
    /// Creates a Detector with default configuration (window=5, min_ratio=0.6).
    pub fn new() -> Self {
        Self::with_config(DetectorConfig::default())
    }

    /// Creates a Detector with the given configuration.
    pub fn with_config(cfg: DetectorConfig) -> Self {
        let window_size = if cfg.window_size > 0 {
            cfg.window_size
        } else {
            5
        };
        let min_ratio = if cfg.min_ratio > 0.0 && cfg.min_ratio <= 1.0 {
            cfg.min_ratio
        } else {
            0.6
        };
        Self {
            window: vec![String::new(); window_size],
            pos: 0,
            filled: 0,
            min_ratio,
        }
    }

    /// Adds a new hash to the window and returns the current speaker state.
    /// Returns `None` if the window has fewer than 2 entries (insufficient data).
    pub fn feed(&mut self, hash: &str) -> Option<SpeakerChunk> {
        // Write to circular buffer.
        self.window[self.pos] = hash.to_string();
        self.pos = (self.pos + 1) % self.window.len();
        if self.filled < self.window.len() {
            self.filled += 1;
        }

        // Need at least 2 samples.
        if self.filled < 2 {
            return None;
        }

        // Count hash frequencies in the window.
        let mut counts: HashMap<&str, usize> = HashMap::with_capacity(4);
        for i in 0..self.filled {
            let idx = (self.pos + self.window.len() - self.filled + i) % self.window.len();
            let entry = counts.entry(self.window[idx].as_str()).or_insert(0);
            *entry += 1;
        }

        // Find top-1 and top-2 hashes.
        let mut top1_hash = "";
        let mut top1_count: usize = 0;
        let mut top2_hash = "";
        let mut top2_count: usize = 0;

        for (&h, &c) in &counts {
            if c > top1_count {
                top2_hash = top1_hash;
                top2_count = top1_count;
                top1_hash = h;
                top1_count = c;
            } else if c > top2_count {
                top2_hash = h;
                top2_count = c;
            }
        }

        let total = self.filled as f32;

        // Single speaker: dominant hash exceeds min_ratio.
        if top1_count as f32 / total >= self.min_ratio {
            return Some(SpeakerChunk {
                status: SpeakerStatus::Single,
                speaker: voice_label(top1_hash),
                candidates: vec![voice_label(top1_hash)],
                confidence: top1_count as f32 / total,
            });
        }

        // Overlap: two hashes together cover most of the window.
        if top2_count > 0 {
            let combined_ratio = (top1_count + top2_count) as f32 / total;
            if combined_ratio >= self.min_ratio {
                return Some(SpeakerChunk {
                    status: SpeakerStatus::Overlap,
                    speaker: voice_label(top1_hash),
                    candidates: vec![voice_label(top1_hash), voice_label(top2_hash)],
                    confidence: combined_ratio,
                });
            }
        }

        // Unknown: too many distinct hashes, unstable.
        Some(SpeakerChunk {
            status: SpeakerStatus::Unknown,
            speaker: String::new(),
            candidates: Vec::new(),
            confidence: top1_count as f32 / total,
        })
    }

    /// Clears the window state.
    pub fn reset(&mut self) {
        self.pos = 0;
        self.filled = 0;
        for s in &mut self.window {
            s.clear();
        }
    }
}

impl Default for Detector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detector_needs_minimum_samples() {
        let mut det = Detector::new();
        assert!(
            det.feed("A3F8").is_none(),
            "first sample should return None"
        );
    }

    #[test]
    fn detector_single_speaker() {
        let mut det = Detector::new();
        // Fill window with same hash.
        for _ in 0..5 {
            det.feed("AAAA");
        }
        let chunk = det.feed("AAAA").unwrap();
        assert_eq!(chunk.status, SpeakerStatus::Single);
        assert_eq!(chunk.speaker, "voice:AAAA");
        assert_eq!(chunk.confidence, 1.0);
    }

    #[test]
    fn detector_overlap() {
        let mut det = Detector::with_config(DetectorConfig {
            window_size: 5,
            min_ratio: 0.6,
        });

        // Alternate between two hashes: A, B, A, B, A
        det.feed("AAAA");
        det.feed("BBBB");
        det.feed("AAAA");
        det.feed("BBBB");
        let chunk = det.feed("AAAA").unwrap();

        // A=3, B=2 out of 5. top1 ratio = 0.6, combined = 1.0.
        // top1 ratio == min_ratio → Single.
        assert_eq!(chunk.status, SpeakerStatus::Single);
    }

    #[test]
    fn detector_overlap_detection() {
        let mut det = Detector::with_config(DetectorConfig {
            window_size: 5,
            min_ratio: 0.7,
        });

        // With higher min_ratio, alternating hashes become Overlap.
        det.feed("AAAA");
        det.feed("BBBB");
        det.feed("AAAA");
        det.feed("BBBB");
        let chunk = det.feed("AAAA").unwrap();

        // A=3, B=2 out of 5. top1 ratio = 0.6 < 0.7, combined = 1.0 >= 0.7.
        assert_eq!(chunk.status, SpeakerStatus::Overlap);
        assert_eq!(chunk.candidates.len(), 2);
    }

    #[test]
    fn detector_unknown() {
        let mut det = Detector::with_config(DetectorConfig {
            window_size: 5,
            min_ratio: 0.6,
        });

        // All different hashes.
        det.feed("AAAA");
        det.feed("BBBB");
        det.feed("CCCC");
        det.feed("DDDD");
        let chunk = det.feed("EEEE").unwrap();

        assert_eq!(chunk.status, SpeakerStatus::Unknown);
    }

    #[test]
    fn detector_reset() {
        let mut det = Detector::new();
        det.feed("AAAA");
        det.feed("BBBB");
        det.feed("CCCC");

        det.reset();
        // After reset, first feed should return None.
        assert!(det.feed("AAAA").is_none());
    }
}
