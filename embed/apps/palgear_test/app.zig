//! Palgear API Test Application (Event-Driven)
//!
//! Tests the Palgear API using embed-zig's HTTP client.
//! Platform-independent: only depends on Board abstraction.
//! Includes HMAC-SHA256 signature for @refresh and Bearer token auth.

const std = @import("std");
const hal = @import("hal");
const http = @import("http");
const dns = @import("dns");
const tls = @import("tls");
const ntp = @import("ntp");

const platform = @import("platform.zig");
const Board = platform.Board;
const log = Board.log;

// Platform-specific helpers (accessed via hw, not HAL Board)
const allocator = platform.hw.allocator;
const debugMemoryUsage = platform.hw.debugMemoryUsage;
const debugStackUsage = platform.hw.debugStackUsage;

const BUILD_TAG = "giztoy_palgear_test_v5_xip_128k";

/// PSRAM task stack size (must match app_128k config)
const PSRAM_STACK_SIZE: usize = 131072; // 128KB

/// HTTP Client type using board's socket and crypto
const HttpClient = http.HttpClient(Board.socket, Board.crypto);

/// DNS Resolver
const DnsResolver = dns.Resolver(Board.socket);

/// TLS Client for custom requests
const TlsClient = tls.Client(Board.socket, Board.crypto);

/// NTP Client for time synchronization
const NtpClient = ntp.Client(Board.socket);

/// Application state machine
const AppState = enum {
    connecting,
    connected,
    syncing_ntp,
    testing,
    done,
};

/// Test results tracking
const TestResult = struct {
    passed: u32 = 0,
    failed: u32 = 0,
    skipped: u32 = 0,

    fn pass(self: *TestResult, name: []const u8) void {
        self.passed += 1;
        log.info("[PASS] {s}", .{name});
    }

    fn fail(self: *TestResult, name: []const u8, msg: []const u8) void {
        self.failed += 1;
        log.err("[FAIL] {s}: {s}", .{ name, msg });
    }

    fn skip(self: *TestResult, name: []const u8, reason: []const u8) void {
        self.skipped += 1;
        log.warn("[SKIP] {s}: {s}", .{ name, reason });
    }

    fn report(self: *const TestResult) void {
        log.info("==========================================", .{});
        log.info("Test Results:", .{});
        log.info("  Passed:  {d}", .{self.passed});
        log.info("  Failed:  {d}", .{self.failed});
        log.info("  Skipped: {d}", .{self.skipped});
        log.info("==========================================", .{});
    }
};


