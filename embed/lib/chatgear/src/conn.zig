//! Chatgear MQTT Connection
//!
//! Client connection implementation over MQTT for the chatgear protocol.
//! Handles the client-side of device-server communication.

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");

// ============================================================================
// Configuration
// ============================================================================

/// Connection configuration
pub const Config = struct {
    /// MQTT topic scope prefix (e.g., "prod/", "stage/", "")
    scope: []const u8 = "",

    /// Device gear ID (unique device identifier)
    gear_id: []const u8,

    /// Receive buffer size
    recv_buf_size: usize = 2048,

    /// Send buffer size (for stamped frames)
    send_buf_size: usize = 1024,
};

// ============================================================================
// Topic Names
// ============================================================================

/// Maximum topic name length
const MAX_TOPIC_LEN: usize = 128;

/// Topic name builder
pub fn TopicBuilder(comptime max_len: usize) type {
    return struct {
        const Self = @This();

        buf: [max_len]u8,
        len: usize,

        pub fn init() Self {
            return Self{
                .buf = undefined,
                .len = 0,
            };
        }

        /// Build topic: {scope}device/{gear_id}/{suffix}
        pub fn build(self: *Self, scope: []const u8, gear_id: []const u8, suffix: []const u8) []const u8 {
            var pos: usize = 0;

            // Scope
            if (scope.len > 0) {
                @memcpy(self.buf[pos..][0..scope.len], scope);
                pos += scope.len;
            }

            // "device/"
            const device_prefix = "device/";
            @memcpy(self.buf[pos..][0..device_prefix.len], device_prefix);
            pos += device_prefix.len;

            // gear_id
            @memcpy(self.buf[pos..][0..gear_id.len], gear_id);
            pos += gear_id.len;

            // "/"
            self.buf[pos] = '/';
            pos += 1;

            // suffix
            @memcpy(self.buf[pos..][0..suffix.len], suffix);
            pos += suffix.len;

            self.len = pos;
            return self.buf[0..pos];
        }
    };
}

// ============================================================================
// Connection Errors
// ============================================================================

pub const ConnError = error{
    NotConnected,
    SendFailed,
    RecvFailed,
    BufferTooSmall,
    InvalidData,
    Timeout,
    TopicMismatch,
    ParseError,
};

// ============================================================================
// Chatgear MQTT Client Connection
// ============================================================================

