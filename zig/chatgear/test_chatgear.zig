//! chatgear Integration Test — Zig broker + chatgear loopback
//!
//! Tests the chatgear protocol end-to-end:
//!   mqtt0.Broker (localhost, handler captures messages)
//!     <-- TCP --> device (chatgear.Conn publishes state/stats/audio)
//!
//! The broker's mux handler captures uplink messages directly (same
//! pattern as mqtt0/test_mqtt0.zig). No separate "server client" needed.

const std = @import("std");
const posix = std.posix;
const mqtt0 = @import("mqtt0");
const chatgear = @import("chatgear");

// ============================================================================
// TestRt — Runtime for host tests (wraps std.Thread primitives)
// ============================================================================

const TestRt = struct {
    pub const Mutex = struct {
        inner: std.Thread.Mutex = .{},
        pub fn init() @This() {
            return .{ .inner = .{} };
        }
        pub fn deinit(_: *@This()) void {}
        pub fn lock(self: *@This()) void {
            self.inner.lock();
        }
        pub fn unlock(self: *@This()) void {
            self.inner.unlock();
        }
    };

    pub const Condition = struct {
        inner: std.Thread.Condition = .{},
        pub fn init() @This() {
            return .{ .inner = .{} };
        }
        pub fn deinit(_: *@This()) void {}
        pub fn wait(self: *@This(), mutex: *Mutex) void {
            self.inner.wait(&mutex.inner);
        }
        pub fn signal(self: *@This()) void {
            self.inner.signal();
        }
        pub fn broadcast(self: *@This()) void {
            self.inner.broadcast();
        }
    };

    pub fn sleepMs(ms: u32) void {
        std.Thread.sleep(@as(u64, ms) * std.time.ns_per_ms);
    }

    pub fn getTimeMs() u64 {
        return @intCast(std.time.milliTimestamp());
    }

    pub const Options = struct {
        stack_size: u32 = 8192,
    };

    pub fn spawn(_: [:0]const u8, func: *const fn (?*anyopaque) void, ctx: ?*anyopaque, _: Options) !void {
        const t = try std.Thread.spawn(.{}, struct {
            fn run(f: *const fn (?*anyopaque) void, c: ?*anyopaque) void {
                f(c);
            }
        }.run, .{ func, ctx });
        t.detach();
    }
};

// ============================================================================
// TcpSocket — Transport wrapper
// ============================================================================

const TcpSocket = struct {
    fd: posix.socket_t,

    fn initServer(port: u16) !struct { listener: posix.socket_t, port: u16 } {
        const fd = try posix.socket(posix.AF.INET, posix.SOCK.STREAM, 0);
        errdefer posix.close(fd);
        const enable: u32 = 1;
        try posix.setsockopt(fd, posix.SOL.SOCKET, posix.SO.REUSEADDR, std.mem.asBytes(&enable));
        const addr = posix.sockaddr.in{
            .family = posix.AF.INET,
            .port = std.mem.nativeToBig(u16, port),
            .addr = 0,
        };
        try posix.bind(fd, @ptrCast(&addr), @sizeOf(@TypeOf(addr)));
        try posix.listen(fd, 5);
        var bound_addr: posix.sockaddr.in = undefined;
        var addr_len: posix.socklen_t = @sizeOf(posix.sockaddr.in);
        try posix.getsockname(fd, @ptrCast(&bound_addr), &addr_len);
        return .{ .listener = fd, .port = std.mem.bigToNative(u16, bound_addr.port) };
    }

    fn accept(listener: posix.socket_t) !TcpSocket {
        const fd = try posix.accept(listener, null, null, 0);
        return .{ .fd = fd };
    }

    fn connect(port: u16) !TcpSocket {
        const fd = try posix.socket(posix.AF.INET, posix.SOCK.STREAM, 0);
        errdefer posix.close(fd);
        const addr = posix.sockaddr.in{
            .family = posix.AF.INET,
            .port = std.mem.nativeToBig(u16, port),
            .addr = std.mem.nativeToBig(u32, 0x7F000001),
        };
        try posix.connect(fd, @ptrCast(&addr), @sizeOf(@TypeOf(addr)));
        return .{ .fd = fd };
    }

    pub fn send(self: *TcpSocket, data: []const u8) !usize {
        return posix.send(self.fd, data, 0) catch |err| switch (err) {
            error.BrokenPipe, error.ConnectionResetByPeer => return error.ConnectionClosed,
            else => return error.SendFailed,
        };
    }

    pub fn recv(self: *TcpSocket, buf: []u8) !usize {
        const n = posix.recv(self.fd, buf, 0) catch |err| switch (err) {
            error.ConnectionResetByPeer => return error.ConnectionClosed,
            else => return error.RecvFailed,
        };
        if (n == 0) return error.ConnectionClosed;
        return n;
    }

    fn close(self: *TcpSocket) void {
        posix.close(self.fd);
    }
};

