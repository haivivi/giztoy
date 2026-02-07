//! Chatgear Client Port
//!
//! High-level client port implementation with:
//! - Periodic state reporting (every 5s)
//! - Periodic stats reporting (tiered: 20s/60s/120s/600s)
//! - Immediate send on state/stats change
//! - Command callback handling
//!
//! Usage:
//! ```zig
//! var port = ClientPort(MqttClient, Log, Time).init(&mqtt, &config);
//! port.on_command = handleCommand;
//! port.on_opus_frame = handleAudio;
//!
//! // Run in separate task
//! try port.run();
//! ```

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");
const conn = @import("conn.zig");

// Re-export types for convenience
pub const State = types.State;
pub const StateEvent = types.StateEvent;
pub const StatsEvent = types.StatsEvent;
pub const CommandEvent = types.CommandEvent;
pub const StampedFrame = types.StampedFrame;
pub const Message = types.Message;

// Timing constants (milliseconds)
pub const STATE_INTERVAL_MS: u64 = 5000; // 5 seconds
pub const STATS_BASE_INTERVAL_MS: u64 = 20000; // 20 seconds

/// Client Port Configuration
pub const PortConfig = struct {
    /// MQTT topic scope prefix
    scope: []const u8 = "",
    /// Device gear ID
    gear_id: []const u8,
    /// State send interval (ms)
    state_interval_ms: u64 = STATE_INTERVAL_MS,
    /// Stats base interval (ms)
    stats_interval_ms: u64 = STATS_BASE_INTERVAL_MS,
};

