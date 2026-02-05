//! Luau script runner for testing Haivivi SDK.
//!
//! Supports multiple runtime modes:
//! - minimal: Basic runtime with HTTP, JSON, KVS, etc.
//! - tool: Extended runtime with rt:* methods for LLM tools

use clap::Parser;
use giztoy_luau_runtime::Runtime;
use std::path::PathBuf;
use std::process;

#[derive(Parser)]
#[command(name = "luau-runner")]
#[command(about = "Luau script runner for testing Haivivi SDK")]
struct Args {
    /// Libs directory path (alias for --libs)
    #[arg(long)]
    dir: Option<PathBuf>,

    /// Libs directory path
    #[arg(long)]
    libs: Option<PathBuf>,

    /// Runtime mode: minimal, tool
    #[arg(long, default_value = "minimal")]
    runtime: String,

    /// Enable async HTTP mode (non-blocking with coroutines)
    #[arg(long, short = 'a')]
    r#async: bool,

    /// Script file to execute
    script: PathBuf,
}

#[tokio::main]
async fn main() {
    let args = Args::parse();

    // Resolve libs directory (--dir takes precedence over --libs)
    let libs_dir = args
        .dir
        .clone()
        .or_else(|| args.libs.clone())
        .unwrap_or_else(|| {
            args.script
                .parent()
                .map(|p| p.join("..").join("libs"))
                .unwrap_or_else(|| PathBuf::from("libs"))
        });

    // Dispatch to appropriate runtime
    match args.runtime.as_str() {
        "minimal" => run_minimal(&args, libs_dir).await,
        "tool" => run_tool(&args, libs_dir).await,
        "agent" => run_agent(&args, libs_dir).await,
        _ => {
            eprintln!("unknown runtime: {}", args.runtime);
            process::exit(1);
        }
    }
}

/// Run script with minimal runtime (HTTP, JSON, KVS, etc.)
async fn run_minimal(args: &Args, libs_dir: PathBuf) {
    // Create runtime
    let mut rt = match Runtime::new(libs_dir) {
        Ok(rt) => rt,
        Err(e) => {
            eprintln!("failed to create runtime: {}", e);
            process::exit(1);
        }
    };

    // Set async mode before registering builtins
    rt.set_async_mode(args.r#async);

    // Register builtins
    if let Err(e) = rt.register_builtins() {
        eprintln!("failed to register builtins: {}", e);
        process::exit(1);
    }

    // Update async context after registering (for thread-local storage)
    rt.update_async_context();

    // Pre-compile all modules in libs directory
    if let Err(e) = rt.precompile_modules() {
        eprintln!("failed to precompile modules: {}", e);
        process::exit(1);
    }

    // Read script
    let source = match std::fs::read_to_string(&args.script) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("failed to read script: {}", e);
            process::exit(1);
        }
    };

    let script_name = args.script.to_string_lossy().to_string();

    if args.r#async {
        // Async mode: run script in a thread with event loop
        if let Err(e) = rt.run_async(&source, &script_name).await {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    } else {
        // Sync mode: execute script directly (blocking HTTP)
        if let Err(e) =
            rt.state
                .do_string_opt(&source, &script_name, giztoy_luau::OptLevel::O2)
        {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    }
}

