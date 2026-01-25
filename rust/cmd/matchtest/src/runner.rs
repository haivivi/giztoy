//! Benchmark runner implementation.

use std::collections::HashMap;
use std::path::Path;
use std::sync::Arc;

use anyhow::Result;
use chrono::Utc;
use giztoy_genx::context::ModelContextBuilder;
use giztoy_genx::r#match::Matcher;
use giztoy_genx::types::Part;
use serde::{Deserialize, Serialize};
use tokio::sync::RwLock;

use crate::config;

/// Test case with input and expected output.
#[derive(Debug, Clone)]
pub struct TestCase {
    pub input: String,
    pub expected: ExpectedResult,
}

/// Expected result for a test case.
#[derive(Debug, Clone)]
pub struct ExpectedResult {
    pub rule: String,
    pub args: HashMap<String, String>,
}

/// Actual result from matching (JSON-friendly).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ActualResult {
    pub rule: String,
    #[serde(default)]
    pub args: HashMap<String, String>,
}

/// Result of running one test case.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CaseResult {
    pub input: String,
    pub expected: ActualResult,
    pub actual: ActualResult,
    pub duration_ms: i64,
    pub status: String, // "pass", "fail", "error"
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Aggregate results for one model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelResult {
    pub model: String,
    pub total_cases: usize,
    pub passed: usize,
    pub failed: usize,
    pub errors: usize,
    pub pass_rate: f64,
    pub p50_ms: i64,
    pub p95_ms: i64,
    pub p99_ms: i64,
    pub cases: Vec<CaseResult>,
}

/// Full benchmark report.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct BenchmarkReport {
    pub timestamp: String,
    pub rule_count: usize,
    pub test_count: usize,
    pub models: Vec<ModelResult>,
}

/// Runner status.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunnerStatus {
    pub status: String, // "idle", "running", "done"
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub started_at: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub finished_at: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub duration: Option<String>,
}

/// Model progress tracking.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelProgress {
    pub model: String,
    pub status: String, // "pending", "running", "done", "error"
    pub total: usize,
    pub done: usize,
    pub passed: usize,
    pub failed: usize,
    pub errors: usize,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    pub percent: f64,
}

/// Benchmark runner with progress tracking.
#[derive(Clone)]
pub struct BenchmarkRunner {
    inner: Arc<RwLock<RunnerInner>>,
}

struct RunnerInner {
    matcher: Matcher,
    models: Vec<String>,
    cases: Vec<TestCase>,
    rule_count: usize,
    status: RunnerStatus,
    progress: HashMap<String, ModelProgress>,
    report: Option<BenchmarkReport>,
}

impl BenchmarkRunner {
    /// Create a new benchmark runner.
    pub fn new(
        matcher: Matcher,
        models: Vec<String>,
        cases: Vec<TestCase>,
        rule_count: usize,
    ) -> Self {
        let mut progress = HashMap::new();
        for m in &models {
            progress.insert(
                m.clone(),
                ModelProgress {
                    model: m.clone(),
                    status: "pending".to_string(),
                    total: cases.len(),
                    done: 0,
                    passed: 0,
                    failed: 0,
                    errors: 0,
                    error: None,
                    percent: 0.0,
                },
            );
        }

        Self {
            inner: Arc::new(RwLock::new(RunnerInner {
                matcher,
                models,
                cases,
                rule_count,
                status: RunnerStatus {
                    status: "idle".to_string(),
                    started_at: None,
                    finished_at: None,
                    duration: None,
                },
                progress,
                report: None,
            })),
        }
    }

    /// Get current status.
    pub async fn get_status(&self) -> RunnerStatus {
        self.inner.read().await.status.clone()
    }

    /// Get progress for all models.
    pub async fn get_progress(&self) -> Vec<ModelProgress> {
        let inner = self.inner.read().await;
        inner
            .models
            .iter()
            .filter_map(|m| inner.progress.get(m).cloned())
            .collect()
    }

    /// Get the current report (may be partial if still running).
    pub async fn get_report(&self) -> Option<BenchmarkReport> {
        self.inner.read().await.report.clone()
    }

