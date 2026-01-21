//! Musical note frequencies.

// Octave 2
pub const C2: f64 = 65.0;
pub const D2: f64 = 73.0;
pub const E2: f64 = 82.0;
pub const F2: f64 = 87.0;
pub const G2: f64 = 98.0;
pub const A2: f64 = 110.0;
pub const B2: f64 = 123.0;

// Octave 3
pub const C3: f64 = 131.0;
pub const D3: f64 = 147.0;
pub const EB3: f64 = 156.0; // Eb3 / D#3
pub const E3: f64 = 165.0;
pub const F3: f64 = 175.0;
pub const FS3: f64 = 185.0; // F#3
pub const G3: f64 = 196.0;
pub const GS3: f64 = 208.0; // G#3
pub const A3: f64 = 220.0;
pub const AS3: f64 = 233.0; // A#3
pub const BB3: f64 = 233.0; // Bb3 (enharmonic with A#3)
pub const B3: f64 = 247.0;

// Octave 4
pub const C4: f64 = 262.0;
pub const CS4: f64 = 277.0; // C#4 / Db4
pub const D4: f64 = 294.0;
pub const DS4: f64 = 311.0; // D#4
pub const EB4: f64 = 311.0; // Eb4 (enharmonic with D#4)
pub const E4: f64 = 330.0;
pub const F4: f64 = 349.0;
pub const FS4: f64 = 370.0; // F#4
pub const G4: f64 = 392.0;
pub const GS4: f64 = 415.0; // G#4 / Ab4
pub const A4: f64 = 440.0;
pub const AS4: f64 = 466.0; // A#4
pub const BB4: f64 = 466.0; // Bb4 (enharmonic with A#4)
pub const B4: f64 = 494.0;

// Octave 5
pub const C5: f64 = 523.0;
pub const CS5: f64 = 554.0; // C#5
pub const D5: f64 = 587.0;
pub const DS5: f64 = 622.0; // D#5
pub const EB5: f64 = 622.0; // Eb5 (enharmonic with D#5)
pub const E5: f64 = 659.0;
pub const F5: f64 = 698.0;
pub const FS5: f64 = 740.0; // F#5
pub const G5: f64 = 784.0;
pub const GS5: f64 = 831.0; // G#5
pub const A5: f64 = 880.0;
pub const AS5: f64 = 932.0; // A#5 / Bb5
pub const B5: f64 = 988.0;

// Octave 6
pub const C6: f64 = 1047.0;

// Additional notes (enharmonic aliases)
pub const BB5: f64 = 932.0; // Bb5 (same as AS5)

// Rest (silence)
pub const REST: f64 = 0.0;

// Note value constants (in terms of beats, quarter note = 1)
pub const WHOLE: f64 = 4.0;      // 全音符
pub const HALF: f64 = 2.0;       // 二分音符
pub const QUARTER: f64 = 1.0;    // 四分音符
pub const EIGHTH: f64 = 0.5;     // 八分音符
pub const SIXTEENTH: f64 = 0.25; // 十六分音符
pub const DOT_WHOLE: f64 = 6.0;  // 附点全音符
pub const DOT_HALF: f64 = 3.0;   // 附点二分音符
pub const DOT_QUARTER: f64 = 1.5; // 附点四分音符
pub const DOT_EIGHTH: f64 = 0.75; // 附点八分音符
pub const TRIPLET8: f64 = 1.0 / 3.0; // 三连音八分音符