/// Run script with tool runtime (extended rt:* methods)
async fn run_tool(args: &Args, libs_dir: PathBuf) {
    // Create runtime
    let mut rt = match Runtime::new(libs_dir) {
        Ok(rt) => rt,
        Err(e) => {
            eprintln!("failed to create runtime: {}", e);
            process::exit(1);
        }
    };

    // Set async mode before registering builtins
    rt.set_async_mode(args.r#async);

    // Register builtins (same as minimal for now)
    if let Err(e) = rt.register_builtins() {
        eprintln!("failed to register builtins: {}", e);
        process::exit(1);
    }

    // Register tool runtime extensions (rt:* methods)
    if let Err(e) = register_tool_runtime(&mut rt) {
        eprintln!("failed to register tool runtime: {}", e);
        process::exit(1);
    }

    // Update async context after registering (for thread-local storage)
    rt.update_async_context();

    // Pre-compile all modules in libs directory
    if let Err(e) = rt.precompile_modules() {
        eprintln!("failed to precompile modules: {}", e);
        process::exit(1);
    }

    // Read script
    let source = match std::fs::read_to_string(&args.script) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("failed to read script: {}", e);
            process::exit(1);
        }
    };

    let script_name = args.script.to_string_lossy().to_string();

    if args.r#async {
        // Async mode: run script in a thread with event loop
        if let Err(e) = rt.run_async(&source, &script_name).await {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    } else {
        // Sync mode: execute script directly (blocking HTTP)
        if let Err(e) =
            rt.state
                .do_string_opt(&source, &script_name, giztoy_luau::OptLevel::O2)
        {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    }
}

/// Register RuntimeCtx functions (core capabilities shared by tool and agent).
/// Note: When called as rt:method(...), the first argument (index 1) is 'self' (the rt table).
/// Actual arguments start at index 2.
fn register_runtime_ctx(rt: &mut Runtime) -> Result<(), giztoy_luau::Error> {
    use giztoy_luau::LuaStackOps;
    use giztoy_luau_runtime::builtin::json::{json_to_lua, lua_to_json};
    use std::time::{SystemTime, UNIX_EPOCH};

    // Create rt table
    rt.state.new_table();

    // rt:time() -> number
    // When called as rt:time(), index 1 is 'self' (rt table)
    rt.state.register_func("__rt_time", |state| {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs_f64())
            .unwrap_or(0.0);
        state.push_number(now);
        1
    })?;
    rt.state.get_global("__rt_time")?;
    rt.state.set_field(-2, "time")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_time")?;

    // rt:parse_time(iso_string) -> number | nil
    // When called as rt:parse_time(str), index 1 is 'self', index 2 is the iso_string
    rt.state.register_func("__rt_parse_time", |state| {
        let iso_str = state.to_string(2).unwrap_or_default(); // index 2 for method call
        if iso_str.is_empty() {
            state.push_nil();
            return 1;
        }

        // Try to parse ISO 8601 date (simple implementation)
        if let Some(ts) = parse_iso8601_simple(&iso_str) {
            state.push_number(ts);
        } else {
            state.push_nil();
        }
        1
    })?;
    rt.state.get_global("__rt_parse_time")?;
    rt.state.set_field(-2, "parse_time")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_parse_time")?;

    // rt:json_encode(value) -> string
    // When called as rt:json_encode(val), index 1 is 'self', index 2 is the value
    rt.state.register_func("__rt_json_encode", |state| {
        let value = lua_to_json(state, 2); // index 2 for method call
        match serde_json::to_string(&value) {
            Ok(s) => {
                state.push_string(&s).ok();
                1
            }
            Err(_) => {
                state.push_nil();
                1
            }
        }
    })?;
    rt.state.get_global("__rt_json_encode")?;
    rt.state.set_field(-2, "json_encode")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_json_encode")?;

    // rt:json_decode(str) -> value, err
    // When called as rt:json_decode(str), index 1 is 'self', index 2 is the string
    rt.state.register_func("__rt_json_decode", |state| {
        let s = state.to_string(2).unwrap_or_default(); // index 2 for method call
        match serde_json::from_str::<serde_json::Value>(&s) {
            Ok(value) => {
                json_to_lua(state, &value);
                state.push_nil(); // no error
                2
            }
            Err(e) => {
                state.push_nil();
                state.push_string(&e.to_string()).ok();
                2
            }
        }
    })?;
    rt.state.get_global("__rt_json_decode")?;
    rt.state.set_field(-2, "json_decode")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_json_decode")?;

    // rt:env(key) -> value
    // When called as rt:env(key), index 1 is 'self', index 2 is the key
    rt.state.register_func("__rt_env", |state| {
        let key = state.to_string(2).unwrap_or_default(); // index 2 for method call
        if key.is_empty() {
            state.push_nil();
            return 1;
        }

        match std::env::var(&key) {
            Ok(value) => {
                state.push_string(&value).ok();
                1
            }
            Err(_) => {
                state.push_nil();
                1
            }
        }
    })?;
    rt.state.get_global("__rt_env")?;
    rt.state.set_field(-2, "env")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_env")?;

    // rt:log(...) - variadic log function
    // When called as rt:log(...), index 1 is 'self', actual args start at index 2
    rt.state.register_func("__rt_log", |state| {
        use giztoy_luau::Type;

        let n = state.get_top();
        let mut parts = Vec::with_capacity(n as usize);

        // Skip index 1 (self), start from index 2
        for i in 2..=n {
            let part = match state.get_type(i) {
                Type::Nil => "nil".to_string(),
                Type::Boolean => {
                    if state.to_boolean(i) {
                        "true".to_string()
                    } else {
                        "false".to_string()
                    }
                }
                Type::Number => format!("{}", state.to_number(i)),
                Type::String => state.to_string(i).unwrap_or_default(),
                Type::Table => "[table]".to_string(),
                Type::Function => "[function]".to_string(),
                _ => format!("[{}]", state.type_name(i)),
            };
            parts.push(part);
        }

        println!("{}", parts.join("\t"));
        0
    })?;
    rt.state.get_global("__rt_log")?;
    rt.state.set_field(-2, "log")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_log")?;

    // Set rt global
    rt.state.set_global("rt")?;

    Ok(())
}