// ============================================================================
// Type aliases
// ============================================================================

const MqttClient = mqtt0.Client(TcpSocket, TestRt);
const Conn = chatgear.MqttClientConn(MqttClient);
const Broker = mqtt0.Broker(TcpSocket, TestRt);

// ============================================================================
// Capture state — broker mux handlers write here
// ============================================================================

const Capture = struct {
    state_received: bool = false,
    state_buf: [512]u8 = undefined,
    state_len: usize = 0,

    stats_received: bool = false,
    stats_buf: [1024]u8 = undefined,
    stats_len: usize = 0,

    audio_received: bool = false,
    audio_buf: [2048]u8 = undefined,
    audio_len: usize = 0,
};

var g_cap = Capture{};

fn onState(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    const n = @min(msg.payload.len, 512);
    @memcpy(g_cap.state_buf[0..n], msg.payload[0..n]);
    g_cap.state_len = n;
    @atomicStore(bool, &g_cap.state_received, true, .release);
}

fn onStats(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    const n = @min(msg.payload.len, 1024);
    @memcpy(g_cap.stats_buf[0..n], msg.payload[0..n]);
    g_cap.stats_len = n;
    @atomicStore(bool, &g_cap.stats_received, true, .release);
}

fn onAudio(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    const n = @min(msg.payload.len, 2048);
    @memcpy(g_cap.audio_buf[0..n], msg.payload[0..n]);
    g_cap.audio_len = n;
    @atomicStore(bool, &g_cap.audio_received, true, .release);
}

fn waitFlag(flag: *bool, timeout_ms: u32) !void {
    var elapsed: u32 = 0;
    while (elapsed < timeout_ms) {
        if (@atomicLoad(bool, flag, .acquire)) return;
        std.Thread.sleep(10 * std.time.ns_per_ms);
        elapsed += 10;
    }
    return error.Timeout;
}

fn serveOne(broker: *Broker, conn: *TcpSocket) void {
    broker.serveConn(conn);
}

// ============================================================================
// Test: sendState -> broker handler receives state JSON
// ============================================================================

