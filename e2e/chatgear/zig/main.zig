//! ChatGear E2E Test — Native macOS
//!
//! Connects to the stage MQTT broker, exercises the full chatgear protocol:
//! - TLS + MQTT connection
//! - Opus encode TTS audio → send uplink
//! - Receive downlink audio + commands
//! - State machine: recording, calling, interrupt, cancel
//!
//! Test audio is pre-generated TTS (MiniMax) stored as raw PCM (16kHz 16bit mono).
//! Opus encoding happens at runtime — same path as the real firmware.
//!
//! Usage:
//!   bazel run //e2e/chatgear/zig:test

const std = @import("std");
const std_impl = @import("std_impl");
const crypto_mod = @import("crypto");
const chatgear = @import("chatgear");
const mqtt0 = @import("mqtt0");
const tls = @import("tls");
const dns = @import("dns");
const opus = @import("opus");

const log = std.log.scoped(.e2e);

// ============================================================================
// Platform Types
// ============================================================================

const Socket = std_impl.Socket;
const Crypto = crypto_mod;

/// Runtime satisfying chatgear trait requirements:
/// time (sleepMs + getTimeMs), sync (Mutex + Condition), spawner (Thread).
const Rt = struct {
    pub const Mutex = std_impl.runtime.Mutex;
    pub const Condition = std_impl.runtime.Condition;
    pub const Thread = std_impl.runtime.Thread;
    pub fn sleepMs(ms: u32) void {
        std_impl.time.sleepMs(ms);
    }
    pub fn getTimeMs() u64 {
        return std_impl.time.nowMs();
    }
};

const TlsClient = tls.Client(Socket, Crypto, Rt);
const MqttClient = mqtt0.Client(TlsClient, Rt);
const Conn = chatgear.MqttClientConn(MqttClient);
const Port = chatgear.ClientPort(MqttClient, Rt);

const alloc = std.heap.page_allocator;

// ============================================================================
// Stage Environment
// ============================================================================

const MQTT_HOST = "mqtt.stage.haivivi.cn";
const MQTT_PORT: u16 = 8883;
const MQTT_USER = "admin";
const MQTT_PASS = "isA953Nx56EBfEu";
const GEAR_ID = "693b0fb7839769199432f516";
const SCOPE = "RyBFG6/";

// ============================================================================
// Audio Config
// ============================================================================

const SAMPLE_RATE: u32 = 16000;
const FRAME_MS: u32 = 20;
const FRAME_SAMPLES: u32 = SAMPLE_RATE * FRAME_MS / 1000; // 320
const MAX_OPUS_OUT: usize = 512;

// ============================================================================
// Test Audio (PCM: 16kHz 16bit mono, from MiniMax TTS)
// ============================================================================

const testdata = @import("testdata.zig");

// ============================================================================
// Downlink State
// ============================================================================

var g_downlink_audio_frames: std.atomic.Value(u32) = std.atomic.Value(u32).init(0);
var g_downlink_commands: std.atomic.Value(u32) = std.atomic.Value(u32).init(0);
var g_streaming_on: std.atomic.Value(bool) = std.atomic.Value(bool).init(false);

var g_port: ?*Port = null;
var g_decoder: ?*opus.Decoder = null;

// Downlink PCM accumulation buffer (max ~60s at 16kHz = 960K samples)
const MAX_DL_SAMPLES = 960_000;
var g_dl_pcm: [MAX_DL_SAMPLES]i16 = undefined;
var g_dl_pcm_len: std.atomic.Value(u32) = std.atomic.Value(u32).init(0);
var g_dl_mutex: std.Thread.Mutex = .{};

fn onDownlinkCommand(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    _ = g_downlink_commands.fetchAdd(1, .monotonic);
    if (g_port) |port| {
        var evt: chatgear.CommandEvent = undefined;
        chatgear.parseCommandEvent(msg.payload, &evt) catch |err| {
            log.err("[rx] command parse: {}", .{err});
            return;
        };
        log.info("[rx] command: {s}", .{evt.cmd_type.toString()});
        if (evt.payload == .streaming) {
            g_streaming_on.store(evt.payload.streaming, .monotonic);
        }
        port.pushCommand(evt);
    }
}

