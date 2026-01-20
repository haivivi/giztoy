// Package songs provides built-in melodies for testing audio playback.
// All songs are defined using the beat-based notation system with tempo and time signature.
package songs

// Note frequencies (Hz) - Extended range
const (
	// Octave 2
	C2 = 65.0
	D2 = 73.0
	E2 = 82.0
	F2 = 87.0
	G2 = 98.0
	A2 = 110.0
	B2 = 123.0

	// Octave 3
	C3  = 131.0
	D3  = 147.0
	Eb3 = 156.0 // Eb3 / D#3
	E3  = 165.0
	F3  = 175.0
	Fs3 = 185.0 // F#3
	G3  = 196.0
	Gs3 = 208.0 // G#3
	A3  = 220.0
	As3 = 233.0 // A#3
	Bb3 = 233.0 // Bb3 (enharmonic with A#3)
	B3  = 247.0

	// Octave 4
	C4  = 262.0
	Cs4 = 277.0 // C#4 / Db4
	D4  = 294.0
	Ds4 = 311.0 // D#4
	Eb4 = 311.0 // Eb4 (enharmonic with D#4)
	E4  = 330.0
	F4  = 349.0
	Fs4 = 370.0 // F#4
	G4  = 392.0
	Gs4 = 415.0 // G#4 / Ab4
	A4  = 440.0
	As4 = 466.0 // A#4
	Bb4 = 466.0 // Bb4 (enharmonic with A#4)
	B4  = 494.0

	// Octave 5
	C5  = 523.0
	Cs5 = 554.0 // C#5
	D5  = 587.0
	Ds5 = 622.0 // D#5
	Eb5 = 622.0 // Eb5 (enharmonic with D#5)
	E5  = 659.0
	F5  = 698.0
	Fs5 = 740.0 // F#5
	G5  = 784.0
	Gs5 = 831.0 // G#5
	A5  = 880.0
	As5 = 932.0 // A#5 / Bb5
	B5  = 988.0

	// Octave 6
	C6 = 1047.0

	Rest = 0.0
)

// ========== Core Types ==========

// Note represents a single musical note with frequency and duration.
type Note struct {
	Freq float64 // Frequency in Hz (use Rest for silence)
	Dur  int     // Duration in milliseconds
}

// Voice represents a single voice/part in a multi-voice composition.
type Voice struct {
	Notes []Note
}

// ========== Tempo and Beat System ==========

// TimeSignature represents the time signature of a song.
type TimeSignature struct {
	BeatsPerBar int // Numerator: how many beats per measure (e.g., 4 for 4/4)
	BeatUnit    int // Denominator: note value that gets one beat (e.g., 4 for quarter note)
}

// Common time signatures
var (
	Time4_4 = TimeSignature{4, 4} // Common time (4/4)
	Time3_4 = TimeSignature{3, 4} // Waltz time (3/4)
	Time2_4 = TimeSignature{2, 4} // March time (2/4)
	Time6_8 = TimeSignature{6, 8} // Compound duple (6/8)
)

// Tempo represents the tempo configuration for a song.
type Tempo struct {
	BPM       int           // Beats per minute
	Signature TimeSignature // Time signature
}

// BeatDuration converts beat count to milliseconds.
func (t Tempo) BeatDuration(beats float64) int {
	return int(beats * 60000 / float64(t.BPM))
}

// BarDuration returns the duration of one full bar/measure in milliseconds.
func (t Tempo) BarDuration() int {
	return t.BeatDuration(float64(t.Signature.BeatsPerBar))
}

// Note value constants (in terms of beats, quarter note = 1)
const (
	Whole      = 4.0     // 全音符
	Half       = 2.0     // 二分音符
	Quarter    = 1.0     // 四分音符
	Eighth     = 0.5     // 八分音符
	Sixteenth  = 0.25    // 十六分音符
	DotWhole   = 6.0     // 附点全音符
	DotHalf    = 3.0     // 附点二分音符
	DotQuarter = 1.5     // 附点四分音符
	DotEighth  = 0.75    // 附点八分音符
	Triplet8   = 1.0 / 3 // 三连音八分音符
)

// BeatNote represents a note defined by beat value instead of milliseconds.
type BeatNote struct {
	Freq  float64 // Frequency in Hz
	Beats float64 // Duration in beats
}

// N is a shorthand constructor for BeatNote.
func N(freq float64, beats float64) BeatNote {
	return BeatNote{Freq: freq, Beats: beats}
}

// ToNote converts a BeatNote to a Note using the given tempo.
func (bn BeatNote) ToNote(tempo Tempo) Note {
	return Note{
		Freq: bn.Freq,
		Dur:  tempo.BeatDuration(bn.Beats),
	}
}

// BeatVoice represents a voice with beat-based notes.
type BeatVoice struct {
	Notes []BeatNote
}

// ToVoice converts a BeatVoice to a Voice using the given tempo.
func (bv BeatVoice) ToVoice(tempo Tempo) Voice {
	notes := make([]Note, len(bv.Notes))
	for i, bn := range bv.Notes {
		notes[i] = bn.ToNote(tempo)
	}
	return Voice{Notes: notes}
}

// TotalBeats calculates the total number of beats in a BeatVoice.
func (bv BeatVoice) TotalBeats() float64 {
	total := 0.0
	for _, bn := range bv.Notes {
		total += bn.Beats
	}
	return total
}

// ========== Metronome ==========

// Metronome generates a metronome click track.
type Metronome struct {
	Tempo      Tempo
	TotalBeats int     // Total number of beats to generate
	HighFreq   float64 // Frequency for downbeat (first beat of bar)
	LowFreq    float64 // Frequency for other beats
	ClickDur   int     // Duration of each click in ms
}

// DefaultMetronome creates a metronome with sensible defaults.
func DefaultMetronome(tempo Tempo, totalBeats int) Metronome {
	return Metronome{
		Tempo:      tempo,
		TotalBeats: totalBeats,
		HighFreq:   1200, // Higher pitch for downbeat
		LowFreq:    800,  // Lower pitch for other beats
		ClickDur:   30,   // Short click
	}
}

// Generate creates the metronome click track as a Voice.
func (m Metronome) Generate() Voice {
	beatDur := m.Tempo.BeatDuration(1)
	notes := make([]Note, 0, m.TotalBeats*2)

	for beat := 0; beat < m.TotalBeats; beat++ {
		// Determine if this is a downbeat (first beat of bar)
		beatInBar := beat % m.Tempo.Signature.BeatsPerBar
		freq := m.LowFreq
		if beatInBar == 0 {
			freq = m.HighFreq // Downbeat gets higher pitch
		}

		// Click sound
		notes = append(notes, Note{Freq: freq, Dur: m.ClickDur})
		// Rest until next beat
		restDur := beatDur - m.ClickDur
		if restDur > 0 {
			notes = append(notes, Note{Freq: Rest, Dur: restDur})
		}
	}

	return Voice{Notes: notes}
}

// ========== Song Definition ==========

// Song represents a complete song with tempo and multiple voices.
type Song struct {
	ID     string             // Unique identifier
	Name   string             // Display name
	Tempo  Tempo              // Tempo configuration
	Voices func() []BeatVoice // Function that returns all voices (melody + accompaniment)
}

// ToVoices converts a Song to []Voice with optional metronome.
func (s Song) ToVoices(withMetronome bool) []Voice {
	beatVoices := s.Voices()
	voices := make([]Voice, len(beatVoices))

	maxBeats := 0.0
	for i, bv := range beatVoices {
		voices[i] = bv.ToVoice(s.Tempo)
		if tb := bv.TotalBeats(); tb > maxBeats {
			maxBeats = tb
		}
	}

	// Add metronome track if requested
	if withMetronome && maxBeats > 0 {
		metronome := DefaultMetronome(s.Tempo, int(maxBeats))
		voices = append(voices, metronome.Generate())
	}

	return voices
}