test "sendState -> broker handler receives state JSON" {
    const allocator = std.heap.page_allocator;
    g_cap = Capture{};

    // Broker with handler on uplink state topic
    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-001/state", onState);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    // Device client connects
    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-1",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    // Chatgear conn: send state
    var conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-001" });
    const now: i64 = @intCast(std.time.milliTimestamp());
    var evt = chatgear.StateEvent{ .version = 1, .time = now, .state = .recording, .update_at = now };
    try conn.sendState(&evt);

    // Verify broker handler captured the state
    try waitFlag(&g_cap.state_received, 2000);
    const json = g_cap.state_buf[0..g_cap.state_len];
    try std.testing.expect(std.mem.indexOf(u8, json, "\"recording\"") != null);
    try std.testing.expect(std.mem.indexOf(u8, json, "\"v\":1") != null);
    try std.testing.expect(std.mem.indexOf(u8, json, "\"s\":") != null);

    // Cleanup
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

// ============================================================================
// Test: sendStats -> broker handler receives stats JSON
// ============================================================================

test "sendStats -> broker handler receives stats JSON" {
    const allocator = std.heap.page_allocator;
    g_cap = Capture{};

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-001/stats", onStats);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-2",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-001" });
    const now: i64 = @intCast(std.time.milliTimestamp());
    var evt = chatgear.StatsEvent{ .time = now, .volume = .{ .percentage = 75, .update_at = now } };
    try conn.sendStats(&evt);

    try waitFlag(&g_cap.stats_received, 2000);
    const json = g_cap.stats_buf[0..g_cap.stats_len];
    try std.testing.expect(std.mem.indexOf(u8, json, "\"volume\"") != null);
    try std.testing.expect(std.mem.indexOf(u8, json, "\"percentage\":75") != null);

    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

// ============================================================================
// Test: sendOpusFrame -> broker handler receives and unstamps
// ============================================================================

test "sendOpusFrame -> broker handler receives and unstamps" {
    const allocator = std.heap.page_allocator;
    g_cap = Capture{};

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-001/input_audio_stream", onAudio);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-3",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-001" });
    const timestamp: i64 = 1706745600000;
    const fake_opus = [_]u8{ 0x48, 0x61, 0x69, 0x56, 0x69, 0x56, 0x69 };
    try conn.sendOpusFrame(timestamp, &fake_opus);

    try waitFlag(&g_cap.audio_received, 2000);
    const raw = g_cap.audio_buf[0..g_cap.audio_len];
    const unstamped = try chatgear.unstampFrame(raw);
    try std.testing.expectEqual(timestamp, unstamped.timestamp_ms);
    try std.testing.expect(std.mem.eql(u8, unstamped.frame, &fake_opus));

    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

// ============================================================================
// Pure wire-format tests (no broker needed)
// ============================================================================

test "command wire format parses correctly" {
    const cmd_json =
        \\{"type":"set_volume","time":123,"pld":50,"issue_at":456}
    ;
    var evt: chatgear.CommandEvent = undefined;
    try chatgear.parseCommandEvent(cmd_json, &evt);
    try std.testing.expectEqual(chatgear.CommandType.set_volume, evt.cmd_type);
    try std.testing.expectEqual(@as(i32, 50), evt.payload.set_volume);
}

test "stamped audio frame roundtrips" {
    const timestamp: i64 = 1706745600000;
    const fake_opus = [_]u8{ 0xDE, 0xAD, 0xBE, 0xEF };
    var stamp_buf: [chatgear.HEADER_SIZE + 4]u8 = undefined;
    const stamped_len = try chatgear.stampFrame(timestamp, &fake_opus, &stamp_buf);
    const unstamped = try chatgear.unstampFrame(stamp_buf[0..stamped_len]);
    try std.testing.expectEqual(timestamp, unstamped.timestamp_ms);
    try std.testing.expect(std.mem.eql(u8, unstamped.frame, &fake_opus));
}

// ============================================================================
// Full loopback: Client Conn -> Broker -> ServerPort
// ============================================================================

// Server port handler context: wraps a ServerPort pointer for use as a
// broker mux handler fn (which only gets clientID + Message).
var g_server_port: ?*chatgear.ServerPort(Broker, TestRt) = null;

fn serverOnState(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_server_port) |sp| sp.handleStatePayload(msg.payload);
}

fn serverOnStats(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_server_port) |sp| sp.handleStatsPayload(msg.payload);
}

fn serverOnAudio(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_server_port) |sp| sp.handleAudioPayload(msg.payload);
}

const ServerConn = chatgear.MqttServerConn(Broker);
const SPort = chatgear.ServerPort(Broker, TestRt);