/// Generic MQTT connection for chatgear protocol
/// Uses generic MqttClient type to support both TCP and TLS transports
pub fn MqttClientConn(comptime MqttClient: type, comptime Log: type) type {
    return struct {
        const Self = @This();

        // MQTT client
        mqtt: *MqttClient,

        // Configuration
        scope: []const u8,
        gear_id: []const u8,

        // Pre-built topic names
        topic_input_audio: TopicBuilder(MAX_TOPIC_LEN),
        topic_state: TopicBuilder(MAX_TOPIC_LEN),
        topic_stats: TopicBuilder(MAX_TOPIC_LEN),
        topic_output_audio: TopicBuilder(MAX_TOPIC_LEN),
        topic_command: TopicBuilder(MAX_TOPIC_LEN),

        // Cached topic strings
        input_audio_topic: []const u8,
        state_topic: []const u8,
        stats_topic: []const u8,
        output_audio_topic: []const u8,
        command_topic: []const u8,

        // Subscribed flag
        subscribed: bool,

        /// Initialize a new chatgear connection
        pub fn init(mqtt: *MqttClient, config: *const Config) Self {
            var self = Self{
                .mqtt = mqtt,
                .scope = config.scope,
                .gear_id = config.gear_id,
                .topic_input_audio = TopicBuilder(MAX_TOPIC_LEN).init(),
                .topic_state = TopicBuilder(MAX_TOPIC_LEN).init(),
                .topic_stats = TopicBuilder(MAX_TOPIC_LEN).init(),
                .topic_output_audio = TopicBuilder(MAX_TOPIC_LEN).init(),
                .topic_command = TopicBuilder(MAX_TOPIC_LEN).init(),
                .input_audio_topic = undefined,
                .state_topic = undefined,
                .stats_topic = undefined,
                .output_audio_topic = undefined,
                .command_topic = undefined,
                .subscribed = false,
            };

            // Build all topic names
            self.input_audio_topic = self.topic_input_audio.build(config.scope, config.gear_id, "input_audio_stream");
            self.state_topic = self.topic_state.build(config.scope, config.gear_id, "state");
            self.stats_topic = self.topic_stats.build(config.scope, config.gear_id, "stats");
            self.output_audio_topic = self.topic_output_audio.build(config.scope, config.gear_id, "output_audio_stream");
            self.command_topic = self.topic_command.build(config.scope, config.gear_id, "command");

            return self;
        }

        /// Subscribe to downlink topics (output_audio_stream, command)
        pub fn subscribe(self: *Self, buf: []u8) ConnError!void {
            if (!self.mqtt.isConnected()) return ConnError.NotConnected;

            const topics = [_][]const u8{
                self.output_audio_topic,
                self.command_topic,
            };

            self.mqtt.subscribe(&topics, buf) catch |e| {
                Log.err("Failed to subscribe: {any}", .{e});
                return ConnError.SendFailed;
            };

            self.subscribed = true;
            Log.info("Subscribed to downlink topics", .{});
        }

        // ====================================================================
        // Uplink: Device -> Server
        // ====================================================================

        /// Send an opus frame with timestamp (uplink)
        pub fn sendOpusFrame(self: *Self, timestamp_ms: i64, frame: []const u8, buf: []u8) ConnError!void {
            if (!self.mqtt.isConnected()) return ConnError.NotConnected;

            // Stamp the frame
            const stamped_len = wire.stampFrame(timestamp_ms, frame, buf) catch {
                return ConnError.BufferTooSmall;
            };

            // Publish
            self.mqtt.publish(self.input_audio_topic, buf[0..stamped_len], buf[stamped_len..]) catch |e| {
                Log.err("Failed to send opus frame: {any}", .{e});
                return ConnError.SendFailed;
            };
        }

        /// Send a state event (uplink)
        pub fn sendState(self: *Self, state: *const types.StateEvent, buf: []u8) ConnError!void {
            if (!self.mqtt.isConnected()) return ConnError.NotConnected;

            // Encode state event to JSON
            const json_len = wire.encodeStateEvent(state, buf) catch {
                return ConnError.BufferTooSmall;
            };

            // Publish
            self.mqtt.publish(self.state_topic, buf[0..json_len], buf[json_len..]) catch |e| {
                Log.err("Failed to send state: {any}", .{e});
                return ConnError.SendFailed;
            };

            Log.debug("Sent state: {s}", .{state.state.toString()});
        }

        /// Send raw stats JSON (uplink)
        pub fn sendStats(self: *Self, stats_json: []const u8, buf: []u8) ConnError!void {
            if (!self.mqtt.isConnected()) return ConnError.NotConnected;

            // Publish
            self.mqtt.publish(self.stats_topic, stats_json, buf) catch |e| {
                Log.err("Failed to send stats: {any}", .{e});
                return ConnError.SendFailed;
            };
        }

        // ====================================================================
        // Downlink: Server -> Device
        // ====================================================================

        /// Receive a message (downlink) - non-blocking
        /// Returns null if no message available
        pub fn recvMessage(self: *Self, buf: []u8) ConnError!?types.Message {
            if (!self.mqtt.isConnected()) return ConnError.NotConnected;

            const maybe_msg = self.mqtt.recvMessage(buf) catch |e| {
                if (e == error.Timeout) return null;
                return ConnError.RecvFailed;
            };

            if (maybe_msg) |msg| {
                // Determine message type by topic
                if (std.mem.eql(u8, msg.topic, self.output_audio_topic)) {
                    // Opus frame
                    const stamped = wire.unstampFrame(msg.payload) catch {
                        Log.warn("Invalid opus frame format", .{});
                        return ConnError.InvalidData;
                    };
                    return types.Message{ .opus_frame = stamped };
                } else if (std.mem.eql(u8, msg.topic, self.command_topic)) {
                    // Command
                    var cmd_event: types.CommandEvent = undefined;
                    wire.parseCommandEvent(msg.payload, &cmd_event) catch {
                        Log.warn("Invalid command format", .{});
                        return ConnError.ParseError;
                    };
                    return types.Message{ .command = cmd_event };
                } else {
                    // Unknown topic, ignore
                    Log.debug("Unknown topic: {s}", .{msg.topic});
                    return null;
                }
            }

            return null;
        }

        /// Check if connected
        pub fn isConnected(self: *const Self) bool {
            return self.mqtt.isConnected();
        }

        /// Check if subscribed
        pub fn isSubscribed(self: *const Self) bool {
            return self.subscribed;
        }

        /// Ping to keep connection alive
        pub fn ping(self: *Self, buf: []u8) ConnError!void {
            self.mqtt.ping(buf) catch return ConnError.SendFailed;
        }

        /// Check if ping is needed
        pub fn needsPing(self: *const Self) bool {
            return self.mqtt.needsPing();
        }

        /// Disconnect
        pub fn disconnect(self: *Self, buf: []u8) void {
            self.mqtt.disconnect(buf);
            self.subscribed = false;
        }
    };
}

// ============================================================================
// Convenient Type Aliases
// ============================================================================

/// Create a chatgear client port type with specific MQTT client
pub fn ClientPort(comptime MqttClient: type, comptime Log: type) type {
    return MqttClientConn(MqttClient, Log);
}

// ============================================================================
// Tests
// ============================================================================

test "TopicBuilder with scope" {
    var builder = TopicBuilder(128).init();
    const topic = builder.build("stage/", "gear-001", "input_audio_stream");

    try std.testing.expectEqualStrings("stage/device/gear-001/input_audio_stream", topic);
}

test "TopicBuilder without scope" {
    var builder = TopicBuilder(128).init();
    const topic = builder.build("", "device-abc", "command");

    try std.testing.expectEqualStrings("device/device-abc/command", topic);
}

test "TopicBuilder various suffixes" {
    const TestCase = struct {
        suffix: []const u8,
        expected: []const u8,
    };

    const cases = [_]TestCase{
        .{ .suffix = "state", .expected = "prod/device/gear-123/state" },
        .{ .suffix = "stats", .expected = "prod/device/gear-123/stats" },
        .{ .suffix = "output_audio_stream", .expected = "prod/device/gear-123/output_audio_stream" },
    };

    for (cases) |tc| {
        var builder = TopicBuilder(128).init();
        const topic = builder.build("prod/", "gear-123", tc.suffix);
        try std.testing.expectEqualStrings(tc.expected, topic);
    }
}

test "TopicBuilder length tracking" {
    var builder = TopicBuilder(128).init();
    _ = builder.build("test/", "dev", "cmd");

    // "test/device/dev/cmd" = 19 chars
    try std.testing.expectEqual(@as(usize, 19), builder.len);
}