/// Main application entry point (event-driven)
pub fn run(env: anytype) void {
    log.info("==========================================", .{});
    log.info("Palgear API Test - giztoy", .{});
    log.info("Build Tag: {s}", .{BUILD_TAG});
    log.info("==========================================", .{});
    log.info("Board:     {s}", .{Board.meta.id});
    log.info("==========================================", .{});
    log.info("Config:", .{});
    log.info("  WiFi SSID:    {s}", .{env.wifi_ssid});
    log.info("  Device SN:    {s}", .{env.device_sn});
    log.info("  Device EID:   {s}", .{env.device_eid});
    log.info("  API Host:     {s}", .{env.api_host});
    log.info("==========================================", .{});

    // Initialize board
    var b: Board = undefined;
    b.init() catch |err| {
        log.err("Board init failed: {}", .{err});
        return;
    };
    defer b.deinit();

    log.info("Board initialized", .{});

    // Set LED to blue (initializing)
    b.rgb_leds.setColor(hal.Color.rgb(0, 0, 64));
    b.rgb_leds.refresh();

    // Start WiFi connection (non-blocking)
    log.info("Connecting to WiFi: {s}", .{env.wifi_ssid});
    b.wifi.connect(env.wifi_ssid, env.wifi_password);

    var state: AppState = .connecting;
    var results = TestResult{};
    var dns_server: [4]u8 = .{ 223, 5, 5, 5 }; // Default: AliDNS
    var ntp_offset: i64 = 0; // Offset from boot time to real time

    // Event loop
    while (Board.isRunning()) {
        // Poll for events
        b.poll();

        // Process events
        while (b.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| {
                    switch (wifi_event) {
                        .connected => {
                            log.info("WiFi connected to AP (waiting for IP...)", .{});
                        },
                        .disconnected => |reason| {
                            log.warn("WiFi disconnected: {}", .{reason});
                            b.rgb_leds.setColor(hal.Color.rgb(64, 64, 0));
                            b.rgb_leds.refresh();
                            state = .connecting;
                        },
                        .connection_failed => |reason| {
                            log.err("WiFi connection failed: {}", .{reason});
                            b.rgb_leds.setColor(hal.Color.rgb(64, 0, 0));
                            b.rgb_leds.refresh();
                            return;
                        },
                        .scan_done, .rssi_low, .ap_sta_connected, .ap_sta_disconnected => {},
                    }
                },
                .net => |net_event| {
                    switch (net_event) {
                        .dhcp_bound, .dhcp_renewed => |info| {
                            const ip = info.ip;
                            log.info("Got IP: {}.{}.{}.{}", .{ ip[0], ip[1], ip[2], ip[3] });
                            log.info("DNS: {}.{}.{}.{}", .{ info.dns_main[0], info.dns_main[1], info.dns_main[2], info.dns_main[3] });

                            if (info.dns_main[0] != 0) {
                                dns_server = info.dns_main;
                            }

                            b.rgb_leds.setColor(hal.Color.rgb(0, 64, 64));
                            b.rgb_leds.refresh();
                            state = .connected;
                        },
                        .ip_lost => {
                            log.warn("IP lost", .{});
                            b.rgb_leds.setColor(hal.Color.rgb(64, 64, 0));
                            b.rgb_leds.refresh();
                            state = .connecting;
                        },
                        else => {},
                    }
                },
                else => {},
            }
        }

        // State machine
        switch (state) {
            .connecting => {},
            .connected => {
                Board.time.sleepMs(500);
                state = .syncing_ntp;
            },
            .syncing_ntp => {
                log.info("", .{});
                log.info("Synchronizing time via NTP...", .{});
                ntp_offset = syncNtpTime();

                debugMemoryUsage("START");
                debugStackUsage("START", PSRAM_STACK_SIZE);

                runApiTests(&results, dns_server, ntp_offset, env);

                debugStackUsage("END", PSRAM_STACK_SIZE);
                state = .testing;
            },
            .testing => {
                results.report();
                debugMemoryUsage("END");
                debugStackUsage("FINAL", PSRAM_STACK_SIZE);

                if (results.failed == 0) {
                    b.rgb_leds.setColor(hal.Color.rgb(0, 64, 0));
                } else {
                    b.rgb_leds.setColor(hal.Color.rgb(64, 0, 0));
                }
                b.rgb_leds.refresh();
                state = .done;
            },
            .done => {
                const now = Board.time.getTimeMs();
                if ((now / 500) % 2 == 0) {
                    if (results.failed == 0) {
                        b.rgb_leds.setColor(hal.Color.rgb(0, 64, 0));
                    } else {
                        b.rgb_leds.setColor(hal.Color.rgb(64, 0, 0));
                    }
                } else {
                    b.rgb_leds.clear();
                }
                b.rgb_leds.refresh();
            },
        }

        Board.time.sleepMs(10);
    }
}

// ============================================================================
// NTP Time Synchronization
// ============================================================================

/// Synchronize time using NTP race query (parallel to multiple servers)
/// Returns the offset from boot time to real UTC time in milliseconds
fn syncNtpTime() i64 {
    var client = NtpClient{ .timeout_ms = 5000 };

    // Record T1 (local monotonic time before query)
    const t1: i64 = @intCast(Board.time.getTimeMs());

    // Query China servers in parallel (Aliyun, Tencent, NTSC, Cloudflare)
    if (client.queryRace(t1, &ntp.ServerLists.china)) |resp| {
        // Record T4 (local monotonic time after query)
        const t4: i64 = @intCast(Board.time.getTimeMs());

        // Calculate offset: ((T2 - T1) + (T3 - T4)) / 2
        const offset = @divFloor(
            (resp.receive_time_ms - t1) + (resp.transmit_time_ms - t4),
            2,
        );

        // Calculate round-trip delay
        const rtt = (t4 - t1) - (resp.transmit_time_ms - resp.receive_time_ms);

        // Current time = T4 + offset
        const current_time_ms = t4 + offset;

        var time_buf: [32]u8 = undefined;
        const formatted = ntp.formatTime(current_time_ms, &time_buf);

        log.info("NTP sync success!", .{});
        log.info("  Server stratum: {d}", .{resp.stratum});
        log.info("  Round-trip: {d} ms", .{rtt});
        log.info("  UTC time: {s}", .{formatted});
        log.info("  Offset: {d} ms", .{offset});

        return offset;
    } else |err| {
        log.err("NTP sync failed: {}", .{err});
        log.warn("Using boot time (tests may fail with REQUEST_EXPIRED)", .{});
        return 0;
    }
}