test "loopback: client sendState -> ServerPort.poll() receives state" {
    const allocator = std.heap.page_allocator;

    // Broker with server port handlers
    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-001/state", serverOnState);
    try broker_mux.handleFn("test/device/zig-001/stats", serverOnStats);
    try broker_mux.handleFn("test/device/zig-001/input_audio_stream", serverOnAudio);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    // Server conn + port
    var server_conn = ServerConn.init(&broker, .{ .scope = "test/", .gear_id = "zig-001" });
    var server_port = SPort.init(&server_conn);
    g_server_port = &server_port;

    // Device client connects
    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-loop-1",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    // Client sends state
    var conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-001" });
    const now: i64 = @intCast(std.time.milliTimestamp());
    var evt = chatgear.StateEvent{ .version = 1, .time = now, .state = .recording, .update_at = now };
    try conn.sendState(&evt);

    // ServerPort should receive it via poll()
    std.Thread.sleep(50 * std.time.ns_per_ms);
    const data = server_port.poll();
    try std.testing.expect(data != null);
    try std.testing.expectEqual(chatgear.UplinkTag.state, std.meta.activeTag(data.?));
    try std.testing.expectEqual(chatgear.State.recording, data.?.state.state);

    // Verify cached state
    const cached = server_port.getState();
    try std.testing.expect(cached != null);
    try std.testing.expectEqual(chatgear.State.recording, cached.?.state);

    // Cleanup
    g_server_port = null;
    server_port.close();
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

test "loopback: client sendStats -> ServerPort.poll() receives stats" {
    const allocator = std.heap.page_allocator;

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-002/state", serverOnState);
    try broker_mux.handleFn("test/device/zig-002/stats", serverOnStats);
    try broker_mux.handleFn("test/device/zig-002/input_audio_stream", serverOnAudio);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var server_conn = ServerConn.init(&broker, .{ .scope = "test/", .gear_id = "zig-002" });
    var server_port = SPort.init(&server_conn);
    g_server_port = &server_port;

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-loop-2",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var client_conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-002" });
    const now: i64 = @intCast(std.time.milliTimestamp());
    var stats_evt = chatgear.StatsEvent{ .time = now, .volume = .{ .percentage = 88, .update_at = now } };
    try client_conn.sendStats(&stats_evt);

    std.Thread.sleep(50 * std.time.ns_per_ms);
    const data = server_port.poll();
    try std.testing.expect(data != null);
    try std.testing.expectEqual(chatgear.UplinkTag.stats, std.meta.activeTag(data.?));
    try std.testing.expect(data.?.stats.volume != null);
    try std.testing.expectEqual(@as(f32, 88), data.?.stats.volume.?.percentage);

    // Verify cached stats
    const cached = server_port.getStats();
    try std.testing.expect(cached != null);
    try std.testing.expect(cached.?.volume != null);

    g_server_port = null;
    server_port.close();
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

test "loopback: client sendOpusFrame -> ServerPort.poll() receives audio" {
    const allocator = std.heap.page_allocator;

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-003/state", serverOnState);
    try broker_mux.handleFn("test/device/zig-003/stats", serverOnStats);
    try broker_mux.handleFn("test/device/zig-003/input_audio_stream", serverOnAudio);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var server_conn = ServerConn.init(&broker, .{ .scope = "test/", .gear_id = "zig-003" });
    var server_port = SPort.init(&server_conn);
    g_server_port = &server_port;

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-loop-3",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var client_conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-003" });
    const timestamp: i64 = 1706745600000;
    const fake_opus = [_]u8{ 0x48, 0x61, 0x69 };
    try client_conn.sendOpusFrame(timestamp, &fake_opus);

    std.Thread.sleep(50 * std.time.ns_per_ms);
    const data = server_port.poll();
    try std.testing.expect(data != null);
    try std.testing.expectEqual(chatgear.UplinkTag.audio, std.meta.activeTag(data.?));
    try std.testing.expectEqual(timestamp, data.?.audio.timestamp_ms);

    g_server_port = null;
    server_port.close();
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

