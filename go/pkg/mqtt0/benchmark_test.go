package mqtt0

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Benchmark port counter to avoid conflicts
var benchPort atomic.Uint32

func init() {
	benchPort.Store(20000)
}

func getBenchAddr() string {
	port := benchPort.Add(1)
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// startBenchBroker starts a broker for benchmarking and returns the address and cleanup function.
func startBenchBroker(b *testing.B) (string, func()) {
	b.Helper()

	addr := getBenchAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		b.Fatalf("listen failed: %v", err)
	}

	broker := &Broker{}
	go broker.Serve(ln)

	// Wait for broker to start
	time.Sleep(50 * time.Millisecond)

	return addr, func() {
		ln.Close()
		broker.Close()
	}
}

// =============================================================================
// Publish Throughput Benchmarks
// =============================================================================

// BenchmarkPublishThroughput measures publish throughput with different payload sizes.
// This matches the Rust benchmark for comparison.
func BenchmarkPublishThroughput(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d_bytes", size), func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()
			client, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-client",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect failed: %v", err)
			}
			defer client.Close()

			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := client.Publish(ctx, "bench/topic", payload); err != nil {
					b.Fatalf("publish failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkPublishThroughputParallel measures parallel publish throughput.
func BenchmarkPublishThroughputParallel(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d_bytes", size), func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()
			client, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-client",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect failed: %v", err)
			}
			defer client.Close()

			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if err := client.Publish(ctx, "bench/topic", payload); err != nil {
						b.Errorf("publish failed: %v", err)
					}
				}
			})
		})
	}
}

// =============================================================================
// End-to-End Latency Benchmarks
// =============================================================================

// BenchmarkE2ELatency measures end-to-end latency (publish -> receive).
// This matches the Rust benchmark for comparison.
func BenchmarkE2ELatency(b *testing.B) {
	sizes := []int{64, 256, 1024}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d_bytes", size), func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()

			// Subscriber
			subscriber, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-sub",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect subscriber failed: %v", err)
			}
			defer subscriber.Close()

			if err := subscriber.Subscribe(ctx, "bench/latency"); err != nil {
				b.Fatalf("subscribe failed: %v", err)
			}

			// Publisher
			publisher, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-pub",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect publisher failed: %v", err)
			}
			defer publisher.Close()

			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := publisher.Publish(ctx, "bench/latency", payload); err != nil {
					b.Fatalf("publish failed: %v", err)
				}
				msg, err := subscriber.RecvTimeout(time.Second)
				if err != nil {
					b.Fatalf("recv failed: %v", err)
				}
				if msg == nil {
					b.Fatal("recv timeout")
				}
			}
		})
	}
}

// =============================================================================
// Message Routing Throughput Benchmarks
// =============================================================================

// BenchmarkMessageRoutingThroughput measures the broker's message routing throughput.
// Multiple subscribers receiving messages from a single publisher.
func BenchmarkMessageRoutingThroughput(b *testing.B) {
	subscriberCounts := []int{1, 5, 10}

	for _, subCount := range subscriberCounts {
		b.Run(fmt.Sprintf("%d_subscribers", subCount), func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()

			// Create subscribers
			subscribers := make([]*Client, subCount)
			for i := 0; i < subCount; i++ {
				sub, err := Connect(ctx, ClientConfig{
					Addr:          "tcp://" + addr,
					ClientID:      fmt.Sprintf("bench-sub-%d", i),
					AutoKeepalive: false,
				})
				if err != nil {
					b.Fatalf("connect subscriber %d failed: %v", i, err)
				}
				defer sub.Close()

				if err := sub.Subscribe(ctx, "bench/routing"); err != nil {
					b.Fatalf("subscribe failed: %v", err)
				}
				subscribers[i] = sub
			}

			// Create publisher
			publisher, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-pub",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect publisher failed: %v", err)
			}
			defer publisher.Close()

			// Start receiver goroutines
			var wg sync.WaitGroup
			stopCh := make(chan struct{})
			var received atomic.Int64

			for _, sub := range subscribers {
				wg.Add(1)
				go func(s *Client) {
					defer wg.Done()
					for {
						select {
						case <-stopCh:
							return
						default:
							msg, _ := s.RecvTimeout(100 * time.Millisecond)
							if msg != nil {
								received.Add(1)
							}
						}
					}
				}(sub)
			}

			payload := make([]byte, 64)
			b.SetBytes(64)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := publisher.Publish(ctx, "bench/routing", payload); err != nil {
					b.Fatalf("publish failed: %v", err)
				}
			}

			// Wait a bit for messages to be delivered
			time.Sleep(100 * time.Millisecond)
			close(stopCh)
			wg.Wait()

			b.ReportMetric(float64(received.Load())/float64(b.N), "msgs_delivered/op")
		})
	}
}