// Duration returns the total duration of the song in milliseconds.
func (s Song) Duration() int {
	beatVoices := s.Voices()
	maxBeats := 0.0
	for _, bv := range beatVoices {
		if tb := bv.TotalBeats(); tb > maxBeats {
			maxBeats = tb
		}
	}
	return s.Tempo.BeatDuration(maxBeats)
}

// ========== All Songs ==========

// All contains all built-in songs with tempo-based notation.
var All = []Song{
	// Classic melodies
	SongTwinkleStar,
	SongHappyBirthday,
	SongTwoTigers,
	SongDollAndBear,
	// Classical piano pieces
	SongFurElise,
	SongCanon,
	SongCastleInTheSky,
	SongRiverFlowsInYou,
	SongDreamWedding,
	// Bach
	SongBachInvention1,
	SongBachMinuet,
	SongCanon3Voice,
	// Etudes & Exercises
	SongCzerny599No1,
	SongCzerny599No19,
	SongCzerny599No38,
	SongCzerny299No1,
	SongHanonNo1,
	SongBurgmullerArabesque,
	// Scales
	SongScaleC,
	SongScaleGMinor,
	// Dance & Rhythm
	SongSimpleWaltz,
	SongTarantella,
}

// ByID returns a song by its ID, or nil if not found.
func ByID(id string) *Song {
	for i := range All {
		if All[i].ID == id {
			return &All[i]
		}
	}
	return nil
}

// ByName returns a song by its name (exact match), or nil if not found.
func ByName(name string) *Song {
	for i := range All {
		if All[i].Name == name {
			return &All[i]
		}
	}
	return nil
}

// IDs returns all song IDs.
func IDs() []string {
	ids := make([]string, len(All))
	for i, s := range All {
		ids[i] = s.ID
	}
	return ids
}

// Names returns all song names.
func Names() []string {
	names := make([]string, len(All))
	for i, s := range All {
		names[i] = s.Name
	}
	return names
}

// ========== Song Definitions ==========

// SongTwinkleStar - 小星星 (Twinkle Twinkle Little Star)
var SongTwinkleStar = Song{
	ID:    "twinkle_star",
	Name:  "小星星",
	Tempo: Tempo{BPM: 100, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// 一闪一闪亮晶晶 (Bar 1-2)
			N(C4, Quarter), N(C4, Quarter), N(G4, Quarter), N(G4, Quarter),
			N(A4, Quarter), N(A4, Quarter), N(G4, Half),
			// 满天都是小星星 (Bar 3-4)
			N(F4, Quarter), N(F4, Quarter), N(E4, Quarter), N(E4, Quarter),
			N(D4, Quarter), N(D4, Quarter), N(C4, Half),
			// 挂在天空放光明 (Bar 5-6)
			N(G4, Quarter), N(G4, Quarter), N(F4, Quarter), N(F4, Quarter),
			N(E4, Quarter), N(E4, Quarter), N(D4, Half),
			// 好像许多小眼睛 (Bar 7-8)
			N(G4, Quarter), N(G4, Quarter), N(F4, Quarter), N(F4, Quarter),
			N(E4, Quarter), N(E4, Quarter), N(D4, Half),
			// Repeat 一闪一闪亮晶晶 (Bar 9-10)
			N(C4, Quarter), N(C4, Quarter), N(G4, Quarter), N(G4, Quarter),
			N(A4, Quarter), N(A4, Quarter), N(G4, Half),
			// 满天都是小星星 (Bar 11-12)
			N(F4, Quarter), N(F4, Quarter), N(E4, Quarter), N(E4, Quarter),
			N(D4, Quarter), N(D4, Quarter), N(C4, Half),
		}}

		// Accompaniment: simple bass + chord
		accomp := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: C - G/B
			N(C3, Quarter), N(E3, Quarter), N(G3, Quarter), N(E3, Quarter),
			N(F3, Quarter), N(A3, Quarter), N(C3, Half),
			// Bar 3-4: F - C - G - C
			N(F3, Quarter), N(A3, Quarter), N(C3, Quarter), N(E3, Quarter),
			N(G3, Quarter), N(B3, Quarter), N(C3, Half),
			// Bar 5-6: C - F - C - G
			N(C3, Quarter), N(E3, Quarter), N(F3, Quarter), N(A3, Quarter),
			N(C3, Quarter), N(E3, Quarter), N(G3, Half),
			// Bar 7-8
			N(C3, Quarter), N(E3, Quarter), N(F3, Quarter), N(A3, Quarter),
			N(C3, Quarter), N(E3, Quarter), N(G3, Half),
			// Bar 9-10
			N(C3, Quarter), N(E3, Quarter), N(G3, Quarter), N(E3, Quarter),
			N(F3, Quarter), N(A3, Quarter), N(C3, Half),
			// Bar 11-12
			N(F3, Quarter), N(A3, Quarter), N(C3, Quarter), N(E3, Quarter),
			N(G3, Quarter), N(B3, Quarter), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongHappyBirthday - 生日快乐 (Happy Birthday)
var SongHappyBirthday = Song{
	ID:    "happy_birthday",
	Name:  "生日快乐",
	Tempo: Tempo{BPM: 120, Signature: Time3_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Happy birthday to you (anacrusis + Bar 1-2)
			N(C4, Eighth), N(C4, Eighth), N(D4, Quarter), N(C4, Quarter), N(F4, Quarter), N(E4, Half),
			// Happy birthday to you (Bar 3-4)
			N(C4, Eighth), N(C4, Eighth), N(D4, Quarter), N(C4, Quarter), N(G4, Quarter), N(F4, Half),
			// Happy birthday dear friend (Bar 5-6)
			N(C4, Eighth), N(C4, Eighth), N(C5, Quarter), N(A4, Quarter), N(F4, Quarter), N(E4, Quarter), N(D4, Half),
			// Happy birthday to you (Bar 7-8)
			N(As4, Eighth), N(As4, Eighth), N(A4, Quarter), N(F4, Quarter), N(G4, Quarter), N(F4, DotHalf),
		}}

		// Waltz-style accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Anacrusis rest + Bar 1-2: F - C
			N(Rest, Quarter), N(F3, Quarter), N(A3, Quarter), N(C4, Quarter), N(C3, Quarter), N(G3, Quarter), N(C4, Quarter),
			// Bar 3-4: F - C
			N(Rest, Quarter), N(F3, Quarter), N(A3, Quarter), N(C4, Quarter), N(C3, Quarter), N(G3, Quarter), N(C4, Quarter),
			// Bar 5-6: F - C/E - Dm - G
			N(Rest, Quarter), N(F3, Quarter), N(A3, Quarter), N(C4, Quarter), N(E3, Quarter), N(G3, Quarter), N(C4, Quarter), N(D3, Quarter), N(F3, Quarter), N(A3, Quarter),
			// Bar 7-8: Bb - F - C - F
			N(Rest, Quarter), N(As3, Quarter), N(D4, Quarter), N(F4, Quarter), N(F3, Quarter), N(A3, Quarter), N(C4, Quarter), N(F3, DotHalf),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongTwoTigers - 两只老虎 (Two Tigers / Frère Jacques)
