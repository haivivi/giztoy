//! matchtest - Benchmark tool for match-based intent matching.

mod config;
mod runner;
mod server;

use std::path::PathBuf;

use anyhow::Result;
use clap::Parser;
use giztoy_genx::r#match::{CompileOptions, Matcher, Rule};

/// Benchmark tool for match-based intent matching.
#[derive(Parser, Debug)]
#[command(name = "matchtest")]
#[command(about = "Benchmark tool for match-based intent matching")]
struct Args {
    /// Model pattern (e.g. gemini/flash, sf/, all)
    #[arg(short, long)]
    model: Option<String>,

    /// List all available models
    #[arg(long)]
    list: bool,

    /// Rules directory (default: embedded rules)
    #[arg(long)]
    rules: Option<PathBuf>,

    /// Custom prompt template file path
    #[arg(long)]
    tpl: Option<PathBuf>,

    /// Output JSON report to file
    #[arg(short = 'o', long)]
    output: Option<PathBuf>,

    /// Start HTTP server for live progress (e.g. :8080)
    #[arg(long)]
    serve: Option<String>,

    /// Load existing report JSON(s) and serve (comma-separated for multiple files)
    #[arg(long)]
    load: Option<String>,

    /// Models config directory (required)
    #[arg(long)]
    models: Option<PathBuf>,

    /// Quiet mode (less output)
    #[arg(short = 'q', long)]
    quiet: bool,

    /// Print the generated system prompt and exit
    #[arg(long)]
    prompt: bool,

    /// Print HTTP request body for debugging
    #[arg(long)]
    verbose: bool,

    /// Path to HTML template file (legacy, use --serve-static instead)
    #[arg(long)]
    template: Option<PathBuf>,

    /// Path to static files directory (e.g., bazel-bin/html/matchtest)
    #[arg(long)]
    serve_static: Option<PathBuf>,
}

/// Rule file with optional tests.
#[derive(Debug, Clone, serde::Deserialize)]
struct RuleFile {
    #[serde(flatten)]
    rule: Rule,
    #[serde(default)]
    tests: Vec<TestDef>,
}

/// Test definition within a rule file.
#[derive(Debug, Clone, serde::Deserialize)]
struct TestDef {
    input: String,
    #[serde(default)]
    args: std::collections::HashMap<String, String>,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // Load and serve existing report(s)
    if let Some(load_paths) = &args.load {
        let paths: Vec<&str> = load_paths.split(',').map(|s| s.trim()).collect();
        let report = runner::load_reports(&paths)?;
        let addr = args.serve.as_deref().unwrap_or(":8080");
        server::start_server(addr, report, args.serve_static.clone()).await?;
        return Ok(());
    }

    // Print prompt mode - doesn't require model selection
    if args.prompt {
        let rule_files = load_rules(&args.rules)?;
        let rules: Vec<Rule> = rule_files.into_iter().map(|rf| rf.rule).collect();
        let opts = load_compile_options(&args.tpl)?;
        let matcher = Matcher::compile(&rules, opts)?;
        println!("{}", matcher.system_prompt());
        return Ok(());
    }

    // Models dir is required for other operations
    let models_dir = match &args.models {
        Some(dir) => dir.clone(),
        None => {
            print_usage();
            return Ok(());
        }
    };

    // Register models from configs
    let all_models = config::load_from_dir(&models_dir, args.verbose).await?;

    // List models and exit
    if args.list {
        println!("Available models:");
        for m in &all_models {
            println!("  {}", m);
        }
        return Ok(());
    }

    // Determine which models to test
    let models = match &args.model {
        Some(pattern) => {
            let matched = match_models(pattern, &all_models);
            if matched.is_empty() {
                anyhow::bail!("no models matched pattern: {}", pattern);
            }
            matched
        }
        None => {
            print_usage();
            return Ok(());
        }
    };

    if !args.quiet {
        println!("=== Models Selected ({}) ===", models.len());
        for m in &models {
            println!("  - {}", m);
        }
    }

    // Load rules and tests from JSON/YAML files
    let rule_files = load_rules(&args.rules)?;

    // Extract rules and tests
    let mut rules = Vec::new();
    let mut cases = Vec::new();
    for rf in rule_files {
        let rule_name = rf.rule.name.clone();
        rules.push(rf.rule);
        for t in rf.tests {
            cases.push(runner::TestCase {
                input: t.input,
                expected: runner::ExpectedResult {
                    rule: rule_name.clone(),
                    args: t.args,
                },
            });
        }
    }

    // Sort rules by name
    rules.sort_by(|a, b| a.name.cmp(&b.name));

    if !args.quiet {
        println!("=== Loaded {} rules, {} tests ===", rules.len(), cases.len());
        for r in &rules {
            println!(
                "  - {} ({} patterns, {} vars)",
                r.name,
                r.patterns.len(),
                r.vars.len()
            );
        }
        println!();
    }

    // Compile rules
    let opts = load_compile_options(&args.tpl)?;
    let matcher = Matcher::compile(&rules, opts)?;

    if !args.quiet {
        println!("=== Compiled Successfully ===");
        println!(
            "\n=== Testing {} model(s) with {} test cases ===",
            models.len(),
            cases.len()
        );
    }

