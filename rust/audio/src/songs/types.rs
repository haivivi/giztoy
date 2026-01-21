//! Core types for song representation.

use super::notes::REST;

/// A single musical note with frequency and duration.
#[derive(Debug, Clone, Copy)]
pub struct Note {
    /// Frequency in Hz (use REST for silence)
    pub freq: f64,
    /// Duration in milliseconds
    pub dur: i32,
}

impl Note {
    /// Creates a new note.
    pub const fn new(freq: f64, dur: i32) -> Self {
        Self { freq, dur }
    }

    /// Returns true if this note is a rest (silence).
    pub fn is_rest(&self) -> bool {
        self.freq == REST
    }
}

/// A single voice/part in a multi-voice composition.
#[derive(Debug, Clone)]
pub struct Voice {
    pub notes: Vec<Note>,
}

impl Voice {
    /// Creates a new voice from notes.
    pub fn new(notes: Vec<Note>) -> Self {
        Self { notes }
    }

    /// Returns the total duration in milliseconds.
    pub fn total_duration(&self) -> i32 {
        self.notes.iter().map(|n| n.dur).sum()
    }
}

/// Time signature of a song.
#[derive(Debug, Clone, Copy)]
pub struct TimeSignature {
    /// Numerator: how many beats per measure (e.g., 4 for 4/4)
    pub beats_per_bar: i32,
    /// Denominator: note value that gets one beat (e.g., 4 for quarter note)
    pub beat_unit: i32,
}

impl TimeSignature {
    pub const fn new(beats_per_bar: i32, beat_unit: i32) -> Self {
        Self { beats_per_bar, beat_unit }
    }
}

// Common time signatures
pub const TIME_4_4: TimeSignature = TimeSignature::new(4, 4); // Common time
pub const TIME_3_4: TimeSignature = TimeSignature::new(3, 4); // Waltz time
pub const TIME_2_4: TimeSignature = TimeSignature::new(2, 4); // March time
pub const TIME_6_8: TimeSignature = TimeSignature::new(6, 8); // Compound duple

/// Tempo configuration for a song.
#[derive(Debug, Clone, Copy)]
pub struct Tempo {
    /// Beats per minute
    pub bpm: i32,
    /// Time signature
    pub signature: TimeSignature,
}

impl Tempo {
    pub const fn new(bpm: i32, signature: TimeSignature) -> Self {
        Self { bpm, signature }
    }

    /// Converts beat count to milliseconds.
    pub fn beat_duration(&self, beats: f64) -> i32 {
        (beats * 60000.0 / self.bpm as f64) as i32
    }

    /// Returns the duration of one full bar/measure in milliseconds.
    pub fn bar_duration(&self) -> i32 {
        self.beat_duration(self.signature.beats_per_bar as f64)
    }
}

/// A note defined by beat value instead of milliseconds.
#[derive(Debug, Clone, Copy)]
pub struct BeatNote {
    /// Frequency in Hz
    pub freq: f64,
    /// Duration in beats
    pub beats: f64,
}

impl BeatNote {
    /// Creates a new beat note.
    pub const fn new(freq: f64, beats: f64) -> Self {
        Self { freq, beats }
    }

    /// Converts to a Note using the given tempo.
    pub fn to_note(&self, tempo: Tempo) -> Note {
        Note {
            freq: self.freq,
            dur: tempo.beat_duration(self.beats),
        }
    }
}

/// Shorthand constructor for BeatNote.
pub const fn n(freq: f64, beats: f64) -> BeatNote {
    BeatNote::new(freq, beats)
}

/// A voice with beat-based notes.
#[derive(Debug, Clone)]
pub struct BeatVoice {
    pub notes: Vec<BeatNote>,
}

impl BeatVoice {
    /// Creates a new beat voice.
    pub fn new(notes: Vec<BeatNote>) -> Self {
        Self { notes }
    }

    /// Converts to a Voice using the given tempo.
    pub fn to_voice(&self, tempo: Tempo) -> Voice {
        Voice {
            notes: self.notes.iter().map(|bn| bn.to_note(tempo)).collect(),
        }
    }

    /// Calculates the total number of beats.
    pub fn total_beats(&self) -> f64 {
        self.notes.iter().map(|bn| bn.beats).sum()
    }
}

/// Metronome generator.
#[derive(Debug, Clone)]
pub struct Metronome {
    pub tempo: Tempo,
    pub total_beats: i32,
    pub high_freq: f64,
    pub low_freq: f64,
    pub click_dur: i32,
}

impl Metronome {
    /// Creates a metronome with sensible defaults.
    pub fn default_for(tempo: Tempo, total_beats: i32) -> Self {
        Self {
            tempo,
            total_beats,
            high_freq: 1200.0,
            low_freq: 800.0,
            click_dur: 30,
        }
    }

