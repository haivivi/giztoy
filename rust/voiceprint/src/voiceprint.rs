use std::fmt;

/// Speaker detection result.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SpeakerStatus {
    /// Speaker cannot be determined (too much noise, hash is unstable).
    Unknown = 0,
    /// A single stable speaker is detected.
    Single = 1,
    /// Multiple speakers are detected (hash alternates between two or more values).
    Overlap = 2,
}

impl fmt::Display for SpeakerStatus {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Unknown => write!(f, "unknown"),
            Self::Single => write!(f, "single"),
            Self::Overlap => write!(f, "overlap"),
        }
    }
}

/// Output of the voiceprint detection pipeline.
/// Represents the speaker state for a window of audio.
#[derive(Debug, Clone)]
pub struct SpeakerChunk {
    /// Detection result.
    pub status: SpeakerStatus,

    /// Primary voice label (e.g., "voice:A3F8").
    /// Empty when status is Unknown.
    pub speaker: String,

    /// All detected voice labels when status is Overlap.
    /// For Single, contains only the primary speaker.
    /// For Unknown, this is empty.
    pub candidates: Vec<String>,

    /// Value in [0, 1] indicating detection confidence.
    /// Higher values mean the hash window is more stable.
    pub confidence: f32,
}

/// Returns a prefixed voice label string for use as a graph entity
/// or segment label. Format: `"voice:{hash}"`.
pub fn voice_label(hash: &str) -> String {
    format!("voice:{hash}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn speaker_status_display() {
        assert_eq!(SpeakerStatus::Unknown.to_string(), "unknown");
        assert_eq!(SpeakerStatus::Single.to_string(), "single");
        assert_eq!(SpeakerStatus::Overlap.to_string(), "overlap");
    }

    #[test]
    fn voice_label_format() {
        assert_eq!(voice_label("A3F8"), "voice:A3F8");
        assert_eq!(voice_label(""), "voice:");
    }
}