var SongTwoTigers = Song{
	ID:    "two_tigers",
	Name:  "两只老虎",
	Tempo: Tempo{BPM: 120, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// 两只老虎 两只老虎 (Bar 1-2)
			N(C4, Quarter), N(D4, Quarter), N(E4, Quarter), N(C4, Quarter),
			N(C4, Quarter), N(D4, Quarter), N(E4, Quarter), N(C4, Quarter),
			// 跑得快 跑得快 (Bar 3-4)
			N(E4, Quarter), N(F4, Quarter), N(G4, Half),
			N(E4, Quarter), N(F4, Quarter), N(G4, Half),
			// 一只没有眼睛 一只没有尾巴 (Bar 5-6)
			N(G4, Eighth), N(A4, Eighth), N(G4, Eighth), N(F4, Eighth), N(E4, Quarter), N(C4, Quarter),
			N(G4, Eighth), N(A4, Eighth), N(G4, Eighth), N(F4, Eighth), N(E4, Quarter), N(C4, Quarter),
			// 真奇怪 真奇怪 (Bar 7-8)
			N(C4, Quarter), N(G3, Quarter), N(C4, Half),
			N(C4, Quarter), N(G3, Quarter), N(C4, Half),
		}}

		// Simple bass line
		accomp := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: C - Am
			N(C3, Half), N(E3, Half),
			N(A3, Half), N(E3, Half),
			// Bar 3-4: C - G
			N(C3, Half), N(G3, Half),
			N(C3, Half), N(G3, Half),
			// Bar 5-6: C - G
			N(C3, Half), N(G3, Half),
			N(C3, Half), N(G3, Half),
			// Bar 7-8: C - G - C
			N(C3, Quarter), N(G3, Quarter), N(C3, Half),
			N(C3, Quarter), N(G3, Quarter), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongDollAndBear - 洋娃娃和小熊跳舞 (Doll and Bear Dancing)
var SongDollAndBear = Song{
	ID:    "doll_and_bear",
	Name:  "洋娃娃和小熊跳舞",
	Tempo: Tempo{BPM: 132, Signature: Time2_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// 洋娃娃和小熊跳舞 跳呀跳呀 一二一 (Phrase 1)
			N(C4, Eighth), N(D4, Eighth), N(E4, Eighth), N(C4, Eighth), N(E4, Eighth), N(C4, Eighth), N(E4, Quarter),
			N(D4, Eighth), N(E4, Eighth), N(F4, Eighth), N(F4, Eighth), N(E4, Eighth), N(D4, Eighth), N(E4, Quarter),
			// 他们在跳圆圈舞呀 跳呀跳呀 一二一 (Phrase 2)
			N(C4, Eighth), N(D4, Eighth), N(E4, Eighth), N(C4, Eighth), N(E4, Eighth), N(C4, Eighth), N(E4, Quarter),
			N(D4, Eighth), N(E4, Eighth), N(F4, Eighth), N(E4, Eighth), N(D4, Quarter), N(C4, Quarter),
			// 小熊小熊点点头呀 点点头呀 一二一 (Phrase 3)
			N(G4, Eighth), N(G4, Eighth), N(E4, Eighth), N(G4, Eighth), N(F4, Eighth), N(F4, Eighth), N(D4, Quarter),
			N(E4, Eighth), N(F4, Eighth), N(E4, Eighth), N(C4, Eighth), N(D4, Quarter), N(Rest, Quarter),
			// 小洋娃娃笑起来呀 笑呀笑呀 哈哈哈 (Phrase 4)
			N(G4, Eighth), N(G4, Eighth), N(E4, Eighth), N(G4, Eighth), N(F4, Eighth), N(E4, Eighth), N(D4, Quarter),
			N(C4, Eighth), N(D4, Eighth), N(E4, Eighth), N(D4, Eighth), N(C4, Half),
		}}

		// March-style accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Phrase 1-2
			N(C3, Quarter), N(G3, Quarter), N(C3, Quarter), N(G3, Quarter),
			N(G3, Quarter), N(D3, Quarter), N(C3, Quarter), N(G3, Quarter),
			N(C3, Quarter), N(G3, Quarter), N(C3, Quarter), N(G3, Quarter),
			N(G3, Quarter), N(D3, Quarter), N(C3, Half),
			// Phrase 3-4
			N(C3, Quarter), N(G3, Quarter), N(D3, Quarter), N(G3, Quarter),
			N(C3, Quarter), N(G3, Quarter), N(G3, Half),
			N(C3, Quarter), N(G3, Quarter), N(G3, Quarter), N(D3, Quarter),
			N(C3, Quarter), N(G3, Quarter), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongFurElise - 献给爱丽丝 (Für Elise)
