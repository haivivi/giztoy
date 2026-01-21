//! Song catalog with built-in melodies.

use super::types::*;
use super::notes::*;

/// All built-in songs.
pub static ALL_SONGS: &[&Song] = &[
    // Classic melodies
    &SONG_TWINKLE_STAR,
    &SONG_HAPPY_BIRTHDAY,
    &SONG_TWO_TIGERS,
    &SONG_DOLL_AND_BEAR,
    // Classical piano pieces
    &SONG_FUR_ELISE,
    &SONG_CANON,
    &SONG_CASTLE_IN_SKY,
    &SONG_RIVER_FLOWS,
    &SONG_DREAM_WEDDING,
    // Bach
    &SONG_BACH_INVENTION1,
    &SONG_BACH_MINUET,
    &SONG_CANON_3VOICE,
    // Etudes & Exercises
    &SONG_CZERNY599_1,
    &SONG_CZERNY599_19,
    &SONG_CZERNY599_38,
    &SONG_CZERNY299_1,
    &SONG_HANON_1,
    &SONG_BURGMULLER_ARABESQUE,
    // Scales
    &SONG_SCALE_C,
    &SONG_SCALE_G_MINOR,
    // Dance & Rhythm
    &SONG_SIMPLE_WALTZ,
    &SONG_TARANTELLA,
];

// ========== Song Definitions ==========

