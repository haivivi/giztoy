//! HTTP server with SSE support for live progress.
//!
//! API endpoints:
//! - GET /           - Static files (index.html, app.js, style.css)
//! - GET /api/status - RunnerStatus JSON
//! - GET /api/progress - {status, models} JSON
//! - GET /api/report - BenchmarkReport JSON
//! - GET /api/events - SSE progress stream

use std::convert::Infallible;
use std::net::SocketAddr;
use std::path::PathBuf;

use anyhow::Result;
use axum::{
    extract::State,
    http::StatusCode,
    response::{
        sse::{Event, Sse},
        Html, IntoResponse, Json,
    },
    routing::get,
    Router,
};
use futures::stream::Stream;
use serde::Serialize;
use tower_http::services::ServeDir;

use crate::runner::{BenchmarkReport, BenchmarkRunner};

/// SSE update message format
#[derive(Debug, Clone, Serialize)]
struct ProgressUpdate {
    #[serde(rename = "type")]
    update_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    status: Option<crate::runner::RunnerStatus>,
    #[serde(skip_serializing_if = "Option::is_none")]
    models: Option<Vec<crate::runner::ModelProgress>>,
}

/// Server state for live benchmark progress.
#[derive(Clone)]
struct LiveServerState {
    runner: BenchmarkRunner,
}

/// Server state for static report viewing.
#[derive(Clone)]
struct StaticServerState {
    report: BenchmarkReport,
}

/// Start HTTP server for live benchmark progress.
pub async fn start_live_server(
    addr: &str,
    runner: BenchmarkRunner,
    static_dir: Option<PathBuf>,
) -> Result<()> {
    let state = LiveServerState { runner };

    let mut app = Router::new()
        .route("/api/status", get(live_status))
        .route("/api/progress", get(live_progress))
        .route("/api/report", get(live_report))
        .route("/api/events", get(live_events))
        .with_state(state);

    // Serve static files or fallback to embedded
    if let Some(dir) = static_dir {
        if dir.exists() {
            app = app.fallback_service(ServeDir::new(dir));
        } else {
            eprintln!("Warning: static dir not found: {:?}", dir);
            app = app.route("/", get(fallback_index));
        }
    } else {
        app = app.route("/", get(fallback_index));
    }

    let addr = parse_addr(addr)?;
    println!("Server started at http://{}", addr);
    println!("  - GET /           Web UI");
    println!("  - GET /api/status  Current status");
    println!("  - GET /api/progress Progress for all models");
    println!("  - GET /api/report   Full report (JSON)");
    println!("  - GET /api/events   SSE progress stream");
    println!();

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}

/// Start HTTP server for static report viewing.
pub async fn start_server(
    addr: &str,
    report: BenchmarkReport,
    static_dir: Option<PathBuf>,
) -> Result<()> {
    let state = StaticServerState { report };

    let mut app = Router::new()
        .route("/api/status", get(static_status))
        .route("/api/report", get(static_report))
        .with_state(state);

    // Serve static files or fallback to embedded
    if let Some(dir) = static_dir {
        if dir.exists() {
            app = app.fallback_service(ServeDir::new(dir));
        } else {
            eprintln!("Warning: static dir not found: {:?}", dir);
            app = app.route("/", get(fallback_index));
        }
    } else {
        app = app.route("/", get(fallback_index));
    }

    let addr = parse_addr(addr)?;
    println!("Server started at http://{}", addr);
    println!("  - GET /           Web UI");
    println!("  - GET /api/status  Current status");
    println!("  - GET /api/report   Full report (JSON)");
    println!();

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}

/// Parse address string to SocketAddr.
fn parse_addr(addr: &str) -> Result<SocketAddr> {
    let addr = if addr.starts_with(':') {
        format!("0.0.0.0{}", addr)
    } else {
        addr.to_string()
    };
    Ok(addr.parse()?)
}

/// Fallback index page when no static dir is provided.
async fn fallback_index() -> impl IntoResponse {
    Html(FALLBACK_HTML)
}

// Live server handlers

async fn live_status(State(state): State<LiveServerState>) -> impl IntoResponse {
    let status = state.runner.get_status().await;
    Json(status)
}

async fn live_progress(State(state): State<LiveServerState>) -> impl IntoResponse {
    let status = state.runner.get_status().await;
    let progress = state.runner.get_progress().await;
    Json(serde_json::json!({
        "status": status,
        "models": progress,
    }))
}

async fn live_report(State(state): State<LiveServerState>) -> impl IntoResponse {
    match state.runner.get_report().await {
        Some(report) => Json(report).into_response(),
        None => Json(BenchmarkReport::default()).into_response(),
    }
}

async fn live_events(
    State(state): State<LiveServerState>,
) -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    let runner = state.runner.clone();

    let stream = async_stream::stream! {
        // Send initial state
        let status = runner.get_status().await;
        let progress = runner.get_progress().await;
        let update = ProgressUpdate {
            update_type: "init".to_string(),
            status: Some(status),
            models: Some(progress),
        };
        let data = serde_json::to_string(&update).unwrap_or_default();
        yield Ok(Event::default().data(data));

        // Poll for updates
        let mut interval = tokio::time::interval(tokio::time::Duration::from_millis(500));
        let mut last_done = 0usize;
        let mut last_status = String::new();

        loop {
            interval.tick().await;

            let status = runner.get_status().await;
            let progress = runner.get_progress().await;

            // Calculate total done
            let total_done: usize = progress.iter().map(|p| p.done).sum();

            // Send update if changed
            if total_done != last_done || status.status != last_status {
                last_done = total_done;
                last_status = status.status.clone();

                let update_type = if status.status == "done" {
                    "all_done"
                } else if total_done > 0 {
                    "case_done"
                } else {
                    "progress"
                };

                let update = ProgressUpdate {
                    update_type: update_type.to_string(),
                    status: Some(status.clone()),
                    models: Some(progress),
                };
                let data = serde_json::to_string(&update).unwrap_or_default();
                yield Ok(Event::default().data(data));

                if status.status == "done" {
                    break;
                }
            }
        }
    };

    Sse::new(stream)
}

// Static server handlers

async fn static_status(State(_state): State<StaticServerState>) -> impl IntoResponse {
    Json(serde_json::json!({"status": "static"}))
}

async fn static_report(State(state): State<StaticServerState>) -> impl IntoResponse {
    Json(state.report)
}

/// Minimal fallback HTML when no static dir is provided
const FALLBACK_HTML: &str = r##"<!DOCTYPE html>
<html lang="zh">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Match Benchmark</title>
    <style>
        :root { --bg: #0d1117; --text: #c9d1d9; --text-muted: #8b949e; --blue: #58a6ff; }
        body { font-family: -apple-system, sans-serif; background: var(--bg); color: var(--text); padding: 2rem; text-align: center; }
        h1 { margin-bottom: 1rem; }
        p { color: var(--text-muted); }
        a { color: var(--blue); }
        code { background: rgba(255,255,255,0.1); padding: 0.2rem 0.5rem; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>🎯 Match Benchmark</h1>
    <p>No static files directory specified.</p>
    <p>Use <code>--serve-static</code> to specify the HTML directory:</p>
    <p><code>--serve-static $(bazel info bazel-bin)/html/matchtest</code></p>
    <p style="margin-top: 2rem;">API endpoints available:</p>
    <p><a href="/api/status">/api/status</a> · <a href="/api/report">/api/report</a></p>
</body>
</html>
"##;