    /// Generates the metronome click track as a Voice.
    pub fn generate(&self) -> Voice {
        let beat_dur = self.tempo.beat_duration(1.0);
        let mut notes = Vec::with_capacity(self.total_beats as usize * 2);

        for beat in 0..self.total_beats {
            let beat_in_bar = beat % self.tempo.signature.beats_per_bar;
            let freq = if beat_in_bar == 0 {
                self.high_freq
            } else {
                self.low_freq
            };

            // Click sound
            notes.push(Note::new(freq, self.click_dur));
            // Rest until next beat
            let rest_dur = beat_dur - self.click_dur;
            if rest_dur > 0 {
                notes.push(Note::new(REST, rest_dur));
            }
        }

        Voice { notes }
    }
}

/// A complete song with tempo and multiple voices.
#[derive(Clone)]
pub struct Song {
    /// Unique identifier
    pub id: &'static str,
    /// Display name
    pub name: &'static str,
    /// Tempo configuration
    pub tempo: Tempo,
    /// Function that returns all voices
    voices_fn: fn() -> Vec<BeatVoice>,
}

impl Song {
    /// Creates a new song.
    pub const fn new(
        id: &'static str,
        name: &'static str,
        tempo: Tempo,
        voices_fn: fn() -> Vec<BeatVoice>,
    ) -> Self {
        Self { id, name, tempo, voices_fn }
    }

    /// Returns the voices for this song.
    pub fn voices(&self) -> Vec<BeatVoice> {
        (self.voices_fn)()
    }

    /// Converts to Voice array with optional metronome.
    pub fn to_voices(&self, with_metronome: bool) -> Vec<Voice> {
        let beat_voices = self.voices();
        let mut voices: Vec<Voice> = beat_voices
            .iter()
            .map(|bv| bv.to_voice(self.tempo))
            .collect();

        let max_beats = beat_voices.iter().map(|bv| bv.total_beats()).fold(0.0, f64::max);

        if with_metronome && max_beats > 0.0 {
            let metronome = Metronome::default_for(self.tempo, max_beats as i32);
            voices.push(metronome.generate());
        }

        voices
    }

    /// Returns the total duration of the song in milliseconds.
    pub fn duration(&self) -> i32 {
        let beat_voices = self.voices();
        let max_beats = beat_voices.iter().map(|bv| bv.total_beats()).fold(0.0, f64::max);
        self.tempo.beat_duration(max_beats)
    }

    /// Returns a song by its ID.
    pub fn by_id(id: &str) -> Option<&'static Song> {
        super::catalog::ALL_SONGS.iter().find(|s| s.id == id).copied()
    }

    /// Returns a song by its name.
    pub fn by_name(name: &str) -> Option<&'static Song> {
        super::catalog::ALL_SONGS.iter().find(|s| s.name == name).copied()
    }

    /// Returns all song IDs.
    pub fn ids() -> Vec<&'static str> {
        super::catalog::ALL_SONGS.iter().map(|s| s.id).collect()
    }

    /// Returns all song names.
    pub fn names() -> Vec<&'static str> {
        super::catalog::ALL_SONGS.iter().map(|s| s.name).collect()
    }
}

impl std::fmt::Debug for Song {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Song")
            .field("id", &self.id)
            .field("name", &self.name)
            .field("tempo", &self.tempo)
            .finish()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_note_new() {
        let note = Note::new(440.0, 500);
        assert_eq!(note.freq, 440.0);
        assert_eq!(note.dur, 500);
    }

    #[test]
    fn test_note_is_rest() {
        let rest = Note::new(REST, 100);
        assert!(rest.is_rest());

        let note = Note::new(440.0, 100);
        assert!(!note.is_rest());
    }

    #[test]
    fn test_voice_new() {
        let notes = vec![
            Note::new(440.0, 100),
            Note::new(880.0, 200),
        ];
        let voice = Voice::new(notes);
        assert_eq!(voice.notes.len(), 2);
    }

    #[test]
    fn test_voice_total_duration() {
        let notes = vec![
            Note::new(440.0, 100),
            Note::new(REST, 50),
            Note::new(880.0, 200),
        ];
        let voice = Voice::new(notes);
        assert_eq!(voice.total_duration(), 350);
    }

    #[test]
    fn test_time_signature_new() {
        let ts = TimeSignature::new(4, 4);
        assert_eq!(ts.beats_per_bar, 4);
        assert_eq!(ts.beat_unit, 4);
    }

    #[test]
    fn test_time_signature_constants() {
        assert_eq!(TIME_4_4.beats_per_bar, 4);
        assert_eq!(TIME_4_4.beat_unit, 4);
        assert_eq!(TIME_3_4.beats_per_bar, 3);
        assert_eq!(TIME_3_4.beat_unit, 4);
        assert_eq!(TIME_2_4.beats_per_bar, 2);
        assert_eq!(TIME_6_8.beats_per_bar, 6);
    }