// ============================================================================
// HMAC-SHA256 Signature (using Board crypto suite)
// ============================================================================

/// Compute HMAC-SHA256 signature and encode as Base64
/// Uses Board.crypto (hardware-accelerated mbedTLS on ESP32)
fn computeSignature(key: []const u8, data: []const u8, out: []u8) []const u8 {
    const HmacSha256 = Board.crypto.HmacSha256;
    var mac: [HmacSha256.mac_length]u8 = undefined;
    HmacSha256.create(&mac, data, key);
    const encoded_len = std.base64.standard.Encoder.calcSize(mac.len);
    if (out.len < encoded_len) return out[0..0];
    return std.base64.standard.Encoder.encode(out[0..encoded_len], &mac);
}

// ============================================================================
// API Test Runner
// ============================================================================

fn runApiTests(results: *TestResult, dns_server: [4]u8, ntp_offset: i64, env: anytype) void {
    log.info("", .{});
    log.info("========================================", .{});
    log.info("  Palgear API Tests", .{});
    log.info("========================================", .{});

    const client = HttpClient{
        .allocator = allocator,
        .dns_server = dns_server,
        .timeout_ms = 30000,
    };

    var response_buf: [8192]u8 = undefined;
    var url_buf: [256]u8 = undefined;

    // ========================================================================
    // Phase 1: No Auth Tests
    // ========================================================================

    // Test 1: GET /hello
    log.info("", .{});
    log.info("Testing GET /palgear/v1/hello...", .{});

    const hello_url = std.fmt.bufPrint(&url_buf, "https://{s}/palgear/v1/hello", .{env.api_host}) catch {
        results.fail("/hello", "URL format error");
        return;
    };

    const start1 = Board.time.getTimeMs();
    if (client.get(hello_url, &response_buf)) |resp| {
        const duration = Board.time.getTimeMs() - start1;
        log.info("  Status: {d} ({s}) in {d}ms", .{ resp.status_code, resp.statusText(), duration });
        if (resp.isSuccess()) {
            log.info("  Body: {s}", .{resp.body()[0..@min(200, resp.body().len)]});
            results.pass("/hello");
        } else {
            results.fail("/hello", "Non-2xx status");
        }
    } else |err| {
        log.err("  Error: {}", .{err});
        results.fail("/hello", "Request failed");
    }

    Board.time.sleepMs(1000);

    // Test 2: GET /ping
    log.info("", .{});
    log.info("Testing GET /palgear/v1/ping...", .{});

    const timestamp = Board.time.getTimeMs();
    const ping_url = std.fmt.bufPrint(&url_buf, "https://{s}/palgear/v1/ping?timestamp={d}", .{ env.api_host, timestamp }) catch {
        results.fail("/ping", "URL format error");
        return;
    };

    const start2 = Board.time.getTimeMs();
    if (client.get(ping_url, &response_buf)) |resp| {
        const duration = Board.time.getTimeMs() - start2;
        log.info("  Status: {d} ({s}) in {d}ms", .{ resp.status_code, resp.statusText(), duration });
        if (resp.isSuccess()) {
            log.info("  Body: {s}", .{resp.body()[0..@min(200, resp.body().len)]});
            results.pass("/ping");
        } else {
            results.fail("/ping", "Non-2xx status");
        }
    } else |err| {
        log.err("  Error: {}", .{err});
        results.fail("/ping", "Request failed");
    }

    Board.time.sleepMs(1000);

    // ========================================================================
    // Phase 2: @refresh with HMAC signature
    // ========================================================================

    var token_buf: [1024]u8 = undefined;
    var token: []const u8 = "";

    if (env.device_sn.len > 0 and env.device_eid.len > 0) {
        log.info("", .{});
        log.info("Testing POST /palgear/v1/@refresh (with HMAC signature)...", .{});

        // Build request body with real timestamp (boot time + ntp offset)
        var body_buf: [256]u8 = undefined;
        const req_time = @as(i64, @intCast(Board.time.getTimeMs())) + ntp_offset;
        const body = std.fmt.bufPrint(&body_buf,
            \\{{"sn":"{s}","reqMilliSec":{d}}}
        , .{ env.device_sn, req_time }) catch {
            results.fail("/@refresh", "Body format error");
            return;
        };

        // Compute HMAC-SHA256 signature using EID as key
        var sig_buf: [64]u8 = undefined;
        const signature = computeSignature(env.device_eid, body, &sig_buf);
        log.info("  Body: {s}", .{body});
        log.info("  Signature: {s}", .{signature});

        // Make request with custom headers
        if (httpsPostWithSignature(env.api_host, "/palgear/v1/@refresh", body, signature, dns_server, &response_buf)) |resp_body| {
            if (std.mem.indexOf(u8, resp_body, "\"token\"")) |_| {
                // Extract token from response
                if (extractJsonString(resp_body, "token", &token_buf)) |t| {
                    token = t;
                    log.info("  Got token: {s}...", .{token[0..@min(50, token.len)]});
                    results.pass("/@refresh");
                } else {
                    log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
                    results.fail("/@refresh", "Failed to extract token");
                }
            } else {
                log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
                results.fail("/@refresh", "No token in response");
            }
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/@refresh", "Request failed");
        }
    } else {
        results.skip("/@refresh", "No device_sn or device_eid configured");
    }

    Board.time.sleepMs(1000);

    // ========================================================================
    // Phase 3: Authenticated requests (Bearer token)
    // ========================================================================

    if (token.len > 0) {
        // Test: GET /settings
        log.info("", .{});
        log.info("Testing GET /palgear/v1/settings (with auth)...", .{});
        if (httpsGetWithAuth(env.api_host, "/palgear/v1/settings", token, dns_server, &response_buf)) |resp_body| {
            log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
            results.pass("/settings");
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/settings", "Request failed");
        }

        Board.time.sleepMs(1000);

        // Test: GET /info
        log.info("", .{});
        log.info("Testing GET /palgear/v1/info (with auth)...", .{});
        if (httpsGetWithAuth(env.api_host, "/palgear/v1/info", token, dns_server, &response_buf)) |resp_body| {
            log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
            results.pass("/info");
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/info", "Request failed");
        }

        Board.time.sleepMs(1000);

        // Test: GET /chat-mode
        log.info("", .{});
        log.info("Testing GET /palgear/v1/chat-mode (with auth)...", .{});
        if (httpsGetWithAuth(env.api_host, "/palgear/v1/chat-mode", token, dns_server, &response_buf)) |resp_body| {
            log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
            results.pass("/chat-mode");
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/chat-mode", "Request failed");
        }

        Board.time.sleepMs(1000);

        // Test: GET /points
        log.info("", .{});
        log.info("Testing GET /palgear/v1/points (with auth)...", .{});
        if (httpsGetWithAuth(env.api_host, "/palgear/v1/points", token, dns_server, &response_buf)) |resp_body| {
            log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
            results.pass("/points");
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/points", "Request failed");
        }

        Board.time.sleepMs(1000);

        // Test: GET /firmwares/@latest
        log.info("", .{});
        log.info("Testing GET /palgear/v1/firmwares/@latest (with auth)...", .{});
        var fw_path_buf: [128]u8 = undefined;
        const fw_path = std.fmt.bufPrint(&fw_path_buf, "/palgear/v1/firmwares/@latest?sn={s}", .{env.device_sn}) catch "/palgear/v1/firmwares/@latest";
        if (httpsGetWithAuth(env.api_host, fw_path, token, dns_server, &response_buf)) |resp_body| {
            log.info("  Response: {s}", .{resp_body[0..@min(300, resp_body.len)]});
            results.pass("/firmwares/@latest");
        } else |err| {
            log.err("  Error: {}", .{err});
            results.fail("/firmwares/@latest", "Request failed");
        }
    } else {
        results.skip("/settings", "No token");
        results.skip("/info", "No token");
        results.skip("/chat-mode", "No token");
        results.skip("/points", "No token");
        results.skip("/firmwares/@latest", "No token");
    }

    log.info("", .{});
    log.info("All tests completed!", .{});
    debugStackUsage("TESTS_DONE", PSRAM_STACK_SIZE);
}