    /// Run the benchmark.
    pub async fn run(&self) -> BenchmarkReport {
        let start_time = Utc::now();

        // Update status
        {
            let mut inner = self.inner.write().await;
            inner.status = RunnerStatus {
                status: "running".to_string(),
                started_at: Some(start_time.to_rfc3339()),
                finished_at: None,
                duration: None,
            };
            inner.report = Some(BenchmarkReport {
                timestamp: start_time.to_rfc3339(),
                rule_count: inner.rule_count,
                test_count: inner.cases.len(),
                models: vec![],
            });
        }

        // Run each model
        let models = self.inner.read().await.models.clone();
        for model in models {
            self.run_model(&model).await;
        }

        // Mark done
        let end_time = Utc::now();
        let duration = end_time.signed_duration_since(start_time);

        let report = {
            let mut inner = self.inner.write().await;
            inner.status = RunnerStatus {
                status: "done".to_string(),
                started_at: Some(start_time.to_rfc3339()),
                finished_at: Some(end_time.to_rfc3339()),
                duration: Some(format!("{}ms", duration.num_milliseconds())),
            };
            inner.report.clone().unwrap()
        };

        report
    }

    /// Run benchmark for a single model.
    async fn run_model(&self, model: &str) {
        // Update progress
        {
            let mut inner = self.inner.write().await;
            if let Some(p) = inner.progress.get_mut(model) {
                p.status = "running".to_string();
            }
        }

        let cases = self.inner.read().await.cases.clone();
        let mut model_result = ModelResult {
            model: model.to_string(),
            total_cases: cases.len(),
            passed: 0,
            failed: 0,
            errors: 0,
            pass_rate: 0.0,
            p50_ms: 0,
            p95_ms: 0,
            p99_ms: 0,
            cases: vec![],
        };

        let mut durations = Vec::new();

        for tc in &cases {
            let cr = self.run_single_test(model, tc).await;
            durations.push(cr.duration_ms);

            match cr.status.as_str() {
                "pass" => model_result.passed += 1,
                "fail" => model_result.failed += 1,
                "error" => model_result.errors += 1,
                _ => {}
            }

            model_result.cases.push(cr);

            // Update progress
            {
                let mut inner = self.inner.write().await;
                if let Some(p) = inner.progress.get_mut(model) {
                    p.done += 1;
                    p.passed = model_result.passed;
                    p.failed = model_result.failed;
                    p.errors = model_result.errors;
                    p.percent = p.done as f64 / p.total as f64 * 100.0;
                }
            }
        }

        // Calculate stats
        if model_result.total_cases > 0 {
            model_result.pass_rate =
                model_result.passed as f64 / model_result.total_cases as f64 * 100.0;
            let (p50, p95, p99) = calc_percentiles(&durations);
            model_result.p50_ms = p50;
            model_result.p95_ms = p95;
            model_result.p99_ms = p99;
        }

        // Update report and progress
        {
            let mut inner = self.inner.write().await;
            if let Some(report) = &mut inner.report {
                report.models.push(model_result);
            }
            if let Some(p) = inner.progress.get_mut(model) {
                p.status = "done".to_string();
            }
        }
    }