/// 小星星 (Twinkle Twinkle Little Star)
pub static SONG_TWINKLE_STAR: Song = Song::new(
    "twinkle_star",
    "小星星",
    Tempo::new(100, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // 一闪一闪亮晶晶 (Bar 1-2)
            n(C4, QUARTER), n(C4, QUARTER), n(G4, QUARTER), n(G4, QUARTER),
            n(A4, QUARTER), n(A4, QUARTER), n(G4, HALF),
            // 满天都是小星星 (Bar 3-4)
            n(F4, QUARTER), n(F4, QUARTER), n(E4, QUARTER), n(E4, QUARTER),
            n(D4, QUARTER), n(D4, QUARTER), n(C4, HALF),
            // 挂在天空放光明 (Bar 5-6)
            n(G4, QUARTER), n(G4, QUARTER), n(F4, QUARTER), n(F4, QUARTER),
            n(E4, QUARTER), n(E4, QUARTER), n(D4, HALF),
            // 好像许多小眼睛 (Bar 7-8)
            n(G4, QUARTER), n(G4, QUARTER), n(F4, QUARTER), n(F4, QUARTER),
            n(E4, QUARTER), n(E4, QUARTER), n(D4, HALF),
            // Repeat 一闪一闪亮晶晶 (Bar 9-10)
            n(C4, QUARTER), n(C4, QUARTER), n(G4, QUARTER), n(G4, QUARTER),
            n(A4, QUARTER), n(A4, QUARTER), n(G4, HALF),
            // 满天都是小星星 (Bar 11-12)
            n(F4, QUARTER), n(F4, QUARTER), n(E4, QUARTER), n(E4, QUARTER),
            n(D4, QUARTER), n(D4, QUARTER), n(C4, HALF),
        ]);

        // Accompaniment: simple bass + chord
        let accomp = BeatVoice::new(vec![
            // Bar 1-2: C - G/B
            n(C3, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(E3, QUARTER),
            n(F3, QUARTER), n(A3, QUARTER), n(C3, HALF),
            // Bar 3-4: F - C - G - C
            n(F3, QUARTER), n(A3, QUARTER), n(C3, QUARTER), n(E3, QUARTER),
            n(G3, QUARTER), n(B3, QUARTER), n(C3, HALF),
            // Bar 5-6: C - F - C - G
            n(C3, QUARTER), n(E3, QUARTER), n(F3, QUARTER), n(A3, QUARTER),
            n(C3, QUARTER), n(E3, QUARTER), n(G3, HALF),
            // Bar 7-8
            n(C3, QUARTER), n(E3, QUARTER), n(F3, QUARTER), n(A3, QUARTER),
            n(C3, QUARTER), n(E3, QUARTER), n(G3, HALF),
            // Bar 9-10
            n(C3, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(E3, QUARTER),
            n(F3, QUARTER), n(A3, QUARTER), n(C3, HALF),
            // Bar 11-12
            n(F3, QUARTER), n(A3, QUARTER), n(C3, QUARTER), n(E3, QUARTER),
            n(G3, QUARTER), n(B3, QUARTER), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 生日快乐 (Happy Birthday)
pub static SONG_HAPPY_BIRTHDAY: Song = Song::new(
    "happy_birthday",
    "生日快乐",
    Tempo::new(120, TIME_3_4),
    || {
        let melody = BeatVoice::new(vec![
            // Happy birthday to you (anacrusis + Bar 1-2)
            n(C4, EIGHTH), n(C4, EIGHTH), n(D4, QUARTER), n(C4, QUARTER), n(F4, QUARTER), n(E4, HALF),
            // Happy birthday to you (Bar 3-4)
            n(C4, EIGHTH), n(C4, EIGHTH), n(D4, QUARTER), n(C4, QUARTER), n(G4, QUARTER), n(F4, HALF),
            // Happy birthday dear friend (Bar 5-6)
            n(C4, EIGHTH), n(C4, EIGHTH), n(C5, QUARTER), n(A4, QUARTER), n(F4, QUARTER), n(E4, QUARTER), n(D4, HALF),
            // Happy birthday to you (Bar 7-8)
            n(BB4, EIGHTH), n(BB4, EIGHTH), n(A4, QUARTER), n(F4, QUARTER), n(G4, QUARTER), n(F4, DOT_HALF),
        ]);

        // Waltz-style accompaniment
        let accomp = BeatVoice::new(vec![
            // Anacrusis rest + Bar 1-2
            n(REST, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(C4, QUARTER), n(C3, QUARTER), n(G3, QUARTER), n(C4, QUARTER),
            // Bar 3-4
            n(REST, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(C4, QUARTER), n(C3, QUARTER), n(G3, QUARTER), n(C4, QUARTER),
            // Bar 5-6
            n(REST, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(C4, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(C4, QUARTER), n(D3, QUARTER), n(F3, QUARTER), n(A3, QUARTER),
            // Bar 7-8
            n(REST, QUARTER), n(AS3, QUARTER), n(D4, QUARTER), n(F4, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(C4, QUARTER), n(F3, DOT_HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 两只老虎 (Two Tigers / Frère Jacques)
pub static SONG_TWO_TIGERS: Song = Song::new(
    "two_tigers",
    "两只老虎",
    Tempo::new(120, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // 两只老虎 两只老虎 (Bar 1-2)
            n(C4, QUARTER), n(D4, QUARTER), n(E4, QUARTER), n(C4, QUARTER),
            n(C4, QUARTER), n(D4, QUARTER), n(E4, QUARTER), n(C4, QUARTER),
            // 跑得快 跑得快 (Bar 3-4)
            n(E4, QUARTER), n(F4, QUARTER), n(G4, HALF),
            n(E4, QUARTER), n(F4, QUARTER), n(G4, HALF),
            // 一只没有眼睛 一只没有尾巴 (Bar 5-6)
            n(G4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(E4, QUARTER), n(C4, QUARTER),
            n(G4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(E4, QUARTER), n(C4, QUARTER),
            // 真奇怪 真奇怪 (Bar 7-8)
            n(C4, QUARTER), n(G3, QUARTER), n(C4, HALF),
            n(C4, QUARTER), n(G3, QUARTER), n(C4, HALF),
        ]);

        // Simple bass line
        let accomp = BeatVoice::new(vec![
            // Bar 1-2
            n(C3, HALF), n(E3, HALF),
            n(A3, HALF), n(E3, HALF),
            // Bar 3-4
            n(C3, HALF), n(G3, HALF),
            n(C3, HALF), n(G3, HALF),
            // Bar 5-6
            n(C3, HALF), n(G3, HALF),
            n(C3, HALF), n(G3, HALF),
            // Bar 7-8
            n(C3, QUARTER), n(G3, QUARTER), n(C3, HALF),
            n(C3, QUARTER), n(G3, QUARTER), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 洋娃娃和小熊跳舞 (Doll and Bear Dancing)
pub static SONG_DOLL_AND_BEAR: Song = Song::new(
    "doll_and_bear",
    "洋娃娃和小熊跳舞",
    Tempo::new(132, TIME_2_4),
    || {
        let melody = BeatVoice::new(vec![
            // 洋娃娃和小熊跳舞 跳呀跳呀 一二一 (Phrase 1)
            n(C4, EIGHTH), n(D4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH), n(E4, QUARTER),
            n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(D4, EIGHTH), n(E4, QUARTER),
            // 他们在跳圆圈舞呀 跳呀跳呀 一二一 (Phrase 2)
            n(C4, EIGHTH), n(D4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH), n(E4, QUARTER),
            n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(D4, QUARTER), n(C4, QUARTER),
            // 小熊小熊点点头呀 点点头呀 一二一 (Phrase 3)
            n(G4, EIGHTH), n(G4, EIGHTH), n(E4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(F4, EIGHTH), n(D4, QUARTER),
            n(E4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH), n(D4, QUARTER), n(REST, QUARTER),
            // 小洋娃娃笑起来呀 笑呀笑呀 哈哈哈 (Phrase 4)
            n(G4, EIGHTH), n(G4, EIGHTH), n(E4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(D4, QUARTER),
            n(C4, EIGHTH), n(D4, EIGHTH), n(E4, EIGHTH), n(D4, EIGHTH), n(C4, HALF),
        ]);

        // March-style accompaniment
        let accomp = BeatVoice::new(vec![
            // Phrase 1-2
            n(C3, QUARTER), n(G3, QUARTER), n(C3, QUARTER), n(G3, QUARTER),
            n(G3, QUARTER), n(D3, QUARTER), n(C3, QUARTER), n(G3, QUARTER),
            n(C3, QUARTER), n(G3, QUARTER), n(C3, QUARTER), n(G3, QUARTER),
            n(G3, QUARTER), n(D3, QUARTER), n(C3, HALF),
            // Phrase 3-4
            n(C3, QUARTER), n(G3, QUARTER), n(D3, QUARTER), n(G3, QUARTER),
            n(C3, QUARTER), n(G3, QUARTER), n(G3, HALF),
            n(C3, QUARTER), n(G3, QUARTER), n(G3, QUARTER), n(D3, QUARTER),
            n(C3, QUARTER), n(G3, QUARTER), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 献给爱丽丝 (Für Elise)
pub static SONG_FUR_ELISE: Song = Song::new(
    "fur_elise",
    "献给爱丽丝",
    Tempo::new(140, TIME_3_4),
    || {
        let melody = BeatVoice::new(vec![
            // Theme A - Main motif
            n(E5, EIGHTH), n(DS5, EIGHTH), n(E5, EIGHTH), n(DS5, EIGHTH), n(E5, EIGHTH), n(B4, EIGHTH), n(D5, EIGHTH), n(C5, EIGHTH),
            n(A4, QUARTER), n(REST, EIGHTH), n(C4, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH),
            n(B4, QUARTER), n(REST, EIGHTH), n(E4, EIGHTH), n(GS4, EIGHTH), n(B4, EIGHTH),
            n(C5, QUARTER), n(REST, EIGHTH), n(E4, EIGHTH), n(E5, EIGHTH), n(DS5, EIGHTH),
            // Theme A repeat
            n(E5, EIGHTH), n(DS5, EIGHTH), n(E5, EIGHTH), n(B4, EIGHTH), n(D5, EIGHTH), n(C5, EIGHTH),
            n(A4, QUARTER), n(REST, EIGHTH), n(C4, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH),
            n(B4, QUARTER), n(REST, EIGHTH), n(E4, EIGHTH), n(C5, EIGHTH), n(B4, EIGHTH),
            n(A4, DOT_QUARTER), n(REST, DOT_QUARTER),
        ]);

        // Left hand accompaniment
        let accomp = BeatVoice::new(vec![
            // Intro rhythm pattern
            n(REST, DOT_HALF),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(REST, DOT_QUARTER),
            n(E3, EIGHTH), n(E4, EIGHTH), n(GS4, EIGHTH), n(REST, DOT_QUARTER),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(REST, DOT_QUARTER),
            // Repeat
            n(REST, DOT_HALF),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(REST, DOT_QUARTER),
            n(E3, EIGHTH), n(E4, EIGHTH), n(GS4, EIGHTH), n(REST, DOT_QUARTER),
            n(A3, DOT_QUARTER), n(REST, DOT_QUARTER),
        ]);

        vec![melody, accomp]
    },
);

/// 卡农 (Pachelbel's Canon in D)
pub static SONG_CANON: Song = Song::new(
    "canon",
    "卡农",
    Tempo::new(60, TIME_4_4),
    || {
        // Main melody (Violin 1)
        let melody = BeatVoice::new(vec![
            // Main theme
            n(FS5, HALF), n(E5, HALF), n(D5, HALF), n(CS5, HALF),
            n(B4, HALF), n(A4, HALF), n(B4, HALF), n(CS5, HALF),
            // Variation
            n(D5, QUARTER), n(FS5, QUARTER), n(A5, QUARTER), n(G5, QUARTER),
            n(FS5, QUARTER), n(D5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(D5, QUARTER), n(B4, QUARTER), n(D5, QUARTER), n(A4, QUARTER),
            n(G4, QUARTER), n(B4, QUARTER), n(A4, QUARTER), n(G4, QUARTER),
            // Continuation
            n(FS4, QUARTER), n(D4, QUARTER), n(E4, QUARTER), n(FS4, QUARTER),
            n(G4, QUARTER), n(A4, QUARTER), n(B4, QUARTER), n(G4, QUARTER),
            n(FS4, HALF), n(D5, HALF), n(D5, WHOLE),
        ]);

        // Ground bass
        let bass = BeatVoice::new(vec![
            // D - A - Bm - F#m - G - D - G - A (repeated)
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(G3, HALF), n(A3, HALF),
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(G3, HALF), n(A3, HALF),
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(D3, WHOLE),
        ]);

        vec![melody, bass]
    },
);

/// 天空之城 (Laputa: Castle in the Sky)
pub static SONG_CASTLE_IN_SKY: Song = Song::new(
    "castle_in_sky",
    "天空之城",
    Tempo::new(80, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // Intro
            n(A4, QUARTER), n(B4, EIGHTH), n(C5, DOT_QUARTER), n(B4, QUARTER), n(C5, QUARTER), n(E5, HALF),
            n(REST, EIGHTH), n(G4, EIGHTH), n(A4, QUARTER), n(G4, EIGHTH), n(A4, EIGHTH), n(C5, HALF),
            n(REST, EIGHTH), n(G4, QUARTER), n(E4, QUARTER), n(E4, EIGHTH), n(D4, EIGHTH), n(E4, DOT_HALF),
            n(REST, QUARTER),
            // Main theme
            n(A4, QUARTER), n(B4, EIGHTH), n(C5, DOT_QUARTER), n(B4, QUARTER), n(C5, QUARTER), n(E5, HALF),
            n(REST, EIGHTH), n(G4, EIGHTH), n(A4, QUARTER), n(G4, EIGHTH), n(A4, EIGHTH), n(C5, HALF),
            n(REST, EIGHTH), n(G4, QUARTER), n(E4, QUARTER), n(D4, EIGHTH), n(C4, EIGHTH), n(D4, DOT_HALF),
            n(REST, QUARTER),
            // Climax
            n(E5, QUARTER), n(E5, EIGHTH), n(E5, QUARTER), n(B4, EIGHTH), n(C5, HALF),
            n(C5, QUARTER), n(C5, EIGHTH), n(C5, EIGHTH), n(B4, EIGHTH), n(A4, HALF),
            n(REST, EIGHTH), n(A4, QUARTER), n(C5, QUARTER), n(B4, EIGHTH), n(B4, EIGHTH), n(E4, HALF),
            n(A4, DOT_HALF), n(REST, QUARTER),
        ]);

        // Arpeggiated accompaniment
        let accomp = BeatVoice::new(vec![
            // Am pattern
            n(A3, QUARTER), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, QUARTER), n(A3, QUARTER), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, QUARTER),
            n(F3, QUARTER), n(C4, EIGHTH), n(F4, EIGHTH), n(C4, QUARTER), n(C3, QUARTER), n(G3, EIGHTH), n(C4, EIGHTH), n(G3, QUARTER),
            n(A3, QUARTER), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, QUARTER), n(E3, QUARTER), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, QUARTER),
            n(A3, HALF), n(REST, HALF),
            // Repeat
            n(A3, QUARTER), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, QUARTER), n(A3, QUARTER), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, QUARTER),
            n(F3, QUARTER), n(C4, EIGHTH), n(F4, EIGHTH), n(C4, QUARTER), n(C3, QUARTER), n(G3, EIGHTH), n(C4, EIGHTH), n(G3, QUARTER),
            n(G3, QUARTER), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, QUARTER), n(G3, QUARTER), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, QUARTER),
            n(REST, WHOLE),
            // Climax
            n(A3, HALF), n(E3, HALF),
            n(F3, HALF), n(C3, HALF),
            n(A3, HALF), n(E3, HALF),
            n(A3, DOT_HALF), n(REST, QUARTER),
        ]);

        vec![melody, accomp]
    },
);

/// River Flows in You (Yiruma)
pub static SONG_RIVER_FLOWS: Song = Song::new(
    "river_flows",
    "River Flows in You",
    Tempo::new(70, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // Intro arpeggio pattern
            n(A4, QUARTER), n(E5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(A4, QUARTER), n(E5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(GS4, QUARTER), n(E5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(GS4, QUARTER), n(E5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            // Main melody
            n(FS5, DOT_QUARTER), n(E5, EIGHTH), n(CS5, EIGHTH), n(E5, DOT_QUARTER), n(FS5, EIGHTH), n(E5, EIGHTH),
            n(CS5, DOT_QUARTER), n(B4, EIGHTH), n(A4, EIGHTH), n(B4, DOT_QUARTER), n(CS5, EIGHTH), n(B4, EIGHTH),
            n(A4, HALF), n(GS4, QUARTER), n(A4, QUARTER), n(B4, HALF),
            // Second phrase
            n(CS5, DOT_QUARTER), n(B4, EIGHTH), n(A4, EIGHTH), n(B4, DOT_QUARTER), n(CS5, EIGHTH), n(B4, EIGHTH),
            n(A4, DOT_QUARTER), n(GS4, EIGHTH), n(A4, EIGHTH), n(B4, DOT_QUARTER), n(A4, EIGHTH), n(GS4, EIGHTH),
            n(FS4, HALF), n(E4, HALF), n(FS4, WHOLE),
        ]);

        // Flowing left hand
        let accomp = BeatVoice::new(vec![
            // A major pattern
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH),
            n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            // Fs minor
            n(FS3, EIGHTH), n(CS4, EIGHTH), n(FS4, EIGHTH), n(CS4, EIGHTH), n(FS3, EIGHTH), n(CS4, EIGHTH), n(FS4, EIGHTH), n(CS4, EIGHTH),
            n(FS3, EIGHTH), n(CS4, EIGHTH), n(FS4, EIGHTH), n(CS4, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            // End
            n(FS3, EIGHTH), n(CS4, EIGHTH), n(FS4, EIGHTH), n(CS4, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            n(A3, WHOLE), n(A3, WHOLE),
        ]);

        vec![melody, accomp]
    },
);

/// 梦中的婚礼 (Mariage d'Amour)
pub static SONG_DREAM_WEDDING: Song = Song::new(
    "dream_wedding",
    "梦中的婚礼",
    Tempo::new(70, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // Intro/Theme
            n(E5, QUARTER), n(E5, EIGHTH), n(D5, EIGHTH), n(E5, HALF),
            n(E5, QUARTER), n(E5, EIGHTH), n(F5, EIGHTH), n(E5, QUARTER), n(D5, QUARTER),
            n(C5, QUARTER), n(B4, EIGHTH), n(A4, EIGHTH), n(B4, HALF),
            n(REST, WHOLE),
            // Second phrase
            n(E5, QUARTER), n(E5, EIGHTH), n(D5, EIGHTH), n(E5, HALF),
            n(E5, QUARTER), n(E5, EIGHTH), n(F5, EIGHTH), n(E5, QUARTER), n(D5, QUARTER),
            n(C5, QUARTER), n(D5, QUARTER), n(E5, QUARTER), n(C5, QUARTER),
            n(A4, WHOLE),
        ]);

        // Arpeggiated accompaniment
        let accomp = BeatVoice::new(vec![
            // Am - G - F - E pattern
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH),
            n(G3, EIGHTH), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, EIGHTH), n(G3, EIGHTH), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, EIGHTH),
            n(F3, EIGHTH), n(C4, EIGHTH), n(F4, EIGHTH), n(C4, EIGHTH), n(F3, EIGHTH), n(C4, EIGHTH), n(F4, EIGHTH), n(C4, EIGHTH),
            n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH), n(E3, EIGHTH), n(B3, EIGHTH), n(E4, EIGHTH), n(B3, EIGHTH),
            // Repeat
            n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH), n(A3, EIGHTH), n(E4, EIGHTH), n(A4, EIGHTH), n(E4, EIGHTH),
            n(G3, EIGHTH), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, EIGHTH), n(G3, EIGHTH), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, EIGHTH),
            n(F3, EIGHTH), n(C4, EIGHTH), n(F4, EIGHTH), n(C4, EIGHTH), n(G3, EIGHTH), n(D4, EIGHTH), n(G4, EIGHTH), n(D4, EIGHTH),
            n(A3, WHOLE),
        ]);

        vec![melody, accomp]
    },
);

/// 巴赫二部创意曲 No.1 (BWV 772)
pub static SONG_BACH_INVENTION1: Song = Song::new(
    "bach_invention1",
    "巴赫二部创意曲 No.1",
    Tempo::new(100, TIME_4_4),
    || {
        // Right hand (Soprano)
        let soprano = BeatVoice::new(vec![
            // Bar 1-2: Main theme
            n(C4, SIXTEENTH), n(D4, SIXTEENTH), n(E4, SIXTEENTH), n(F4, SIXTEENTH), n(D4, SIXTEENTH), n(E4, SIXTEENTH), n(C4, EIGHTH),
            n(REST, EIGHTH), n(G4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(G4, EIGHTH),
            // Bar 3-4
            n(REST, EIGHTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(C5, EIGHTH),
            n(A4, SIXTEENTH), n(G4, SIXTEENTH), n(F4, SIXTEENTH), n(E4, SIXTEENTH), n(F4, SIXTEENTH), n(A4, SIXTEENTH), n(G4, QUARTER),
            // Bar 5-6
            n(G4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(G4, EIGHTH),
            n(REST, EIGHTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH), n(G5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH), n(D5, EIGHTH),
            // Bar 7-8 (ending)
            n(E5, QUARTER), n(D5, QUARTER), n(C5, QUARTER), n(B4, QUARTER),
            n(C5, HALF),
        ]);

        // Left hand (Bass)
        let bass = BeatVoice::new(vec![
            // Bar 1: Rest then theme
            n(REST, DOT_QUARTER),
            n(C3, SIXTEENTH), n(D3, SIXTEENTH), n(E3, SIXTEENTH), n(F3, SIXTEENTH), n(D3, SIXTEENTH), n(E3, SIXTEENTH), n(C3, EIGHTH),
            // Bar 2
            n(REST, EIGHTH), n(G3, SIXTEENTH), n(A3, SIXTEENTH), n(B3, SIXTEENTH), n(C4, SIXTEENTH), n(A3, SIXTEENTH), n(B3, SIXTEENTH), n(G3, EIGHTH),
            // Bar 3-4
            n(REST, EIGHTH), n(E3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH), n(A3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH), n(E3, EIGHTH),
            n(F3, SIXTEENTH), n(E3, SIXTEENTH), n(D3, SIXTEENTH), n(C3, SIXTEENTH), n(D3, SIXTEENTH), n(F3, SIXTEENTH), n(E3, QUARTER),
            // Bar 5-6
            n(E3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH), n(A3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH), n(E3, EIGHTH),
            n(REST, EIGHTH), n(B3, SIXTEENTH), n(C4, SIXTEENTH), n(D4, SIXTEENTH), n(E4, SIXTEENTH), n(C4, SIXTEENTH), n(D4, SIXTEENTH), n(B3, EIGHTH),
            // Bar 7-8
            n(C4, QUARTER), n(G3, QUARTER), n(E3, QUARTER), n(G3, QUARTER),
            n(C3, HALF),
        ]);

        vec![soprano, bass]
    },
);

/// 巴赫小步舞曲 (BWV Anh. 114)
pub static SONG_BACH_MINUET: Song = Song::new(
    "bach_minuet",
    "巴赫小步舞曲",
    Tempo::new(110, TIME_3_4),
    || {
        let melody = BeatVoice::new(vec![
            // Bar 1-4
            n(D5, QUARTER), n(G4, QUARTER), n(A4, QUARTER), n(B4, QUARTER), n(C5, QUARTER), n(D5, QUARTER),
            n(G4, HALF), n(G4, QUARTER), n(E5, QUARTER), n(C5, QUARTER), n(D5, QUARTER),
            n(E5, QUARTER), n(FS5, QUARTER), n(G5, QUARTER), n(G4, HALF), n(G4, QUARTER),
            n(C5, QUARTER), n(D5, QUARTER), n(C5, QUARTER), n(B4, QUARTER), n(A4, QUARTER), n(B4, QUARTER),
            // Bar 5-8
            n(A4, HALF), n(D4, QUARTER), n(D5, QUARTER), n(G4, QUARTER), n(A4, QUARTER),
            n(B4, QUARTER), n(C5, QUARTER), n(D5, QUARTER), n(G4, HALF), n(G4, QUARTER),
            n(E5, QUARTER), n(C5, QUARTER), n(D5, QUARTER), n(E5, QUARTER), n(FS5, QUARTER), n(G5, QUARTER),
            n(G4, HALF), n(G4, QUARTER),
            // Ending
            n(B4, QUARTER), n(G4, QUARTER), n(A4, QUARTER), n(B4, QUARTER), n(G4, QUARTER), n(FS4, QUARTER),
            n(G4, DOT_HALF),
        ]);

        // Left hand
        let bass = BeatVoice::new(vec![
            // Bar 1-4
            n(G3, HALF), n(REST, QUARTER), n(G3, HALF), n(REST, QUARTER),
            n(B3, HALF), n(REST, QUARTER), n(C4, HALF), n(REST, QUARTER),
            n(D4, HALF), n(REST, QUARTER), n(B3, HALF), n(REST, QUARTER),
            n(C4, HALF), n(REST, QUARTER), n(D4, HALF), n(REST, QUARTER),
            // Bar 5-8
            n(D3, DOT_HALF), n(G3, HALF), n(REST, QUARTER),
            n(G3, HALF), n(REST, QUARTER), n(E3, HALF), n(REST, QUARTER),
            n(C4, HALF), n(REST, QUARTER), n(D4, HALF), n(REST, QUARTER),
            n(B3, HALF), n(REST, QUARTER),
            // Ending
            n(E3, HALF), n(REST, QUARTER), n(D3, HALF), n(REST, QUARTER),
            n(G3, DOT_HALF),
        ]);

        vec![melody, bass]
    },
);

/// 卡农三声部
pub static SONG_CANON_3VOICE: Song = Song::new(
    "canon_3voice",
    "卡农三声部",
    Tempo::new(60, TIME_4_4),
    || {
        // Voice 1 - enters first
        let voice1 = BeatVoice::new(vec![
            // Main theme
            n(FS5, HALF), n(E5, HALF), n(D5, HALF), n(CS5, HALF),
            n(B4, HALF), n(A4, HALF), n(B4, HALF), n(CS5, HALF),
            // Variation
            n(D5, QUARTER), n(FS5, QUARTER), n(A5, QUARTER), n(G5, QUARTER), n(FS5, QUARTER), n(D5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(D5, QUARTER), n(B4, QUARTER), n(D5, QUARTER), n(A4, QUARTER), n(G4, QUARTER), n(B4, QUARTER), n(A4, QUARTER), n(G4, QUARTER),
            // Continuation
            n(FS4, QUARTER), n(D4, QUARTER), n(E4, QUARTER), n(FS4, QUARTER), n(G4, QUARTER), n(A4, QUARTER), n(B4, QUARTER), n(G4, QUARTER),
            n(FS4, HALF), n(D5, HALF), n(D5, WHOLE),
        ]);

        // Voice 2 - enters 2 bars later
        let voice2 = BeatVoice::new(vec![
            // Wait 2 bars
            n(REST, WHOLE), n(REST, WHOLE),
            // Main theme (delayed)
            n(FS5, HALF), n(E5, HALF), n(D5, HALF), n(CS5, HALF),
            n(B4, HALF), n(A4, HALF), n(B4, HALF), n(CS5, HALF),
            // Variation
            n(D5, QUARTER), n(FS5, QUARTER), n(A5, QUARTER), n(G5, QUARTER), n(FS5, QUARTER), n(D5, QUARTER), n(FS5, QUARTER), n(E5, QUARTER),
            n(D5, HALF), n(A4, HALF), n(A4, WHOLE),
        ]);

        // Voice 3 - ground bass
        let voice3 = BeatVoice::new(vec![
            // Ground bass (D - A - Bm - F#m - G - D - G - A)
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(G3, HALF), n(A3, HALF),
            // Repeat
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(G3, HALF), n(A3, HALF),
            // Third time
            n(D3, HALF), n(A3, HALF), n(B3, HALF), n(FS3, HALF),
            n(G3, HALF), n(D3, HALF), n(D3, WHOLE),
        ]);

        vec![voice1, voice2, voice3]
    },
);

/// 车尔尼 Op.599 No.1 (初学者练习曲)
pub static SONG_CZERNY599_1: Song = Song::new(
    "czerny_599_1",
    "车尔尼 599-1",
    Tempo::new(100, TIME_4_4),
    || {
        // Right hand - simple scale exercise
        let melody = BeatVoice::new(vec![
            // Bar 1-2: C major scale up
            n(C4, QUARTER), n(D4, QUARTER), n(E4, QUARTER), n(F4, QUARTER),
            n(G4, QUARTER), n(A4, QUARTER), n(B4, QUARTER), n(C5, QUARTER),
            // Bar 3-4: scale down
            n(C5, QUARTER), n(B4, QUARTER), n(A4, QUARTER), n(G4, QUARTER),
            n(F4, QUARTER), n(E4, QUARTER), n(D4, QUARTER), n(C4, QUARTER),
            // Bar 5-6: broken thirds
            n(C4, EIGHTH), n(E4, EIGHTH), n(D4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(A4, EIGHTH),
            n(G4, EIGHTH), n(B4, EIGHTH), n(A4, EIGHTH), n(C5, EIGHTH), n(B4, EIGHTH), n(D5, EIGHTH), n(C5, QUARTER),
            // Bar 7-8: ending
            n(G4, QUARTER), n(E4, QUARTER), n(D4, QUARTER), n(B3, QUARTER),
            n(C4, WHOLE),
        ]);

        // Left hand - simple accompaniment
        let accomp = BeatVoice::new(vec![
            // Bar 1-4: whole notes
            n(C3, WHOLE), n(C3, WHOLE),
            n(C3, WHOLE), n(C3, WHOLE),
            // Bar 5-6
            n(C3, HALF), n(G3, HALF),
            n(C3, HALF), n(G3, HALF),
            // Bar 7-8
            n(G3, HALF), n(G3, HALF),
            n(C3, WHOLE),
        ]);

        vec![melody, accomp]
    },
);

/// 车尔尼 Op.599 No.19 (八分音符练习)
pub static SONG_CZERNY599_19: Song = Song::new(
    "czerny_599_19",
    "车尔尼 599-19",
    Tempo::new(120, TIME_4_4),
    || {
        // Right hand - eighth note patterns
        let melody = BeatVoice::new(vec![
            // Bar 1-2: running eighths
            n(C4, EIGHTH), n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH), n(D4, EIGHTH),
            n(C4, EIGHTH), n(E4, EIGHTH), n(G4, EIGHTH), n(E4, EIGHTH), n(C5, EIGHTH), n(G4, EIGHTH), n(E4, EIGHTH), n(C4, EIGHTH),
            // Bar 3-4
            n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH), n(G4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH), n(E4, EIGHTH),
            n(D4, EIGHTH), n(F4, EIGHTH), n(A4, EIGHTH), n(F4, EIGHTH), n(D5, EIGHTH), n(A4, EIGHTH), n(F4, EIGHTH), n(D4, EIGHTH),
            // Bar 5-6
            n(E4, EIGHTH), n(F4, EIGHTH), n(G4, EIGHTH), n(A4, EIGHTH), n(B4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH),
            n(E4, EIGHTH), n(G4, EIGHTH), n(B4, EIGHTH), n(G4, EIGHTH), n(E5, EIGHTH), n(B4, EIGHTH), n(G4, EIGHTH), n(E4, EIGHTH),
            // Bar 7-8: cadence
            n(F4, EIGHTH), n(A4, EIGHTH), n(C5, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(B4, EIGHTH), n(D5, EIGHTH), n(B4, EIGHTH),
            n(C5, HALF), n(C4, HALF),
        ]);

        // Left hand - quarter note bass
        let accomp = BeatVoice::new(vec![
            n(C3, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(E3, QUARTER),
            n(C3, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(E3, QUARTER),
            n(D3, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(F3, QUARTER),
            n(D3, QUARTER), n(F3, QUARTER), n(A3, QUARTER), n(F3, QUARTER),
            n(E3, QUARTER), n(G3, QUARTER), n(B3, QUARTER), n(G3, QUARTER),
            n(E3, QUARTER), n(G3, QUARTER), n(B3, QUARTER), n(G3, QUARTER),
            n(F3, QUARTER), n(A3, QUARTER), n(G3, QUARTER), n(B3, QUARTER),
            n(C3, HALF), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 车尔尼 Op.599 No.38 (三连音练习)
pub static SONG_CZERNY599_38: Song = Song::new(
    "czerny_599_38",
    "车尔尼 599-38",
    Tempo::new(90, TIME_4_4),
    || {
        // Triplet constant
        let t: f64 = 1.0 / 3.0; // triplet eighth

        // Right hand - triplet patterns
        let melody = BeatVoice::new(vec![
            // Bar 1-2: triplet arpeggios
            n(C4, t), n(E4, t), n(G4, t), n(C5, t), n(G4, t), n(E4, t), n(C4, t), n(E4, t), n(G4, t), n(C5, t), n(G4, t), n(E4, t),
            n(D4, t), n(F4, t), n(A4, t), n(D5, t), n(A4, t), n(F4, t), n(D4, t), n(F4, t), n(A4, t), n(D5, t), n(A4, t), n(F4, t),
            // Bar 3-4
            n(E4, t), n(G4, t), n(B4, t), n(E5, t), n(B4, t), n(G4, t), n(E4, t), n(G4, t), n(B4, t), n(E5, t), n(B4, t), n(G4, t),
            n(F4, t), n(A4, t), n(C5, t), n(F5, t), n(C5, t), n(A4, t), n(G4, t), n(B4, t), n(D5, t), n(G5, t), n(D5, t), n(B4, t),
            // Bar 5-6: descending
            n(C5, t), n(G4, t), n(E4, t), n(C4, t), n(E4, t), n(G4, t), n(C5, t), n(G4, t), n(E4, t), n(C4, t), n(E4, t), n(G4, t),
            n(B4, t), n(G4, t), n(D4, t), n(B3, t), n(D4, t), n(G4, t), n(B4, t), n(G4, t), n(D4, t), n(B3, t), n(D4, t), n(G4, t),
            // Bar 7-8: ending
            n(C4, t), n(E4, t), n(G4, t), n(C5, t), n(E5, t), n(G5, t), n(G4, t), n(B4, t), n(D5, t), n(G5, t), n(D5, t), n(B4, t),
            n(C5, WHOLE),
        ]);

        // Left hand - sustained bass
        let accomp = BeatVoice::new(vec![
            n(C3, WHOLE), n(D3, WHOLE),
            n(E3, WHOLE), n(F3, HALF), n(G3, HALF),
            n(C3, WHOLE), n(G3, WHOLE),
            n(C3, HALF), n(G3, HALF), n(C3, WHOLE),
        ]);

        vec![melody, accomp]
    },
);

/// 车尔尼 Op.299 No.1 (快速练习曲)
pub static SONG_CZERNY299_1: Song = Song::new(
    "czerny_299_1",
    "车尔尼 299-1",
    Tempo::new(140, TIME_4_4),
    || {
        // Right hand - fast sixteenth note scales
        let melody = BeatVoice::new(vec![
            // Bar 1-2: rapid scale passages
            n(C5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH),
            n(G5, SIXTEENTH), n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH),
            n(C5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH),
            n(G5, SIXTEENTH), n(A5, SIXTEENTH), n(B5, SIXTEENTH), n(C6, SIXTEENTH),

            n(C6, SIXTEENTH), n(B5, SIXTEENTH), n(A5, SIXTEENTH), n(G5, SIXTEENTH),
            n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH), n(C5, SIXTEENTH),
            n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH),
            n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH), n(C5, SIXTEENTH),

            // Bar 3-4: broken chord patterns
            n(C5, SIXTEENTH), n(E5, SIXTEENTH), n(G5, SIXTEENTH), n(E5, SIXTEENTH),
            n(C5, SIXTEENTH), n(E5, SIXTEENTH), n(G5, SIXTEENTH), n(E5, SIXTEENTH),
            n(C5, SIXTEENTH), n(E5, SIXTEENTH), n(G5, SIXTEENTH), n(C6, SIXTEENTH),
            n(G5, SIXTEENTH), n(E5, SIXTEENTH), n(C5, SIXTEENTH), n(G4, SIXTEENTH),

            n(D5, SIXTEENTH), n(F5, SIXTEENTH), n(A5, SIXTEENTH), n(F5, SIXTEENTH),
            n(D5, SIXTEENTH), n(F5, SIXTEENTH), n(A5, SIXTEENTH), n(F5, SIXTEENTH),
            n(G4, SIXTEENTH), n(B4, SIXTEENTH), n(D5, SIXTEENTH), n(G5, SIXTEENTH),
            n(D5, SIXTEENTH), n(B4, SIXTEENTH), n(G4, SIXTEENTH), n(D4, SIXTEENTH),

            // Bar 5-6: ending
            n(C5, EIGHTH), n(E5, EIGHTH), n(G5, QUARTER), n(C6, HALF),
            n(G5, QUARTER), n(E5, QUARTER), n(C5, HALF),
        ]);

        // Left hand - alberti bass style
        let accomp = BeatVoice::new(vec![
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(C3, EIGHTH), n(G3, EIGHTH), n(E3, EIGHTH), n(G3, EIGHTH),
            n(D3, EIGHTH), n(A3, EIGHTH), n(F3, EIGHTH), n(A3, EIGHTH),
            n(G3, EIGHTH), n(D4, EIGHTH), n(B3, EIGHTH), n(D4, EIGHTH),
            n(C3, QUARTER), n(E3, QUARTER), n(G3, QUARTER), n(C4, QUARTER),
            n(G3, QUARTER), n(E3, QUARTER), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 哈农指法练习 No.1
pub static SONG_HANON_1: Song = Song::new(
    "hanon_1",
    "哈农练习 1",
    Tempo::new(100, TIME_4_4),
    || {
        // Right hand - ascending/descending finger exercise
        let melody = BeatVoice::new(vec![
            // Ascending pattern
            n(C4, SIXTEENTH), n(E4, SIXTEENTH), n(F4, SIXTEENTH), n(G4, SIXTEENTH),
            n(A4, SIXTEENTH), n(G4, SIXTEENTH), n(F4, SIXTEENTH), n(E4, SIXTEENTH),
            n(D4, SIXTEENTH), n(F4, SIXTEENTH), n(G4, SIXTEENTH), n(A4, SIXTEENTH),
            n(B4, SIXTEENTH), n(A4, SIXTEENTH), n(G4, SIXTEENTH), n(F4, SIXTEENTH),

            n(E4, SIXTEENTH), n(G4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH),
            n(C5, SIXTEENTH), n(B4, SIXTEENTH), n(A4, SIXTEENTH), n(G4, SIXTEENTH),
            n(F4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH),
            n(D5, SIXTEENTH), n(C5, SIXTEENTH), n(B4, SIXTEENTH), n(A4, SIXTEENTH),

            // Descending pattern
            n(G4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH),
            n(E5, SIXTEENTH), n(D5, SIXTEENTH), n(C5, SIXTEENTH), n(B4, SIXTEENTH),
            n(A4, SIXTEENTH), n(C5, SIXTEENTH), n(B4, SIXTEENTH), n(A4, SIXTEENTH),
            n(G4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH),

            // Final descent
            n(B4, SIXTEENTH), n(G4, SIXTEENTH), n(F4, SIXTEENTH), n(E4, SIXTEENTH),
            n(D4, SIXTEENTH), n(E4, SIXTEENTH), n(F4, SIXTEENTH), n(G4, SIXTEENTH),
            n(A4, SIXTEENTH), n(F4, SIXTEENTH), n(E4, SIXTEENTH), n(D4, SIXTEENTH),
            n(C4, QUARTER), n(REST, QUARTER), n(C4, HALF),
        ]);

        // Left hand - parallel motion
        let accomp = BeatVoice::new(vec![
            n(C3, SIXTEENTH), n(E3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH),
            n(A3, SIXTEENTH), n(G3, SIXTEENTH), n(F3, SIXTEENTH), n(E3, SIXTEENTH),
            n(D3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH), n(A3, SIXTEENTH),
            n(B3, SIXTEENTH), n(A3, SIXTEENTH), n(G3, SIXTEENTH), n(F3, SIXTEENTH),

            n(E3, SIXTEENTH), n(G3, SIXTEENTH), n(A3, SIXTEENTH), n(B3, SIXTEENTH),
            n(C4, SIXTEENTH), n(B3, SIXTEENTH), n(A3, SIXTEENTH), n(G3, SIXTEENTH),
            n(F3, SIXTEENTH), n(A3, SIXTEENTH), n(B3, SIXTEENTH), n(C4, SIXTEENTH),
            n(D4, SIXTEENTH), n(C4, SIXTEENTH), n(B3, SIXTEENTH), n(A3, SIXTEENTH),

            n(G3, SIXTEENTH), n(B3, SIXTEENTH), n(C4, SIXTEENTH), n(D4, SIXTEENTH),
            n(E4, SIXTEENTH), n(D4, SIXTEENTH), n(C4, SIXTEENTH), n(B3, SIXTEENTH),
            n(A3, SIXTEENTH), n(C4, SIXTEENTH), n(B3, SIXTEENTH), n(A3, SIXTEENTH),
            n(G3, SIXTEENTH), n(A3, SIXTEENTH), n(B3, SIXTEENTH), n(C4, SIXTEENTH),

            n(B3, SIXTEENTH), n(G3, SIXTEENTH), n(F3, SIXTEENTH), n(E3, SIXTEENTH),
            n(D3, SIXTEENTH), n(E3, SIXTEENTH), n(F3, SIXTEENTH), n(G3, SIXTEENTH),
            n(A3, SIXTEENTH), n(F3, SIXTEENTH), n(E3, SIXTEENTH), n(D3, SIXTEENTH),
            n(C3, QUARTER), n(REST, QUARTER), n(C3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 布格缪勒 阿拉伯风格曲 Op.100 No.2
pub static SONG_BURGMULLER_ARABESQUE: Song = Song::new(
    "burgmuller_arabesque",
    "阿拉伯风格曲",
    Tempo::new(130, TIME_2_4),
    || {
        // Right hand - characteristic staccato sixteenths
        let melody = BeatVoice::new(vec![
            // Opening theme
            n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH),
            n(E5, EIGHTH), n(REST, EIGHTH),
            n(E5, SIXTEENTH), n(D5, SIXTEENTH), n(C5, SIXTEENTH), n(B4, SIXTEENTH),
            n(A4, EIGHTH), n(REST, EIGHTH),

            n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH),
            n(E5, SIXTEENTH), n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH),
            n(C5, SIXTEENTH), n(B4, SIXTEENTH), n(A4, SIXTEENTH), n(GS4, SIXTEENTH),
            n(A4, QUARTER),

            // Second phrase
            n(A4, SIXTEENTH), n(B4, SIXTEENTH), n(C5, SIXTEENTH), n(D5, SIXTEENTH),
            n(E5, EIGHTH), n(REST, EIGHTH),
            n(E5, SIXTEENTH), n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH),
            n(C5, EIGHTH), n(REST, EIGHTH),

            n(C5, SIXTEENTH), n(D5, SIXTEENTH), n(E5, SIXTEENTH), n(F5, SIXTEENTH),
            n(G5, SIXTEENTH), n(F5, SIXTEENTH), n(E5, SIXTEENTH), n(D5, SIXTEENTH),
            n(C5, SIXTEENTH), n(B4, SIXTEENTH), n(A4, SIXTEENTH), n(B4, SIXTEENTH),
            n(A4, QUARTER),
        ]);

        // Left hand - bass notes
        let accomp = BeatVoice::new(vec![
            n(A3, QUARTER), n(A3, EIGHTH), n(REST, EIGHTH),
            n(A3, QUARTER), n(A3, EIGHTH), n(REST, EIGHTH),
            n(A3, QUARTER), n(E3, QUARTER),
            n(A3, HALF),

            n(A3, QUARTER), n(A3, EIGHTH), n(REST, EIGHTH),
            n(C4, QUARTER), n(C4, EIGHTH), n(REST, EIGHTH),
            n(G3, QUARTER), n(G3, QUARTER),
            n(A3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// C大调音阶 (C Major Scale)
pub static SONG_SCALE_C: Song = Song::new(
    "scale_c_major",
    "C大调音阶",
    Tempo::new(120, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // Ascending
            n(C4, EIGHTH), n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH),
            n(G4, EIGHTH), n(A4, EIGHTH), n(B4, EIGHTH), n(C5, EIGHTH),
            n(D5, EIGHTH), n(E5, EIGHTH), n(F5, EIGHTH), n(G5, EIGHTH),
            n(A5, EIGHTH), n(B5, EIGHTH), n(C6, QUARTER),
            // Descending
            n(C6, EIGHTH), n(B5, EIGHTH), n(A5, EIGHTH), n(G5, EIGHTH),
            n(F5, EIGHTH), n(E5, EIGHTH), n(D5, EIGHTH), n(C5, EIGHTH),
            n(B4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH), n(F4, EIGHTH),
            n(E4, EIGHTH), n(D4, EIGHTH), n(C4, QUARTER),
        ]);

        // Parallel bass
        let accomp = BeatVoice::new(vec![
            n(C3, EIGHTH), n(D3, EIGHTH), n(E3, EIGHTH), n(F3, EIGHTH),
            n(G3, EIGHTH), n(A3, EIGHTH), n(B3, EIGHTH), n(C4, EIGHTH),
            n(D4, EIGHTH), n(E4, EIGHTH), n(F4, EIGHTH), n(G4, EIGHTH),
            n(A4, EIGHTH), n(B4, EIGHTH), n(C5, QUARTER),
            n(C5, EIGHTH), n(B4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH),
            n(F4, EIGHTH), n(E4, EIGHTH), n(D4, EIGHTH), n(C4, EIGHTH),
            n(B3, EIGHTH), n(A3, EIGHTH), n(G3, EIGHTH), n(F3, EIGHTH),
            n(E3, EIGHTH), n(D3, EIGHTH), n(C3, QUARTER),
        ]);

        vec![melody, accomp]
    },
);

/// G小调音阶 (G Minor Scale - harmonic minor)
pub static SONG_SCALE_G_MINOR: Song = Song::new(
    "scale_g_minor",
    "G小调音阶",
    Tempo::new(110, TIME_4_4),
    || {
        let melody = BeatVoice::new(vec![
            // Ascending G harmonic minor
            n(G4, EIGHTH), n(A4, EIGHTH), n(BB4, EIGHTH), n(C5, EIGHTH),
            n(D5, EIGHTH), n(EB5, EIGHTH), n(FS5, EIGHTH), n(G5, EIGHTH),
            n(G5, QUARTER), n(REST, QUARTER),
            // Descending
            n(G5, EIGHTH), n(FS5, EIGHTH), n(EB5, EIGHTH), n(D5, EIGHTH),
            n(C5, EIGHTH), n(BB4, EIGHTH), n(A4, EIGHTH), n(G4, EIGHTH),
            n(G4, HALF),
        ]);

        let accomp = BeatVoice::new(vec![
            n(G3, EIGHTH), n(A3, EIGHTH), n(BB3, EIGHTH), n(C4, EIGHTH),
            n(D4, EIGHTH), n(EB4, EIGHTH), n(FS4, EIGHTH), n(G4, EIGHTH),
            n(G4, QUARTER), n(REST, QUARTER),
            n(G4, EIGHTH), n(FS4, EIGHTH), n(EB4, EIGHTH), n(D4, EIGHTH),
            n(C4, EIGHTH), n(BB3, EIGHTH), n(A3, EIGHTH), n(G3, EIGHTH),
            n(G3, HALF),
        ]);

        vec![melody, accomp]
    },
);

/// 简单圆舞曲
pub static SONG_SIMPLE_WALTZ: Song = Song::new(
    "simple_waltz",
    "简单圆舞曲",
    Tempo::new(120, TIME_3_4),
    || {
        let melody = BeatVoice::new(vec![
            // Bar 1-2
            n(E5, DOT_QUARTER), n(D5, EIGHTH), n(C5, QUARTER),
            n(D5, DOT_QUARTER), n(E5, EIGHTH), n(D5, QUARTER),
            // Bar 3-4
            n(C5, DOT_QUARTER), n(B4, EIGHTH), n(A4, QUARTER),
            n(B4, HALF), n(REST, QUARTER),
            // Bar 5-6
            n(E5, DOT_QUARTER), n(D5, EIGHTH), n(C5, QUARTER),
            n(D5, DOT_QUARTER), n(E5, EIGHTH), n(D5, QUARTER),
            // Bar 7-8 (ending)
            n(C5, HALF), n(B4, QUARTER),
            n(C5, DOT_HALF),
        ]);

        // Waltz bass: "oom-pah-pah" pattern
        let bass = BeatVoice::new(vec![
            // Bar 1-2: C major, G major
            n(C3, QUARTER), n(E4, QUARTER), n(G4, QUARTER),
            n(G3, QUARTER), n(D4, QUARTER), n(G4, QUARTER),
            // Bar 3-4: A minor, E major
            n(A3, QUARTER), n(E4, QUARTER), n(A4, QUARTER),
            n(E3, QUARTER), n(GS3, QUARTER), n(B3, QUARTER),
            // Bar 5-6: C major, G major
            n(C3, QUARTER), n(E4, QUARTER), n(G4, QUARTER),
            n(G3, QUARTER), n(D4, QUARTER), n(G4, QUARTER),
            // Bar 7-8: F major -> C major
            n(F3, QUARTER), n(A3, QUARTER), n(C4, QUARTER),
            n(C3, DOT_HALF),
        ]);

        vec![melody, bass]
    },
);

/// 塔兰泰拉舞曲 (6/8拍示例)
pub static SONG_TARANTELLA: Song = Song::new(
    "tarantella",
    "塔兰泰拉舞曲",
    Tempo::new(140, TIME_6_8),
    || {
        // Characteristic rapid triplet-feel dance
        let melody = BeatVoice::new(vec![
            // Theme A
            n(A4, EIGHTH), n(C5, EIGHTH), n(E5, EIGHTH), n(A5, EIGHTH), n(E5, EIGHTH), n(C5, EIGHTH),
            n(A4, EIGHTH), n(C5, EIGHTH), n(E5, EIGHTH), n(A5, EIGHTH), n(E5, EIGHTH), n(C5, EIGHTH),
            n(B4, EIGHTH), n(D5, EIGHTH), n(F5, EIGHTH), n(B5, EIGHTH), n(F5, EIGHTH), n(D5, EIGHTH),
            n(E5, QUARTER + EIGHTH), n(E5, QUARTER + EIGHTH),

            // Theme B
            n(E5, EIGHTH), n(F5, EIGHTH), n(E5, EIGHTH), n(D5, EIGHTH), n(C5, EIGHTH), n(B4, EIGHTH),
            n(A4, EIGHTH), n(B4, EIGHTH), n(C5, EIGHTH), n(D5, EIGHTH), n(E5, EIGHTH), n(F5, EIGHTH),
            n(E5, EIGHTH), n(D5, EIGHTH), n(C5, EIGHTH), n(B4, EIGHTH), n(A4, EIGHTH), n(GS4, EIGHTH),
            n(A4, QUARTER + EIGHTH), n(A4, QUARTER + EIGHTH),
        ]);

        // Left hand - 6/8 bass pattern
        let accomp = BeatVoice::new(vec![
            n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH),
            n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH),
            n(E3, EIGHTH), n(B2, EIGHTH), n(E3, EIGHTH), n(B2, EIGHTH), n(E3, EIGHTH), n(B2, EIGHTH),
            n(A3, QUARTER + EIGHTH), n(E3, QUARTER + EIGHTH),

            n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH),
            n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH), n(A3, EIGHTH), n(E3, EIGHTH),
            n(E3, EIGHTH), n(B2, EIGHTH), n(E3, EIGHTH), n(B2, EIGHTH), n(E3, EIGHTH), n(B2, EIGHTH),
            n(A3, QUARTER + EIGHTH), n(A2, QUARTER + EIGHTH),
        ]);

        vec![melody, accomp]
    },
);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_all_songs_count() {
        // Should have 22 songs like Go version
        assert_eq!(ALL_SONGS.len(), 22);
    }

    #[test]
    fn test_all_songs_not_empty() {
        assert!(!ALL_SONGS.is_empty());
    }

    #[test]
    fn test_all_songs_have_valid_data() {
        for song in ALL_SONGS {
            assert!(!song.id.is_empty(), "Song ID should not be empty");
            assert!(!song.name.is_empty(), "Song name should not be empty");
            assert!(song.tempo.bpm > 0, "Song BPM should be positive");
            
            let voices = song.voices();
            assert!(!voices.is_empty(), "Song {} should have voices", song.id);
            
            for voice in &voices {
                assert!(voice.total_beats() > 0.0, "Voice in song {} should have positive beats", song.id);
            }
        }
    }

    #[test]
    fn test_songs_duration() {
        for song in ALL_SONGS {
            let duration = song.duration();
            assert!(duration > 0, "Song {} should have positive duration", song.id);
        }
    }

    #[test]
    fn test_songs_to_voices() {
        for song in ALL_SONGS {
            let voices = song.to_voices(false);
            assert!(!voices.is_empty());
            
            for voice in &voices {
                assert!(voice.total_duration() > 0);
            }
        }
    }

    #[test]
    fn test_songs_to_voices_with_metronome() {
        for song in ALL_SONGS {
            let voices_without = song.to_voices(false);
            let voices_with = song.to_voices(true);
            
            // With metronome should have one more voice
            assert_eq!(voices_with.len(), voices_without.len() + 1);
        }
    }
}
