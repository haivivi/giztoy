//! mqtt0 Test Application
//!
//! Comprehensive test suite for MQTT 5.0 client with Topic Alias support.
//!
//! Test Suites:
//! - basic: Connect, subscribe, publish, disconnect
//! - multi_topic: Multiple topic concurrent publish with topic alias
//! - throughput_up: Uplink throughput measurement
//! - throughput_down: Downlink throughput measurement
//! - long_connection: Long-running connection stability
//! - reconnect: Disconnect and reconnect handling
//! - all: Run all tests

const platform = @import("platform.zig");
const Board = platform.Board;
const hw = platform.hw;
const log = Board.log;

const mqtt0 = @import("mqtt0");
const tls = @import("tls");
const dns = @import("dns");


// ============================================================================
// Configuration (from environment)
// ============================================================================

fn parsePort(port_str: []const u8) u16 {
    var port: u16 = 0;
    for (port_str) |c| {
        if (c >= '0' and c <= '9') {
            port = port * 10 + (c - '0');
        }
    }
    return if (port == 0) 8883 else port;
}

fn parseBool(s: []const u8) bool {
    if (s.len == 0) return false;
    if (s[0] == 't' or s[0] == 'T' or s[0] == '1') return true;
    return false;
}

// Config struct populated at runtime from env parameter
var config: struct {
    wifi_ssid: []const u8,
    wifi_password: []const u8,
    mqtt_host: []const u8,
    mqtt_port: u16,
    mqtt_username: []const u8,
    mqtt_password: []const u8,
    client_id: []const u8,
    test_suite: []const u8,
    skip_verify: bool,
} = undefined;

// ============================================================================
// Test Suite Selection
// ============================================================================

const TestSuite = enum {
    basic,
    multi_topic,
    throughput_up,
    throughput_down,
    long_connection,
    reconnect,
    concurrent,
    bandwidth,      // 单topic大包带宽测试
    realtime,       // 真实场景：1上1下 50msg/s 100B
    latency,        // RTT延迟测试(echo模式)
    remote_latency, // 远程服务器延迟测试(双向topic)
    remote_bandwidth, // 远程服务器带宽测试
    tcp_duplex,     // 纯TCP双向通信测试
    tls_duplex,     // TLS双向通信测试
    all,

    fn fromString(s: []const u8) TestSuite {
        if (eql(s, "basic")) return .basic;
        if (eql(s, "multi_topic")) return .multi_topic;
        if (eql(s, "throughput_up")) return .throughput_up;
        if (eql(s, "throughput_down")) return .throughput_down;
        if (eql(s, "long_connection")) return .long_connection;
        if (eql(s, "reconnect")) return .reconnect;
        if (eql(s, "concurrent")) return .concurrent;
        if (eql(s, "bandwidth")) return .bandwidth;
        if (eql(s, "realtime")) return .realtime;
        if (eql(s, "latency")) return .latency;
        if (eql(s, "remote_latency")) return .remote_latency;
        if (eql(s, "remote_bandwidth")) return .remote_bandwidth;
        if (eql(s, "tcp_duplex")) return .tcp_duplex;
        if (eql(s, "tls_duplex")) return .tls_duplex;
        return .all;
    }

    fn eql(a: []const u8, b: []const u8) bool {
        if (a.len != b.len) return false;
        for (a, b) |ca, cb| {
            if (ca != cb) return false;
        }
        return true;
    }
};

// ============================================================================
// Test Results
// ============================================================================

const TestResult = struct {
    name: []const u8,
    passed: bool,
    message: []const u8,
    duration_ms: u64,
    extra_info: []const u8,
};

var test_results: [8]TestResult = undefined;
var test_count: usize = 0;

fn recordResult(name: []const u8, passed: bool, message: []const u8, duration_ms: u64, extra: []const u8) void {
    if (test_count < test_results.len) {
        test_results[test_count] = .{
            .name = name,
            .passed = passed,
            .message = message,
            .duration_ms = duration_ms,
            .extra_info = extra,
        };
        test_count += 1;
    }
}

fn printReport() void {
    log.info("", .{});
    log.info("=== mqtt0 Test Report ===", .{});
    var passed: usize = 0;
    var failed: usize = 0;

    for (test_results[0..test_count]) |result| {
        const status = if (result.passed) "PASS" else "FAIL";
        if (result.passed) passed += 1 else failed += 1;

        if (result.extra_info.len > 0) {
            log.info("{s}: {s} ({s}) - {s}", .{ result.name, status, result.message, result.extra_info });
        } else {
            log.info("{s}: {s} ({s})", .{ result.name, status, result.message });
        }
    }

    log.info("=========================", .{});
    log.info("Total: {d} passed, {d} failed", .{ passed, failed });
}

// ============================================================================
// Application State Machine
// ============================================================================

const AppState = enum {
    init,
    connecting_wifi,
    waiting_for_ip,
    running_tests,
    done,
};

// ============================================================================
// Platform Helpers
// ============================================================================

// PSRAM allocator for TLS buffers
const allocator = hw.allocator;

// ============================================================================
// Type Definitions
// ============================================================================

const TlsClient = tls.Client(Board.socket, Board.crypto);
const DnsResolver = dns.Resolver(Board.socket);
// Note: log is already a type (std.log.scoped), not a value
const MqttClient = mqtt0.MqttClient(TlsClient, log, Board.time);

// ============================================================================
// Main Entry Point
// ============================================================================

pub fn run(env: anytype) void {
    // Initialize config from env (field names are lowercase)
    config = .{
        .wifi_ssid = env.wifi_ssid,
        .wifi_password = env.wifi_password,
        .mqtt_host = env.mqtt_host,
        .mqtt_port = parsePort(env.mqtt_port),
        .mqtt_username = env.mqtt_username,
        .mqtt_password = env.mqtt_password,
        .client_id = env.client_id,
        .test_suite = env.test_suite,
        .skip_verify = parseBool(env.mqtt_skip_verify),
    };

    log.info("mqtt0 Test Application Starting", .{});
    log.info("Board: {s}", .{hw.Hardware.name});
    log.info("MQTT Host: {s}:{d}", .{ config.mqtt_host, config.mqtt_port });
    log.info("Client ID: {s}", .{config.client_id});
    log.info("Test Suite: {s}", .{config.test_suite});
    log.info("Skip TLS Verify: {}", .{config.skip_verify});

    var board: Board = undefined;
    board.init() catch |e| {
        log.err("Board init failed: {}", .{e});
        return;
    };
    defer board.deinit();

    hw.debugMemoryUsage("init");

    var state: AppState = .init;

    while (Board.isRunning()) {
        board.poll();

        // Process events
        while (board.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| {
                    switch (wifi_event) {
                        .connected => {
                            log.info("WiFi connected, waiting for IP...", .{});
                            state = .waiting_for_ip;
                        },
                        .disconnected => {
                            log.warn("WiFi disconnected", .{});
                            state = .connecting_wifi;
                        },
                        else => {}, // Ignore other events (rssi, ap_sta, etc.)
                    }
                },
                .net => |net_event| {
                    switch (net_event) {
                        .dhcp_bound => |info| {
                            log.info("Got IP: {d}.{d}.{d}.{d}", .{
                                info.ip[0], info.ip[1], info.ip[2], info.ip[3],
                            });
                            state = .running_tests;
                        },
                        else => {},
                    }
                },
                else => {},
            }
        }

        // State machine
        switch (state) {
            .init => {
                log.info("Connecting to WiFi: {s}", .{config.wifi_ssid});
                board.wifi.connect(config.wifi_ssid, config.wifi_password);
                state = .connecting_wifi;
            },
            .connecting_wifi, .waiting_for_ip => {
                // Wait for events
                Board.time.sleepMs(100);
            },
            .running_tests => {
                runTests();
                state = .done;
            },
            .done => {
                printReport();
                hw.debugMemoryUsage("done");
                hw.debugStackUsage("done", 131072);
                log.info("Tests complete. Halting.", .{});
                while (true) {
                    Board.time.sleepMs(10000);
                }
            },
        }
    }
}