/// Client Port - high-level chatgear client
pub fn ClientPort(comptime MqttClient: type, comptime Log: type, comptime Time: type) type {
    const Conn = conn.MqttClientConn(MqttClient, Log);

    return struct {
        const Self = @This();

        // MQTT connection
        mqtt_conn: Conn,

        // Configuration
        config: PortConfig,

        // State management
        current_state: State,
        last_state_send_ms: u64,
        state_pending: bool, // True if state changed and needs immediate send

        // Stats management
        stats: StatsData,
        last_stats_send_ms: u64,
        stats_pending: bool, // True if stats changed and needs immediate send
        stats_rounds: u32, // Counter for tiered reporting

        // Callbacks
        on_command: ?*const fn (*CommandEvent) void,
        on_opus_frame: ?*const fn (*StampedFrame) void,

        // Control
        running: bool,

        // Work buffer
        buf: [2048]u8,

        /// Stats data storage (simplified for embedded)
        pub const StatsData = struct {
            volume: ?i32 = null,
            brightness: ?i32 = null,
            battery_pct: ?i32 = null,
            battery_charging: bool = false,
            light_mode: ?[]const u8 = null,
            wifi_ssid: ?[]const u8 = null,
            wifi_rssi: ?f32 = null,
            system_version: ?[]const u8 = null,
        };

        /// Initialize a new ClientPort
        pub fn init(mqtt: *MqttClient, port_config: PortConfig) Self {
            const mqtt_conn_config = conn.Config{
                .scope = port_config.scope,
                .gear_id = port_config.gear_id,
            };

            return Self{
                .mqtt_conn = Conn.init(mqtt, &mqtt_conn_config),
                .config = port_config,
                .current_state = .unknown,
                .last_state_send_ms = 0,
                .state_pending = false,
                .stats = StatsData{},
                .last_stats_send_ms = 0,
                .stats_pending = false,
                .stats_rounds = 0,
                .on_command = null,
                .on_opus_frame = null,
                .running = false,
                .buf = undefined,
            };
        }

        /// Subscribe to downlink topics
        pub fn subscribe(self: *Self) !void {
            try self.mqtt_conn.subscribe(&self.buf);
        }

        /// Main run loop - call from a dedicated task
        pub fn run(self: *Self) !void {
            self.running = true;

            // Subscribe to downlink topics
            try self.subscribe();

            // Send initial state
            const now = Time.getTimeMs();
            self.last_state_send_ms = now;
            self.last_stats_send_ms = now;
            try self.sendStateNow(now);

            Log.info("ClientPort: run loop started", .{});

            while (self.running) {
                const current_time = Time.getTimeMs();

                // 1. Handle periodic sends
                try self.handlePeriodicSends(current_time);

                // 2. Handle pending immediate sends
                try self.handlePendingSends(current_time);

                // 3. Receive and dispatch messages
                self.handleRecv();

                // 4. Handle MQTT keepalive
                if (self.mqtt_conn.needsPing()) {
                    self.mqtt_conn.ping(&self.buf) catch {};
                }

                // 5. Brief sleep to avoid busy-waiting
                Time.sleepMs(10);
            }

            Log.info("ClientPort: run loop stopped", .{});
        }

        /// Stop the run loop
        pub fn stop(self: *Self) void {
            self.running = false;
        }

        /// Check if running
        pub fn isRunning(self: *const Self) bool {
            return self.running;
        }

        // ====================================================================
        // State Management
        // ====================================================================

        /// Set state - sends immediately if changed
        pub fn setState(self: *Self, new_state: State) void {
            if (self.current_state == new_state) {
                return; // No change
            }

            self.current_state = new_state;
            self.state_pending = true;

            Log.debug("ClientPort: state changed to {s}", .{new_state.toString()});
        }

        /// Get current state
        pub fn getState(self: *const Self) State {
            return self.current_state;
        }

        // ====================================================================
        // Stats Management
        // ====================================================================

        /// Set volume (0-100)
        pub fn setVolume(self: *Self, vol: i32) void {
            self.stats.volume = vol;
            self.stats_pending = true;
        }

        /// Set brightness (0-100)
        pub fn setBrightness(self: *Self, brightness: i32) void {
            self.stats.brightness = brightness;
            self.stats_pending = true;
        }

        /// Set battery status
        pub fn setBattery(self: *Self, pct: i32, charging: bool) void {
            self.stats.battery_pct = pct;
            self.stats.battery_charging = charging;
            self.stats_pending = true;
        }

        /// Set light mode
        pub fn setLightMode(self: *Self, mode: []const u8) void {
            self.stats.light_mode = mode;
            self.stats_pending = true;
        }

        /// Set WiFi info
        pub fn setWifi(self: *Self, ssid: []const u8, rssi: f32) void {
            self.stats.wifi_ssid = ssid;
            self.stats.wifi_rssi = rssi;
            self.stats_pending = true;
        }

        /// Set system version
        pub fn setSystemVersion(self: *Self, version: []const u8) void {
            self.stats.system_version = version;
            self.stats_pending = true;
        }

        // ====================================================================
        // Audio
        // ====================================================================

        /// Send opus frame (call from mic task)
        pub fn sendOpusFrame(self: *Self, timestamp_ms: i64, frame: []const u8) !void {
            try self.mqtt_conn.sendOpusFrame(timestamp_ms, frame, &self.buf);
        }

        // ====================================================================
        // Internal: Periodic Sends
        // ====================================================================

        fn handlePeriodicSends(self: *Self, now: u64) !void {
            // State: every 5s
            if (now - self.last_state_send_ms >= self.config.state_interval_ms) {
                try self.sendStateNow(now);
                self.last_state_send_ms = now;
            }

            // Stats: every 20s (base interval)
            if (now - self.last_stats_send_ms >= self.config.stats_interval_ms) {
                self.stats_rounds += 1;
                try self.sendPeriodicStats(now);
                self.last_stats_send_ms = now;
            }
        }

        fn handlePendingSends(self: *Self, now: u64) !void {
            // Immediate state send on change
            if (self.state_pending) {
                try self.sendStateNow(now);
                self.state_pending = false;
                self.last_state_send_ms = now; // Reset timer
            }

            // Immediate stats send on change
            if (self.stats_pending) {
                try self.sendStatsNow(now);
                self.stats_pending = false;
            }
        }

        fn sendStateNow(self: *Self, now: u64) !void {
            const event = StateEvent{
                .version = 1,
                .time = @intCast(now),
                .state = self.current_state,
                .cause = null,
                .update_at = @intCast(now),
            };

            self.mqtt_conn.sendState(&event, &self.buf) catch |e| {
                Log.err("ClientPort: failed to send state: {any}", .{e});
                return e;
            };
        }

        fn sendStatsNow(self: *Self, now: u64) !void {
            // Build JSON manually for efficiency
            var json_buf: [1024]u8 = undefined;
            const json_len = self.buildStatsJson(&json_buf, now);

            if (json_len > 0) {
                self.mqtt_conn.sendStats(json_buf[0..json_len], &self.buf) catch |e| {
                    Log.err("ClientPort: failed to send stats: {any}", .{e});
                    return e;
                };
            }
        }

        fn sendPeriodicStats(self: *Self, now: u64) !void {
            // Tiered reporting based on rounds
            // rounds % 3 == 0: Every 60s (battery, volume, brightness, light_mode, wifi, sys_ver)
            // rounds % 6 == 1: Every 120s (cellular, shaking) - simplified, just send all
            // rounds % 30 == 2: Every 600s (wifi_store) - not implemented

            try self.sendStatsNow(now);
        }

        /// Build stats JSON (public for testing)
        pub fn buildStatsJson(self: *Self, output: []u8, now: u64) usize {
            var stream = std.io.fixedBufferStream(output);
            var writer = stream.writer();

            writer.writeAll("{\"time\":") catch return 0;
            std.fmt.format(writer, "{d}", .{now}) catch return 0;

            if (self.stats.volume) |vol| {
                writer.writeAll(",\"volume\":{\"percentage\":") catch return 0;
                std.fmt.format(writer, "{d}", .{vol}) catch return 0;
                writer.writeAll(",\"update_at\":") catch return 0;
                std.fmt.format(writer, "{d}", .{now}) catch return 0;
                writer.writeByte('}') catch return 0;
            }

            if (self.stats.brightness) |br| {
                writer.writeAll(",\"brightness\":{\"percentage\":") catch return 0;
                std.fmt.format(writer, "{d}", .{br}) catch return 0;
                writer.writeAll(",\"update_at\":") catch return 0;
                std.fmt.format(writer, "{d}", .{now}) catch return 0;
                writer.writeByte('}') catch return 0;
            }

            if (self.stats.battery_pct) |pct| {
                writer.writeAll(",\"battery\":{\"percentage\":") catch return 0;
                std.fmt.format(writer, "{d}", .{pct}) catch return 0;
                writer.writeAll(",\"is_charging\":") catch return 0;
                if (self.stats.battery_charging) {
                    writer.writeAll("true") catch return 0;
                } else {
                    writer.writeAll("false") catch return 0;
                }
                writer.writeByte('}') catch return 0;
            }

            if (self.stats.light_mode) |mode| {
                writer.writeAll(",\"light_mode\":{\"mode\":\"") catch return 0;
                writer.writeAll(mode) catch return 0;
                writer.writeAll("\",\"update_at\":") catch return 0;
                std.fmt.format(writer, "{d}", .{now}) catch return 0;
                writer.writeByte('}') catch return 0;
            }

            if (self.stats.wifi_ssid) |ssid| {
                writer.writeAll(",\"wifi_network\":{\"ssid\":\"") catch return 0;
                writer.writeAll(ssid) catch return 0;
                writer.writeByte('"') catch return 0;
                if (self.stats.wifi_rssi) |rssi| {
                    writer.writeAll(",\"rssi\":") catch return 0;
                    std.fmt.format(writer, "{d:.1}", .{rssi}) catch return 0;
                }
                writer.writeByte('}') catch return 0;
            }

            if (self.stats.system_version) |ver| {
                writer.writeAll(",\"system_version\":{\"current_version\":\"") catch return 0;
                writer.writeAll(ver) catch return 0;
                writer.writeAll("\"}") catch return 0;
            }

            writer.writeByte('}') catch return 0;

            return stream.pos;
        }

        // ====================================================================
        // Internal: Receive Handling
        // ====================================================================

        fn handleRecv(self: *Self) void {
            const result = self.mqtt_conn.recvMessage(&self.buf);

            if (result) |maybe_msg| {
                if (maybe_msg) |msg| {
                    switch (msg) {
                        .opus_frame => |frame| {
                            if (self.on_opus_frame) |callback| {
                                var mutable_frame = frame;
                                callback(&mutable_frame);
                            }
                        },
                        .command => |cmd| {
                            if (self.on_command) |callback| {
                                var mutable_cmd = cmd;
                                callback(&mutable_cmd);
                            }
                        },
                    }
                }
            } else |_| {
                // Timeout or error, ignore
            }
        }

        // ====================================================================
        // Cleanup
        // ====================================================================

        /// Disconnect and cleanup
        pub fn disconnect(self: *Self) void {
            self.running = false;
            self.mqtt_conn.disconnect(&self.buf);
        }
    };
}

// ============================================================================
// Tests
// ============================================================================

test "ClientPort.StatsData defaults" {
    const stats = ClientPort(void, void, void).StatsData{};
    try std.testing.expectEqual(@as(?i32, null), stats.volume);
    try std.testing.expectEqual(@as(?i32, null), stats.brightness);
    try std.testing.expectEqual(false, stats.battery_charging);
}

test "PortConfig defaults" {
    const config = PortConfig{
        .gear_id = "test-device",
    };
    try std.testing.expectEqual(@as(u64, 5000), config.state_interval_ms);
    try std.testing.expectEqual(@as(u64, 20000), config.stats_interval_ms);
    try std.testing.expectEqualStrings("", config.scope);
}