var SongFurElise = Song{
	ID:    "fur_elise",
	Name:  "献给爱丽丝",
	Tempo: Tempo{BPM: 140, Signature: Time3_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Theme A - Main motif
			N(E5, Eighth), N(Ds5, Eighth), N(E5, Eighth), N(Ds5, Eighth), N(E5, Eighth), N(B4, Eighth), N(D5, Eighth), N(C5, Eighth),
			N(A4, Quarter), N(Rest, Eighth), N(C4, Eighth), N(E4, Eighth), N(A4, Eighth),
			N(B4, Quarter), N(Rest, Eighth), N(E4, Eighth), N(Gs4, Eighth), N(B4, Eighth),
			N(C5, Quarter), N(Rest, Eighth), N(E4, Eighth), N(E5, Eighth), N(Ds5, Eighth),
			// Theme A repeat
			N(E5, Eighth), N(Ds5, Eighth), N(E5, Eighth), N(B4, Eighth), N(D5, Eighth), N(C5, Eighth),
			N(A4, Quarter), N(Rest, Eighth), N(C4, Eighth), N(E4, Eighth), N(A4, Eighth),
			N(B4, Quarter), N(Rest, Eighth), N(E4, Eighth), N(C5, Eighth), N(B4, Eighth),
			N(A4, DotQuarter), N(Rest, DotQuarter),
		}}

		// Left hand accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Intro rhythm pattern
			N(Rest, DotHalf),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(Rest, DotQuarter),
			N(E3, Eighth), N(E4, Eighth), N(Gs4, Eighth), N(Rest, DotQuarter),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(Rest, DotQuarter),
			// Repeat
			N(Rest, DotHalf),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(Rest, DotQuarter),
			N(E3, Eighth), N(E4, Eighth), N(Gs4, Eighth), N(Rest, DotQuarter),
			N(A3, DotQuarter), N(Rest, DotQuarter),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongCanon - 卡农 (Pachelbel's Canon in D)
var SongCanon = Song{
	ID:    "canon",
	Name:  "卡农",
	Tempo: Tempo{BPM: 60, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Main melody (Violin 1)
		melody := BeatVoice{Notes: []BeatNote{
			// Main theme
			N(Fs5, Half), N(E5, Half), N(D5, Half), N(Cs5, Half),
			N(B4, Half), N(A4, Half), N(B4, Half), N(Cs5, Half),
			// Variation
			N(D5, Quarter), N(Fs5, Quarter), N(A5, Quarter), N(G5, Quarter),
			N(Fs5, Quarter), N(D5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(D5, Quarter), N(B4, Quarter), N(D5, Quarter), N(A4, Quarter),
			N(G4, Quarter), N(B4, Quarter), N(A4, Quarter), N(G4, Quarter),
			// Continuation
			N(Fs4, Quarter), N(D4, Quarter), N(E4, Quarter), N(Fs4, Quarter),
			N(G4, Quarter), N(A4, Quarter), N(B4, Quarter), N(G4, Quarter),
			N(Fs4, Half), N(D5, Half), N(D5, Whole),
		}}

		// Ground bass
		bass := BeatVoice{Notes: []BeatNote{
			// D - A - Bm - F#m - G - D - G - A (repeated)
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(G3, Half), N(A3, Half),
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(G3, Half), N(A3, Half),
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(D3, Whole),
		}}

		return []BeatVoice{melody, bass}
	},
}

// SongCastleInTheSky - 天空之城 (Laputa: Castle in the Sky)
var SongCastleInTheSky = Song{
	ID:    "castle_in_sky",
	Name:  "天空之城",
	Tempo: Tempo{BPM: 80, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Intro
			N(A4, Quarter), N(B4, Eighth), N(C5, DotQuarter), N(B4, Quarter), N(C5, Quarter), N(E5, Half),
			N(Rest, Eighth), N(G4, Eighth), N(A4, Quarter), N(G4, Eighth), N(A4, Eighth), N(C5, Half),
			N(Rest, Eighth), N(G4, Quarter), N(E4, Quarter), N(E4, Eighth), N(D4, Eighth), N(E4, DotHalf),
			N(Rest, Quarter),
			// Main theme
			N(A4, Quarter), N(B4, Eighth), N(C5, DotQuarter), N(B4, Quarter), N(C5, Quarter), N(E5, Half),
			N(Rest, Eighth), N(G4, Eighth), N(A4, Quarter), N(G4, Eighth), N(A4, Eighth), N(C5, Half),
			N(Rest, Eighth), N(G4, Quarter), N(E4, Quarter), N(D4, Eighth), N(C4, Eighth), N(D4, DotHalf),
			N(Rest, Quarter),
			// Climax
			N(E5, Quarter), N(E5, Eighth), N(E5, Quarter), N(B4, Eighth), N(C5, Half),
			N(C5, Quarter), N(C5, Eighth), N(C5, Eighth), N(B4, Eighth), N(A4, Half),
			N(Rest, Eighth), N(A4, Quarter), N(C5, Quarter), N(B4, Eighth), N(B4, Eighth), N(E4, Half),
			N(A4, DotHalf), N(Rest, Quarter),
		}}

		// Arpeggiated accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Am pattern
			N(A3, Quarter), N(E4, Eighth), N(A4, Eighth), N(E4, Quarter), N(A3, Quarter), N(E4, Eighth), N(A4, Eighth), N(E4, Quarter),
			N(F3, Quarter), N(C4, Eighth), N(F4, Eighth), N(C4, Quarter), N(C3, Quarter), N(G3, Eighth), N(C4, Eighth), N(G3, Quarter),
			N(Am, Quarter), N(E4, Eighth), N(A4, Eighth), N(E4, Quarter), N(E3, Quarter), N(B3, Eighth), N(E4, Eighth), N(B3, Quarter),
			N(A3, Half), N(Rest, Half),
			// Repeat
			N(A3, Quarter), N(E4, Eighth), N(A4, Eighth), N(E4, Quarter), N(A3, Quarter), N(E4, Eighth), N(A4, Eighth), N(E4, Quarter),
			N(F3, Quarter), N(C4, Eighth), N(F4, Eighth), N(C4, Quarter), N(C3, Quarter), N(G3, Eighth), N(C4, Eighth), N(G3, Quarter),
			N(G3, Quarter), N(D4, Eighth), N(G4, Eighth), N(D4, Quarter), N(G3, Quarter), N(D4, Eighth), N(G4, Eighth), N(D4, Quarter),
			N(Rest, Whole),
			// Climax
			N(A3, Half), N(E3, Half),
			N(F3, Half), N(C3, Half),
			N(A3, Half), N(E3, Half),
			N(A3, DotHalf), N(Rest, Quarter),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// Am is A minor root note (alias for readability)
const Am = A3

// SongRiverFlowsInYou - River Flows in You (Yiruma)
var SongRiverFlowsInYou = Song{
	ID:    "river_flows",
	Name:  "River Flows in You",
	Tempo: Tempo{BPM: 70, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Intro arpeggio pattern
			N(A4, Quarter), N(E5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(A4, Quarter), N(E5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(Gs4, Quarter), N(E5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(Gs4, Quarter), N(E5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			// Main melody
			N(Fs5, DotQuarter), N(E5, Eighth), N(Cs5, Eighth), N(E5, DotQuarter), N(Fs5, Eighth), N(E5, Eighth),
			N(Cs5, DotQuarter), N(B4, Eighth), N(A4, Eighth), N(B4, DotQuarter), N(Cs5, Eighth), N(B4, Eighth),
			N(A4, Half), N(Gs4, Quarter), N(A4, Quarter), N(B4, Half),
			// Second phrase
			N(Cs5, DotQuarter), N(B4, Eighth), N(A4, Eighth), N(B4, DotQuarter), N(Cs5, Eighth), N(B4, Eighth),
			N(A4, DotQuarter), N(Gs4, Eighth), N(A4, Eighth), N(B4, DotQuarter), N(A4, Eighth), N(Gs4, Eighth),
			N(Fs4, Half), N(E4, Half), N(Fs4, Whole),
		}}

		// Flowing left hand
		accomp := BeatVoice{Notes: []BeatNote{
			// A major pattern
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth),
			N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			// Fs minor
			N(Fs3, Eighth), N(Cs4, Eighth), N(Fs4, Eighth), N(Cs4, Eighth), N(Fs3, Eighth), N(Cs4, Eighth), N(Fs4, Eighth), N(Cs4, Eighth),
			N(Fs3, Eighth), N(Cs4, Eighth), N(Fs4, Eighth), N(Cs4, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			// End
			N(Fs3, Eighth), N(Cs4, Eighth), N(Fs4, Eighth), N(Cs4, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			N(A3, Whole), N(A3, Whole),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongBachInvention1 - 巴赫二部创意曲 No.1 (BWV 772)
var SongBachInvention1 = Song{
	ID:    "bach_invention1",
	Name:  "巴赫二部创意曲 No.1",
	Tempo: Tempo{BPM: 100, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Right hand (Soprano)
		soprano := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: Main theme
			N(C4, Sixteenth), N(D4, Sixteenth), N(E4, Sixteenth), N(F4, Sixteenth), N(D4, Sixteenth), N(E4, Sixteenth), N(C4, Eighth),
			N(Rest, Eighth), N(G4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(G4, Eighth),
			// Bar 3-4
			N(Rest, Eighth), N(C5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth), N(C5, Eighth),
			N(A4, Sixteenth), N(G4, Sixteenth), N(F4, Sixteenth), N(E4, Sixteenth), N(F4, Sixteenth), N(A4, Sixteenth), N(G4, Quarter),
			// Bar 5-6
			N(G4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(G4, Eighth),
			N(Rest, Eighth), N(D5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth), N(G5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth), N(D5, Eighth),
			// Bar 7-8 (ending)
			N(E5, Quarter), N(D5, Quarter), N(C5, Quarter), N(B4, Quarter),
			N(C5, Half),
		}}

		// Left hand (Bass)
		bass := BeatVoice{Notes: []BeatNote{
			// Bar 1: Rest then theme
			N(Rest, DotQuarter),
			N(C3, Sixteenth), N(D3, Sixteenth), N(E3, Sixteenth), N(F3, Sixteenth), N(D3, Sixteenth), N(E3, Sixteenth), N(C3, Eighth),
			// Bar 2
			N(Rest, Eighth), N(G3, Sixteenth), N(A3, Sixteenth), N(B3, Sixteenth), N(C4, Sixteenth), N(A3, Sixteenth), N(B3, Sixteenth), N(G3, Eighth),
			// Bar 3-4
			N(Rest, Eighth), N(E3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth), N(A3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth), N(E3, Eighth),
			N(F3, Sixteenth), N(E3, Sixteenth), N(D3, Sixteenth), N(C3, Sixteenth), N(D3, Sixteenth), N(F3, Sixteenth), N(E3, Quarter),
			// Bar 5-6
			N(E3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth), N(A3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth), N(E3, Eighth),
			N(Rest, Eighth), N(B3, Sixteenth), N(C4, Sixteenth), N(D4, Sixteenth), N(E4, Sixteenth), N(C4, Sixteenth), N(D4, Sixteenth), N(B3, Eighth),
			// Bar 7-8
			N(C4, Quarter), N(G3, Quarter), N(E3, Quarter), N(G3, Quarter),
			N(C3, Half),
		}}

		return []BeatVoice{soprano, bass}
	},
}

// SongBachMinuet - 巴赫小步舞曲 (BWV Anh. 114)
var SongBachMinuet = Song{
	ID:    "bach_minuet",
	Name:  "巴赫小步舞曲",
	Tempo: Tempo{BPM: 110, Signature: Time3_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-4
			N(D5, Quarter), N(G4, Quarter), N(A4, Quarter), N(B4, Quarter), N(C5, Quarter), N(D5, Quarter),
			N(G4, Half), N(G4, Quarter), N(E5, Quarter), N(C5, Quarter), N(D5, Quarter),
			N(E5, Quarter), N(Fs5, Quarter), N(G5, Quarter), N(G4, Half), N(G4, Quarter),
			N(C5, Quarter), N(D5, Quarter), N(C5, Quarter), N(B4, Quarter), N(A4, Quarter), N(B4, Quarter),
			// Bar 5-8
			N(A4, Half), N(D4, Quarter), N(D5, Quarter), N(G4, Quarter), N(A4, Quarter),
			N(B4, Quarter), N(C5, Quarter), N(D5, Quarter), N(G4, Half), N(G4, Quarter),
			N(E5, Quarter), N(C5, Quarter), N(D5, Quarter), N(E5, Quarter), N(Fs5, Quarter), N(G5, Quarter),
			N(G4, Half), N(G4, Quarter),
			// Ending
			N(B4, Quarter), N(G4, Quarter), N(A4, Quarter), N(B4, Quarter), N(G4, Quarter), N(Fs4, Quarter),
			N(G4, DotHalf),
		}}

		// Left hand
		bass := BeatVoice{Notes: []BeatNote{
			// Bar 1-4
			N(G3, Half), N(Rest, Quarter), N(G3, Half), N(Rest, Quarter),
			N(B3, Half), N(Rest, Quarter), N(C4, Half), N(Rest, Quarter),
			N(D4, Half), N(Rest, Quarter), N(B3, Half), N(Rest, Quarter),
			N(C4, Half), N(Rest, Quarter), N(D4, Half), N(Rest, Quarter),
			// Bar 5-8
			N(D3, DotHalf), N(G3, Half), N(Rest, Quarter),
			N(G3, Half), N(Rest, Quarter), N(E3, Half), N(Rest, Quarter),
			N(C4, Half), N(Rest, Quarter), N(D4, Half), N(Rest, Quarter),
			N(B3, Half), N(Rest, Quarter),
			// Ending
			N(E3, Half), N(Rest, Quarter), N(D3, Half), N(Rest, Quarter),
			N(G3, DotHalf),
		}}

		return []BeatVoice{melody, bass}
	},
}

// SongCanon3Voice - 卡农三声部
var SongCanon3Voice = Song{
	ID:    "canon_3voice",
	Name:  "卡农三声部",
	Tempo: Tempo{BPM: 60, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Voice 1 - enters first
		voice1 := BeatVoice{Notes: []BeatNote{
			// Main theme
			N(Fs5, Half), N(E5, Half), N(D5, Half), N(Cs5, Half),
			N(B4, Half), N(A4, Half), N(B4, Half), N(Cs5, Half),
			// Variation
			N(D5, Quarter), N(Fs5, Quarter), N(A5, Quarter), N(G5, Quarter), N(Fs5, Quarter), N(D5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(D5, Quarter), N(B4, Quarter), N(D5, Quarter), N(A4, Quarter), N(G4, Quarter), N(B4, Quarter), N(A4, Quarter), N(G4, Quarter),
			// Continuation
			N(Fs4, Quarter), N(D4, Quarter), N(E4, Quarter), N(Fs4, Quarter), N(G4, Quarter), N(A4, Quarter), N(B4, Quarter), N(G4, Quarter),
			N(Fs4, Half), N(D5, Half), N(D5, Whole),
		}}

		// Voice 2 - enters 2 bars later
		voice2 := BeatVoice{Notes: []BeatNote{
			// Wait 2 bars
			N(Rest, Whole), N(Rest, Whole),
			// Main theme (delayed)
			N(Fs5, Half), N(E5, Half), N(D5, Half), N(Cs5, Half),
			N(B4, Half), N(A4, Half), N(B4, Half), N(Cs5, Half),
			// Variation
			N(D5, Quarter), N(Fs5, Quarter), N(A5, Quarter), N(G5, Quarter), N(Fs5, Quarter), N(D5, Quarter), N(Fs5, Quarter), N(E5, Quarter),
			N(D5, Half), N(A4, Half), N(A4, Whole),
		}}

		// Voice 3 - ground bass
		voice3 := BeatVoice{Notes: []BeatNote{
			// Ground bass (D - A - Bm - F#m - G - D - G - A)
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(G3, Half), N(A3, Half),
			// Repeat
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(G3, Half), N(A3, Half),
			// Third time
			N(D3, Half), N(A3, Half), N(B3, Half), N(Fs3, Half),
			N(G3, Half), N(D3, Half), N(D3, Whole),
		}}

		return []BeatVoice{voice1, voice2, voice3}
	},
}

// SongSimpleWaltz - 简单圆舞曲
var SongSimpleWaltz = Song{
	ID:    "simple_waltz",
	Name:  "简单圆舞曲",
	Tempo: Tempo{BPM: 120, Signature: Time3_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-2
			N(E5, DotQuarter), N(D5, Eighth), N(C5, Quarter),
			N(D5, DotQuarter), N(E5, Eighth), N(D5, Quarter),
			// Bar 3-4
			N(C5, DotQuarter), N(B4, Eighth), N(A4, Quarter),
			N(B4, Half), N(Rest, Quarter),
			// Bar 5-6
			N(E5, DotQuarter), N(D5, Eighth), N(C5, Quarter),
			N(D5, DotQuarter), N(E5, Eighth), N(D5, Quarter),
			// Bar 7-8 (ending)
			N(C5, Half), N(B4, Quarter),
			N(C5, DotHalf),
		}}

		// Waltz bass: "oom-pah-pah" pattern
		bass := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: C major, G major
			N(C3, Quarter), N(E4, Quarter), N(G4, Quarter),
			N(G3, Quarter), N(D4, Quarter), N(G4, Quarter),
			// Bar 3-4: A minor, E major
			N(A3, Quarter), N(E4, Quarter), N(A4, Quarter),
			N(E3, Quarter), N(Gs3, Quarter), N(B3, Quarter),
			// Bar 5-6: C major, G major
			N(C3, Quarter), N(E4, Quarter), N(G4, Quarter),
			N(G3, Quarter), N(D4, Quarter), N(G4, Quarter),
			// Bar 7-8: F major -> C major
			N(F3, Quarter), N(A3, Quarter), N(C4, Quarter),
			N(C3, DotHalf),
		}}

		return []BeatVoice{melody, bass}
	},
}

// SongDreamWedding - 梦中的婚礼 (Mariage d'Amour)
var SongDreamWedding = Song{
	ID:    "dream_wedding",
	Name:  "梦中的婚礼",
	Tempo: Tempo{BPM: 70, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Intro/Theme
			N(E5, Quarter), N(E5, Eighth), N(D5, Eighth), N(E5, Half),
			N(E5, Quarter), N(E5, Eighth), N(F5, Eighth), N(E5, Quarter), N(D5, Quarter),
			N(C5, Quarter), N(B4, Eighth), N(A4, Eighth), N(B4, Half),
			N(Rest, Whole),
			// Second phrase
			N(E5, Quarter), N(E5, Eighth), N(D5, Eighth), N(E5, Half),
			N(E5, Quarter), N(E5, Eighth), N(F5, Eighth), N(E5, Quarter), N(D5, Quarter),
			N(C5, Quarter), N(D5, Quarter), N(E5, Quarter), N(C5, Quarter),
			N(A4, Whole),
		}}

		// Arpeggiated accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Am - G - F - E pattern
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth),
			N(G3, Eighth), N(D4, Eighth), N(G4, Eighth), N(D4, Eighth), N(G3, Eighth), N(D4, Eighth), N(G4, Eighth), N(D4, Eighth),
			N(F3, Eighth), N(C4, Eighth), N(F4, Eighth), N(C4, Eighth), N(F3, Eighth), N(C4, Eighth), N(F4, Eighth), N(C4, Eighth),
			N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth), N(E3, Eighth), N(B3, Eighth), N(E4, Eighth), N(B3, Eighth),
			// Repeat
			N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth), N(A3, Eighth), N(E4, Eighth), N(A4, Eighth), N(E4, Eighth),
			N(G3, Eighth), N(D4, Eighth), N(G4, Eighth), N(D4, Eighth), N(G3, Eighth), N(D4, Eighth), N(G4, Eighth), N(D4, Eighth),
			N(F3, Eighth), N(C4, Eighth), N(F4, Eighth), N(C4, Eighth), N(G3, Eighth), N(D4, Eighth), N(G4, Eighth), N(D4, Eighth),
			N(A3, Whole),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// ========== Czerny Etudes (车尔尼练习曲) ==========

// SongCzerny599No1 - 车尔尼 Op.599 No.1 (初学者练习曲)
var SongCzerny599No1 = Song{
	ID:    "czerny_599_1",
	Name:  "车尔尼 599-1",
	Tempo: Tempo{BPM: 100, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Right hand - simple scale exercise
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: C major scale up
			N(C4, Quarter), N(D4, Quarter), N(E4, Quarter), N(F4, Quarter),
			N(G4, Quarter), N(A4, Quarter), N(B4, Quarter), N(C5, Quarter),
			// Bar 3-4: scale down
			N(C5, Quarter), N(B4, Quarter), N(A4, Quarter), N(G4, Quarter),
			N(F4, Quarter), N(E4, Quarter), N(D4, Quarter), N(C4, Quarter),
			// Bar 5-6: broken thirds
			N(C4, Eighth), N(E4, Eighth), N(D4, Eighth), N(F4, Eighth), N(E4, Eighth), N(G4, Eighth), N(F4, Eighth), N(A4, Eighth),
			N(G4, Eighth), N(B4, Eighth), N(A4, Eighth), N(C5, Eighth), N(B4, Eighth), N(D5, Eighth), N(C5, Quarter),
			// Bar 7-8: ending
			N(G4, Quarter), N(E4, Quarter), N(D4, Quarter), N(B3, Quarter),
			N(C4, Whole),
		}}

		// Left hand - simple accompaniment
		accomp := BeatVoice{Notes: []BeatNote{
			// Bar 1-4: whole notes
			N(C3, Whole), N(C3, Whole),
			N(C3, Whole), N(C3, Whole),
			// Bar 5-6
			N(C3, Half), N(G3, Half),
			N(C3, Half), N(G3, Half),
			// Bar 7-8
			N(G3, Half), N(G3, Half),
			N(C3, Whole),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongCzerny599No19 - 车尔尼 Op.599 No.19 (八分音符练习)
var SongCzerny599No19 = Song{
	ID:    "czerny_599_19",
	Name:  "车尔尼 599-19",
	Tempo: Tempo{BPM: 120, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Right hand - eighth note patterns
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: running eighths
			N(C4, Eighth), N(D4, Eighth), N(E4, Eighth), N(F4, Eighth), N(G4, Eighth), N(F4, Eighth), N(E4, Eighth), N(D4, Eighth),
			N(C4, Eighth), N(E4, Eighth), N(G4, Eighth), N(E4, Eighth), N(C5, Eighth), N(G4, Eighth), N(E4, Eighth), N(C4, Eighth),
			// Bar 3-4
			N(D4, Eighth), N(E4, Eighth), N(F4, Eighth), N(G4, Eighth), N(A4, Eighth), N(G4, Eighth), N(F4, Eighth), N(E4, Eighth),
			N(D4, Eighth), N(F4, Eighth), N(A4, Eighth), N(F4, Eighth), N(D5, Eighth), N(A4, Eighth), N(F4, Eighth), N(D4, Eighth),
			// Bar 5-6
			N(E4, Eighth), N(F4, Eighth), N(G4, Eighth), N(A4, Eighth), N(B4, Eighth), N(A4, Eighth), N(G4, Eighth), N(F4, Eighth),
			N(E4, Eighth), N(G4, Eighth), N(B4, Eighth), N(G4, Eighth), N(E5, Eighth), N(B4, Eighth), N(G4, Eighth), N(E4, Eighth),
			// Bar 7-8: cadence
			N(F4, Eighth), N(A4, Eighth), N(C5, Eighth), N(A4, Eighth), N(G4, Eighth), N(B4, Eighth), N(D5, Eighth), N(B4, Eighth),
			N(C5, Half), N(C4, Half),
		}}

		// Left hand - quarter note bass
		accomp := BeatVoice{Notes: []BeatNote{
			N(C3, Quarter), N(E3, Quarter), N(G3, Quarter), N(E3, Quarter),
			N(C3, Quarter), N(E3, Quarter), N(G3, Quarter), N(E3, Quarter),
			N(D3, Quarter), N(F3, Quarter), N(A3, Quarter), N(F3, Quarter),
			N(D3, Quarter), N(F3, Quarter), N(A3, Quarter), N(F3, Quarter),
			N(E3, Quarter), N(G3, Quarter), N(B3, Quarter), N(G3, Quarter),
			N(E3, Quarter), N(G3, Quarter), N(B3, Quarter), N(G3, Quarter),
			N(F3, Quarter), N(A3, Quarter), N(G3, Quarter), N(B3, Quarter),
			N(C3, Half), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongCzerny599No38 - 车尔尼 Op.599 No.38 (三连音练习)
var SongCzerny599No38 = Song{
	ID:    "czerny_599_38",
	Name:  "车尔尼 599-38",
	Tempo: Tempo{BPM: 90, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Triplet constant
		t := 1.0 / 3.0 // triplet eighth

		// Right hand - triplet patterns
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: triplet arpeggios
			N(C4, t), N(E4, t), N(G4, t), N(C5, t), N(G4, t), N(E4, t), N(C4, t), N(E4, t), N(G4, t), N(C5, t), N(G4, t), N(E4, t),
			N(D4, t), N(F4, t), N(A4, t), N(D5, t), N(A4, t), N(F4, t), N(D4, t), N(F4, t), N(A4, t), N(D5, t), N(A4, t), N(F4, t),
			// Bar 3-4
			N(E4, t), N(G4, t), N(B4, t), N(E5, t), N(B4, t), N(G4, t), N(E4, t), N(G4, t), N(B4, t), N(E5, t), N(B4, t), N(G4, t),
			N(F4, t), N(A4, t), N(C5, t), N(F5, t), N(C5, t), N(A4, t), N(G4, t), N(B4, t), N(D5, t), N(G5, t), N(D5, t), N(B4, t),
			// Bar 5-6: descending
			N(C5, t), N(G4, t), N(E4, t), N(C4, t), N(E4, t), N(G4, t), N(C5, t), N(G4, t), N(E4, t), N(C4, t), N(E4, t), N(G4, t),
			N(B4, t), N(G4, t), N(D4, t), N(B3, t), N(D4, t), N(G4, t), N(B4, t), N(G4, t), N(D4, t), N(B3, t), N(D4, t), N(G4, t),
			// Bar 7-8: ending
			N(C4, t), N(E4, t), N(G4, t), N(C5, t), N(E5, t), N(G5, t), N(G4, t), N(B4, t), N(D5, t), N(G5, t), N(D5, t), N(B4, t),
			N(C5, Whole),
		}}

		// Left hand - sustained bass
		accomp := BeatVoice{Notes: []BeatNote{
			N(C3, Whole), N(D3, Whole),
			N(E3, Whole), N(F3, Half), N(G3, Half),
			N(C3, Whole), N(G3, Whole),
			N(C3, Half), N(G3, Half), N(C3, Whole),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongCzerny299No1 - 车尔尼 Op.299 No.1 (快速练习曲)
var SongCzerny299No1 = Song{
	ID:    "czerny_299_1",
	Name:  "车尔尼 299-1",
	Tempo: Tempo{BPM: 140, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Right hand - fast sixteenth note scales
		melody := BeatVoice{Notes: []BeatNote{
			// Bar 1-2: rapid scale passages
			N(C5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth),
			N(G5, Sixteenth), N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth),
			N(C5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth),
			N(G5, Sixteenth), N(A5, Sixteenth), N(B5, Sixteenth), N(C6, Sixteenth),

			N(C6, Sixteenth), N(B5, Sixteenth), N(A5, Sixteenth), N(G5, Sixteenth),
			N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth), N(C5, Sixteenth),
			N(B4, Sixteenth), N(C5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth),
			N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth), N(C5, Sixteenth),

			// Bar 3-4: broken chord patterns
			N(C5, Sixteenth), N(E5, Sixteenth), N(G5, Sixteenth), N(E5, Sixteenth),
			N(C5, Sixteenth), N(E5, Sixteenth), N(G5, Sixteenth), N(E5, Sixteenth),
			N(C5, Sixteenth), N(E5, Sixteenth), N(G5, Sixteenth), N(C6, Sixteenth),
			N(G5, Sixteenth), N(E5, Sixteenth), N(C5, Sixteenth), N(G4, Sixteenth),

			N(D5, Sixteenth), N(F5, Sixteenth), N(A5, Sixteenth), N(F5, Sixteenth),
			N(D5, Sixteenth), N(F5, Sixteenth), N(A5, Sixteenth), N(F5, Sixteenth),
			N(G4, Sixteenth), N(B4, Sixteenth), N(D5, Sixteenth), N(G5, Sixteenth),
			N(D5, Sixteenth), N(B4, Sixteenth), N(G4, Sixteenth), N(D4, Sixteenth),

			// Bar 5-6: ending
			N(C5, Eighth), N(E5, Eighth), N(G5, Quarter), N(C6, Half),
			N(G5, Quarter), N(E5, Quarter), N(C5, Half),
		}}

		// Left hand - alberti bass style
		accomp := BeatVoice{Notes: []BeatNote{
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(C3, Eighth), N(G3, Eighth), N(E3, Eighth), N(G3, Eighth),
			N(D3, Eighth), N(A3, Eighth), N(F3, Eighth), N(A3, Eighth),
			N(G3, Eighth), N(D4, Eighth), N(B3, Eighth), N(D4, Eighth),
			N(C3, Quarter), N(E3, Quarter), N(G3, Quarter), N(C4, Quarter),
			N(G3, Quarter), N(E3, Quarter), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongHanonNo1 - 哈农指法练习 No.1
var SongHanonNo1 = Song{
	ID:    "hanon_1",
	Name:  "哈农练习 1",
	Tempo: Tempo{BPM: 100, Signature: Time4_4},
	Voices: func() []BeatVoice {
		// Right hand - ascending/descending finger exercise
		melody := BeatVoice{Notes: []BeatNote{
			// Ascending pattern
			N(C4, Sixteenth), N(E4, Sixteenth), N(F4, Sixteenth), N(G4, Sixteenth),
			N(A4, Sixteenth), N(G4, Sixteenth), N(F4, Sixteenth), N(E4, Sixteenth),
			N(D4, Sixteenth), N(F4, Sixteenth), N(G4, Sixteenth), N(A4, Sixteenth),
			N(B4, Sixteenth), N(A4, Sixteenth), N(G4, Sixteenth), N(F4, Sixteenth),

			N(E4, Sixteenth), N(G4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth),
			N(C5, Sixteenth), N(B4, Sixteenth), N(A4, Sixteenth), N(G4, Sixteenth),
			N(F4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth),
			N(D5, Sixteenth), N(C5, Sixteenth), N(B4, Sixteenth), N(A4, Sixteenth),

			// Descending pattern
			N(G4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(D5, Sixteenth),
			N(E5, Sixteenth), N(D5, Sixteenth), N(C5, Sixteenth), N(B4, Sixteenth),
			N(A4, Sixteenth), N(C5, Sixteenth), N(B4, Sixteenth), N(A4, Sixteenth),
			N(G4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth),

			// Final descent
			N(B4, Sixteenth), N(G4, Sixteenth), N(F4, Sixteenth), N(E4, Sixteenth),
			N(D4, Sixteenth), N(E4, Sixteenth), N(F4, Sixteenth), N(G4, Sixteenth),
			N(A4, Sixteenth), N(F4, Sixteenth), N(E4, Sixteenth), N(D4, Sixteenth),
			N(C4, Quarter), N(Rest, Quarter), N(C4, Half),
		}}

		// Left hand - parallel motion
		accomp := BeatVoice{Notes: []BeatNote{
			N(C3, Sixteenth), N(E3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth),
			N(A3, Sixteenth), N(G3, Sixteenth), N(F3, Sixteenth), N(E3, Sixteenth),
			N(D3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth), N(A3, Sixteenth),
			N(B3, Sixteenth), N(A3, Sixteenth), N(G3, Sixteenth), N(F3, Sixteenth),

			N(E3, Sixteenth), N(G3, Sixteenth), N(A3, Sixteenth), N(B3, Sixteenth),
			N(C4, Sixteenth), N(B3, Sixteenth), N(A3, Sixteenth), N(G3, Sixteenth),
			N(F3, Sixteenth), N(A3, Sixteenth), N(B3, Sixteenth), N(C4, Sixteenth),
			N(D4, Sixteenth), N(C4, Sixteenth), N(B3, Sixteenth), N(A3, Sixteenth),

			N(G3, Sixteenth), N(B3, Sixteenth), N(C4, Sixteenth), N(D4, Sixteenth),
			N(E4, Sixteenth), N(D4, Sixteenth), N(C4, Sixteenth), N(B3, Sixteenth),
			N(A3, Sixteenth), N(C4, Sixteenth), N(B3, Sixteenth), N(A3, Sixteenth),
			N(G3, Sixteenth), N(A3, Sixteenth), N(B3, Sixteenth), N(C4, Sixteenth),

			N(B3, Sixteenth), N(G3, Sixteenth), N(F3, Sixteenth), N(E3, Sixteenth),
			N(D3, Sixteenth), N(E3, Sixteenth), N(F3, Sixteenth), N(G3, Sixteenth),
			N(A3, Sixteenth), N(F3, Sixteenth), N(E3, Sixteenth), N(D3, Sixteenth),
			N(C3, Quarter), N(Rest, Quarter), N(C3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongBurgmullerArabesque - 布格缪勒 阿拉伯风格曲 Op.100 No.2
var SongBurgmullerArabesque = Song{
	ID:    "burgmuller_arabesque",
	Name:  "阿拉伯风格曲",
	Tempo: Tempo{BPM: 130, Signature: Time2_4},
	Voices: func() []BeatVoice {
		// Right hand - characteristic staccato sixteenths
		melody := BeatVoice{Notes: []BeatNote{
			// Opening theme
			N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(D5, Sixteenth),
			N(E5, Eighth), N(Rest, Eighth),
			N(E5, Sixteenth), N(D5, Sixteenth), N(C5, Sixteenth), N(B4, Sixteenth),
			N(A4, Eighth), N(Rest, Eighth),

			N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(D5, Sixteenth),
			N(E5, Sixteenth), N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth),
			N(C5, Sixteenth), N(B4, Sixteenth), N(A4, Sixteenth), N(Gs4, Sixteenth),
			N(A4, Quarter),

			// Second phrase
			N(A4, Sixteenth), N(B4, Sixteenth), N(C5, Sixteenth), N(D5, Sixteenth),
			N(E5, Eighth), N(Rest, Eighth),
			N(E5, Sixteenth), N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth),
			N(C5, Eighth), N(Rest, Eighth),

			N(C5, Sixteenth), N(D5, Sixteenth), N(E5, Sixteenth), N(F5, Sixteenth),
			N(G5, Sixteenth), N(F5, Sixteenth), N(E5, Sixteenth), N(D5, Sixteenth),
			N(C5, Sixteenth), N(B4, Sixteenth), N(A4, Sixteenth), N(B4, Sixteenth),
			N(A4, Quarter),
		}}

		// Left hand - bass notes
		accomp := BeatVoice{Notes: []BeatNote{
			N(A3, Quarter), N(A3, Eighth), N(Rest, Eighth),
			N(A3, Quarter), N(A3, Eighth), N(Rest, Eighth),
			N(A3, Quarter), N(E3, Quarter),
			N(A3, Half),

			N(A3, Quarter), N(A3, Eighth), N(Rest, Eighth),
			N(C4, Quarter), N(C4, Eighth), N(Rest, Eighth),
			N(G3, Quarter), N(G3, Quarter),
			N(A3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongScaleC - C大调音阶练习
var SongScaleC = Song{
	ID:    "scale_c_major",
	Name:  "C大调音阶",
	Tempo: Tempo{BPM: 120, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Two octave ascending
			N(C4, Eighth), N(D4, Eighth), N(E4, Eighth), N(F4, Eighth),
			N(G4, Eighth), N(A4, Eighth), N(B4, Eighth), N(C5, Eighth),
			N(D5, Eighth), N(E5, Eighth), N(F5, Eighth), N(G5, Eighth),
			N(A5, Eighth), N(B5, Eighth), N(C6, Quarter),
			// Descending
			N(C6, Eighth), N(B5, Eighth), N(A5, Eighth), N(G5, Eighth),
			N(F5, Eighth), N(E5, Eighth), N(D5, Eighth), N(C5, Eighth),
			N(B4, Eighth), N(A4, Eighth), N(G4, Eighth), N(F4, Eighth),
			N(E4, Eighth), N(D4, Eighth), N(C4, Quarter),
		}}

		// Parallel bass
		accomp := BeatVoice{Notes: []BeatNote{
			N(C3, Eighth), N(D3, Eighth), N(E3, Eighth), N(F3, Eighth),
			N(G3, Eighth), N(A3, Eighth), N(B3, Eighth), N(C4, Eighth),
			N(D4, Eighth), N(E4, Eighth), N(F4, Eighth), N(G4, Eighth),
			N(A4, Eighth), N(B4, Eighth), N(C5, Quarter),
			N(C5, Eighth), N(B4, Eighth), N(A4, Eighth), N(G4, Eighth),
			N(F4, Eighth), N(E4, Eighth), N(D4, Eighth), N(C4, Eighth),
			N(B3, Eighth), N(A3, Eighth), N(G3, Eighth), N(F3, Eighth),
			N(E3, Eighth), N(D3, Eighth), N(C3, Quarter),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongScaleGMinor - G小调音阶练习 (harmonic minor)
var SongScaleGMinor = Song{
	ID:    "scale_g_minor",
	Name:  "G小调音阶",
	Tempo: Tempo{BPM: 110, Signature: Time4_4},
	Voices: func() []BeatVoice {
		melody := BeatVoice{Notes: []BeatNote{
			// Ascending G harmonic minor
			N(G4, Eighth), N(A4, Eighth), N(Bb4, Eighth), N(C5, Eighth),
			N(D5, Eighth), N(Eb5, Eighth), N(Fs5, Eighth), N(G5, Eighth),
			N(G5, Quarter), N(Rest, Quarter),
			// Descending
			N(G5, Eighth), N(Fs5, Eighth), N(Eb5, Eighth), N(D5, Eighth),
			N(C5, Eighth), N(Bb4, Eighth), N(A4, Eighth), N(G4, Eighth),
			N(G4, Half),
		}}

		accomp := BeatVoice{Notes: []BeatNote{
			N(G3, Eighth), N(A3, Eighth), N(Bb3, Eighth), N(C4, Eighth),
			N(D4, Eighth), N(Eb4, Eighth), N(Fs4, Eighth), N(G4, Eighth),
			N(G4, Quarter), N(Rest, Quarter),
			N(G4, Eighth), N(Fs4, Eighth), N(Eb4, Eighth), N(D4, Eighth),
			N(C4, Eighth), N(Bb3, Eighth), N(A3, Eighth), N(G3, Eighth),
			N(G3, Half),
		}}

		return []BeatVoice{melody, accomp}
	},
}

// SongTarantella - 塔兰泰拉舞曲 (6/8拍示例)
var SongTarantella = Song{
	ID:    "tarantella",
	Name:  "塔兰泰拉舞曲",
	Tempo: Tempo{BPM: 140, Signature: Time6_8},
	Voices: func() []BeatVoice {
		// Characteristic rapid triplet-feel dance
		melody := BeatVoice{Notes: []BeatNote{
			// Theme A
			N(A4, Eighth), N(C5, Eighth), N(E5, Eighth), N(A5, Eighth), N(E5, Eighth), N(C5, Eighth),
			N(A4, Eighth), N(C5, Eighth), N(E5, Eighth), N(A5, Eighth), N(E5, Eighth), N(C5, Eighth),
			N(B4, Eighth), N(D5, Eighth), N(F5, Eighth), N(B5, Eighth), N(F5, Eighth), N(D5, Eighth),
			N(E5, Quarter+Eighth), N(E5, Quarter+Eighth),

			// Theme B
			N(E5, Eighth), N(F5, Eighth), N(E5, Eighth), N(D5, Eighth), N(C5, Eighth), N(B4, Eighth),
			N(A4, Eighth), N(B4, Eighth), N(C5, Eighth), N(D5, Eighth), N(E5, Eighth), N(F5, Eighth),
			N(E5, Eighth), N(D5, Eighth), N(C5, Eighth), N(B4, Eighth), N(A4, Eighth), N(Gs4, Eighth),
			N(A4, Quarter+Eighth), N(A4, Quarter+Eighth),
		}}

		// Left hand - 6/8 bass pattern
		accomp := BeatVoice{Notes: []BeatNote{
			N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth),
			N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth),
			N(E3, Eighth), N(B2, Eighth), N(E3, Eighth), N(B2, Eighth), N(E3, Eighth), N(B2, Eighth),
			N(A3, Quarter+Eighth), N(E3, Quarter+Eighth),

			N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth),
			N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth), N(A3, Eighth), N(E3, Eighth),
			N(E3, Eighth), N(B2, Eighth), N(E3, Eighth), N(B2, Eighth), N(E3, Eighth), N(B2, Eighth),
			N(A3, Quarter+Eighth), N(A2, Quarter+Eighth),
		}}

		return []BeatVoice{melody, accomp}
	},
}