fn onDownlinkAudio(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    _ = g_downlink_audio_frames.fetchAdd(1, .monotonic);
    if (g_port) |port| {
        const f = chatgear.unstampFrame(msg.payload) catch return;
        port.pushDownlinkAudio(f);

        // Decode opus → PCM and accumulate
        if (g_decoder) |dec| {
            var pcm_buf: [FRAME_SAMPLES * 2]i16 = undefined;
            const decoded = dec.decode(f.frame(), &pcm_buf, false) catch return;
            if (decoded.len > 0) {
                g_dl_mutex.lock();
                defer g_dl_mutex.unlock();
                const pos = g_dl_pcm_len.load(.monotonic);
                const room = MAX_DL_SAMPLES - pos;
                const n: u32 = @intCast(@min(decoded.len, room));
                @memcpy(g_dl_pcm[pos..][0..n], decoded[0..n]);
                g_dl_pcm_len.store(pos + n, .monotonic);
            }
        }
    }
}

// ============================================================================
// Opus Encode + Send
// ============================================================================

fn sendPcmAsOpus(port: *Port, encoder: *opus.Encoder, pcm_data: anytype) !u32 {
    var opus_buf: [MAX_OPUS_OUT]u8 = undefined;
    var offset: usize = 0;
    var frame_count: u32 = 0;

    while (offset + FRAME_SAMPLES <= pcm_data.len) {
        const frame_pcm = pcm_data[offset..][0..FRAME_SAMPLES];
        const encoded = try encoder.encode(frame_pcm, FRAME_SAMPLES, &opus_buf);

        const ts: i64 = @intCast(std_impl.time.nowMs());
        port.sendOpusFrame(ts, encoded);
        frame_count += 1;
        offset += FRAME_SAMPLES;

        // Pace at real-time (20ms per frame)
        std_impl.time.sleepMs(FRAME_MS);
    }
    return frame_count;
}

// ============================================================================
// Connection Setup
// ============================================================================

