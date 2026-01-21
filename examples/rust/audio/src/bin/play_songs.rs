//! Play songs demonstration - similar to Go's songs example.
//!
//! This example renders and plays songs using the songs library with mixer.
//!
//! Usage:
//!   bazel run //examples/rust/audio:play_songs
//!   bazel run //examples/rust/audio:play_songs -- --song=twinkle_star
//!   bazel run //examples/rust/audio:play_songs -- --list
//!
//! On macOS, this uses `afplay` to play the audio.
//! On other systems, it saves the WAV file for manual playback.

use giztoy_audio::pcm::Format;
use giztoy_audio::songs::{RenderOptions, Song};
use std::env;
use std::fs::File;
use std::io::Write;
use std::process::Command;

fn main() {
    let args: Vec<String> = env::args().collect();
    
    // Parse arguments
    let mut song_id: Option<&str> = None;
    let mut list_songs = false;
    let mut volume = 0.5f64;
    let mut output_file: Option<String> = None;
    let mut loop_play = false;
    
    let mut i = 1;
    while i < args.len() {
        let arg = &args[i];
        if arg == "--list" || arg == "-l" {
            list_songs = true;
        } else if arg.starts_with("--song=") {
            song_id = Some(arg.strip_prefix("--song=").unwrap());
        } else if arg == "--song" || arg == "-s" {
            i += 1;
            if i < args.len() {
                song_id = Some(&args[i]);
            }
        } else if arg.starts_with("--volume=") {
            volume = arg.strip_prefix("--volume=").unwrap().parse().unwrap_or(0.5);
        } else if arg.starts_with("--output=") || arg.starts_with("-o=") {
            output_file = Some(arg.split('=').nth(1).unwrap().to_string());
        } else if arg == "--loop" {
            loop_play = true;
        } else if arg == "--help" || arg == "-h" {
            print_help();
            return;
        }
        i += 1;
    }
    
    if list_songs {
        list_all_songs();
        return;
    }
    
    // Get playlist
    let playlist = if let Some(id) = song_id {
        match Song::by_id(id) {
            Some(song) => vec![song],
            None => {
                eprintln!("❌ Song not found: {}", id);
                eprintln!("Use --list to see available songs");
                std::process::exit(1);
            }
        }
    } else {
        // Play all songs
        Song::ids()
            .iter()
            .filter_map(|id| Song::by_id(id))
            .collect()
    };
    
    println!("🎵 Rust Songs Player");
    println!("   Playlist: {} songs, Volume: {:.0}%", playlist.len(), volume * 100.0);
    if loop_play {
        println!("   🔄 Loop mode enabled (Ctrl+C to stop)");
    }
    println!();
    
    let mut round = 0;
    loop {
        round += 1;
        if loop_play && round > 1 {
            println!("\n🔄 Restarting playlist (round {})...\n", round);
        }
        
        for (i, song) in playlist.iter().enumerate() {
            println!("▶️  [{}/{}] {} ({})", i + 1, playlist.len(), song.name, song.id);
            
            // Render the song
            let opts = RenderOptions::default()
                .with_format(Format::L16Mono16K)
                .with_volume(volume)
                .with_rich_sound(true);
            
            print!("   Rendering...");
            std::io::stdout().flush().ok();
            
            let pcm_data = song.render_bytes(opts.clone());
            
            let duration_ms = pcm_data.len() as u64 * 1000 / (opts.format.bytes_rate() as u64);
            println!(" {} bytes ({:.1}s)", pcm_data.len(), duration_ms as f64 / 1000.0);
            
            // Create WAV data
            let wav_data = create_wav(&pcm_data, opts.format);
            
            // Determine output file
            let wav_path = output_file.clone().unwrap_or_else(|| {
                let tmp_dir = std::env::temp_dir();
                tmp_dir.join(format!("rust_song_{}.wav", song.id)).to_string_lossy().to_string()
            });
            
            // Write WAV file
            let mut file = File::create(&wav_path).expect("Failed to create WAV file");
            file.write_all(&wav_data).expect("Failed to write WAV file");
            println!("   📁 Saved to: {}", wav_path);
            
            // Try to play with afplay (macOS) or aplay (Linux)
            print!("   🔊 Playing...");
            std::io::stdout().flush().ok();
            
            let play_result = if cfg!(target_os = "macos") {
                Command::new("afplay").arg(&wav_path).status()
            } else if cfg!(target_os = "linux") {
                Command::new("aplay").arg(&wav_path).status()
            } else {
                eprintln!(" (no player available, please play {} manually)", wav_path);
                continue;
            };
            
            match play_result {
                Ok(status) if status.success() => println!(" ✓"),
                Ok(_) => println!(" (player exited with error)"),
                Err(e) => println!(" (failed to play: {})", e),
            }
            
            println!();
        }
        
        if !loop_play {
            break;
        }
    }
    
    println!("🎶 Done!");
}

fn print_help() {
    println!("Play songs demonstration - Rust version");
    println!();
    println!("Usage: play_songs [OPTIONS]");
    println!();
    println!("Options:");
    println!("  --song=ID, -s ID    Play specific song by ID");
    println!("  --list, -l          List available songs");
    println!("  --volume=VOL        Set volume (0.0-1.0, default: 0.5)");
    println!("  --loop              Loop playback continuously");
    println!("  --output=FILE, -o   Save WAV to specific file");
    println!("  --help, -h          Show this help");
    println!();
    println!("Examples:");
    println!("  play_songs --list");
    println!("  play_songs --song=twinkle_star");
    println!("  play_songs --song=happy_birthday --volume=0.8");
    println!("  play_songs --loop   # Play all songs in loop");
}

fn list_all_songs() {
    println!("🎵 Available Songs:\n");
    
    let ids = Song::ids();
    for id in &ids {
        if let Some(song) = Song::by_id(id) {
            let voices = song.to_voices(false);
            let duration_s = song.duration() as f64 / 1000.0;
            println!("  {:20} {} ({}声部, {:.1}s)", 
                song.id, song.name, voices.len(), duration_s);
        }
    }
    
    println!("\nTotal: {} songs", ids.len());
}

/// Creates a WAV file from raw PCM data.
fn create_wav(pcm: &[u8], format: Format) -> Vec<u8> {
    let sample_rate = format.sample_rate();
    let channels = format.channels() as u16;
    let bits_per_sample = format.depth() as u16;
    let byte_rate = sample_rate * channels as u32 * bits_per_sample as u32 / 8;
    let block_align = channels * bits_per_sample / 8;
    let data_size = pcm.len() as u32;
    let file_size = 36 + data_size;
    
    let mut wav = Vec::with_capacity(44 + pcm.len());
    
    // RIFF header
    wav.extend_from_slice(b"RIFF");
    wav.extend_from_slice(&file_size.to_le_bytes());
    wav.extend_from_slice(b"WAVE");
    
    // fmt chunk
    wav.extend_from_slice(b"fmt ");
    wav.extend_from_slice(&16u32.to_le_bytes()); // chunk size
    wav.extend_from_slice(&1u16.to_le_bytes());  // audio format (PCM)
    wav.extend_from_slice(&channels.to_le_bytes());
    wav.extend_from_slice(&sample_rate.to_le_bytes());
    wav.extend_from_slice(&byte_rate.to_le_bytes());
    wav.extend_from_slice(&block_align.to_le_bytes());
    wav.extend_from_slice(&bits_per_sample.to_le_bytes());
    
    // data chunk
    wav.extend_from_slice(b"data");
    wav.extend_from_slice(&data_size.to_le_bytes());
    wav.extend_from_slice(pcm);
    
    wav
}