    // Create runner
    let runner_handle = runner::BenchmarkRunner::new(matcher, models.clone(), cases, rules.len());

    // If serve mode, start server first then run benchmark
    if let Some(addr) = &args.serve {
        let server_runner = runner_handle.clone();
        let addr_clone = addr.clone();
        let static_dir = args.serve_static.clone();

        // Start server in background
        tokio::spawn(async move {
            if let Err(e) =
                server::start_live_server(&addr_clone, server_runner, static_dir).await
            {
                eprintln!("Server error: {}", e);
            }
        });

        println!("\nOpen http://{} in browser to view progress\n", addr);
    }

    // Run benchmark
    let report = runner_handle.run().await;

    // Print summary
    if !args.quiet {
        runner::print_summary(&report);
    }

    // Save to file
    if let Some(output) = &args.output {
        runner::save_report(&report, output)?;
        println!("\nReport saved to {}", output.display());
    }

    // If serve mode, keep server running
    if args.serve.is_some() {
        println!(
            "\nBenchmark complete. Server still running at http://{}",
            args.serve.as_ref().unwrap()
        );
        println!("Press Ctrl+C to stop");
        tokio::signal::ctrl_c().await?;
    } else if args.output.is_some() {
        // Print hint on how to view results
        println!(
            "\nTo view results in browser:\n  matchtest -load {} -serve :8080",
            args.output.as_ref().unwrap().display()
        );
    }

    Ok(())
}

/// Load rules from directory or use embedded rules.
fn load_rules(rules_dir: &Option<PathBuf>) -> Result<Vec<RuleFile>> {
    match rules_dir {
        Some(dir) => load_rules_from_dir(dir),
        None => {
            // For now, return empty - embedded rules will be added later
            Ok(vec![])
        }
    }
}

/// Load rules from a directory recursively.
fn load_rules_from_dir(dir: &PathBuf) -> Result<Vec<RuleFile>> {
    let mut rule_files = Vec::new();

    for entry in walkdir(dir)? {
        let path = entry;
        let ext = path.extension().and_then(|s| s.to_str()).unwrap_or("");

        if ext != "json" && ext != "yaml" && ext != "yml" {
            continue;
        }

        let data = std::fs::read(&path)?;
        let rf: RuleFile = match ext {
            "json" => serde_json::from_slice(&data)?,
            "yaml" | "yml" => serde_yaml::from_slice(&data)?,
            _ => continue,
        };
        rule_files.push(rf);
    }

    Ok(rule_files)
}

/// Simple directory walk.
fn walkdir(dir: &PathBuf) -> Result<Vec<PathBuf>> {
    let mut paths = Vec::new();

    fn walk(dir: &PathBuf, paths: &mut Vec<PathBuf>) -> Result<()> {
        for entry in std::fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_dir() {
                walk(&path, paths)?;
            } else {
                paths.push(path);
            }
        }
        Ok(())
    }

    walk(dir, &mut paths)?;
    Ok(paths)
}

/// Load compile options from template file.
fn load_compile_options(tpl_path: &Option<PathBuf>) -> Result<CompileOptions> {
    let mut opts = CompileOptions::default();

    if let Some(path) = tpl_path {
        let data = std::fs::read_to_string(path)?;
        opts = opts.with_template(data);
    }

    Ok(opts)
}

/// Match models against pattern.
fn match_models(pattern: &str, all_models: &[String]) -> Vec<String> {
    let pattern = pattern.trim();
    if pattern == "all" {
        return all_models.to_vec();
    }

    let patterns: Vec<&str> = pattern.split(',').map(|s| s.trim()).collect();
    let mut matched = Vec::new();
    let mut seen = std::collections::HashSet::new();

    for p in patterns {
        if p.is_empty() {
            continue;
        }
        if p == "all" {
            return all_models.to_vec();
        }

        for m in all_models {
            if seen.contains(m) {
                continue;
            }
            // Check exact match or prefix match
            if m == p || m.starts_with(p) {
                matched.push(m.clone());
                seen.insert(m.clone());
            }
        }
    }

    matched
}

fn print_usage() {
    println!("Usage:");
    println!("  matchtest -models <dir> -model <pattern>              Run benchmark");
    println!("  matchtest -models <dir> -model <pattern> -serve :8080 Run with web UI");
    println!("  matchtest -models <dir> -list                         List available models");
    println!("  matchtest -load <file.json> -serve :8080              View saved results");
    println!();
    println!("Model patterns:");
    println!("  -model gemini/flash            Exact model name");
    println!("  -model sf/                     All models starting with 'sf/'");
    println!("  -model sf/,openai/             Multiple prefixes (comma-separated)");
    println!("  -model all                     All registered models");
    println!();
    println!("Options:");
    println!("  -models <dir>                  Models config directory (required)");
    println!("  -rules <dir>                   Rules directory (default: embedded)");
    println!("  -tpl <file.gotmpl>             Custom prompt template file");
    println!("  -o <file.json>                 Save results to JSON file");
    println!("  -serve :8080                   Start web server with live progress");
    println!("  -q                             Quiet mode (no console output)");
    println!("  -prompt                        Print system prompt and exit");
}