// ============================================================================
// Custom HTTPS helpers with headers
// ============================================================================

const HttpError = error{
    DnsLookupFailed,
    SocketCreateFailed,
    ConnectFailed,
    TlsInitFailed,
    TlsHandshakeFailed,
    SendFailed,
    RecvFailed,
    InvalidResponse,
    BufferTooSmall,
};

/// HTTPS POST with X-Body-Signature header (for @refresh)
fn httpsPostWithSignature(
    host: []const u8,
    path: []const u8,
    body: []const u8,
    signature: []const u8,
    dns_server: [4]u8,
    buf: []u8,
) HttpError![]const u8 {
    var request_buf: [1024]u8 = undefined;
    const request = std.fmt.bufPrint(&request_buf,
        "POST {s} HTTP/1.1\r\n" ++
            "Host: {s}\r\n" ++
            "Content-Type: application/json\r\n" ++
            "Content-Length: {d}\r\n" ++
            "X-Body-Signature: {s}\r\n" ++
            "User-Agent: ESP32-Zig/1.0\r\n" ++
            "Connection: close\r\n" ++
            "\r\n" ++
            "{s}",
        .{ path, host, body.len, signature, body },
    ) catch return HttpError.BufferTooSmall;

    return httpsRequest(host, request, dns_server, buf);
}