/// Register ToolIO functions (input/output for one-shot tool execution).
fn register_tool_io(rt: &mut Runtime) -> Result<(), giztoy_luau::Error> {
    use giztoy_luau::LuaStackOps;

    // Get rt table
    rt.state.get_global("rt")?;

    // rt:input() -> table (placeholder)
    // No args besides self
    rt.state.register_func("__rt_input", |state| {
        state.new_table();
        1
    })?;
    rt.state.get_global("__rt_input")?;
    rt.state.set_field(-2, "input")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_input")?;

    // rt:output(value, err) (placeholder)
    // Args: self (1), value (2), err (3)
    rt.state.register_func("__rt_output", |_state| 0)?;
    rt.state.get_global("__rt_output")?;
    rt.state.set_field(-2, "output")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_output")?;

    // Pop rt table
    rt.state.pop(1);

    Ok(())
}

/// Register tool runtime (RuntimeCtx + ToolIO).
fn register_tool_runtime(rt: &mut Runtime) -> Result<(), giztoy_luau::Error> {
    register_runtime_ctx(rt)?;
    register_tool_io(rt)?;
    Ok(())
}

/// Simple ISO 8601 parser for tool runtime
fn parse_iso8601_simple(s: &str) -> Option<f64> {
    let s = s.trim();
    if s.len() < 10 {
        return None;
    }

    // Parse date part: YYYY-MM-DD
    let year: i32 = s.get(0..4)?.parse().ok()?;
    if s.get(4..5)? != "-" {
        return None;
    }
    let month: u32 = s.get(5..7)?.parse().ok()?;
    if s.get(7..8)? != "-" {
        return None;
    }
    let day: u32 = s.get(8..10)?.parse().ok()?;

    // Default time to midnight
    let mut hour: u32 = 0;
    let mut minute: u32 = 0;
    let mut second: u32 = 0;
    let mut tz_offset_seconds: i64 = 0;

    // Parse time part if present
    if let Some(rest) = s.get(10..) {
        if !rest.is_empty() {
            let rest = if rest.starts_with('T') || rest.starts_with(' ') {
                &rest[1..]
            } else {
                return Some(calculate_timestamp(year, month, day, 0, 0, 0, 0)?);
            };

            if rest.len() >= 8 {
                hour = rest.get(0..2)?.parse().ok()?;
                minute = rest.get(3..5)?.parse().ok()?;
                second = rest.get(6..8)?.parse().ok()?;

                // Parse timezone
                if let Some(tz_part) = rest.get(8..) {
                    if tz_part.starts_with('+') || tz_part.starts_with('-') {
                        let sign = if tz_part.starts_with('+') { 1 } else { -1 };
                        let tz_str = &tz_part[1..];
                        if tz_str.len() >= 5 && tz_str.get(2..3) == Some(":") {
                            let tz_hour: i64 = tz_str.get(0..2)?.parse().ok()?;
                            let tz_min: i64 = tz_str.get(3..5)?.parse().ok()?;
                            tz_offset_seconds = sign * (tz_hour * 3600 + tz_min * 60);
                        }
                    }
                }
            }
        }
    }

    calculate_timestamp(year, month, day, hour, minute, second, tz_offset_seconds)
}

