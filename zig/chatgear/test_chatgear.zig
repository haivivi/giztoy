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