fn connect() !struct { port: *Port, conn: *Conn, client: *MqttClient, encoder: *opus.Encoder } {
    // DNS
    log.info("Resolving {s}...", .{MQTT_HOST});
    const DnsResolver = dns.Resolver(Socket);
    var resolver = DnsResolver{ .server = .{ 8, 8, 8, 8 }, .protocol = .udp };
    const ip = try resolver.resolve(MQTT_HOST);
    log.info("Resolved -> {d}.{d}.{d}.{d}", .{ ip[0], ip[1], ip[2], ip[3] });

    // TCP
    const sock = try alloc.create(Socket);
    sock.* = try Socket.tcp();
    sock.setRecvTimeout(30000);
    try sock.connect(ip, MQTT_PORT);
    log.info("TCP connected", .{});

    // TLS
    log.info("TLS handshake...", .{});
    const tls_c = try alloc.create(TlsClient);
    tls_c.* = try TlsClient.init(sock, .{
        .hostname = MQTT_HOST,
        .allocator = alloc,
        .skip_verify = true,
        .timeout_ms = 30000,
    });
    try tls_c.connect();
    log.info("TLS connected!", .{});

    // MQTT
    log.info("MQTT connecting...", .{});
    const mux = try alloc.create(mqtt0.Mux(Rt));
    mux.* = try mqtt0.Mux(Rt).init(alloc);
    const client = try alloc.create(MqttClient);
    client.* = try MqttClient.init(tls_c, mux, .{
        .client_id = GEAR_ID,
        .username = MQTT_USER,
        .password = MQTT_PASS,
        .keep_alive = 30,
        .protocol_version = .v5,
        .allocator = alloc,
    });
    log.info("MQTT connected!", .{});

    // ChatGear
    const conn = try alloc.create(Conn);
    conn.* = Conn.init(client, .{ .scope = SCOPE, .gear_id = GEAR_ID });
    try mux.handleFn(conn.commandTopic(), onDownlinkCommand);
    try mux.handleFn(conn.outputAudioTopic(), onDownlinkAudio);
    try conn.subscribe();
    log.info("Subscribed to downlink topics", .{});

    // Port
    const port = try alloc.create(Port);
    port.* = Port.init(conn, 0);
    g_port = port;
    port.beginBatch();
    port.setVolume(50);
    port.setBattery(100, false);
    port.setSystemVersion("zig-e2e-native-0.1");
    port.endBatch();
    try port.startPeriodicReporting();
    port.setState(.ready);
    log.info("ChatGear ready! Gear ID: {s}, Scope: {s}", .{ GEAR_ID, SCOPE });

    // MQTT background
    const rx = try std.Thread.spawn(.{}, struct {
        fn run(c: *MqttClient) void {
            c.readLoop() catch |err| log.err("MQTT readLoop: {}", .{err});
        }
    }.run, .{client});
    rx.detach();

    const ka = try std.Thread.spawn(.{}, struct {
        fn run(c: *MqttClient) void {
            while (true) {
                std_impl.time.sleepMs(10000);
                c.ping() catch |err| {
                    log.err("MQTT ping: {}", .{err});
                    return;
                };
                log.info("MQTT ping OK", .{});
            }
        }
    }.run, .{client});
    ka.detach();

    // Opus encoder
    const encoder = try alloc.create(opus.Encoder);
    encoder.* = try opus.Encoder.init(alloc, SAMPLE_RATE, 1, .voip);
    try encoder.setBitrate(24000);
    try encoder.setComplexity(5);
    try encoder.setSignal(.voice);

    // Opus decoder (for downlink audio verification)
    const decoder = try alloc.create(opus.Decoder);
    decoder.* = try opus.Decoder.init(alloc, SAMPLE_RATE, 1);
    g_decoder = decoder;

    return .{ .port = port, .conn = conn, .client = client, .encoder = encoder };
}

// ============================================================================
// Wait Helpers
// ============================================================================

fn waitForStreaming(timeout_s: u32) bool {
    var waited: u32 = 0;
    while (waited < timeout_s * 1000) : (waited += 100) {
        if (g_streaming_on.load(.monotonic)) return true;
        std_impl.time.sleepMs(100);
    }
    return false;
}

fn waitForStreamingOff(timeout_s: u32) bool {
    var waited: u32 = 0;
    while (waited < timeout_s * 1000) : (waited += 100) {
        if (!g_streaming_on.load(.monotonic)) return true;
        std_impl.time.sleepMs(100);
    }
    return false;
}

fn resetCounters() void {
    g_downlink_audio_frames.store(0, .monotonic);
    g_downlink_commands.store(0, .monotonic);
    g_streaming_on.store(false, .monotonic);
    g_dl_pcm_len.store(0, .monotonic);
}

/// Save accumulated downlink PCM to file and return sample count.
fn saveDlPcm(filename: []const u8) u32 {
    g_dl_mutex.lock();
    defer g_dl_mutex.unlock();
    const n = g_dl_pcm_len.load(.monotonic);
    if (n == 0) {
        log.info("No downlink audio to save", .{});
        return 0;
    }
    const bytes = std.mem.sliceAsBytes(g_dl_pcm[0..n]);
    const file = std.fs.cwd().createFile(filename, .{}) catch |err| {
        log.err("Failed to create {s}: {}", .{ filename, err });
        return n;
    };
    defer file.close();
    file.writeAll(bytes) catch |err| {
        log.err("Failed to write {s}: {}", .{ filename, err });
        return n;
    };
    log.info("Saved {d} samples ({d:.1}s) to {s}", .{ n, @as(f32, @floatFromInt(n)) / 16000.0, filename });
    return n;
}