fn calculate_timestamp(
    year: i32,
    month: u32,
    day: u32,
    hour: u32,
    minute: u32,
    second: u32,
    tz_offset_seconds: i64,
) -> Option<f64> {
    if month < 1 || month > 12 || day < 1 || day > 31 {
        return None;
    }

    let days_in_month = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    let is_leap = |y: i32| (y % 4 == 0 && y % 100 != 0) || (y % 400 == 0);

    let mut days: i64 = 0;
    if year >= 1970 {
        for y in 1970..year {
            days += if is_leap(y) { 366 } else { 365 };
        }
    } else {
        for y in year..1970 {
            days -= if is_leap(y) { 366 } else { 365 };
        }
    }

    for m in 1..month {
        days += days_in_month[(m - 1) as usize] as i64;
        if m == 2 && is_leap(year) {
            days += 1;
        }
    }

    days += (day - 1) as i64;

    let total_seconds =
        days * 86400 + hour as i64 * 3600 + minute as i64 * 60 + second as i64 - tz_offset_seconds;

    Some(total_seconds as f64)
}

/// Run script with agent runtime (rt:recv/rt:emit for streaming I/O)
async fn run_agent(args: &Args, libs_dir: PathBuf) {
    use std::sync::{Arc, Mutex};
    use tokio::sync::mpsc;

    // Create runtime
    let mut rt = match Runtime::new(libs_dir) {
        Ok(rt) => rt,
        Err(e) => {
            eprintln!("failed to create runtime: {}", e);
            process::exit(1);
        }
    };

    // Set async mode before registering builtins
    rt.set_async_mode(args.r#async);

    // Register builtins (same as minimal)
    if let Err(e) = rt.register_builtins() {
        eprintln!("failed to register builtins: {}", e);
        process::exit(1);
    }

    // Register RuntimeCtx (core capabilities, without ToolIO)
    if let Err(e) = register_runtime_ctx(&mut rt) {
        eprintln!("failed to register runtime ctx: {}", e);
        process::exit(1);
    }

    // Create channels for agent I/O
    let (input_tx, input_rx) = mpsc::channel::<String>(1);
    let (output_tx, output_rx) = mpsc::channel::<String>(16);

    // Store channels in Arc<Mutex<>> for access from Luau callbacks
    let input_rx = Arc::new(Mutex::new(Some(input_rx)));
    let output_tx = Arc::new(Mutex::new(Some(output_tx)));

    // Register AgentIO functions (recv/emit)
    if let Err(e) = register_agent_io(&mut rt, input_rx.clone(), output_tx.clone()) {
        eprintln!("failed to register agent io: {}", e);
        process::exit(1);
    }

    // Update async context after registering (for thread-local storage)
    rt.update_async_context();

    // Pre-compile all modules in libs directory
    if let Err(e) = rt.precompile_modules() {
        eprintln!("failed to precompile modules: {}", e);
        process::exit(1);
    }

    // Read script
    let source = match std::fs::read_to_string(&args.script) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("failed to read script: {}", e);
            process::exit(1);
        }
    };

    let script_name = args.script.to_string_lossy().to_string();

    // Send initial empty input
    let _ = input_tx.send(String::new()).await;
    // Close input
    drop(input_tx);

    // Spawn output collector
    let output_rx = Arc::new(Mutex::new(Some(output_rx)));
    let output_rx_clone = output_rx.clone();
    let output_collector = tokio::spawn(async move {
        let mut output = String::new();
        let rx = output_rx_clone.lock().unwrap().take();
        if let Some(mut rx) = rx {
            while let Some(chunk) = rx.recv().await {
                output.push_str(&chunk);
            }
        }
        output
    });

    // Run script
    if args.r#async {
        if let Err(e) = rt.run_async(&source, &script_name).await {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    } else {
        if let Err(e) =
            rt.state
                .do_string_opt(&source, &script_name, giztoy_luau::OptLevel::O2)
        {
            eprintln!("script error: {}", e);
            process::exit(1);
        }
    }

    // Close output channel
    if let Some(tx) = output_tx.lock().unwrap().take() {
        drop(tx);
    }

    // Collect and print output
    if let Ok(output) = output_collector.await {
        if !output.is_empty() {
            print!("{}", output);
        }
    }
}

