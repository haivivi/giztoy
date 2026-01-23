//! Benchmarks for Trie implementation.

use criterion::{black_box, criterion_group, criterion_main, Criterion, BenchmarkId};
use giztoy_trie::Trie;

/// Generate test paths for benchmarking
fn generate_paths(count: usize) -> Vec<String> {
    let mut paths = Vec::with_capacity(count);
    for i in 0..count {
        let a = i % 10;
        let b = (i / 10) % 10;
        let c = (i / 100) % 10;
        paths.push(format!("device/gear-{:03}/sensor/{}/data/{}", i, a, b * 10 + c));
    }
    paths
}

/// Generate wildcard patterns
fn generate_wildcard_patterns() -> Vec<&'static str> {
    vec![
        "device/+/sensor/+/data/+",
        "device/gear-001/+/+/data/+",
        "device/#",
        "device/+/#",
        "logs/#",
    ]
}

fn bench_set(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_set");
    
    for size in [100, 1000, 10000].iter() {
        let paths = generate_paths(*size);
        
        group.bench_with_input(BenchmarkId::new("exact_paths", size), size, |b, _| {
            b.iter(|| {
                let mut trie = Trie::<i32>::new();
                for (i, path) in paths.iter().enumerate() {
                    trie.set_value(path, i as i32).unwrap();
                }
                black_box(trie)
            });
        });
    }
    
    group.finish();
}

fn bench_get_exact(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_get_exact");
    
    for size in [100, 1000, 10000].iter() {
        let paths = generate_paths(*size);
        let mut trie = Trie::<i32>::new();
        for (i, path) in paths.iter().enumerate() {
            trie.set_value(path, i as i32).unwrap();
        }
        
        group.bench_with_input(BenchmarkId::new("lookup", size), size, |b, _| {
            b.iter(|| {
                for path in &paths {
                    black_box(trie.get(path));
                }
            });
        });
    }
    
    group.finish();
}

fn bench_get_wildcard(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_get_wildcard");
    
    // Setup trie with wildcard patterns
    let mut trie = Trie::<&str>::new();
    for pattern in generate_wildcard_patterns() {
        trie.set_value(pattern, pattern).unwrap();
    }
    
    // Test paths that match wildcards
    let test_paths = vec![
        "device/gear-001/sensor/0/data/1",
        "device/gear-999/sensor/5/data/99",
        "device/gear-001/state/online",
        "logs/app/debug/line1",
        "logs/system/error",
    ];
    
    group.bench_function("wildcard_match", |b| {
        b.iter(|| {
            for path in &test_paths {
                black_box(trie.get(path));
            }
        });
    });
    
    group.finish();
}

fn bench_match_path(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_match_path");
    
    // Setup mixed trie with exact and wildcard patterns
    let mut trie = Trie::<i32>::new();
    
    // Add exact paths
    let exact_paths = generate_paths(1000);
    for (i, path) in exact_paths.iter().enumerate() {
        trie.set_value(path, i as i32).unwrap();
    }
    
    // Add wildcard patterns
    trie.set_value("device/+/sensor/+/data/+", -1).unwrap();
    trie.set_value("device/#", -2).unwrap();
    trie.set_value("logs/#", -3).unwrap();
    
    let test_paths = vec![
        "device/gear-500/sensor/5/data/50",  // exact match exists
        "device/gear-9999/sensor/0/data/0",  // wildcard match only
        "device/unknown/state",               // # wildcard
        "logs/anything/here",                 // # wildcard
    ];
    
    group.bench_function("mixed_match", |b| {
        b.iter(|| {
            for path in &test_paths {
                black_box(trie.match_path(path));
            }
        });
    });
    
    group.finish();
}

fn bench_walk(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_walk");
    
    for size in [100, 1000].iter() {
        let paths = generate_paths(*size);
        let mut trie = Trie::<i32>::new();
        for (i, path) in paths.iter().enumerate() {
            trie.set_value(path, i as i32).unwrap();
        }
        
        group.bench_with_input(BenchmarkId::new("walk_all", size), size, |b, _| {
            b.iter(|| {
                let mut count = 0;
                trie.walk(|_, _| count += 1);
                black_box(count)
            });
        });
    }
    
    group.finish();
}

fn bench_deep_paths(c: &mut Criterion) {
    let mut group = c.benchmark_group("trie_deep_paths");
    
    // Create very deep paths
    let deep_paths: Vec<String> = (0..100)
        .map(|i| format!("a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/{}", i))
        .collect();
    
    group.bench_function("deep_set", |b| {
        b.iter(|| {
            let mut trie = Trie::<i32>::new();
            for (i, path) in deep_paths.iter().enumerate() {
                trie.set_value(path, i as i32).unwrap();
            }
            black_box(trie)
        });
    });
    
    let mut trie = Trie::<i32>::new();
    for (i, path) in deep_paths.iter().enumerate() {
        trie.set_value(path, i as i32).unwrap();
    }
    
    group.bench_function("deep_get", |b| {
        b.iter(|| {
            for path in &deep_paths {
                black_box(trie.get(path));
            }
        });
    });
    
    group.finish();
}

criterion_group!(
    benches,
    bench_set,
    bench_get_exact,
    bench_get_wildcard,
    bench_match_path,
    bench_walk,
    bench_deep_paths,
);

criterion_main!(benches);
