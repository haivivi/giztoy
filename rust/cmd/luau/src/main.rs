//! Luau script runner for testing Haivivi SDK.

mod builtin;
mod runtime;

use clap::Parser;
use runtime::Runtime;
use std::path::PathBuf;
use std::process;

#[derive(Parser)]
#[command(name = "luau-runner")]
#[command(about = "Luau script runner for testing Haivivi SDK")]
struct Args {
    /// Libs directory path
    #[arg(long)]
    libs: Option<PathBuf>,

    /// Script file to execute
    script: PathBuf,
}

fn main() {
    let args = Args::parse();

    // Resolve libs directory
    let libs_dir = args.libs.unwrap_or_else(|| {
        args.script
            .parent()
            .map(|p| p.join("..").join("libs"))
            .unwrap_or_else(|| PathBuf::from("libs"))
    });

    // Create runtime
    let mut rt = match Runtime::new(libs_dir) {
        Ok(rt) => rt,
        Err(e) => {
            eprintln!("failed to create runtime: {}", e);
            process::exit(1);
        }
    };

    // Register builtins
    if let Err(e) = rt.register_builtins() {
        eprintln!("failed to register builtins: {}", e);
        process::exit(1);
    }

    // Read and execute script
    let source = match std::fs::read_to_string(&args.script) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("failed to read script: {}", e);
            process::exit(1);
        }
    };

    let script_name = args.script.to_string_lossy().to_string();
    if let Err(e) = rt.state.do_string_opt(&source, &script_name, giztoy_luau::OptLevel::O2) {
        eprintln!("script error: {}", e);
        process::exit(1);
    }
}