// =============================================================================
// Wildcard Subscription Throughput
// =============================================================================

// BenchmarkWildcardRoutingThroughput measures routing with wildcard subscriptions.
func BenchmarkWildcardRoutingThroughput(b *testing.B) {
	patterns := []struct {
		name    string
		pattern string
		topic   string
	}{
		{"single_level", "sensor/+/temp", "sensor/room1/temp"},
		{"multi_level", "sensor/#", "sensor/room1/temp/value"},
		{"complex", "sensor/+/+/value", "sensor/room1/temp/value"},
	}

	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()

			// Subscriber with wildcard
			subscriber, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-sub",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect failed: %v", err)
			}
			defer subscriber.Close()

			if err := subscriber.Subscribe(ctx, p.pattern); err != nil {
				b.Fatalf("subscribe failed: %v", err)
			}

			// Publisher
			publisher, err := Connect(ctx, ClientConfig{
				Addr:          "tcp://" + addr,
				ClientID:      "bench-pub",
				AutoKeepalive: false,
			})
			if err != nil {
				b.Fatalf("connect failed: %v", err)
			}
			defer publisher.Close()

			payload := make([]byte, 64)
			b.SetBytes(64)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := publisher.Publish(ctx, p.topic, payload); err != nil {
					b.Fatalf("publish failed: %v", err)
				}
				msg, err := subscriber.RecvTimeout(time.Second)
				if err != nil {
					b.Fatalf("recv failed: %v", err)
				}
				if msg == nil {
					b.Fatal("recv timeout")
				}
			}
		})
	}
}

// =============================================================================
// Connection Throughput
// =============================================================================

// BenchmarkConnectionThroughput measures connection establishment throughput.
func BenchmarkConnectionThroughput(b *testing.B) {
	protocols := []struct {
		name    string
		version ProtocolVersion
	}{
		{"v4", ProtocolV4},
		{"v5", ProtocolV5},
	}

	for _, p := range protocols {
		b.Run(p.name, func(b *testing.B) {
			addr, cleanup := startBenchBroker(b)
			defer cleanup()

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				client, err := Connect(ctx, ClientConfig{
					Addr:            "tcp://" + addr,
					ClientID:        fmt.Sprintf("bench-client-%d", i),
					ProtocolVersion: p.version,
					AutoKeepalive:   false,
				})
				if err != nil {
					b.Fatalf("connect failed: %v", err)
				}
				client.Close()
			}
		})
	}
}

// =============================================================================
// Trie Benchmarks (matches Rust)
// =============================================================================

// BenchmarkTrieMatching measures trie matching performance.
// This matches the Rust benchmark for comparison.
func BenchmarkTrieMatching(b *testing.B) {
	trie := NewTrie[string]()

	// Add patterns (same as Rust benchmark)
	patterns := []string{
		"device/+/state",
		"device/+/stats",
		"device/+/events/#",
		"server/push/#",
		"system/+/+/metrics",
	}

	for _, p := range patterns {
		trie.Insert(p, p)
	}

	b.Run("exact_match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			trie.Get("device/gear-001/state")
		}
	})

	b.Run("wildcard_match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			trie.Get("device/gear-001/events/click/button")
		}
	})

	b.Run("no_match", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			trie.Get("unknown/topic/path")
		}
	})
}