// ============================================================================
// Test Cases
// ============================================================================

fn testBasicRecording(port: *Port, encoder: *opus.Encoder) !void {
    log.info("", .{});
    log.info("========== TEST 1: Basic Recording ==========", .{});
    resetCounters();

    // Press: ready → recording
    log.info("setState(recording) — sending hello.pcm...", .{});
    port.setState(.recording);
    const frames = try sendPcmAsOpus(port, encoder, &testdata.hello);
    log.info("Sent {d} opus frames ({d}ms)", .{ frames, frames * FRAME_MS });

    // Release: recording → waiting_for_response
    port.setState(.waiting_for_response);
    log.info("setState(waiting_for_response) — waiting for server...", .{});

    // Wait for streaming command
    if (waitForStreaming(15)) {
        log.info("Got streaming ON! Receiving audio...", .{});
        // Wait for streaming to finish
        if (waitForStreamingOff(30)) {
            log.info("Streaming OFF. Received {d} audio frames, {d} commands", .{
                g_downlink_audio_frames.load(.monotonic),
                g_downlink_commands.load(.monotonic),
            });
        } else {
            log.warn("Timeout waiting for streaming OFF", .{});
        }
    } else {
        log.warn("Timeout waiting for streaming — server may not be running", .{});
    }

    port.setState(.ready);
    _ = saveDlPcm("/tmp/chatgear_test1_reply.pcm");
    log.info("TEST 1 DONE: audio_frames={d} commands={d}", .{
        g_downlink_audio_frames.load(.monotonic),
        g_downlink_commands.load(.monotonic),
    });
    std_impl.time.sleepMs(2000);
}

fn testRecordingInterrupt(port: *Port, encoder: *opus.Encoder) !void {
    log.info("", .{});
    log.info("========== TEST 2: Recording Interrupt ==========", .{});
    resetCounters();

    // First recording
    port.setState(.recording);
    log.info("First recording: hello.pcm...", .{});
    _ = try sendPcmAsOpus(port, encoder, &testdata.hello);
    port.setState(.waiting_for_response);
    log.info("Waiting for response...", .{});

    // Wait for streaming to start
    if (waitForStreaming(15)) {
        log.info("Got streaming — interrupting with second recording!", .{});
        std_impl.time.sleepMs(500); // Let some audio play

        // Interrupt: streaming → recording
        port.setState(.recording);
        log.info("INTERRUPT: sending weather.pcm...", .{});
        resetCounters();
        _ = try sendPcmAsOpus(port, encoder, &testdata.weather);
        port.setState(.waiting_for_response);

        // Wait for second response
        if (waitForStreaming(15)) {
            log.info("Got second streaming response!", .{});
            _ = waitForStreamingOff(30);
        } else {
            log.warn("Timeout on second response", .{});
        }
    } else {
        log.warn("Timeout waiting for first streaming", .{});
    }

    port.setState(.ready);
    _ = saveDlPcm("/tmp/chatgear_test2_reply.pcm");
    log.info("TEST 2 DONE: audio_frames={d} commands={d}", .{
        g_downlink_audio_frames.load(.monotonic),
        g_downlink_commands.load(.monotonic),
    });
    std_impl.time.sleepMs(2000);
}