test "loopback: ServerConn.issueCommand encodes correctly" {
    // Test that server conn encodes command JSON that the client can parse.
    // This is a wire-level roundtrip: encodeCommandEvent -> parseCommandEvent.
    const allocator = std.heap.page_allocator;

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});

    var server_conn = ServerConn.init(&broker, .{ .scope = "test/", .gear_id = "zig-004" });

    // Encode a command
    const now: i64 = @intCast(std.time.milliTimestamp());
    var cmd_evt = chatgear.CommandEvent{
        .cmd_type = .set_volume,
        .time = now,
        .payload = .{ .set_volume = 42 },
        .issue_at = now,
    };
    var buf: [chatgear.COMMAND_EVENT_JSON_SIZE]u8 = undefined;
    const written = try chatgear.encodeCommandEvent(&cmd_evt, &buf);
    const json = buf[0..written];

    // Parse it back (as the device client would)
    var parsed: chatgear.CommandEvent = undefined;
    try chatgear.parseCommandEvent(json, &parsed);

    try std.testing.expectEqual(chatgear.CommandType.set_volume, parsed.cmd_type);
    try std.testing.expectEqual(@as(i32, 42), parsed.payload.set_volume);

    // Also verify the conn can publish without error
    try server_conn.issueCommand(&cmd_evt);

    _ = &server_conn;
    broker.deinit();
    broker_mux.deinit();
}

// ============================================================================
// Full bidirectional: ClientPort ↔ Broker ↔ ServerPort
// ============================================================================

const CPort = chatgear.ClientPort(MqttClient, TestRt);

// Global client port pointer for device-side mux handlers
var g_client_port: ?*CPort = null;

fn deviceOnCommand(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_client_port) |cp| {
        var evt: chatgear.CommandEvent = undefined;
        chatgear.parseCommandEvent(msg.payload, &evt) catch return;
        cp.pushCommand(evt);
    }
}

fn deviceOnAudio(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_client_port) |cp| {
        const frame = chatgear.unstampFrame(msg.payload) catch return;
        cp.pushDownlinkAudio(frame);
    }
}

test "channel: spawn task sends, main thread receives" {
    const Ch = @import("channel").Channel(i32, 16, TestRt);
    var ch = Ch.init();
    defer ch.deinit();

    // Spawn a task that sends a value
    const Ctx = struct {
        chan: *Ch,
        fn run(ctx: ?*anyopaque) void {
            const self: *@This() = @ptrCast(@alignCast(ctx));
            self.chan.trySend(42) catch {};
        }
    };
    var ctx = Ctx{ .chan = &ch };
    try TestRt.spawn("test_ch", Ctx.run, @ptrCast(&ctx), .{});

    // Main thread receives
    const val = ch.recv();
    try std.testing.expect(val != null);
    try std.testing.expectEqual(@as(i32, 42), val.?);
}

