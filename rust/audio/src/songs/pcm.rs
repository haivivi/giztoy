//! PCM audio generation for songs.

use crate::pcm::{Format, Mixer, MixerOptions, TrackOptions};
use super::types::{Note, Song, Voice};
use super::notes::REST;
use std::f64::consts::PI;
use std::io::Read;
use std::sync::Arc;
use std::thread;

/// Default audio format for songs.
pub const DEFAULT_FORMAT: Format = Format::L16Mono16K;

/// Rendering options for songs.
#[derive(Debug, Clone)]
pub struct RenderOptions {
    /// Audio format (default: L16Mono16K)
    pub format: Format,
    /// Volume 0.0-1.0 (default: 0.5)
    pub volume: f64,
    /// Include metronome track
    pub metronome: bool,
    /// Use piano-like rich harmonics (default: true)
    pub rich_sound: bool,
}

impl Default for RenderOptions {
    fn default() -> Self {
        Self {
            format: DEFAULT_FORMAT,
            volume: 0.5,
            metronome: false,
            rich_sound: true,
        }
    }
}

impl RenderOptions {
    /// Creates options with the specified format.
    pub fn with_format(mut self, format: Format) -> Self {
        self.format = format;
        self
    }

    /// Creates options with the specified volume.
    pub fn with_volume(mut self, volume: f64) -> Self {
        self.volume = volume;
        self
    }

    /// Enables or disables metronome.
    pub fn with_metronome(mut self, metronome: bool) -> Self {
        self.metronome = metronome;
        self
    }

    /// Enables or disables rich sound.
    pub fn with_rich_sound(mut self, rich_sound: bool) -> Self {
        self.rich_sound = rich_sound;
        self
    }
}

/// Generates a pure sine wave as 16-bit PCM data (little-endian).
pub fn generate_sine_wave(freq: f64, samples: usize, sample_rate: u32) -> Vec<u8> {
    let mut data = vec![0u8; samples * 2];
    if freq == REST {
        return data;
    }

    for i in 0..samples {
        let t = i as f64 / sample_rate as f64;
        let value = (2.0 * PI * freq * t).sin();
        let sample = (value * 16000.0) as i16;
        let bytes = sample.to_le_bytes();
        data[i * 2] = bytes[0];
        data[i * 2 + 1] = bytes[1];
    }
    data
}

// Normalization factor: approximate sum of harmonic amplitudes.
// 1.0 + 0.7 + 0.45 + 0.3 + 0.2 + 0.12 + 0.08 + 0.05 + 0.03 + 0.02 ≈ 2.95
// Using 2.5 to allow some headroom without clipping.
const HARMONIC_NORMALIZATION: f64 = 2.5;

/// Generates a piano-like note with realistic harmonics and envelope.
pub fn generate_rich_note(freq: f64, samples: usize, sample_rate: u32, volume: f64) -> Vec<i16> {
    let mut data = vec![0i16; samples];
    if freq == REST {
        return data;
    }

    // Piano harmonic structure: (harmonic_ratio, amplitude, decay_rate)
    let harmonics: [(f64, f64, f64); 10] = [
        (1.0, 1.0, 1.0),    // fundamental
        (2.0, 0.7, 1.2),    // 2nd harmonic
        (3.0, 0.45, 1.5),   // 3rd harmonic
        (4.0, 0.3, 1.8),    // 4th harmonic
        (5.0, 0.2, 2.2),    // 5th harmonic
        (6.0, 0.12, 2.6),   // 6th harmonic
        (7.0, 0.08, 3.0),   // 7th harmonic
        (8.0, 0.05, 3.5),   // 8th harmonic
        (9.0, 0.03, 4.0),   // 9th harmonic
        (10.0, 0.02, 4.5),  // 10th harmonic
    ];

    // Piano inharmonicity coefficient
    let inharmonicity = 0.0001 * (freq / 440.0) * (freq / 440.0);

    // Note duration for decay calculation
    let note_duration = samples as f64 / sample_rate as f64;

    for i in 0..samples {
        let t = i as f64 / sample_rate as f64;
        let progress = t / note_duration;

        let mut sample = 0.0;

        for (ratio, amplitude, decay_rate) in &harmonics {
            // Apply inharmonicity
            let actual_ratio = ratio * (1.0 + inharmonicity * ratio * ratio).sqrt();
            let phase = 2.0 * PI * freq * actual_ratio * t;

            // Harmonic amplitude with time-dependent decay
            let harmonic_decay = (-progress * decay_rate * 3.0).exp();
            let amp = amplitude * harmonic_decay;

            sample += amp * phase.sin();
        }

        // Normalize by sum of harmonic amplitudes
        sample /= HARMONIC_NORMALIZATION;

        // Piano envelope
        let envelope = compute_piano_envelope(i, samples, sample_rate, note_duration);

        // Apply volume and convert to i16
        sample *= volume * envelope;
        data[i] = (clamp(sample, -1.0, 1.0) * 32767.0 * 0.85) as i16;
    }

    data
}