fn testCallingMode(port: *Port, encoder: *opus.Encoder) !void {
    log.info("", .{});
    log.info("========== TEST 3: Calling Mode ==========", .{});
    resetCounters();

    port.setState(.calling);
    log.info("setState(calling) — continuous audio...", .{});

    // Send hello
    log.info("Sending hello.pcm...", .{});
    _ = try sendPcmAsOpus(port, encoder, &testdata.hello);

    // 1 second silence gap (send silent frames)
    log.info("1s silence gap...", .{});
    var silent_frame: [FRAME_SAMPLES]i16 = [_]i16{0} ** FRAME_SAMPLES;
    var opus_buf: [MAX_OPUS_OUT]u8 = undefined;
    var i: u32 = 0;
    while (i < 1000 / FRAME_MS) : (i += 1) {
        const encoded = try encoder.encode(&silent_frame, FRAME_SAMPLES, &opus_buf);
        const ts: i64 = @intCast(std_impl.time.nowMs());
        port.sendOpusFrame(ts, encoded);
        std_impl.time.sleepMs(FRAME_MS);
    }

    // Send weather
    log.info("Sending weather.pcm...", .{});
    _ = try sendPcmAsOpus(port, encoder, &testdata.weather);

    // Wait for any responses
    log.info("Waiting 10s for responses...", .{});
    std_impl.time.sleepMs(10000);

    port.setState(.ready);
    _ = saveDlPcm("/tmp/chatgear_test3_reply.pcm");
    log.info("TEST 3 DONE: audio_frames={d} commands={d}", .{
        g_downlink_audio_frames.load(.monotonic),
        g_downlink_commands.load(.monotonic),
    });
    std_impl.time.sleepMs(2000);
}

fn testCancel(port: *Port, encoder: *opus.Encoder) !void {
    log.info("", .{});
    log.info("========== TEST 4: Cancel ==========", .{});
    resetCounters();

    port.setState(.recording);
    log.info("setState(recording) — sending 5 frames then cancel...", .{});

    // Send just 5 frames (~100ms)
    const samples: []const i16 = &testdata.hello;
    var opus_buf: [MAX_OPUS_OUT]u8 = undefined;
    var sent: u32 = 0;
    var offset: usize = 0;
    while (sent < 5 and offset + FRAME_SAMPLES <= samples.len) {
        const frame = samples[offset..][0..FRAME_SAMPLES];
        const encoded = try encoder.encode(frame, FRAME_SAMPLES, &opus_buf);
        const ts: i64 = @intCast(std_impl.time.nowMs());
        port.sendOpusFrame(ts, encoded);
        offset += FRAME_SAMPLES;
        sent += 1;
        std_impl.time.sleepMs(FRAME_MS);
    }

    // Cancel immediately
    port.setState(.ready);
    log.info("CANCELLED after {d} frames. Waiting 5s...", .{sent});
    std_impl.time.sleepMs(5000);

    log.info("TEST 4 DONE: audio_frames={d} commands={d} (expect 0)", .{
        g_downlink_audio_frames.load(.monotonic),
        g_downlink_commands.load(.monotonic),
    });
}

// ============================================================================
// Main
// ============================================================================

pub fn main() !void {
    log.info("==========================================", .{});
    log.info("  ChatGear E2E Test (Native macOS)", .{});
    log.info("  MQTT:    mqtts://{s}:{d}", .{ MQTT_HOST, MQTT_PORT });
    log.info("  Gear ID: {s}", .{GEAR_ID});
    log.info("  Scope:   {s}", .{SCOPE});
    log.info("==========================================", .{});

    const ctx = connect() catch |err| {
        log.err("Connection failed: {}", .{err});
        return;
    };

    // Run all test cases
    testBasicRecording(ctx.port, ctx.encoder) catch |err| {
        log.err("TEST 1 failed: {}", .{err});
    };
    testRecordingInterrupt(ctx.port, ctx.encoder) catch |err| {
        log.err("TEST 2 failed: {}", .{err});
    };
    testCallingMode(ctx.port, ctx.encoder) catch |err| {
        log.err("TEST 3 failed: {}", .{err});
    };
    testCancel(ctx.port, ctx.encoder) catch |err| {
        log.err("TEST 4 failed: {}", .{err});
    };

    log.info("", .{});
    log.info("========== ALL TESTS COMPLETE ==========", .{});
    log.info("Protocol validation done. Check logs above for results.", .{});

    // Keep alive a bit for any trailing responses
    std_impl.time.sleepMs(5000);
    ctx.port.close();
}