    /// Run a single test case.
    async fn run_single_test(&self, model: &str, tc: &TestCase) -> CaseResult {
        let start = std::time::Instant::now();

        // Get the generator for this model
        let generator = match config::get_generator(model).await {
            Some(g) => g,
            None => {
                return CaseResult {
                    input: tc.input.clone(),
                    expected: ActualResult {
                        rule: tc.expected.rule.clone(),
                        args: tc.expected.args.clone(),
                    },
                    actual: ActualResult {
                        rule: String::new(),
                        args: HashMap::new(),
                    },
                    duration_ms: start.elapsed().as_millis() as i64,
                    status: "error".to_string(),
                    error: Some(format!("Generator for model '{}' not found", model)),
                };
            }
        };

        // Get matcher from inner
        let matcher = {
            let inner = self.inner.read().await;
            // Clone the system prompt to create a new matcher for this request
            inner.matcher.system_prompt().to_string()
        };

        // Create a matcher-like context that includes system prompt
        let mut full_mcb = ModelContextBuilder::new();
        full_mcb.add_prompt(giztoy_genx::context::Prompt::new("", &matcher));
        full_mcb.user_text("user", &tc.input);
        let full_ctx = full_mcb.build();

        // Run matching with timeout
        let timeout_duration = tokio::time::Duration::from_secs(60);
        let (result, raw_text) = match tokio::time::timeout(
            timeout_duration,
            generator.generate_stream("", &full_ctx),
        )
        .await
        {
            Ok(Ok(mut stream)) => {
                let mut output = String::new();
                let mut iterations = 0;
                const MAX_ITERATIONS: usize = 1000;

                // Collect all text from stream with timeout per chunk
                loop {
                    iterations += 1;
                    if iterations > MAX_ITERATIONS {
                        eprintln!("Warning: Max iterations reached for model {}", model);
                        break;
                    }

                    let chunk_timeout = tokio::time::Duration::from_secs(30);
                    match tokio::time::timeout(chunk_timeout, stream.next()).await {
                        Ok(Ok(Some(chunk))) => {
                            if let Some(Part::Text(text)) = chunk.part {
                                output.push_str(&text);
                            }
                        }
                        Ok(Ok(None)) => break, // Stream finished normally
                        Ok(Err(e)) => {
                            // Check if it's a normal termination
                            let err_str = e.to_string();
                            if err_str.contains("done") || err_str.contains("Done") {
                                break;
                            }
                            return CaseResult {
                                input: tc.input.clone(),
                                expected: ActualResult {
                                    rule: tc.expected.rule.clone(),
                                    args: tc.expected.args.clone(),
                                },
                                actual: ActualResult {
                                    rule: String::new(),
                                    args: HashMap::new(),
                                },
                                duration_ms: start.elapsed().as_millis() as i64,
                                status: "error".to_string(),
                                error: Some(format!("Stream error: {}", e)),
                            };
                        }
                        Err(_) => {
                            // Chunk timeout
                            eprintln!("Warning: Chunk timeout for model {}", model);
                            break;
                        }
                    }
                }

                // Parse the first line of output
                let first_line = output.lines().next().unwrap_or("").trim();
                (self.parse_output_line(first_line), output)
            }
            Ok(Err(e)) => {
                return CaseResult {
                    input: tc.input.clone(),
                    expected: ActualResult {
                        rule: tc.expected.rule.clone(),
                        args: tc.expected.args.clone(),
                    },
                    actual: ActualResult {
                        rule: String::new(),
                        args: HashMap::new(),
                    },
                    duration_ms: start.elapsed().as_millis() as i64,
                    status: "error".to_string(),
                    error: Some(format!("Generation error: {}", e)),
                };
            }
            Err(_) => {
                return CaseResult {
                    input: tc.input.clone(),
                    expected: ActualResult {
                        rule: tc.expected.rule.clone(),
                        args: tc.expected.args.clone(),
                    },
                    actual: ActualResult {
                        rule: String::new(),
                        args: HashMap::new(),
                    },
                    duration_ms: start.elapsed().as_millis() as i64,
                    status: "error".to_string(),
                    error: Some("Request timeout (60s)".to_string()),
                };
            }
        };

        let duration_ms = start.elapsed().as_millis() as i64;

        // Compare results (pass raw_text for "nothing" rule handling)
        let status = compare_results(&tc.expected, &result, &raw_text);

        CaseResult {
            input: tc.input.clone(),
            expected: ActualResult {
                rule: tc.expected.rule.clone(),
                args: tc.expected.args.clone(),
            },
            actual: result,
            duration_ms,
            status,
            error: None,
        }
    }