// =============================================================================
// High Throughput Stress Test
// =============================================================================

// BenchmarkHighThroughputStress measures sustained message throughput under load.
// This is similar to the Rust benchmark for direct comparison.
func BenchmarkHighThroughputStress(b *testing.B) {
	addr, cleanup := startBenchBroker(b)
	defer cleanup()

	ctx := context.Background()

	// Create multiple publisher-subscriber pairs
	const numPairs = 4
	type pair struct {
		pub *Client
		sub *Client
	}
	pairs := make([]pair, numPairs)

	for i := 0; i < numPairs; i++ {
		topic := fmt.Sprintf("stress/%d", i)

		sub, err := Connect(ctx, ClientConfig{
			Addr:          "tcp://" + addr,
			ClientID:      fmt.Sprintf("stress-sub-%d", i),
			AutoKeepalive: false,
		})
		if err != nil {
			b.Fatalf("connect subscriber failed: %v", err)
		}
		defer sub.Close()

		if err := sub.Subscribe(ctx, topic); err != nil {
			b.Fatalf("subscribe failed: %v", err)
		}

		pub, err := Connect(ctx, ClientConfig{
			Addr:          "tcp://" + addr,
			ClientID:      fmt.Sprintf("stress-pub-%d", i),
			AutoKeepalive: false,
		})
		if err != nil {
			b.Fatalf("connect publisher failed: %v", err)
		}
		defer pub.Close()

		pairs[i] = pair{pub: pub, sub: sub}
	}

	payload := make([]byte, 256)
	b.SetBytes(256 * numPairs)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Publish to all topics in parallel
		var wg sync.WaitGroup
		for j, p := range pairs {
			wg.Add(1)
			go func(idx int, pr pair) {
				defer wg.Done()
				topic := fmt.Sprintf("stress/%d", idx)
				_ = pr.pub.Publish(ctx, topic, payload)
			}(j, p)
		}
		wg.Wait()

		// Receive from all subscribers
		for _, p := range pairs {
			_, _ = p.sub.RecvTimeout(time.Second)
		}
	}
}

// BenchmarkMessageRate measures pure message rate (messages per second).
func BenchmarkMessageRate(b *testing.B) {
	addr, cleanup := startBenchBroker(b)
	defer cleanup()

	ctx := context.Background()

	// Single fast path
	client, err := Connect(ctx, ClientConfig{
		Addr:          "tcp://" + addr,
		ClientID:      "rate-client",
		AutoKeepalive: false,
	})
	if err != nil {
		b.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	payload := []byte("x") // minimal payload
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := client.Publish(ctx, "rate/test", payload); err != nil {
			b.Fatalf("publish failed: %v", err)
		}
	}

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msg/s")
}

// =============================================================================
// Protocol Encoding/Decoding Benchmarks
// =============================================================================

// BenchmarkPacketEncode measures packet encoding performance.
func BenchmarkPacketEncode(b *testing.B) {
	b.Run("v4_publish_64b", func(b *testing.B) {
		payload := make([]byte, 64)
		b.SetBytes(64)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := &V4Publish{
				Topic:   "bench/topic",
				Payload: payload,
			}
			_, _ = p.encode()
		}
	})

	b.Run("v4_publish_1kb", func(b *testing.B) {
		payload := make([]byte, 1024)
		b.SetBytes(1024)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := &V4Publish{
				Topic:   "bench/topic",
				Payload: payload,
			}
			_, _ = p.encode()
		}
	})

	b.Run("v5_publish_64b", func(b *testing.B) {
		payload := make([]byte, 64)
		b.SetBytes(64)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := &V5Publish{
				Topic:   "bench/topic",
				Payload: payload,
			}
			_, _ = p.encodeV5()
		}
	})

	b.Run("v5_publish_1kb", func(b *testing.B) {
		payload := make([]byte, 1024)
		b.SetBytes(1024)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			p := &V5Publish{
				Topic:   "bench/topic",
				Payload: payload,
			}
			_, _ = p.encodeV5()
		}
	})
}
