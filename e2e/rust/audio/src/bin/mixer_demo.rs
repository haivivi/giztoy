//! Mixer demonstration.
//!
//! This example demonstrates the audio mixer functionality:
//! - Creating a mixer with multiple tracks
//! - Writing audio data to tracks from different threads
//! - Reading mixed audio output
//! - Gain control
//!
//! Run with: cargo run --bin mixer_demo
//! Or with bazel: bazel run //examples/rust/audio:mixer_demo

use giztoy_audio::pcm::{Format, Mixer, MixerOptions, TrackOptions};
use std::f64::consts::PI;
use std::io::Read;
use std::thread;
use std::time::Duration;

/// Generates a sine wave as raw PCM bytes.
fn generate_sine_wave(freq: f64, sample_rate: u32, duration_ms: u32) -> Vec<u8> {
    let samples = (sample_rate * duration_ms / 1000) as usize;
    let mut data = Vec::with_capacity(samples * 2);
    for i in 0..samples {
        let t = i as f64 / sample_rate as f64;
        let value = (2.0 * PI * freq * t).sin();
        let sample = (value * 16000.0) as i16;
        data.extend_from_slice(&sample.to_le_bytes());
    }
    data
}

/// Analyzes audio data and returns statistics.
fn analyze_audio(data: &[u8]) -> (i16, i16, usize) {
    let mut min = i16::MAX;
    let mut max = i16::MIN;
    let mut zero_crossings = 0;
    let mut prev_sign = true;

    for chunk in data.chunks_exact(2) {
        let sample = i16::from_le_bytes([chunk[0], chunk[1]]);
        min = min.min(sample);
        max = max.max(sample);

        let sign = sample >= 0;
        if sign != prev_sign {
            zero_crossings += 1;
        }
        prev_sign = sign;
    }

    (min, max, zero_crossings)
}

fn main() {
    println!("=== Audio Mixer Demo ===\n");

    let format = Format::L16Mono16K;
    println!("Output format: {}", format);
    println!();

    // Example 1: Single track
    println!("--- Example 1: Single Track ---");
    {
        let mixer = Mixer::new(format, MixerOptions::default().with_auto_close());

        let (track, ctrl) = mixer
            .create_track(Some(TrackOptions::with_label("sine_440")))
            .unwrap();

        println!("Created track: {:?}", ctrl.label());

        // Generate 100ms of 440Hz tone
        let wave = generate_sine_wave(440.0, format.sample_rate(), 100);
        println!("Generated {} bytes of 440Hz sine wave", wave.len());

        // Write in a separate thread
        let handle = thread::spawn(move || {
            track.write_bytes(&wave).unwrap();
            ctrl.close_write();
        });

        // Read output
        let mut output = Vec::new();
        let mut buf = [0u8; 1024];
        loop {
            match (&*mixer).read(&mut buf) {
                Ok(n) if n > 0 => output.extend_from_slice(&buf[..n]),
                Ok(_) => break,
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                Err(e) => {
                    eprintln!("Read error: {}", e);
                    break;
                }
            }
        }

        handle.join().unwrap();

        let (min, max, crossings) = analyze_audio(&output);
        println!("Output: {} bytes", output.len());
        println!("  Min sample: {}, Max sample: {}", min, max);
        println!("  Zero crossings: {}", crossings);
        println!();
    }

    // Example 2: Two tracks mixed together
    println!("--- Example 2: Two Tracks Mixed ---");
    {
        let mixer = Mixer::new(format, MixerOptions::default().with_auto_close());

        let (track1, ctrl1) = mixer
            .create_track(Some(TrackOptions::with_label("sine_440")))
            .unwrap();
        let (track2, ctrl2) = mixer
            .create_track(Some(TrackOptions::with_label("sine_880")))
            .unwrap();

        println!(
            "Created tracks: {:?} and {:?}",
            ctrl1.label(),
            ctrl2.label()
        );

        // Generate different frequencies
        let wave1 = generate_sine_wave(440.0, format.sample_rate(), 100);
        let wave2 = generate_sine_wave(880.0, format.sample_rate(), 100);

        // Write in parallel
        let h1 = thread::spawn(move || {
            track1.write_bytes(&wave1).unwrap();
            ctrl1.close_write();
        });

        let h2 = thread::spawn(move || {
            track2.write_bytes(&wave2).unwrap();
            ctrl2.close_write();
        });

        // Read output
        let mut output = Vec::new();
        let mut buf = [0u8; 1024];
        loop {
            match (&*mixer).read(&mut buf) {
                Ok(n) if n > 0 => output.extend_from_slice(&buf[..n]),
                Ok(_) => break,
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                Err(e) => {
                    eprintln!("Read error: {}", e);
                    break;
                }
            }
        }

        h1.join().unwrap();
        h2.join().unwrap();

        let (min, max, crossings) = analyze_audio(&output);
        println!("Output: {} bytes", output.len());
        println!("  Min sample: {}, Max sample: {}", min, max);
        println!("  Zero crossings: {} (should be higher than single track)", crossings);
        println!();
    }

    // Example 3: Gain control
    println!("--- Example 3: Gain Control ---");
    {
        let mixer = Mixer::new(format, MixerOptions::default().with_auto_close());

        let (track, ctrl) = mixer.create_track(None).unwrap();

        // Set gain to 0.5 (50% volume)
        ctrl.set_gain(0.5);
        println!("Track gain set to: {}", ctrl.gain());

        // Generate constant amplitude signal
        let data: Vec<u8> = (0..1600)
            .flat_map(|_| 20000i16.to_le_bytes())
            .collect();

        let handle = thread::spawn(move || {
            track.write_bytes(&data).unwrap();
            ctrl.close_write();
        });

        // Read output
        let mut output = Vec::new();
        let mut buf = [0u8; 1024];
        loop {
            match (&*mixer).read(&mut buf) {
                Ok(n) if n > 0 => output.extend_from_slice(&buf[..n]),
                Ok(_) => break,
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                Err(e) => {
                    eprintln!("Read error: {}", e);
                    break;
                }
            }
        }

        handle.join().unwrap();

        let (_min, max, _) = analyze_audio(&output);
        println!("Input amplitude: 20000");
        println!(
            "Output amplitude: ~{} (with 0.5 gain, expected ~10000)",
            max
        );
        println!();
    }

    // Example 4: Duration calculations for mixing
    println!("--- Example 4: Mixing Duration Info ---");
    {
        let duration_ms = 100;
        let bytes = format.bytes_in_duration(Duration::from_millis(duration_ms));
        let samples = format.samples_in_duration(Duration::from_millis(duration_ms));

        println!("For {}ms of audio at {}Hz:", duration_ms, format.sample_rate());
        println!("  Bytes: {}", bytes);
        println!("  Samples: {}", samples);
        println!("  Byte rate: {} bytes/second", format.bytes_rate());
    }

    println!("\n=== Demo Complete ===");
}