test "spawn: mqtt publish from spawned task reaches broker" {
    const allocator = std.heap.page_allocator;
    g_cap = Capture{};

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-spawn/state", onState);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-spawn",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    // Spawn a task that publishes a message
    const Ctx = struct {
        client: *MqttClient,
        fn run(ctx_ptr: ?*anyopaque) void {
            const self: *@This() = @ptrCast(@alignCast(ctx_ptr));
            self.client.publish("test/device/zig-spawn/state", "{\"v\":1,\"t\":100,\"s\":\"ready\",\"ut\":100}") catch {};
        }
    };
    var ctx = Ctx{ .client = &device_client };
    try TestRt.spawn("test_pub", Ctx.run, @ptrCast(&ctx), .{});

    try waitFlag(&g_cap.state_received, 2000);
    const json = g_cap.state_buf[0..g_cap.state_len];
    try std.testing.expect(std.mem.indexOf(u8, json, "\"ready\"") != null);

    // Wait for detached spawn thread to fully return before cleanup.
    // Without this, the detached thread may still be in mqtt.publish()
    // touching stack/client memory when it gets reused by the next test.
    std.Thread.sleep(50 * std.time.ns_per_ms);

    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

test "CPort: spawn task reads from uplink_state channel" {
    // Test if a spawned task can recv from CPort's internal channel.
    const MockMqtt = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) !void {}
        pub fn subscribe(_: *@This(), _: []const []const u8) !void {}
    };
    const MockConn = chatgear.MqttClientConn(MockMqtt);
    const MockPort = chatgear.ClientPort(MockMqtt, TestRt);

    var mock_mqtt = MockMqtt{};
    var mock_conn = MockConn.init(&mock_mqtt, .{ .scope = "", .gear_id = "m" });
    var cp = MockPort.init(&mock_conn, 0);

    var got_it = std.atomic.Value(bool).init(false);

    const SpawnCtx = struct {
        port: *MockPort,
        flag: *std.atomic.Value(bool),
        fn run(ctx_ptr: ?*anyopaque) void {
            const self: *@This() = @ptrCast(@alignCast(ctx_ptr));
            if (self.port.uplink_state.recv()) |_| {
                self.flag.store(true, .release);
            }
        }
    };
    var ctx = SpawnCtx{ .port = &cp, .flag = &got_it };
    try TestRt.spawn("cp_recv", SpawnCtx.run, @ptrCast(&ctx), .{});

    std.Thread.sleep(10 * std.time.ns_per_ms);

    // Now push data — spawned task should wake up
    cp.setState(.recording);

    // Wait for spawned task to recv
    var elapsed: u32 = 0;
    while (elapsed < 2000) {
        if (got_it.load(.acquire)) break;
        std.Thread.sleep(10 * std.time.ns_per_ms);
        elapsed += 10;
    }
    try std.testing.expect(got_it.load(.acquire));

    cp.close();
}

test "CPort: spawn task reads channel + publishes directly to mqtt client" {
    const allocator = std.heap.page_allocator;
    g_cap = Capture{};

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-cp/state", onState);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-cp",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var device_conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-cp" });
    var cp = CPort.init(&device_conn, 0);

    // Spawn: recv from channel -> publish DIRECTLY via mqtt client (bypass Conn)
    var got_it = std.atomic.Value(bool).init(false);
    const SpawnCtx = struct {
        port: *CPort,
        client: *MqttClient,
        flag: *std.atomic.Value(bool),
        fn run(ctx_ptr: ?*anyopaque) void {
            const self: *@This() = @ptrCast(@alignCast(ctx_ptr));
            if (self.port.uplink_state.recv()) |_| {
                // Direct publish, bypass Conn.sendState entirely
                self.client.publish(
                    "test/device/zig-cp/state",
                    "{\"v\":1,\"t\":100,\"s\":\"recording\",\"ut\":100}",
                ) catch {};
                self.flag.store(true, .release);
            }
        }
    };
    var ctx = SpawnCtx{ .port = &cp, .client = &device_client, .flag = &got_it };
    try TestRt.spawn("cp_tx", SpawnCtx.run, @ptrCast(&ctx), .{});

    std.Thread.sleep(10 * std.time.ns_per_ms);

    cp.setState(.recording);

    // Wait for flag (confirms channel recv + publish completed)
    var elapsed: u32 = 0;
    while (elapsed < 3000) {
        if (got_it.load(.acquire)) break;
        std.Thread.sleep(10 * std.time.ns_per_ms);
        elapsed += 10;
    }
    try std.testing.expect(got_it.load(.acquire));

    // Wait for broker handler
    try waitFlag(&g_cap.state_received, 2000);
    const json = g_cap.state_buf[0..g_cap.state_len];
    try std.testing.expect(std.mem.indexOf(u8, json, "\"recording\"") != null);

    cp.close();
    std.Thread.sleep(50 * std.time.ns_per_ms);
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}