    #[test]
    fn test_tempo_new() {
        let tempo = Tempo::new(120, TIME_4_4);
        assert_eq!(tempo.bpm, 120);
    }

    #[test]
    fn test_tempo_beat_duration() {
        let tempo = Tempo::new(120, TIME_4_4);
        // At 120 BPM, one beat = 500ms
        assert_eq!(tempo.beat_duration(1.0), 500);
        assert_eq!(tempo.beat_duration(2.0), 1000);
        assert_eq!(tempo.beat_duration(0.5), 250);
    }

    #[test]
    fn test_tempo_bar_duration() {
        let tempo = Tempo::new(120, TIME_4_4);
        // 4 beats at 500ms each = 2000ms
        assert_eq!(tempo.bar_duration(), 2000);
    }

    #[test]
    fn test_beat_note_new() {
        let bn = BeatNote::new(440.0, 1.0);
        assert_eq!(bn.freq, 440.0);
        assert_eq!(bn.beats, 1.0);
    }

    #[test]
    fn test_beat_note_to_note() {
        let tempo = Tempo::new(120, TIME_4_4);
        let bn = BeatNote::new(440.0, 1.0);
        let note = bn.to_note(tempo);
        assert_eq!(note.freq, 440.0);
        assert_eq!(note.dur, 500); // 1 beat at 120 BPM = 500ms
    }

    #[test]
    fn test_n_shorthand() {
        let bn = n(440.0, 2.0);
        assert_eq!(bn.freq, 440.0);
        assert_eq!(bn.beats, 2.0);
    }

    #[test]
    fn test_beat_voice_new() {
        let notes = vec![n(440.0, 1.0), n(880.0, 2.0)];
        let bv = BeatVoice::new(notes);
        assert_eq!(bv.notes.len(), 2);
    }

    #[test]
    fn test_beat_voice_total_beats() {
        let notes = vec![n(440.0, 1.0), n(880.0, 2.0), n(REST, 0.5)];
        let bv = BeatVoice::new(notes);
        assert_eq!(bv.total_beats(), 3.5);
    }

    #[test]
    fn test_beat_voice_to_voice() {
        let tempo = Tempo::new(120, TIME_4_4);
        let notes = vec![n(440.0, 1.0), n(880.0, 2.0)];
        let bv = BeatVoice::new(notes);
        let voice = bv.to_voice(tempo);
        assert_eq!(voice.notes.len(), 2);
        assert_eq!(voice.notes[0].dur, 500);
        assert_eq!(voice.notes[1].dur, 1000);
    }

    #[test]
    fn test_metronome_default_for() {
        let tempo = Tempo::new(120, TIME_4_4);
        let metro = Metronome::default_for(tempo, 8);
        assert_eq!(metro.total_beats, 8);
        assert_eq!(metro.high_freq, 1200.0);
        assert_eq!(metro.low_freq, 800.0);
        assert_eq!(metro.click_dur, 30);
    }

    #[test]
    fn test_metronome_generate() {
        let tempo = Tempo::new(120, TIME_4_4);
        let metro = Metronome::default_for(tempo, 4);
        let voice = metro.generate();
        // 4 beats, each with click + rest = 8 notes
        assert_eq!(voice.notes.len(), 8);
        // First click should be high freq (downbeat)
        assert_eq!(voice.notes[0].freq, 1200.0);
        // Third click should be low freq
        assert_eq!(voice.notes[2].freq, 800.0);
    }

    #[test]
    fn test_song_ids_and_names() {
        let ids = Song::ids();
        assert!(!ids.is_empty());
        
        let names = Song::names();
        assert!(!names.is_empty());
    }

    #[test]
    fn test_song_by_id() {
        let song = Song::by_id("twinkle_star");
        assert!(song.is_some());
        let s = song.unwrap();
        assert_eq!(s.id, "twinkle_star");
    }

    #[test]
    fn test_song_by_name() {
        let song = Song::by_name("小星星");
        assert!(song.is_some());
        let s = song.unwrap();
        assert_eq!(s.name, "小星星");
    }

    #[test]
    fn test_song_voices() {
        let song = Song::by_id("twinkle_star").unwrap();
        let voices = song.voices();
        assert!(!voices.is_empty());
    }

    #[test]
    fn test_song_to_voices() {
        let song = Song::by_id("twinkle_star").unwrap();
        let voices = song.to_voices(false);
        assert!(!voices.is_empty());

        let voices_with_metro = song.to_voices(true);
        assert_eq!(voices_with_metro.len(), voices.len() + 1);
    }

    #[test]
    fn test_song_duration() {
        let song = Song::by_id("twinkle_star").unwrap();
        let dur = song.duration();
        assert!(dur > 0);
    }

    #[test]
    fn test_song_debug() {
        let song = Song::by_id("twinkle_star").unwrap();
        let debug_str = format!("{:?}", song);
        assert!(debug_str.contains("twinkle_star"));
    }
}