/// Computes a piano-like ADSR envelope.
fn compute_piano_envelope(i: usize, samples: usize, sample_rate: u32, note_duration: f64) -> f64 {
    let t = i as f64 / sample_rate as f64;
    let progress = i as f64 / samples as f64;

    // Fast attack (2-5ms for piano hammer strike)
    let attack_time = 0.003;

    // Decay depends on note length
    let mut decay_rate = 2.0 / note_duration;
    decay_rate = decay_rate.clamp(0.5, 8.0);

    // Release at the end
    let release_start = 0.85;

    if t < attack_time {
        // Attack phase
        let attack_progress = t / attack_time;
        1.0 - (-5.0 * attack_progress).exp()
    } else if progress < release_start {
        // Decay/Sustain phase
        let decay_t = t - attack_time;
        (-decay_t * decay_rate).exp() * 0.95 + 0.05
    } else {
        // Release phase
        let release_progress = (progress - release_start) / (1.0 - release_start);
        let base_level = (-(t - attack_time) * decay_rate).exp() * 0.95 + 0.05;
        base_level * (1.0 - release_progress * release_progress)
    }
}

/// Clamps a value to the given range.
fn clamp(value: f64, min: f64, max: f64) -> f64 {
    value.max(min).min(max)
}

/// Converts i16 samples to raw PCM bytes (little-endian).
pub fn int16_to_bytes(samples: &[i16]) -> Vec<u8> {
    let mut data = vec![0u8; samples.len() * 2];
    for (i, &s) in samples.iter().enumerate() {
        let bytes = s.to_le_bytes();
        data[i * 2] = bytes[0];
        data[i * 2 + 1] = bytes[1];
    }
    data
}

/// Calculates the number of samples for a given duration in ms.
pub fn duration_samples(dur_ms: i32, sample_rate: u32) -> usize {
    (sample_rate as usize * dur_ms as usize) / 1000
}

/// Returns the total duration of a melody in milliseconds.
pub fn total_duration(notes: &[Note]) -> i32 {
    notes.iter().map(|n| n.dur).sum()
}

/// Converts a Voice to PCM bytes.
pub fn voice_to_bytes(voice: &Voice, format: Format, volume: f64) -> Vec<u8> {
    let sample_rate = format.sample_rate();
    let mut total_samples = 0usize;
    for n in &voice.notes {
        total_samples += duration_samples(n.dur, sample_rate);
    }

    let mut data = vec![0u8; total_samples * 2];
    let mut offset = 0;

    for n in &voice.notes {
        let samples = duration_samples(n.dur, sample_rate);
        let note_data = generate_rich_note(n.freq, samples, sample_rate, volume);

        for s in note_data {
            let bytes = s.to_le_bytes();
            data[offset] = bytes[0];
            data[offset + 1] = bytes[1];
            offset += 2;
        }
    }

    data
}