/// HTTPS GET with Authorization Bearer header
fn httpsGetWithAuth(
    host: []const u8,
    path: []const u8,
    token: []const u8,
    dns_server: [4]u8,
    buf: []u8,
) HttpError![]const u8 {
    var request_buf: [2048]u8 = undefined;
    const request = std.fmt.bufPrint(&request_buf,
        "GET {s} HTTP/1.1\r\n" ++
            "Host: {s}\r\n" ++
            "Authorization: Bearer {s}\r\n" ++
            "User-Agent: ESP32-Zig/1.0\r\n" ++
            "Connection: close\r\n" ++
            "\r\n",
        .{ path, host, token },
    ) catch return HttpError.BufferTooSmall;

    return httpsRequest(host, request, dns_server, buf);
}

/// Low-level HTTPS request using TLS client
fn httpsRequest(host: []const u8, request: []const u8, dns_server: [4]u8, buf: []u8) HttpError![]const u8 {
    // DNS resolve
    var resolver = DnsResolver{
        .server = dns_server,
        .protocol = .udp,
        .timeout_ms = 5000,
    };

    const server_ip = resolver.resolve(host) catch {
        return HttpError.DnsLookupFailed;
    };

    // Create socket
    var sock = Board.socket.tcp() catch return HttpError.SocketCreateFailed;
    sock.setRecvTimeout(30000);
    sock.setSendTimeout(30000);

    // Connect
    sock.connect(server_ip, 443) catch {
        sock.close();
        return HttpError.ConnectFailed;
    };

    // TLS handshake
    var tls_client = TlsClient.init(&sock, .{
        .allocator = allocator,
        .hostname = host,
        .skip_verify = true, // Skip cert verify for testing
        .timeout_ms = 30000,
    }) catch {
        sock.close();
        return HttpError.TlsInitFailed;
    };

    tls_client.connect() catch {
        tls_client.deinit();
        return HttpError.TlsHandshakeFailed;
    };

    // Send request
    _ = tls_client.send(request) catch {
        tls_client.deinit();
        return HttpError.SendFailed;
    };

    // Receive response
    var total: usize = 0;
    while (total < buf.len) {
        const n = tls_client.recv(buf[total..]) catch |err| {
            if (err == error.EndOfStream) break;
            break;
        };
        if (n == 0) break;
        total += n;
    }

    tls_client.deinit();

    if (total == 0) return HttpError.RecvFailed;

    // Find body (after \r\n\r\n)
    const response = buf[0..total];
    if (std.mem.indexOf(u8, response, "\r\n\r\n")) |pos| {
        return response[pos + 4 ..];
    }

    return HttpError.InvalidResponse;
}

// ============================================================================
// JSON helpers
// ============================================================================

/// Extract a string value from JSON (simple parser)
fn extractJsonString(json: []const u8, key: []const u8, out: []u8) ?[]const u8 {
    // Look for "key":"value"
    var search_buf: [64]u8 = undefined;
    const search = std.fmt.bufPrint(&search_buf, "\"{s}\":\"", .{key}) catch return null;

    if (std.mem.indexOf(u8, json, search)) |start| {
        const value_start = start + search.len;
        if (std.mem.indexOfPos(u8, json, value_start, "\"")) |end| {
            const value = json[value_start..end];
            if (value.len <= out.len) {
                @memcpy(out[0..value.len], value);
                return out[0..value.len];
            }
        }
    }
    return null;
}
