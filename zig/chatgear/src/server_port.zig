//! Chatgear Server Port — Go-style async architecture
//!
//! Server-side port that mirrors go/pkg/chatgear/port_server.go:
//! - Receives uplink data (audio, state, stats) from the device via broker mux
//! - Sends downlink commands to the device via broker.publish()
//! - Caches latest state/stats for query
//! - Channel-based uplink queue for poll()
//!
//! Simplified compared to Go: no pcm.Mixer / Track system.
//! Audio downlink is raw opus frames via sendOpusFrame().
//!
//! Generic over BrokerType and Rt (runtime), so the same code
//! runs on ESP32 (RTOS tasks) and macOS (std.Thread).
//!
//! Usage:
//!   var server_conn = MqttServerConn(Broker).init(&broker, .{...});
//!   var server_port = ServerPort(Broker, Rt).init(&server_conn);
//!   try server_port.start(&broker_mux);  // registers handlers + spawns tx task
//!   while (server_port.poll()) |data| { ... }
//!   server_port.close();

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");
const server_conn_mod = @import("server_conn.zig");
const trait = @import("trait");
const channel = @import("channel");
const cancellation = @import("cancellation");

const CancellationToken = cancellation.CancellationToken;

// ============================================================================
// Uplink Data — what poll() returns
// ============================================================================

/// Tag for UplinkData union.
pub const UplinkTag = enum {
    audio,
    state,
    stats,
};

/// Uplink data received from the device. Returned by poll().
/// Mirrors Go's chatgear.UplinkData.
pub const UplinkData = union(UplinkTag) {
    audio: types.StampedFrame,
    state: types.StateEvent,
    stats: types.StatsEvent,
};

// ============================================================================
// Server Port
// ============================================================================

