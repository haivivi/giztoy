//! Basic PCM format demonstration.
//!
//! This example demonstrates basic PCM format operations including:
//! - Format properties (sample rate, channels, bit depth)
//! - Duration and byte calculations
//! - Creating data and silence chunks
//!
//! Run with: cargo run --bin pcm_basic
//! Or with bazel: bazel run //examples/rust/audio:pcm_basic

use giztoy_audio::pcm::{Chunk, Format, FormatExt};
use std::time::Duration;

fn main() {
    println!("=== PCM Format Basic Demo ===\n");

    // Demonstrate different formats
    let formats = [Format::L16Mono16K, Format::L16Mono24K, Format::L16Mono48K];

    for format in formats {
        println!("Format: {}", format);
        println!("  Sample rate: {} Hz", format.sample_rate());
        println!("  Channels: {}", format.channels());
        println!("  Bit depth: {}", format.depth());
        println!("  Bit rate: {} bps", format.bits_rate());
        println!("  Byte rate: {} bytes/s", format.bytes_rate());
        println!();
    }

    // Duration calculations
    let format = Format::L16Mono16K;
    println!("=== Duration Calculations ({}Hz) ===\n", format.sample_rate());

    let durations = [
        Duration::from_millis(10),
        Duration::from_millis(20),
        Duration::from_millis(100),
        Duration::from_secs(1),
    ];

    for d in durations {
        let bytes = format.bytes_in_duration(d);
        let samples = format.samples_in_duration(d);
        println!(
            "  {:>6?}: {} bytes, {} samples",
            d, bytes, samples
        );
    }

    // Reverse calculation
    println!("\n=== Bytes to Duration ===\n");
    let byte_counts = [320, 640, 3200, 32000];
    for bytes in byte_counts {
        let duration = format.duration(bytes);
        let samples = format.samples(bytes);
        println!(
            "  {:>5} bytes: {:?}, {} samples",
            bytes, duration, samples
        );
    }

    // Create chunks
    println!("\n=== Creating Chunks ===\n");

    // Create a silence chunk
    let silence = format.silence_chunk(Duration::from_millis(100));
    println!(
        "Silence chunk: {} bytes ({:?})",
        silence.len(),
        silence.duration()
    );

    // Write silence to buffer
    let mut buf = Vec::new();
    silence.write_to(&mut buf).unwrap();
    println!("  Written {} bytes (all zeros: {})", buf.len(), buf.iter().all(|&b| b == 0));

    // Create a data chunk from samples
    let samples: Vec<i16> = (0..1600)
        .map(|i| {
            // Generate a 440Hz sine wave
            let t = i as f64 / 16000.0;
            ((t * 440.0 * 2.0 * std::f64::consts::PI).sin() * 16000.0) as i16
        })
        .collect();

    let data_chunk = format.data_chunk_from_samples(&samples);
    println!(
        "\nData chunk (440Hz sine): {} bytes ({:?})",
        data_chunk.len(),
        format.duration(data_chunk.len())
    );

    // Show some samples
    let chunk_samples = data_chunk.samples();
    println!("  First 10 samples: {:?}", &chunk_samples[..10]);
    println!("  Peak amplitude: {}", chunk_samples.iter().map(|s| s.abs()).max().unwrap());

    println!("\n=== Demo Complete ===");
}