// ============================================================================
// Test Runner
// ============================================================================

fn runTests() void {
    const suite = TestSuite.fromString(config.test_suite);

    log.info("Running test suite: {s}", .{config.test_suite});
    hw.debugMemoryUsage("before_tests");

    switch (suite) {
        .basic => runBasicTest(),
        .multi_topic => runMultiTopicTest(),
        .throughput_up => runThroughputUpTest(),
        .throughput_down => runThroughputDownTest(),
        .long_connection => runLongConnectionTest(),
        .reconnect => runReconnectTest(),
        .concurrent => runConcurrentThroughputTest(),
        .bandwidth => runBandwidthTest(),
        .realtime => runRealtimeTest(),
        .latency => runLatencyTest(),
        .remote_latency => runRemoteLatencyTest(),
        .remote_bandwidth => runRemoteBandwidthTest(),
        .tcp_duplex => runTcpDuplexTest(),
        .tls_duplex => runTlsDuplexTest(),
        .all => {
            runBasicTest();
            runMultiTopicTest();
            runBandwidthTest();
            runRealtimeTest();
        },
    }
}

// ============================================================================
// Test: Basic Connectivity
// ============================================================================

fn runBasicTest() void {
    log.info("--- Basic Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    // Resolve DNS
    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Basic", false, "DNS failed", 0, "");
        return;
    };

    // Create TLS connection
    var socket = Board.socket.tcp() catch {
        recordResult("Basic", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Basic", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Basic", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Basic", false, "TLS connect failed", 0, "");
        return;
    };

    // Create MQTT client
    var client = MqttClient.init(&tls_client);

    // Connect
    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Basic", false, "MQTT connect failed", 0, "");
        return;
    };

    // Subscribe
    const topics = [_][]const u8{"test/mqtt0/basic"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Basic", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Publish
    client.publish("test/mqtt0/basic", "hello from zig mqtt0", &buf) catch {
        recordResult("Basic", false, "Publish failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Disconnect
    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    recordResult("Basic", true, "connect/subscribe/publish/disconnect", elapsed, "");
}

// ============================================================================
// Test: Multi-Topic with Topic Alias
// ============================================================================

fn runMultiTopicTest() void {
    log.info("--- Multi-Topic Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Multi-Topic", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Multi-Topic", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Multi-Topic", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Multi-Topic", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Multi-Topic", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Multi-Topic", false, "MQTT connect failed", 0, "");
        return;
    };

    // Multiple topics
    const topics = [_][]const u8{
        "test/mqtt0/topic1",
        "test/mqtt0/topic2",
        "test/mqtt0/topic3",
        "test/mqtt0/topic4",
        "test/mqtt0/topic5",
    };

    // Subscribe to all
    client.subscribe(&topics, &buf) catch {
        recordResult("Multi-Topic", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Publish to each topic twice (second time should use alias only)
    var publish_count: u32 = 0;
    for (topics) |topic| {
        // First publish: establishes alias
        client.publish(topic, "first message", &buf) catch continue;
        publish_count += 1;

        // Second publish: uses alias (empty topic)
        client.publish(topic, "second message", &buf) catch continue;
        publish_count += 1;
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    var extra_buf: [64]u8 = undefined;
    const extra = formatExtra(&extra_buf, "{d} topics, {d} publishes", .{ topics.len, publish_count });
    recordResult("Multi-Topic", publish_count == 10, "topic alias test", elapsed, extra);
}

// ============================================================================
// Test: Uplink Throughput
// ============================================================================

fn runThroughputUpTest() void {
    log.info("--- Throughput Up Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;
    var payload: [256]u8 = undefined;
    for (&payload, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Throughput-Up", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Throughput-Up", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Throughput-Up", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Throughput-Up", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Throughput-Up", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Throughput-Up", false, "MQTT connect failed", 0, "");
        return;
    };

    // Send 500 messages
    const msg_count: u32 = 500;
    const send_start = Board.time.getTimeMs();
    var sent: u32 = 0;

    while (sent < msg_count) : (sent += 1) {
        client.publish("test/mqtt0/throughput", &payload, &buf) catch break;
    }

    const send_elapsed = Board.time.getTimeMs() - send_start;
    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;

    if (sent > 0 and send_elapsed > 0) {
        const msg_per_sec = (sent * 1000) / @as(u32, @truncate(send_elapsed));
        const kb_per_sec = (sent * 256) / @as(u32, @truncate(send_elapsed));

        var extra_buf: [64]u8 = undefined;
        const extra = formatExtra(&extra_buf, "{d} msg/s, {d} KB/s", .{ msg_per_sec, kb_per_sec });
        recordResult("Throughput-Up", sent == msg_count, "uplink test", elapsed, extra);
    } else {
        recordResult("Throughput-Up", false, "no messages sent", elapsed, "");
    }
}

// ============================================================================
// Test: Downlink Throughput (simplified - just verify recv works)
// ============================================================================

fn runThroughputDownTest() void {
    log.info("--- Throughput Down Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Throughput-Down", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Throughput-Down", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Throughput-Down", false, "Connect failed", 0, "");
        return;
    };

    socket.setRecvTimeout(1000); // 1 second timeout for recv

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Throughput-Down", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Throughput-Down", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Throughput-Down", false, "MQTT connect failed", 0, "");
        return;
    };

    // Subscribe
    const topics = [_][]const u8{"test/mqtt0/downlink"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Throughput-Down", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Publish a message to ourselves
    client.publish("test/mqtt0/downlink", "echo test", &buf) catch {
        recordResult("Throughput-Down", false, "Publish failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Try to receive (with timeout)
    var received: u32 = 0;
    var attempts: u32 = 0;
    while (attempts < 10) : (attempts += 1) {
        if (client.recvMessage(&buf)) |msg| {
            if (msg) |_| {
                received += 1;
                break;
            }
        } else |_| {
            break;
        }
        Board.time.sleepMs(100);
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    var extra_buf: [64]u8 = undefined;
    const extra = formatExtra(&extra_buf, "recv {d} messages", .{received});
    recordResult("Throughput-Down", true, "downlink test", elapsed, extra);
}

// ============================================================================
// Test: Long Connection
// ============================================================================

fn runLongConnectionTest() void {
    log.info("--- Long Connection Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Long-Connection", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Long-Connection", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Long-Connection", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Long-Connection", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Long-Connection", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 30, // 30 second keepalive
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Long-Connection", false, "MQTT connect failed", 0, "");
        return;
    };

    // Run for 60 seconds
    const test_duration_ms: u64 = 60 * 1000;
    const publish_interval_ms: u64 = 10 * 1000;

    var last_publish: u64 = 0;
    var ping_count: u32 = 0;
    var publish_count: u32 = 0;
    var errors: u32 = 0;

    while (Board.time.getTimeMs() - start < test_duration_ms) {
        // Check if ping needed
        if (client.needsPing()) {
            client.ping(&buf) catch {
                errors += 1;
                break;
            };
            ping_count += 1;
        }

        // Periodic publish
        const now = Board.time.getTimeMs();
        if (now - last_publish >= publish_interval_ms) {
            client.publish("test/mqtt0/longconn", "heartbeat", &buf) catch {
                errors += 1;
            };
            publish_count += 1;
            last_publish = now;
        }

        if (!client.isConnected()) {
            errors += 1;
            break;
        }

        Board.time.sleepMs(1000);
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    var extra_buf: [64]u8 = undefined;
    const extra = formatExtra(&extra_buf, "{d}s, {d} pings, {d} pub, {d} err", .{
        elapsed / 1000,
        ping_count,
        publish_count,
        errors,
    });
    recordResult("Long-Connection", errors == 0, "stability test", elapsed, extra);
}

// ============================================================================
// Test: Reconnect
// ============================================================================

fn runReconnectTest() void {
    log.info("--- Reconnect Test ---", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;
    var successful_reconnects: u32 = 0;
    const max_reconnects: u32 = 3;

    var attempt: u32 = 0;
    while (attempt < max_reconnects) : (attempt += 1) {
        log.info("Reconnect attempt {d}/{d}", .{ attempt + 1, max_reconnects });

        const ip = resolveDns(config.mqtt_host) orelse continue;

        var socket = Board.socket.tcp() catch continue;
        defer socket.close();

        socket.connect(ip, config.mqtt_port) catch continue;

        var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch continue;
        defer tls_client.deinit();

        tls_client.connect() catch continue;

        var client = MqttClient.init(&tls_client);

        client.connect(&.{
            .client_id = config.client_id,
            .keep_alive = 60,
            .topic_alias_maximum = 16,
        }, &buf) catch continue;

        // Quick publish to verify connection
        client.publish("test/mqtt0/reconnect", "test", &buf) catch {
            client.disconnect(&buf);
            continue;
        };

        client.disconnect(&buf);
        successful_reconnects += 1;

        // Wait before next attempt
        if (attempt < max_reconnects - 1) {
            Board.time.sleepMs(2000);
        }
    }

    const elapsed = Board.time.getTimeMs() - start;
    var extra_buf: [64]u8 = undefined;
    const extra = formatExtra(&extra_buf, "{d}/{d} successful", .{ successful_reconnects, max_reconnects });
    recordResult("Reconnect", successful_reconnects == max_reconnects, "reconnect test", elapsed, extra);
}

// ============================================================================
// Test: Concurrent Throughput (3 uplink + 2 downlink topics)
// ============================================================================

fn runConcurrentThroughputTest() void {
    log.info("=== Concurrent Throughput Test ===", .{});
    log.info("3 uplink topics + 2 downlink topics", .{});
    log.info("50 msg/s per topic, 10 seconds", .{});
    const start = Board.time.getTimeMs();

    var buf: [8192]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Concurrent", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Concurrent", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Concurrent", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Concurrent", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Concurrent", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Concurrent", false, "MQTT connect failed", 0, "");
        return;
    };

    // Subscribe to downlink topics
    const down_topics = [_][]const u8{
        "bench/down/0",
        "bench/down/1",
    };
    client.subscribe(&down_topics, &buf) catch {
        recordResult("Concurrent", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Uplink topics
    const up_topics = [_][]const u8{
        "bench/up/0",
        "bench/up/1",
        "bench/up/2",
    };

    // Test parameters
    const test_duration_ms: u64 = 10 * 1000; // 10 seconds
    const msg_per_sec: u32 = 50; // per topic
    const interval_ms: u64 = 1000 / msg_per_sec; // 20ms between sends per topic

    // Message buffer with header space: seq(4) + timestamp(8) + payload
    var msg_buf: [400]u8 = undefined;

    // PRNG state (simple LCG)
    var prng_state: u32 = @truncate(Board.time.getTimeMs());

    // Counters
    var total_sent: u32 = 0;
    var total_bytes: u64 = 0;
    var errors: u32 = 0;

    // Per-topic state
    var topic_seq: [3]u32 = .{ 0, 0, 0 };
    var topic_last_send: [3]u64 = .{ 0, 0, 0 };

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        // Send to each uplink topic
        for (up_topics, 0..) |topic, ti| {
            if (now - topic_last_send[ti] >= interval_ms) {
                // Generate random size 100-400 bytes
                prng_state = prng_state *% 1103515245 +% 12345;
                const rand_size: usize = 100 + (prng_state >> 16) % 301;

                // Build message: seq(4) + timestamp_ms(8) + random_payload
                const seq = topic_seq[ti];
                const relative_ts = now - test_start;

                // Write seq (big-endian)
                msg_buf[0] = @truncate(seq >> 24);
                msg_buf[1] = @truncate(seq >> 16);
                msg_buf[2] = @truncate(seq >> 8);
                msg_buf[3] = @truncate(seq);

                // Write timestamp (big-endian u64)
                msg_buf[4] = @truncate(relative_ts >> 56);
                msg_buf[5] = @truncate(relative_ts >> 48);
                msg_buf[6] = @truncate(relative_ts >> 40);
                msg_buf[7] = @truncate(relative_ts >> 32);
                msg_buf[8] = @truncate(relative_ts >> 24);
                msg_buf[9] = @truncate(relative_ts >> 16);
                msg_buf[10] = @truncate(relative_ts >> 8);
                msg_buf[11] = @truncate(relative_ts);

                // Fill random payload
                var i: usize = 12;
                while (i < rand_size) : (i += 1) {
                    prng_state = prng_state *% 1103515245 +% 12345;
                    msg_buf[i] = @truncate(prng_state >> 16);
                }

                // Publish
                client.publish(topic, msg_buf[0..rand_size], &buf) catch {
                    errors += 1;
                    continue;
                };

                topic_seq[ti] += 1;
                topic_last_send[ti] = now;
                total_sent += 1;
                total_bytes += rand_size;
            }
        }

        // Print status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            log.info("[{d}s] sent={d} bytes={d}KB errors={d}", .{
                elapsed_sec,
                total_sent,
                total_bytes / 1024,
                errors,
            });
            last_status = now;
        }

        // Small delay to prevent busy loop
        Board.time.sleepMs(1);
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    const elapsed_sec = elapsed / 1000;

    // Calculate rates
    var msg_per_sec_actual: u32 = 0;
    var kb_per_sec: u32 = 0;
    if (elapsed_sec > 0) {
        msg_per_sec_actual = @truncate(total_sent / elapsed_sec);
        kb_per_sec = @truncate(total_bytes / 1024 / elapsed_sec);
    }

    log.info("=== Concurrent Test Complete ===", .{});
    log.info("Total: {d} messages, {d} KB", .{ total_sent, total_bytes / 1024 });
    log.info("Rate: {d} msg/s, {d} KB/s", .{ msg_per_sec_actual, kb_per_sec });
    log.info("Errors: {d}", .{errors});

    // Expected: 3 topics * 50 msg/s * 10s = 1500 messages
    const expected_min = 1200; // Allow some tolerance
    recordResult("Concurrent", total_sent >= expected_min and errors == 0, "concurrent throughput", elapsed, "");
}

// ============================================================================
// Test: Bandwidth (single topic, 50 msg/s, max payload size)
// ============================================================================

fn runBandwidthTest() void {
    log.info("=== Bandwidth Test ===", .{});
    log.info("Single topic, 50 msg/s, 1KB payload", .{});
    const start = Board.time.getTimeMs();

    var buf: [8192]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Bandwidth", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Bandwidth", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Bandwidth", false, "Connect failed", 0, "");
        return;
    };

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Bandwidth", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Bandwidth", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Bandwidth", false, "MQTT connect failed", 0, "");
        return;
    };

    // Test parameters
    const test_duration_ms: u64 = 10 * 1000; // 10 seconds
    const msg_per_sec: u32 = 50;
    const interval_ms: u64 = 1000 / msg_per_sec; // 20ms
    const payload_size: usize = 1024; // 1KB payload

    // Large payload buffer
    var msg_buf: [1024]u8 = undefined;
    for (&msg_buf, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    // Counters
    var total_sent: u32 = 0;
    var total_bytes: u64 = 0;
    var errors: u32 = 0;
    var seq: u32 = 0;
    var last_send: u64 = 0;

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        if (now - last_send >= interval_ms) {
            // Write seq number at start of payload
            msg_buf[0] = @truncate(seq >> 24);
            msg_buf[1] = @truncate(seq >> 16);
            msg_buf[2] = @truncate(seq >> 8);
            msg_buf[3] = @truncate(seq);

            client.publish("bench/bandwidth", &msg_buf, &buf) catch {
                errors += 1;
                last_send = now;
                continue;
            };

            seq += 1;
            last_send = now;
            total_sent += 1;
            total_bytes += payload_size;
        }

        // Print status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            log.info("[{d}s] sent={d} bytes={d}KB errors={d}", .{
                elapsed_sec,
                total_sent,
                total_bytes / 1024,
                errors,
            });
            last_status = now;
        }

        Board.time.sleepMs(1);
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;
    const elapsed_sec = elapsed / 1000;

    var msg_per_sec_actual: u32 = 0;
    var kb_per_sec: u32 = 0;
    if (elapsed_sec > 0) {
        msg_per_sec_actual = @truncate(total_sent / elapsed_sec);
        kb_per_sec = @truncate(total_bytes / 1024 / elapsed_sec);
    }

    log.info("=== Bandwidth Test Complete ===", .{});
    log.info("Total: {d} messages, {d} KB", .{ total_sent, total_bytes / 1024 });
    log.info("Rate: {d} msg/s, {d} KB/s ({d} Kbit/s)", .{ msg_per_sec_actual, kb_per_sec, kb_per_sec * 8 });
    log.info("Errors: {d}", .{errors});

    recordResult("Bandwidth", errors == 0, "bandwidth test", elapsed, "");
}

// ============================================================================
// Test: Realtime - Max throughput (uplink + downlink echo)
// ============================================================================

fn runRealtimeTest() void {
    log.info("=== Max Throughput Test (Echo Mode) ===", .{});
    log.info("Uplink + Downlink, 512B payload, batch=50, TCP_NODELAY=on", .{});
    const start = Board.time.getTimeMs();

    var buf: [8192]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Realtime", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Realtime", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Realtime", false, "Connect failed", 0, "");
        return;
    };

    // TCP optimizations for throughput
    socket.setTcpNoDelay(true); // Disable Nagle algorithm
    socket.setRecvTimeout(1); // 1ms timeout for non-blocking receive

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Realtime", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Realtime", false, "TLS connect failed", 0, "");
        return;
    };

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Realtime", false, "MQTT connect failed", 0, "");
        return;
    };

    // Subscribe to echo topic (will receive what we send)
    const topics = [_][]const u8{"bench/echo"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Realtime", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    // Test parameters
    const test_duration_ms: u64 = 10 * 1000; // 10 seconds
    const payload_size: usize = 512; // 512B payload
    const batch_size: u32 = 50; // Send N messages then try recv

    // Payload buffer
    var msg_buf: [512]u8 = undefined;
    for (&msg_buf, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    // Counters
    var up_sent: u32 = 0;
    var up_bytes: u64 = 0;
    var down_recv: u32 = 0;
    var down_bytes: u64 = 0;
    var errors: u32 = 0;
    var seq: u32 = 0;
    var batch_count: u32 = 0;

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        // Uplink: send message
        msg_buf[0] = @truncate(seq >> 24);
        msg_buf[1] = @truncate(seq >> 16);
        msg_buf[2] = @truncate(seq >> 8);
        msg_buf[3] = @truncate(seq);

        client.publish("bench/echo", &msg_buf, &buf) catch {
            errors += 1;
            continue;
        };

        seq += 1;
        up_sent += 1;
        up_bytes += payload_size;
        batch_count += 1;

        // After sending batch_size messages, try to receive
        if (batch_count >= batch_size) {
            batch_count = 0;
            // Drain recv buffer (try up to batch_size times)
            var recv_attempts: u32 = 0;
            while (recv_attempts < batch_size) : (recv_attempts += 1) {
                if (client.recvMessage(&buf)) |msg_opt| {
                    if (msg_opt) |msg| {
                        down_recv += 1;
                        down_bytes += msg.payload.len;
                    } else {
                        break; // No more messages
                    }
                } else |_| {
                    break; // Timeout or error
                }
            }
        }

        // Print status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            if (elapsed_sec > 0) {
                const up_rate = up_sent / @as(u32, @truncate(elapsed_sec));
                const down_rate = down_recv / @as(u32, @truncate(elapsed_sec));
                log.info("[{d}s] UP: {d}/s {d}KB  DOWN: {d}/s {d}KB  err={d}", .{
                    elapsed_sec,
                    up_rate,
                    up_bytes / 1024,
                    down_rate,
                    down_bytes / 1024,
                    errors,
                });
            }
            last_status = now;
        }
    }

    // Calculate test duration (actual test time, not including connection setup)
    const test_elapsed = Board.time.getTimeMs() - test_start;
    const test_elapsed_sec = test_elapsed / 1000;

    // Quick drain of any remaining messages (limit attempts to avoid long wait)
    var final_drain: u32 = 0;
    while (final_drain < 100) : (final_drain += 1) {
        if (client.recvMessage(&buf)) |msg_opt| {
            if (msg_opt) |msg| {
                down_recv += 1;
                down_bytes += msg.payload.len;
            } else {
                break;
            }
        } else |_| {
            break;
        }
    }

    client.disconnect(&buf);

    const total_elapsed = Board.time.getTimeMs() - start;

    var up_rate: u32 = 0;
    var down_rate: u32 = 0;
    var up_kbps: u32 = 0;
    var down_kbps: u32 = 0;
    if (test_elapsed_sec > 0) {
        up_rate = @truncate(up_sent / test_elapsed_sec);
        down_rate = @truncate(down_recv / test_elapsed_sec);
        up_kbps = @truncate(up_bytes * 8 / 1000 / test_elapsed_sec);
        down_kbps = @truncate(down_bytes * 8 / 1000 / test_elapsed_sec);
    }

    log.info("=== Max Throughput Test Complete (Echo Mode) ===", .{});
    log.info("Test Duration: {d} ms", .{test_elapsed});
    log.info("Uplink:   {d} msg, {d} KB, {d} msg/s, {d} Kbit/s", .{ up_sent, up_bytes / 1024, up_rate, up_kbps });
    log.info("Downlink: {d} msg, {d} KB, {d} msg/s, {d} Kbit/s", .{ down_recv, down_bytes / 1024, down_rate, down_kbps });
    log.info("Errors:   {d}", .{errors});

    recordResult("Realtime", errors == 0, "max throughput echo", total_elapsed, "");
}

