//! Benchmark tests for mqtt0.
//!
//! Run with: cargo bench -p giztoy-mqtt0
//! Or with bazel: bazel run //rust/mqtt0:mqtt0_bench

use criterion::{criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::Duration;

use giztoy_mqtt0::{Broker, BrokerConfig, Client, ClientConfig};

/// Find an available port.
fn find_port() -> u16 {
    static PORT: AtomicUsize = AtomicUsize::new(19000);
    PORT.fetch_add(1, Ordering::SeqCst) as u16
}

/// Benchmark: Client publish throughput (QoS 0, fire and forget).
fn bench_publish_throughput(c: &mut Criterion) {
    let rt = tokio::runtime::Runtime::new().unwrap();

    let port = find_port();
    let addr = format!("127.0.0.1:{}", port);

    // Start broker
    let broker = Broker::new(BrokerConfig::new(&addr));
    rt.spawn(async move {
        let _ = broker.serve().await;
    });

    // Wait for broker to start
    std::thread::sleep(Duration::from_millis(100));

    // Connect client
    let client = rt.block_on(async {
        Client::connect(ClientConfig::new(&addr, "bench-client"))
            .await
            .unwrap()
    });

    let mut group = c.benchmark_group("publish_throughput");

    for size in [64, 256, 1024, 4096].iter() {
        let payload = vec![0u8; *size];

        group.throughput(Throughput::Bytes(*size as u64));
        group.bench_with_input(BenchmarkId::from_parameter(size), size, |b, _| {
            b.to_async(&rt).iter(|| async {
                client.publish("bench/topic", &payload).await.unwrap();
            });
        });
    }

    group.finish();

    rt.block_on(async {
        client.disconnect().await.unwrap();
    });
}

/// Benchmark: End-to-end latency (publish -> receive).
fn bench_e2e_latency(c: &mut Criterion) {
    let rt = tokio::runtime::Runtime::new().unwrap();

    let port = find_port();
    let addr = format!("127.0.0.1:{}", port);

    // Start broker
    let broker = Broker::new(BrokerConfig::new(&addr));
    rt.spawn(async move {
        let _ = broker.serve().await;
    });

    std::thread::sleep(Duration::from_millis(100));

    // Connect publisher and subscriber
    let (publisher, subscriber) = rt.block_on(async {
        let pub_client = Client::connect(ClientConfig::new(&addr, "bench-pub"))
            .await
            .unwrap();
        let sub_client = Client::connect(ClientConfig::new(&addr, "bench-sub"))
            .await
            .unwrap();

        sub_client.subscribe(&["bench/latency"]).await.unwrap();

        (pub_client, sub_client)
    });

    let publisher = Arc::new(publisher);
    let subscriber = Arc::new(subscriber);

    c.bench_function("e2e_latency_64b", |b| {
        let payload = vec![0u8; 64];
        let pub_clone = Arc::clone(&publisher);
        let sub = Arc::clone(&subscriber);

        b.to_async(&rt).iter(|| {
            let pub_clone = Arc::clone(&pub_clone);
            let sub = Arc::clone(&sub);
            let payload = payload.clone();
            async move {
                pub_clone.publish("bench/latency", &payload).await.unwrap();
                sub.recv_timeout(Duration::from_secs(1)).await.unwrap();
            }
        });
    });

    rt.block_on(async {
        publisher.disconnect().await.unwrap();
        subscriber.disconnect().await.unwrap();
    });
}

/// Benchmark: Topic matching with trie.
fn bench_trie_matching(c: &mut Criterion) {
    use giztoy_mqtt0::trie::Trie;

    let trie: Trie<String> = Trie::new();

    // Add patterns
    let patterns = [
        "device/+/state",
        "device/+/stats",
        "device/+/events/#",
        "server/push/#",
        "system/+/+/metrics",
    ];

    for pattern in patterns {
        trie.insert(pattern, pattern.to_string()).unwrap();
    }

    let mut group = c.benchmark_group("trie_matching");

    // Benchmark exact match (with clone - old API)
    group.bench_function("exact_match", |b| {
        b.iter(|| {
            trie.get("device/gear-001/state");
        });
    });

    // Benchmark exact match (zero-copy - new API)
    group.bench_function("exact_match_zerocopy", |b| {
        b.iter(|| {
            trie.with_values("device/gear-001/state", |values| values.len());
        });
    });

    // Benchmark wildcard match
    group.bench_function("wildcard_match", |b| {
        b.iter(|| {
            trie.get("device/gear-001/events/click/button");
        });
    });

    // Benchmark wildcard match (zero-copy)
    group.bench_function("wildcard_match_zerocopy", |b| {
        b.iter(|| {
            trie.with_values("device/gear-001/events/click/button", |values| values.len());
        });
    });

    // Benchmark no match
    group.bench_function("no_match", |b| {
        b.iter(|| {
            trie.get("unknown/topic/path");
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_publish_throughput,
    bench_e2e_latency,
    bench_trie_matching,
);

criterion_main!(benches);