/// ServerPort — bidirectional port for server-side communication.
///
/// Mirrors go/pkg/chatgear/port_server.go with Go-style async:
/// - Uplink: broker mux handlers push into channels -> poll()
/// - Downlink: issueCommand() -> channel -> tx task -> broker.publish()
/// - Cached latest state/stats for query
///
/// BrokerType: mqtt0.Broker type with publish() method
/// Rt: runtime providing Mutex, Condition, spawn
pub fn ServerPort(comptime BrokerType: type, comptime Rt: type) type {
    // Validate Runtime at comptime
    comptime {
        _ = trait.sync.Mutex(Rt.Mutex);
        _ = trait.sync.Condition(Rt.Condition, Rt.Mutex);
        trait.spawner.from(Rt);
    }

    const Time = trait.time.from(Rt);
    const Conn = server_conn_mod.MqttServerConn(BrokerType);

    // Channel types
    const UplinkCh = channel.Channel(UplinkData, 256, Rt);
    const CommandCh = channel.Channel(types.CommandEvent, 32, Rt);

    return struct {
        const Self = @This();

        conn: *Conn,

        // Uplink queue (broker handlers -> poll)
        uplink: UplinkCh,

        // Downlink command queue (issueCommand -> tx task -> broker.publish)
        commands: CommandCh,

        // Lifecycle
        token: CancellationToken,

        // Cached state (protected by mutex)
        mutex: Rt.Mutex,
        cached_state: ?types.StateEvent,
        cached_stats: ?types.StatsEvent,

        /// Initialize a new ServerPort.
        pub fn init(connection: *Conn) Self {
            return Self{
                .conn = connection,
                .uplink = UplinkCh.init(),
                .commands = CommandCh.init(),
                .token = CancellationToken.init(),
                .mutex = Rt.Mutex.init(),
                .cached_state = null,
                .cached_stats = null,
            };
        }

        /// Release resources.
        pub fn deinit(self: *Self) void {
            self.uplink.deinit();
            self.commands.deinit();
            self.mutex.deinit();
        }

        /// Stack size for spawned tasks.
        const TASK_STACK_SIZE: u32 = 65536;
        const task_opts: Rt.Options = .{ .stack_size = TASK_STACK_SIZE };

        /// Start the server port. Spawns the command tx task.
        ///
        /// Uplink message handling is done via broker mux handlers
        /// that the caller must register. Use `handleState`, `handleStats`,
        /// `handleAudio` as the handler functions.
        pub fn start(self: *Self) !void {
            try Rt.spawn("cg_srv_tx_cmd", txCommandLoopFn, @ptrCast(self), task_opts);
        }

        /// Close the server port. Cancels background tasks and closes channels.
        pub fn close(self: *Self) void {
            self.token.cancel();
            self.uplink.close();
            self.commands.close();
        }

        // ====================================================================
        // Uplink: poll() — blocking receive from uplink queue
        // ====================================================================

        /// Poll for the next uplink data. Blocks until data is available
        /// or the port is closed (returns null).
        /// Mirrors Go ServerPort.Poll().
        pub fn poll(self: *Self) ?UplinkData {
            return self.uplink.recv();
        }

        // ====================================================================
        // Uplink handlers — called from broker mux
        // ====================================================================

        /// Handle an incoming state event (JSON payload from broker mux).
        /// Parses the JSON, caches the state, and pushes to uplink queue.
        pub fn handleStatePayload(self: *Self, payload: []const u8) void {
            var evt: types.StateEvent = undefined;
            wire.parseStateEvent(payload, &evt) catch return;

            self.mutex.lock();
            // Filter out-of-order events (same as Go)
            if (self.cached_state) |cached| {
                if (evt.time < cached.time) {
                    self.mutex.unlock();
                    return;
                }
            }
            self.cached_state = evt;
            self.mutex.unlock();

            self.uplink.trySend(.{ .state = evt }) catch {};
        }

        /// Handle an incoming stats event (JSON payload from broker mux).
        /// Parses the JSON, caches the stats, and pushes to uplink queue.
        pub fn handleStatsPayload(self: *Self, payload: []const u8) void {
            var evt: types.StatsEvent = undefined;
            wire.parseStatsEvent(payload, &evt) catch return;

            self.mutex.lock();
            self.cached_stats = evt;
            self.mutex.unlock();

            self.uplink.trySend(.{ .stats = evt }) catch {};
        }

        /// Handle an incoming audio frame (binary payload from broker mux).
        /// Unstamps the frame and pushes to uplink queue.
        pub fn handleAudioPayload(self: *Self, payload: []const u8) void {
            const frame = wire.unstampFrame(payload) catch return;
            self.uplink.trySend(.{ .audio = frame }) catch {};
        }

        // ====================================================================
        // Downlink: issueCommand + convenience wrappers
        // ====================================================================

        /// Queue a command for sending to the device.
        pub fn issueCommand(self: *Self, cmd_type: types.CommandType, payload: types.CommandPayload) void {
            const now: i64 = @intCast(Time.getTimeMs());
            const evt = types.CommandEvent{
                .cmd_type = cmd_type,
                .time = now,
                .payload = payload,
                .issue_at = now,
            };
            self.commands.trySend(evt) catch {};
        }

        /// Set the device volume.
        pub fn setVolume(self: *Self, volume: i32) void {
            self.issueCommand(.set_volume, .{ .set_volume = volume });
        }

        /// Set the device brightness.
        pub fn setBrightness(self: *Self, brightness: i32) void {
            self.issueCommand(.set_brightness, .{ .set_brightness = brightness });
        }

        /// Send streaming command.
        pub fn setStreaming(self: *Self, enabled: bool) void {
            self.issueCommand(.streaming, .{ .streaming = enabled });
        }

        /// Reset the device.
        pub fn reset(self: *Self) void {
            self.issueCommand(.reset, .{ .reset = .{} });
        }

        /// Unpair the device.
        pub fn unpair(self: *Self) void {
            self.issueCommand(.reset, .{ .reset = .{ .unpair = true } });
        }

        /// Put the device to sleep.
        pub fn sleep(self: *Self) void {
            self.issueCommand(.halt, .{ .halt = .{ .sleep = true } });
        }

        /// Shut down the device.
        pub fn shutdown(self: *Self) void {
            self.issueCommand(.halt, .{ .halt = .{ .shutdown = true } });
        }

        /// Raise a call on the device.
        pub fn raiseCall(self: *Self) void {
            self.issueCommand(.raise, .{ .raise = .{ .call = true } });
        }

        // ====================================================================
        // State/Stats Getters
        // ====================================================================

        /// Get the latest cached device state.
        pub fn getState(self: *Self) ?types.StateEvent {
            self.mutex.lock();
            defer self.mutex.unlock();
            return self.cached_state;
        }

        /// Get the latest cached device stats.
        pub fn getStats(self: *Self) ?types.StatsEvent {
            self.mutex.lock();
            defer self.mutex.unlock();
            return self.cached_stats;
        }

        // ====================================================================
        // Background task: command tx loop
        // ====================================================================

        fn txCommandLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            while (self.commands.recv()) |evt| {
                var event = evt;
                self.conn.issueCommand(&event) catch {};
            }
        }
    };
}