test "CPort: setState + uplink_state.recv works without mqtt" {
    // Test CPort's internal channel in isolation — no mqtt, no broker.
    // This isolates whether CPort.init() struct copy breaks the channel.
    const MockMqtt = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) !void {}
        pub fn subscribe(_: *@This(), _: []const []const u8) !void {}
    };
    const MockConn = chatgear.MqttClientConn(MockMqtt);
    const MockPort = chatgear.ClientPort(MockMqtt, TestRt);

    var mock_mqtt = MockMqtt{};
    var mock_conn = MockConn.init(&mock_mqtt, .{ .scope = "test/", .gear_id = "mock" });
    var cp = MockPort.init(&mock_conn, 0);
    defer cp.close();

    // setState puts a StateEvent into uplink_state channel
    cp.setState(.recording);

    // recv on the same thread should return immediately (data already in channel)
    const val = cp.uplink_state.tryRecv();
    try std.testing.expect(val != null);
    try std.testing.expectEqual(chatgear.State.recording, val.?.state);
}

// ============================================================================
// Continuous audio streaming test
// ============================================================================

test "loopback: continuous audio — 200 opus frames streamed through broker" {
    const allocator = std.heap.page_allocator;
    const FRAME_COUNT = 200;

    var broker_mux = try mqtt0.Mux(TestRt).init(allocator);
    try broker_mux.handleFn("test/device/zig-stream/state", serverOnState);
    try broker_mux.handleFn("test/device/zig-stream/stats", serverOnStats);
    try broker_mux.handleFn("test/device/zig-stream/input_audio_stream", serverOnAudio);
    var broker = try Broker.init(allocator, broker_mux.handler(), .{});
    const srv = try TcpSocket.initServer(0);

    var server_conn = ServerConn.init(&broker, .{ .scope = "test/", .gear_id = "zig-stream" });
    var server_port = SPort.init(&server_conn);
    g_server_port = &server_port;

    // Device client connects
    var device_sock = try TcpSocket.connect(srv.port);
    var device_conn_sock = try TcpSocket.accept(srv.listener);
    const serve_t = try std.Thread.spawn(.{}, serveOne, .{ &broker, &device_conn_sock });

    var device_mux = try mqtt0.Mux(TestRt).init(allocator);
    var device_client = try MqttClient.init(&device_sock, &device_mux, .{
        .client_id = "device-stream",
        .protocol_version = .v5,
        .allocator = allocator,
    });

    std.Thread.sleep(10 * std.time.ns_per_ms);

    var client_conn = Conn.init(&device_client, .{ .scope = "test/", .gear_id = "zig-stream" });

    // Send FRAME_COUNT opus frames (simulating 20ms * 200 = 4 seconds of audio)
    const base_ts: i64 = 1706745600000;
    var fake_opus: [80]u8 = undefined; // typical opus voice frame size
    for (&fake_opus, 0..) |*b, i| b.* = @truncate(i);

    var sent: usize = 0;
    while (sent < FRAME_COUNT) : (sent += 1) {
        const ts = base_ts + @as(i64, @intCast(sent)) * 20; // 20ms per frame
        try client_conn.sendOpusFrame(ts, &fake_opus);
    }

    // Receive all frames from ServerPort
    var received: usize = 0;
    var last_ts: i64 = 0;
    const deadline = std.time.milliTimestamp() + 5000; // 5s timeout
    while (received < FRAME_COUNT) {
        if (std.time.milliTimestamp() > deadline) break;
        if (server_port.poll()) |data| {
            switch (std.meta.activeTag(data)) {
                .audio => {
                    received += 1;
                    last_ts = data.audio.timestamp_ms;
                },
                else => {},
            }
        }
    }

    // Verify all frames received
    try std.testing.expectEqual(@as(usize, FRAME_COUNT), received);
    // Last timestamp should be base + (FRAME_COUNT-1)*20
    try std.testing.expectEqual(base_ts + (@as(i64, FRAME_COUNT) - 1) * 20, last_ts);

    // Cleanup
    g_server_port = null;
    server_port.close();
    device_client.deinit();
    device_sock.close();
    serve_t.join();
    device_conn_sock.close();
    posix.close(srv.listener);
    broker.deinit();
    broker_mux.deinit();
}