// ============================================================================
// Test: Latency - RTT measurement with TCP_NODELAY
// ============================================================================

fn runLatencyTest() void {
    log.info("=== Latency Test (RTT with TCP_NODELAY) ===", .{});
    log.info("Echo mode, 100 samples, small payload", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Latency", false, "DNS failed", 0, "");
        return;
    };

    var socket = Board.socket.tcp() catch {
        recordResult("Latency", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Latency", false, "Connect failed", 0, "");
        return;
    };

    // TCP_NODELAY for low latency
    socket.setTcpNoDelay(true);
    socket.setRecvTimeout(1000); // 1s timeout for recv

    var tls_client = TlsClient.init(&socket, .{ .allocator = allocator, .hostname = config.mqtt_host, .skip_verify = config.skip_verify }) catch {
        recordResult("Latency", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Latency", false, "TLS connect failed", 0, "");
        return;
    };

    log.info("Connected to MQTT broker", .{});

    var client = MqttClient.init(&tls_client);

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
    }, &buf) catch {
        recordResult("Latency", false, "MQTT connect failed", 0, "");
        return;
    };

    // Subscribe to echo topic
    const topics = [_][]const u8{"bench/latency"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Latency", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    log.info("Subscribed, starting RTT measurements...", .{});

    // Warmup: send a few messages to stabilize connection
    var warmup: u32 = 0;
    while (warmup < 5) : (warmup += 1) {
        client.publish("bench/latency", "warmup", &buf) catch continue;
        Board.time.sleepMs(10);
        _ = client.recvMessage(&buf) catch {};
    }

    // Latency samples
    const sample_count: u32 = 100;
    var latencies: [100]u32 = undefined;
    var valid_samples: u32 = 0;
    var total_latency: u64 = 0;
    var min_latency: u32 = 0xFFFFFFFF;
    var max_latency: u32 = 0;
    var timeouts: u32 = 0;

    // Message payload: seq(4) + send_timestamp(8)
    var msg_buf: [64]u8 = undefined;

    var seq: u32 = 0;
    while (seq < sample_count) : (seq += 1) {
        // Encode seq
        msg_buf[0] = @truncate(seq >> 24);
        msg_buf[1] = @truncate(seq >> 16);
        msg_buf[2] = @truncate(seq >> 8);
        msg_buf[3] = @truncate(seq);

        // Record send time
        const send_time = Board.time.getTimeMs();

        // Encode send time
        msg_buf[4] = @truncate(send_time >> 56);
        msg_buf[5] = @truncate(send_time >> 48);
        msg_buf[6] = @truncate(send_time >> 40);
        msg_buf[7] = @truncate(send_time >> 32);
        msg_buf[8] = @truncate(send_time >> 24);
        msg_buf[9] = @truncate(send_time >> 16);
        msg_buf[10] = @truncate(send_time >> 8);
        msg_buf[11] = @truncate(send_time);

        // Send
        client.publish("bench/latency", msg_buf[0..12], &buf) catch {
            timeouts += 1;
            continue;
        };

        // Wait for echo
        var received = false;
        var attempts: u32 = 0;
        while (attempts < 100) : (attempts += 1) { // Max 100ms wait
            if (client.recvMessage(&buf)) |msg_opt| {
                if (msg_opt) |msg| {
                    // Verify it's our message by checking seq
                    if (msg.payload.len >= 4) {
                        const recv_seq: u32 = (@as(u32, msg.payload[0]) << 24) |
                            (@as(u32, msg.payload[1]) << 16) |
                            (@as(u32, msg.payload[2]) << 8) |
                            @as(u32, msg.payload[3]);
                        if (recv_seq == seq) {
                            received = true;
                            break;
                        }
                    }
                }
            } else |_| {
                // Timeout, try again
            }
            Board.time.sleepMs(1);
        }

        if (received) {
            const recv_time = Board.time.getTimeMs();
            const rtt: u32 = @truncate(recv_time - send_time);
            latencies[valid_samples] = rtt;
            valid_samples += 1;
            total_latency += rtt;
            if (rtt < min_latency) min_latency = rtt;
            if (rtt > max_latency) max_latency = rtt;
        } else {
            timeouts += 1;
        }

        // Print progress every 20 samples
        if ((seq + 1) % 20 == 0) {
            log.info("Progress: {d}/{d} samples, {d} timeouts", .{ seq + 1, sample_count, timeouts });
        }
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;

    // Calculate statistics
    var avg_latency: u32 = 0;
    var p50: u32 = 0;
    var p95: u32 = 0;
    var p99: u32 = 0;

    if (valid_samples > 0) {
        avg_latency = @truncate(total_latency / valid_samples);

        // Simple bubble sort for percentile calculation
        var i: u32 = 0;
        while (i < valid_samples) : (i += 1) {
            var j: u32 = i + 1;
            while (j < valid_samples) : (j += 1) {
                if (latencies[j] < latencies[i]) {
                    const tmp = latencies[i];
                    latencies[i] = latencies[j];
                    latencies[j] = tmp;
                }
            }
        }

        // Percentiles
        const p50_idx = valid_samples / 2;
        const p95_idx = (valid_samples * 95) / 100;
        const p99_idx = (valid_samples * 99) / 100;

        p50 = latencies[p50_idx];
        p95 = if (p95_idx < valid_samples) latencies[p95_idx] else max_latency;
        p99 = if (p99_idx < valid_samples) latencies[p99_idx] else max_latency;
    }

    log.info("=== Latency Test Complete ===", .{});
    log.info("Samples: {d}/{d} (timeouts: {d})", .{ valid_samples, sample_count, timeouts });
    log.info("RTT Min:  {d} ms", .{min_latency});
    log.info("RTT Avg:  {d} ms", .{avg_latency});
    log.info("RTT Max:  {d} ms", .{max_latency});
    log.info("RTT P50:  {d} ms", .{p50});
    log.info("RTT P95:  {d} ms", .{p95});
    log.info("RTT P99:  {d} ms", .{p99});

    recordResult("Latency", valid_samples >= 90, "RTT measurement", elapsed, "");
}

// ============================================================================
// Test: Remote Latency - RTT to remote server (ESP <-> Go)
// ============================================================================

fn runRemoteLatencyTest() void {
    log.info("=== Remote Latency Test (ESP <-> Go) ===", .{});
    log.info("Server: {s}:{d}", .{ config.mqtt_host, config.mqtt_port });
    log.info("TX: bench/esp/tx, RX: bench/esp/rx", .{});
    const start = Board.time.getTimeMs();

    var buf: [4096]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Remote-Latency", false, "DNS failed", 0, "");
        return;
    };
    log.info("Resolved IP: {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });

    var socket = Board.socket.tcp() catch {
        recordResult("Remote-Latency", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Remote-Latency", false, "Connect failed", 0, "");
        return;
    };

    // TCP_NODELAY for low latency
    socket.setTcpNoDelay(true);
    socket.setRecvTimeout(5000); // 5s timeout for remote server

    var tls_client = TlsClient.init(&socket, .{
        .allocator = allocator,
        .hostname = config.mqtt_host,
        .skip_verify = config.skip_verify,
    }) catch {
        recordResult("Remote-Latency", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Remote-Latency", false, "TLS connect failed", 0, "");
        return;
    };

    log.info("TLS connected", .{});

    var client = MqttClient.init(&tls_client);

    // Get optional username/password (treat "-" as empty/anonymous)
    const username: ?[]const u8 = if (config.mqtt_username.len > 0 and config.mqtt_username[0] != '-') config.mqtt_username else null;
    const password: ?[]const u8 = if (config.mqtt_password.len > 0 and config.mqtt_password[0] != '-') config.mqtt_password else null;

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
        .username = username,
        .password = password,
    }, &buf) catch |e| {
        log.err("MQTT connect failed: {}", .{e});
        recordResult("Remote-Latency", false, "MQTT connect failed", 0, "");
        return;
    };

    log.info("MQTT connected (user: {s})", .{if (username) |u| u else "anonymous"});

    // Subscribe to RX topic (Go -> ESP)
    const topics = [_][]const u8{"bench/esp/rx"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Remote-Latency", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    log.info("Subscribed to bench/esp/rx", .{});
    log.info("Waiting for Go peer... (publish to bench/esp/tx)", .{});

    // Warmup: wait for Go peer to be ready
    Board.time.sleepMs(2000);

    // Latency samples
    const sample_count: u32 = 100;
    var latencies: [100]u32 = undefined;
    var valid_samples: u32 = 0;
    var total_latency: u64 = 0;
    var min_latency: u32 = 0xFFFFFFFF;
    var max_latency: u32 = 0;
    var timeouts: u32 = 0;

    // Message payload: seq(4) + send_timestamp(8)
    var msg_buf: [64]u8 = undefined;

    var seq: u32 = 0;
    while (seq < sample_count) : (seq += 1) {
        // Encode seq
        msg_buf[0] = @truncate(seq >> 24);
        msg_buf[1] = @truncate(seq >> 16);
        msg_buf[2] = @truncate(seq >> 8);
        msg_buf[3] = @truncate(seq);

        // Record send time
        const send_time = Board.time.getTimeMs();

        // Encode send time
        msg_buf[4] = @truncate(send_time >> 56);
        msg_buf[5] = @truncate(send_time >> 48);
        msg_buf[6] = @truncate(send_time >> 40);
        msg_buf[7] = @truncate(send_time >> 32);
        msg_buf[8] = @truncate(send_time >> 24);
        msg_buf[9] = @truncate(send_time >> 16);
        msg_buf[10] = @truncate(send_time >> 8);
        msg_buf[11] = @truncate(send_time);

        // Publish to TX topic (ESP -> Go)
        client.publish("bench/esp/tx", msg_buf[0..12], &buf) catch {
            timeouts += 1;
            continue;
        };

        // Wait for response on RX topic (Go -> ESP)
        var received = false;
        var attempts: u32 = 0;
        while (attempts < 500) : (attempts += 1) { // Max 500ms wait (remote server)
            if (client.recvMessage(&buf)) |msg_opt| {
                if (msg_opt) |msg| {
                    // Verify it's our response by checking seq
                    if (msg.payload.len >= 4) {
                        const recv_seq: u32 = (@as(u32, msg.payload[0]) << 24) |
                            (@as(u32, msg.payload[1]) << 16) |
                            (@as(u32, msg.payload[2]) << 8) |
                            @as(u32, msg.payload[3]);
                        if (recv_seq == seq) {
                            received = true;
                            break;
                        }
                    }
                }
            } else |_| {
                // Timeout, try again
            }
            Board.time.sleepMs(1);
        }

        if (received) {
            const recv_time = Board.time.getTimeMs();
            const rtt: u32 = @truncate(recv_time - send_time);
            latencies[valid_samples] = rtt;
            valid_samples += 1;
            total_latency += rtt;
            if (rtt < min_latency) min_latency = rtt;
            if (rtt > max_latency) max_latency = rtt;
        } else {
            timeouts += 1;
        }

        // Print progress every 20 samples
        if ((seq + 1) % 20 == 0) {
            log.info("Progress: {d}/{d} samples, {d} timeouts", .{ seq + 1, sample_count, timeouts });
        }
    }

    client.disconnect(&buf);

    const elapsed = Board.time.getTimeMs() - start;

    // Calculate statistics
    var avg_latency: u32 = 0;
    var p50: u32 = 0;
    var p95: u32 = 0;
    var p99: u32 = 0;

    if (valid_samples > 0) {
        avg_latency = @truncate(total_latency / valid_samples);

        // Simple bubble sort for percentile calculation
        var i: u32 = 0;
        while (i < valid_samples) : (i += 1) {
            var j: u32 = i + 1;
            while (j < valid_samples) : (j += 1) {
                if (latencies[j] < latencies[i]) {
                    const tmp = latencies[i];
                    latencies[i] = latencies[j];
                    latencies[j] = tmp;
                }
            }
        }

        // Percentiles
        const p50_idx = valid_samples / 2;
        const p95_idx = (valid_samples * 95) / 100;
        const p99_idx = (valid_samples * 99) / 100;

        p50 = latencies[p50_idx];
        p95 = if (p95_idx < valid_samples) latencies[p95_idx] else max_latency;
        p99 = if (p99_idx < valid_samples) latencies[p99_idx] else max_latency;
    }

    log.info("=== Remote Latency Test Complete ===", .{});
    log.info("Samples: {d}/{d} (timeouts: {d})", .{ valid_samples, sample_count, timeouts });
    if (valid_samples > 0) {
        log.info("RTT Min:  {d} ms", .{min_latency});
        log.info("RTT Avg:  {d} ms", .{avg_latency});
        log.info("RTT Max:  {d} ms", .{max_latency});
        log.info("RTT P50:  {d} ms", .{p50});
        log.info("RTT P95:  {d} ms", .{p95});
        log.info("RTT P99:  {d} ms", .{p99});
    }

    recordResult("Remote-Latency", valid_samples >= 50, "Remote RTT", elapsed, "");
}

// ============================================================================
// Test: Remote Bandwidth - Throughput to remote server
// ============================================================================

fn runRemoteBandwidthTest() void {
    log.info("=== Remote Bandwidth Test ===", .{});
    log.info("Server: {s}:{d}", .{ config.mqtt_host, config.mqtt_port });
    log.info("Uplink + Downlink echo, 512B payload, 10 seconds", .{});
    const start = Board.time.getTimeMs();

    var buf: [8192]u8 = undefined;

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("Remote-BW", false, "DNS failed", 0, "");
        return;
    };
    log.info("Resolved IP: {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });

    var socket = Board.socket.tcp() catch {
        recordResult("Remote-BW", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    socket.connect(ip, config.mqtt_port) catch {
        recordResult("Remote-BW", false, "Connect failed", 0, "");
        return;
    };

    // TCP optimizations
    socket.setTcpNoDelay(true);
    socket.setRecvTimeout(100); // 100ms timeout for recv

    var tls_client = TlsClient.init(&socket, .{
        .allocator = allocator,
        .hostname = config.mqtt_host,
        .skip_verify = config.skip_verify,
    }) catch {
        recordResult("Remote-BW", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("Remote-BW", false, "TLS connect failed", 0, "");
        return;
    };

    log.info("TLS connected", .{});

    var client = MqttClient.init(&tls_client);

    // Get optional username/password (treat "-" as empty/anonymous)
    const username: ?[]const u8 = if (config.mqtt_username.len > 0 and config.mqtt_username[0] != '-') config.mqtt_username else null;
    const password: ?[]const u8 = if (config.mqtt_password.len > 0 and config.mqtt_password[0] != '-') config.mqtt_password else null;

    client.connect(&.{
        .client_id = config.client_id,
        .keep_alive = 60,
        .topic_alias_maximum = 16,
        .username = username,
        .password = password,
    }, &buf) catch |e| {
        log.err("MQTT connect failed: {}", .{e});
        recordResult("Remote-BW", false, "MQTT connect failed", 0, "");
        return;
    };

    log.info("MQTT connected (user: {s})", .{if (username) |u| u else "anonymous"});

    // Subscribe to echo topic
    const topics = [_][]const u8{"bench/esp/rx"};
    client.subscribe(&topics, &buf) catch {
        recordResult("Remote-BW", false, "Subscribe failed", 0, "");
        client.disconnect(&buf);
        return;
    };

    log.info("Subscribed, starting throughput test...", .{});

    // Set recv timeout AFTER TLS connect (in case TLS resets it)
    log.info("Setting recv timeout to 2000ms...", .{});
    socket.setRecvTimeout(2000);

    // Warmup
    Board.time.sleepMs(500);
    log.info("Warmup done, starting test loop...", .{});

    // Test parameters
    const test_duration_ms: u64 = 10 * 1000;
    const payload_size: usize = 512;

    // Payload buffer
    var msg_buf: [512]u8 = undefined;
    for (&msg_buf, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    // Counters
    var up_sent: u32 = 0;
    var up_bytes: u64 = 0;
    var errors: u32 = 0;
    var seq: u32 = 0;

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    // Bidirectional bandwidth test (TLS fix in embed-zig a8d5a3d)
    log.info("Starting BIDIRECTIONAL bandwidth test...", .{});
    log.info("TX: bench/esp/tx, RX: bench/esp/rx", .{});

    // Downlink counters
    var down_recv: u32 = 0;
    var down_bytes: u64 = 0;
    var timeouts: u32 = 0;

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        // Uplink: send message
        msg_buf[0] = @truncate(seq >> 24);
        msg_buf[1] = @truncate(seq >> 16);
        msg_buf[2] = @truncate(seq >> 8);
        msg_buf[3] = @truncate(seq);

        client.publish("bench/esp/tx", &msg_buf, &buf) catch {
            errors += 1;
            continue;
        };

        seq += 1;
        up_sent += 1;
        up_bytes += payload_size;

        // Downlink: try to receive (with timeout)
        const recv_result = client.recvMessage(&buf);
        if (recv_result) |maybe_msg| {
            if (maybe_msg) |msg| {
                down_recv += 1;
                down_bytes += msg.payload.len;
            }
        } else |_| {
            timeouts += 1;
        }

        // Status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            if (elapsed_sec > 0) {
                const up_rate = up_sent / @as(u32, @truncate(elapsed_sec));
                const down_rate = down_recv / @as(u32, @truncate(elapsed_sec));
                log.info("[{d}s] UP: {d}/s {d}KB | DOWN: {d}/s {d}KB | err={d} to={d}", .{
                    elapsed_sec,
                    up_rate,
                    up_bytes / 1024,
                    down_rate,
                    down_bytes / 1024,
                    errors,
                    timeouts,
                });
            }
            last_status = now;
        }
    }

    client.disconnect(&buf);

    const test_elapsed = Board.time.getTimeMs() - test_start;
    const test_elapsed_sec = test_elapsed / 1000;
    const total_elapsed = Board.time.getTimeMs() - start;

    var up_rate: u32 = 0;
    var up_kbps: u32 = 0;
    var down_rate: u32 = 0;
    var down_kbps: u32 = 0;
    if (test_elapsed_sec > 0) {
        up_rate = @truncate(up_sent / test_elapsed_sec);
        up_kbps = @truncate(up_bytes * 8 / 1000 / test_elapsed_sec);
        down_rate = @truncate(down_recv / test_elapsed_sec);
        down_kbps = @truncate(down_bytes * 8 / 1000 / test_elapsed_sec);
    }

    log.info("=== Bidirectional Bandwidth Test Complete ===", .{});
    log.info("Test Duration: {d} ms", .{test_elapsed});
    log.info("Uplink:   {d} msg, {d} KB, {d} msg/s, {d} Kbit/s", .{ up_sent, up_bytes / 1024, up_rate, up_kbps });
    log.info("Downlink: {d} msg, {d} KB, {d} msg/s, {d} Kbit/s", .{ down_recv, down_bytes / 1024, down_rate, down_kbps });
    log.info("Errors: {d}, Timeouts: {d}", .{ errors, timeouts });

    recordResult("Remote-BW", errors == 0, "Bidirectional throughput", total_elapsed, "");
}

// ============================================================================
// Test: TCP Duplex - Pure TCP bidirectional communication test
// ============================================================================

fn runTcpDuplexTest() void {
    log.info("=== TCP Duplex Test ===", .{});
    log.info("Pure TCP echo test (no TLS, no MQTT)", .{});
    log.info("Server: {s}:11880", .{config.mqtt_host});
    const start = Board.time.getTimeMs();

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("TCP-Duplex", false, "DNS failed", 0, "");
        return;
    };
    log.info("Resolved IP: {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });

    var socket = Board.socket.tcp() catch {
        recordResult("TCP-Duplex", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    // Connect to TCP echo server (port 11880)
    socket.connect(ip, 11880) catch {
        recordResult("TCP-Duplex", false, "Connect failed", 0, "");
        return;
    };

    log.info("TCP connected to :11880", .{});

    // Set TCP options
    socket.setTcpNoDelay(true);
    socket.setRecvTimeout(100); // 100ms timeout

    // Test parameters
    const test_duration_ms: u64 = 5 * 1000; // 5 seconds
    const payload_size: usize = 64;

    // Payload buffer
    var send_buf: [64]u8 = undefined;
    var recv_buf: [256]u8 = undefined;
    for (&send_buf, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    // Counters
    var sent: u32 = 0;
    var recv: u32 = 0;
    var send_bytes: u64 = 0;
    var recv_bytes: u64 = 0;
    var timeouts: u32 = 0;
    var errors: u32 = 0;
    var seq: u32 = 0;

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    log.info("Starting TCP duplex test...", .{});

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        // Encode seq in payload
        send_buf[0] = @truncate(seq >> 24);
        send_buf[1] = @truncate(seq >> 16);
        send_buf[2] = @truncate(seq >> 8);
        send_buf[3] = @truncate(seq);

        // Send
        _ = socket.send(&send_buf) catch {
            errors += 1;
            continue;
        };
        seq += 1;
        sent += 1;
        send_bytes += payload_size;

        // Receive (should echo back immediately)
        if (socket.recv(&recv_buf)) |n| {
            recv += 1;
            recv_bytes += n;
        } else |e| {
            if (e == error.Timeout) {
                timeouts += 1;
            } else {
                errors += 1;
            }
        }

        // Status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            if (elapsed_sec > 0) {
                log.info("[{d}s] SENT: {d} RECV: {d} timeout={d} err={d}", .{
                    elapsed_sec,
                    sent,
                    recv,
                    timeouts,
                    errors,
                });
            }
            last_status = now;
        }
    }

    const test_elapsed = Board.time.getTimeMs() - test_start;
    const total_elapsed = Board.time.getTimeMs() - start;

    log.info("=== TCP Duplex Test Complete ===", .{});
    log.info("Duration: {d} ms", .{test_elapsed});
    log.info("Sent:     {d} packets, {d} bytes", .{ sent, send_bytes });
    log.info("Recv:     {d} packets, {d} bytes", .{ recv, recv_bytes });
    log.info("Timeouts: {d}", .{timeouts});
    log.info("Errors:   {d}", .{errors});

    // Success if we received most of what we sent
    const success = recv > 0 and errors == 0;
    recordResult("TCP-Duplex", success, "TCP echo test", total_elapsed, "");
}

// ============================================================================
// Test: TLS Duplex - TLS bidirectional communication test
// ============================================================================

fn runTlsDuplexTest() void {
    log.info("=== TLS Duplex Test ===", .{});
    log.info("TLS echo test (TLS only, no MQTT)", .{});
    log.info("Server: {s}:11881", .{config.mqtt_host});
    const start = Board.time.getTimeMs();

    const ip = resolveDns(config.mqtt_host) orelse {
        recordResult("TLS-Duplex", false, "DNS failed", 0, "");
        return;
    };
    log.info("Resolved IP: {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });

    var socket = Board.socket.tcp() catch {
        recordResult("TLS-Duplex", false, "Socket failed", 0, "");
        return;
    };
    defer socket.close();

    // Connect to TLS echo server (port 11881)
    socket.connect(ip, 11881) catch {
        recordResult("TLS-Duplex", false, "Connect failed", 0, "");
        return;
    };

    log.info("TCP connected to :11881", .{});

    // Set TCP options BEFORE TLS handshake
    socket.setTcpNoDelay(true);
    socket.setRecvTimeout(100); // 100ms timeout

    // Create TLS client
    var tls_client = TlsClient.init(&socket, .{
        .allocator = allocator,
        .hostname = config.mqtt_host,
        .skip_verify = true, // Self-signed cert
    }) catch {
        recordResult("TLS-Duplex", false, "TLS init failed", 0, "");
        return;
    };
    defer tls_client.deinit();

    tls_client.connect() catch {
        recordResult("TLS-Duplex", false, "TLS handshake failed", 0, "");
        return;
    };

    log.info("TLS handshake complete", .{});

    // Re-set recv timeout after TLS handshake (in case it was reset)
    socket.setRecvTimeout(100);

    // Test parameters
    const test_duration_ms: u64 = 5 * 1000; // 5 seconds
    const payload_size: usize = 64;

    // Payload buffer
    var send_buf: [64]u8 = undefined;
    var recv_buf: [256]u8 = undefined;
    for (&send_buf, 0..) |*b, i| {
        b.* = @truncate(i);
    }

    // Counters
    var sent: u32 = 0;
    var recv: u32 = 0;
    var send_bytes: u64 = 0;
    var recv_bytes: u64 = 0;
    var timeouts: u32 = 0;
    var errors: u32 = 0;
    var seq: u32 = 0;

    const test_start = Board.time.getTimeMs();
    var last_status: u64 = test_start;

    log.info("Starting TLS duplex test...", .{});

    while (Board.time.getTimeMs() - test_start < test_duration_ms) {
        const now = Board.time.getTimeMs();

        // Encode seq in payload
        send_buf[0] = @truncate(seq >> 24);
        send_buf[1] = @truncate(seq >> 16);
        send_buf[2] = @truncate(seq >> 8);
        send_buf[3] = @truncate(seq);

        // Send via TLS
        _ = tls_client.send(&send_buf) catch {
            errors += 1;
            log.err("TLS send failed", .{});
            continue;
        };
        seq += 1;
        sent += 1;
        send_bytes += payload_size;

        // Receive via TLS (should echo back immediately)
        if (tls_client.recv(&recv_buf)) |n| {
            recv += 1;
            recv_bytes += n;
        } else |_| {
            // TLS recv returned error (could be timeout or other)
            timeouts += 1;
        }

        // Status every second
        if (now - last_status >= 1000) {
            const elapsed_sec = (now - test_start) / 1000;
            if (elapsed_sec > 0) {
                log.info("[{d}s] SENT: {d} RECV: {d} timeout={d} err={d}", .{
                    elapsed_sec,
                    sent,
                    recv,
                    timeouts,
                    errors,
                });
            }
            last_status = now;
        }
    }

    const test_elapsed = Board.time.getTimeMs() - test_start;
    const total_elapsed = Board.time.getTimeMs() - start;

    log.info("=== TLS Duplex Test Complete ===", .{});
    log.info("Duration: {d} ms", .{test_elapsed});
    log.info("Sent:     {d} packets, {d} bytes", .{ sent, send_bytes });
    log.info("Recv:     {d} packets, {d} bytes", .{ recv, recv_bytes });
    log.info("Timeouts: {d}", .{timeouts});
    log.info("Errors:   {d}", .{errors});

    // Success if we received most of what we sent
    const success = recv > 0 and errors == 0;
    recordResult("TLS-Duplex", success, "TLS echo test", total_elapsed, "");
}

// ============================================================================
// Helper Functions
// ============================================================================

fn resolveDns(host: []const u8) ?[4]u8 {
    // Try to parse as IP address first
    if (parseIpAddress(host)) |ip| {
        log.info("Using IP address directly: {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });
        return ip;
    }

    // getDns returns (primary_dns, secondary_dns) tuple
    const dns_servers = Board.net_impl.getDns();
    var resolver = DnsResolver{
        .server = dns_servers[0], // Use primary DNS
        .protocol = .udp,
        .timeout_ms = 5000,
    };
    return resolver.resolve(host) catch |e| {
        log.err("DNS resolve failed: {}", .{e});
        return null;
    };
}

fn parseIpAddress(s: []const u8) ?[4]u8 {
    var result: [4]u8 = undefined;
    var octet_index: usize = 0;
    var current_value: u16 = 0;
    var has_digit = false;

    for (s) |c| {
        if (c >= '0' and c <= '9') {
            current_value = current_value * 10 + (c - '0');
            if (current_value > 255) return null; // Invalid octet
            has_digit = true;
        } else if (c == '.') {
            if (!has_digit or octet_index >= 3) return null;
            result[octet_index] = @intCast(current_value);
            octet_index += 1;
            current_value = 0;
            has_digit = false;
        } else {
            return null; // Invalid character - must be a hostname
        }
    }

    // Final octet
    if (!has_digit or octet_index != 3) return null;
    result[3] = @intCast(current_value);

    return result;
}

fn formatExtra(buf: []u8, comptime fmt: []const u8, args: anytype) []const u8 {
    // Simple format without std.fmt
    _ = fmt;
    _ = args;
    // For now, return empty - proper formatting would need custom impl
    buf[0] = 0;
    return buf[0..0];
}