/// Returns a label for a voice index.
fn voice_label(i: usize) -> &'static str {
    match i {
        0 => "melody",
        1 => "bass",
        2 => "harmony",
        _ => "voice",
    }
}

impl Song {
    /// Renders a song to mixed PCM audio using pcm::Mixer.
    /// Returns the mixer which implements Read.
    pub fn render(&self, opts: RenderOptions) -> SongReader {
        let voices = self.to_voices(opts.metronome);
        if voices.is_empty() {
            return SongReader::empty();
        }

        let mixer = Mixer::new(opts.format, MixerOptions::default().with_auto_close());

        // Pre-generate all voice data
        let voice_data: Vec<Vec<u8>> = voices
            .iter()
            .map(|v| voice_to_bytes(v, opts.format, opts.volume))
            .collect();

        // Create tracks and write all data
        let mut handles = Vec::new();

        for (i, data) in voice_data.into_iter().enumerate() {
            let label = voice_label(i);
            if let Ok((track, ctrl)) = mixer.create_track(Some(TrackOptions::with_label(label))) {
                let handle = thread::spawn(move || {
                    let _ = track.write_bytes(&data);
                    ctrl.close_write();
                });
                handles.push(handle);
            }
        }

        // Spawn a thread to wait for writers and close mixer
        let mixer_clone = mixer.clone();
        thread::spawn(move || {
            for h in handles {
                let _ = h.join();
            }
            let _ = mixer_clone.close_write();
        });

        SongReader { mixer }
    }

    /// Renders a song to a byte vector.
    pub fn render_bytes(&self, opts: RenderOptions) -> Vec<u8> {
        let mut reader = self.render(opts);
        let mut data = Vec::new();
        let mut buf = [0u8; 4096];
        loop {
            match reader.read(&mut buf) {
                Ok(0) => break,
                Ok(n) => data.extend_from_slice(&buf[..n]),
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                Err(_) => break,
            }
        }
        data
    }
}

/// Reader for rendered song audio.
pub struct SongReader {
    mixer: Arc<Mixer>,
}

impl SongReader {
    fn empty() -> Self {
        Self {
            mixer: Mixer::new(DEFAULT_FORMAT, MixerOptions::default().with_auto_close()),
        }
    }
}

impl Read for SongReader {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        (&*self.mixer).read(buf)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_sine_wave() {
        let samples = 1600; // 100ms at 16kHz
        let data = generate_sine_wave(440.0, samples, 16000);
        assert_eq!(data.len(), samples * 2);

        // Check that we have non-zero samples
        let non_zero = data.iter().filter(|&&b| b != 0).count();
        assert!(non_zero > 0, "Should have non-zero samples");
    }

    #[test]
    fn test_generate_rich_note() {
        let samples = 1600;
        let data = generate_rich_note(440.0, samples, 16000, 0.5);
        assert_eq!(data.len(), samples);

        // Check that we have non-zero samples
        let non_zero = data.iter().filter(|&&s| s != 0).count();
        assert!(non_zero > 0, "Should have non-zero samples");
    }

    #[test]
    fn test_voice_to_bytes() {
        use super::super::types::Voice;
        
        let voice = Voice::new(vec![
            Note::new(440.0, 100),
            Note::new(880.0, 100),
        ]);

        let data = voice_to_bytes(&voice, Format::L16Mono16K, 0.5);
        let expected_samples = duration_samples(200, 16000);
        assert_eq!(data.len(), expected_samples * 2);
    }

    #[test]
    fn test_song_render() {
        use super::super::catalog::SONG_TWINKLE_STAR;

        let opts = RenderOptions::default().with_volume(0.3);
        let data = SONG_TWINKLE_STAR.render_bytes(opts);

        assert!(!data.is_empty(), "Rendered song should not be empty");
        
        // Check for non-zero audio
        let non_zero = data.iter().filter(|&&b| b != 0).count();
        assert!(non_zero > data.len() / 10, "Should have significant audio content");
    }
}
