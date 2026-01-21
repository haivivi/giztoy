# mqtt0 Benchmark Results

Performance comparison between Go and Rust mqtt0 implementations.

## How to Run

### Go Benchmarks

```bash
# All benchmarks
cd go && go test ./pkg/mqtt0/... -bench=. -benchmem -run='^$'

# Specific benchmarks
go test ./pkg/mqtt0/... -bench=BenchmarkPublishThroughput -benchmem
go test ./pkg/mqtt0/... -bench=BenchmarkE2ELatency -benchmem
go test ./pkg/mqtt0/... -bench=BenchmarkTrieMatching -benchmem
go test ./pkg/mqtt0/... -bench=BenchmarkMessageRate -benchmem
```

### Rust Benchmarks

```bash
# With cargo
cd rust && cargo bench -p giztoy-mqtt0
```

## Test Environment

- **CPU**: Apple M4 Max
- **OS**: macOS
- **Go**: 1.24+ (GOMAXPROCS=16)
- **Rust**: stable-aarch64-apple-darwin

## Go vs Rust Performance Comparison

### Publish Throughput (QoS 0)

| Payload | Go | Rust | Rust/Go |
|---------|-----|------|---------|
| 64 bytes | 3.1μs (20.6 MB/s) | **0.85μs (70.4 MB/s)** | 3.6x faster |
| 256 bytes | 3.4μs (74.5 MB/s) | **0.73μs (333 MB/s)** | 4.5x faster |
| 1024 bytes | 3.6μs (280 MB/s) | **1.05μs (927 MB/s)** | 3.3x faster |
| 4096 bytes | 4.3μs (950 MB/s) | **1.82μs (2.1 GB/s)** | 2.2x faster |

### End-to-End Latency (pub → recv)

| Payload | Go | Rust | Notes |
|---------|-----|------|-------|
| 64 bytes | 21-22μs | 24-25μs | Network I/O dominates |

### Trie Pattern Matching

| Operation | Go | Rust | Winner |
|-----------|-----|------|--------|
| exact_match | **60ns** | 196ns | Go 3.3x faster |
| wildcard_match | **85ns** | 270ns | Go 3.2x faster |
| no_match | **12ns** | 14ns | Similar |

### Message Rate

| Metric | Go | Rust |
|--------|-----|------|
| Messages/sec | **324,000** | ~1,200,000 (estimated) |
| Allocs/op | 17-18 | 0 |

## Key Findings

### Rust Advantages
- **3-4x faster** in pure CPU-bound publish operations
- Zero memory allocations per message
- Better suited for extremely high throughput scenarios

### Go Advantages  
- **3x faster** trie-based topic matching
- Similar E2E latency (network-bound)
- Lower development complexity
- Sufficient performance for most real-world MQTT use cases

### Shared Characteristics
- Both achieve sub-25μs E2E latency
- Both handle 300K+ msg/s on modern hardware
- Network I/O is the bottleneck in real deployments

## Optimization Opportunities (Go)

1. **Buffer pooling**: Reduce allocations from 17-18/op to ~5/op
2. **Batch publishing**: Amortize syscall overhead
3. **Zero-copy receive**: Avoid payload copying in broker routing
4. **Pre-allocated packet buffers**: Reuse encode/decode buffers

## Detailed Go Benchmarks

### Connection Establishment

| Protocol | ops/sec | latency |
|----------|---------|---------|
| MQTT v4 | 2,900 | 340μs |
| MQTT v5 | 2,600 | 380μs |

### Packet Encoding

| Packet | ops/sec | throughput |
|--------|---------|------------|
| v4 PUBLISH 64b | 11.3M | 720 MB/s |
| v4 PUBLISH 1kb | 3.7M | 3.7 GB/s |
| v5 PUBLISH 64b | 10.2M | 650 MB/s |
| v5 PUBLISH 1kb | 3.5M | 3.5 GB/s |