/// Register AgentIO functions (recv/emit for streaming conversation).
fn register_agent_io(
    rt: &mut Runtime,
    input_rx: Arc<Mutex<Option<tokio::sync::mpsc::Receiver<String>>>>,
    output_tx: Arc<Mutex<Option<tokio::sync::mpsc::Sender<String>>>>,
) -> Result<(), giztoy_luau::Error> {
    use giztoy_luau::LuaStackOps;

    // Get rt table (should exist from register_tool_runtime)
    rt.state.get_global("rt")?;

    // rt:recv() -> contents, err
    // In CLI mode, this is a simple blocking recv from the input channel.
    // For real async, this would need to yield and be resumed.
    let input_rx_clone = input_rx.clone();
    rt.state.register_func("__rt_recv", move |state| {
        let rx = input_rx_clone.lock().unwrap().take();
        if let Some(mut rx) = rx {
            // Try to receive (blocking in sync mode)
            match rx.try_recv() {
                Ok(text) => {
                    // Return as contents table (array with one text element)
                    state.new_table(); // contents array

                    // Create element at index 1
                    state.push_number(1.0); // key
                    state.new_table(); // value = text part
                    state.push_string("text").ok();
                    state.set_field(-2, "type").ok();
                    state.push_string(&text).ok();
                    state.set_field(-2, "text").ok();
                    state.set_table(-3); // contents[1] = part

                    state.push_nil(); // no error
                    // Put receiver back
                    *input_rx_clone.lock().unwrap() = Some(rx);
                    2
                }
                Err(_) => {
                    // No more input
                    state.push_nil();
                    state.push_nil();
                    *input_rx_clone.lock().unwrap() = Some(rx);
                    2
                }
            }
        } else {
            // Channel already taken (closed)
            state.push_nil();
            state.push_string("agent closed").ok();
            2
        }
    })?;
    rt.state.get_global("__rt_recv")?;
    rt.state.set_field(-2, "recv")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_recv")?;

    // rt:emit(chunk) -> err
    // chunk is a table with: text (string), eof (boolean, optional)
    let output_tx_clone = output_tx.clone();
    rt.state.register_func("__rt_emit", move |state| {
        use giztoy_luau_runtime::builtin::json::lua_to_json;

        // Get chunk table (arg 2, since arg 1 is self)
        let chunk = lua_to_json(state, 2);

        // Extract text from chunk
        let text = chunk.get("text").and_then(|v| v.as_str()).unwrap_or("");

        if let Some(tx) = output_tx_clone.lock().unwrap().as_ref() {
            // Try to send
            if tx.try_send(text.to_string()).is_err() {
                state.push_string("failed to send output").ok();
                return 1;
            }
        }

        state.push_nil(); // no error
        1
    })?;
    rt.state.get_global("__rt_emit")?;
    rt.state.set_field(-2, "emit")?;
    rt.state.push_nil();
    rt.state.set_global("__rt_emit")?;

    // Pop rt table
    rt.state.pop(1);

    Ok(())
}

use std::sync::{Arc, Mutex};