    /// Parse an output line into actual result.
    fn parse_output_line(&self, line: &str) -> ActualResult {
        let line = line.trim();
        if line.is_empty() {
            return ActualResult {
                rule: String::new(),
                args: HashMap::new(),
            };
        }

        // Parse "rule_name: key1=value1, key2=value2" or just "rule_name"
        let (name, kv) = match line.split_once(':') {
            Some((n, k)) => (n.trim(), Some(k.trim())),
            None => (line, None),
        };

        let mut args = HashMap::new();
        if let Some(kv_str) = kv {
            for part in kv_str.split(',') {
                let part = part.trim();
                if let Some((k, v)) = part.split_once('=') {
                    args.insert(k.trim().to_string(), v.trim().to_string());
                }
            }
        }

        ActualResult {
            rule: name.to_string(),
            args,
        }
    }
}

/// Compare expected result with actual result.
fn compare_results(expected: &ExpectedResult, actual: &ActualResult, raw_text: &str) -> String {
    // Special handling for "nothing" rule (matching Go behavior)
    if expected.rule == "nothing" {
        // expected "nothing" matches actual empty (no rule matched)
        if actual.rule.is_empty() || actual.rule == "nothing" {
            return "pass".to_string();
        }
        // Check raw text for "nothing"
        let raw_lower = raw_text.trim().to_lowercase();
        if raw_lower == "nothing" || raw_lower.contains("nothing") {
            return "pass".to_string();
        }
        return "fail".to_string();
    }

    // Rule must match
    if expected.rule != actual.rule {
        return "fail".to_string();
    }

    // All expected args must be present and match
    for (k, v) in &expected.args {
        match actual.args.get(k) {
            Some(av) if av == v => continue,
            _ => return "fail".to_string(),
        }
    }

    "pass".to_string()
}

/// Calculate percentiles from durations.
fn calc_percentiles(durations: &[i64]) -> (i64, i64, i64) {
    if durations.is_empty() {
        return (0, 0, 0);
    }

    let mut sorted = durations.to_vec();
    sorted.sort();

    // Use (n-1)*P/100 for more accurate percentile calculation
    let n = sorted.len();
    let p50 = sorted[(n - 1) * 50 / 100];
    let p95 = sorted[(n - 1) * 95 / 100];
    let p99 = sorted[(n - 1) * 99 / 100];

    (p50, p95, p99)
}

/// Save report to file.
pub fn save_report(report: &BenchmarkReport, path: &Path) -> Result<()> {
    let data = serde_json::to_string_pretty(report)?;
    std::fs::write(path, data)?;
    Ok(())
}

/// Load report from file.
pub fn load_report(path: &str) -> Result<BenchmarkReport> {
    let data = std::fs::read(path)?;
    let report: BenchmarkReport = serde_json::from_slice(&data)?;
    Ok(report)
}

/// Load and merge multiple reports.
pub fn load_reports(paths: &[&str]) -> Result<BenchmarkReport> {
    if paths.is_empty() {
        anyhow::bail!("no report files specified");
    }

    let mut merged = load_report(paths[0])?;

    for path in &paths[1..] {
        let report = load_report(path)?;
        merged.models.extend(report.models);
        merged.rule_count = merged.rule_count.max(report.rule_count);
        merged.test_count = merged.test_count.max(report.test_count);
    }

    Ok(merged)
}

/// Print benchmark summary.
pub fn print_summary(report: &BenchmarkReport) {
    println!("\n{}", "=".repeat(110));
    println!("BENCHMARK SUMMARY");
    println!("{}", "=".repeat(110));

    println!(
        "\n{:<25} {:>8} {:>8} {:>8} {:>8} {:>8} {:>8} {:>8} {:>10}",
        "Model", "Total", "Passed", "Failed", "Errors", "P50(ms)", "P95(ms)", "P99(ms)", "PassRate"
    );
    println!("{}", "-".repeat(110));

    for mr in &report.models {
        println!(
            "{:<25} {:>8} {:>8} {:>8} {:>8} {:>8} {:>8} {:>8} {:>9.1}%",
            mr.model,
            mr.total_cases,
            mr.passed,
            mr.failed,
            mr.errors,
            mr.p50_ms,
            mr.p95_ms,
            mr.p99_ms,
            mr.pass_rate
        );
    }
    println!("{}", "-".repeat(110));
}
