# mqtt0 Benchmark Results

Benchmarks comparing Go and Rust mqtt0 implementations for performance parity verification.

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

## Benchmark Results

### Test Environment
- **CPU**: Apple M4 Max
- **OS**: macOS
- **Go**: 1.24+ (GOMAXPROCS=16)

### Message Throughput (QoS 0)

| Benchmark | ops/sec | latency | throughput | allocs/op |
|-----------|---------|---------|------------|-----------|
| **Publish (single client)** |
| 64 bytes | 323,000 | 3.1μs | 20.6 MB/s | 17 |
| 256 bytes | 290,000 | 3.4μs | 74.5 MB/s | 18 |
| 1024 bytes | 280,000 | 3.6μs | 280 MB/s | 18 |
| 4096 bytes | 230,000 | 4.3μs | 950 MB/s | 18 |

### Message Rate

| Test | messages/sec |
|------|--------------|
| Minimal payload (1 byte) | **324,000 msg/s** |
| High throughput stress (4 pairs) | ~72,000 ops/s |

### End-to-End Latency (pub → recv)

| Payload | latency | throughput |
|---------|---------|------------|
| 64 bytes | 21μs | 3 MB/s |
| 256 bytes | 22μs | 11 MB/s |
| 1024 bytes | 25μs | 40 MB/s |

### Trie Pattern Matching

| Operation | ops/sec | latency | allocs |
|-----------|---------|---------|--------|
| exact_match | 16.6M | 60ns | 2 |
| wildcard_match (`+`, `#`) | 11.7M | 85ns | 3 |
| no_match | 86M | 12ns | 0 |

### Packet Encoding

| Packet | ops/sec | throughput |
|--------|---------|------------|
| v4 PUBLISH 64b | 11.3M | 720 MB/s |
| v4 PUBLISH 1kb | 3.7M | 3.7 GB/s |
| v5 PUBLISH 64b | 10.2M | 650 MB/s |
| v5 PUBLISH 1kb | 3.5M | 3.5 GB/s |

### Connection Establishment

| Protocol | ops/sec | latency |
|----------|---------|---------|
| MQTT v4 | 2,900 | 340μs |
| MQTT v5 | 2,600 | 380μs |

## Comparison with Rust

Both implementations measure:
1. **Publish Throughput** - Single client QoS 0 publishing
2. **E2E Latency** - Round-trip time publish → receive
3. **Trie Matching** - Topic pattern matching performance

### Expected Differences

- **Go**: Garbage collection overhead (~17-18 allocs/op for publish)
- **Rust**: Zero-copy optimizations possible
- **Both**: Network I/O is typically the bottleneck

## Optimization Opportunities

1. **Buffer pooling**: Reduce allocations per message
2. **Batch publishing**: Amortize syscall overhead
3. **Zero-copy receive**: Avoid payload copying in broker routing
