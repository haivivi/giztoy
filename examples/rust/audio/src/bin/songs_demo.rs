//! Songs demo - demonstrates built-in melodies and the mixer.
//!
//! Run with:
//!   bazel run //examples/rust/audio:songs_demo

use giztoy_audio::songs::{Song, RenderOptions, ALL_SONGS};
use giztoy_audio::pcm::Format;
use std::io::Write;

fn main() {
    println!("=== Songs Demo ===\n");

    // List all available songs
    println!("Available songs:");
    for (i, song) in ALL_SONGS.iter().enumerate() {
        let duration_ms = song.duration();
        println!("  {}. {} ({}) - {} ms", i + 1, song.name, song.id, duration_ms);
    }
    println!();

    // Render Twinkle Star
    if let Some(song) = Song::by_id("twinkle_star") {
        println!("Rendering '{}' to PCM...", song.name);

        let opts = RenderOptions::default()
            .with_format(Format::L16Mono16K)
            .with_volume(0.5)
            .with_rich_sound(true);

        let data = song.render_bytes(opts);
        let duration_ms = song.duration();

        println!("  Format: L16Mono16K (16-bit, 16kHz, mono)");
        println!("  Duration: {} ms", duration_ms);
        println!("  Audio data size: {} bytes", data.len());
        println!("  Expected bytes: {} (16kHz * 2 bytes * {} ms / 1000)", 
                 16000 * 2 * duration_ms / 1000, duration_ms);

        // Verify audio content
        let non_zero = data.iter().filter(|&&b| b != 0).count();
        let zero_pct = ((data.len() - non_zero) as f64 / data.len() as f64) * 100.0;
        println!("  Non-zero bytes: {} ({:.1}% silence)", non_zero, zero_pct);

        // Save to file
        let filename = "twinkle_star.pcm";
        if let Ok(mut file) = std::fs::File::create(filename) {
            let _ = file.write_all(&data);
            println!("  Saved to: {}", filename);
            println!("  Play with: ffplay -f s16le -ar 16000 -ac 1 {}", filename);
        }
    }
    println!();

    // Render Two Tigers with metronome
    if let Some(song) = Song::by_id("two_tigers") {
        println!("Rendering '{}' with metronome...", song.name);

        let opts = RenderOptions::default()
            .with_format(Format::L16Mono16K)
            .with_volume(0.4)
            .with_metronome(true);

        let data = song.render_bytes(opts);
        println!("  Audio data size: {} bytes (with metronome)", data.len());

        let filename = "two_tigers_metro.pcm";
        if let Ok(mut file) = std::fs::File::create(filename) {
            let _ = file.write_all(&data);
            println!("  Saved to: {}", filename);
        }
    }
    println!();

    // Render C major scale
    if let Some(song) = Song::by_id("scale_c_major") {
        println!("Rendering '{}'...", song.name);

        let opts = RenderOptions::default()
            .with_format(Format::L16Mono24K)
            .with_volume(0.6);

        let data = song.render_bytes(opts);
        println!("  Format: L16Mono24K (24kHz)");
        println!("  Audio data size: {} bytes", data.len());

        let filename = "scale_c_major.pcm";
        if let Ok(mut file) = std::fs::File::create(filename) {
            let _ = file.write_all(&data);
            println!("  Saved to: {}", filename);
            println!("  Play with: ffplay -f s16le -ar 24000 -ac 1 {}", filename);
        }
    }

    println!("\n=== Demo Complete ===");
}
