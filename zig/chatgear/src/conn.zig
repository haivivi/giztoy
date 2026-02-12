//! Chatgear MQTT Connection
//!
//! Client-side connection layer for the chatgear protocol over MQTT.
//! Generic over MqttClient — works with any MQTT client that provides
//! publish() and subscribe() methods (e.g., embed-zig mqtt0.Client).
//!
//! Handles:
//! - Topic name building ({scope}device/{gear_id}/{suffix})
//! - Uplink: sendOpusFrame, sendState, sendStats
//! - Downlink: subscribe to output_audio_stream + command topics
//!
//! Downlink message dispatch is handled externally via mqtt0's Mux pattern.
//! See port.zig for the full Channel-based async architecture.

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");

// ============================================================================
// Configuration
// ============================================================================

/// Connection configuration.
pub const Config = struct {
    /// MQTT topic scope prefix (e.g., "palr/cn/", "stage/", "").
    /// Must end with '/' if non-empty.
    scope: []const u8 = "",

    /// Device gear ID (unique device identifier).
    gear_id: []const u8,
};

// ============================================================================
// Topic Name Builder
// ============================================================================

/// Maximum topic name length.
const MAX_TOPIC_LEN: usize = 128;

/// Builds MQTT topic names: {scope}device/{gear_id}/{suffix}
///
/// Stores the built topic in an internal buffer. Use `slice()` to get
/// the topic string — always returns a slice into `self.buf`, safe
/// regardless of struct moves/copies.
pub fn TopicBuilder(comptime max_len: usize) type {
    return struct {
        const Self = @This();

        buf: [max_len]u8 = undefined,
        len: usize = 0,

        /// Build topic: {scope}device/{gear_id}/{suffix}
        /// Stores result in buf. Returns slice into buf (caller must not
        /// store the returned slice if the struct may be copied — use
        /// slice() instead for stable access).
        pub fn build(self: *Self, scope: []const u8, gear_id: []const u8, suffix: []const u8) []const u8 {
            var pos: usize = 0;

            if (scope.len > 0) {
                @memcpy(self.buf[pos..][0..scope.len], scope);
                pos += scope.len;
            }

            const device_prefix = "device/";
            @memcpy(self.buf[pos..][0..device_prefix.len], device_prefix);
            pos += device_prefix.len;

            @memcpy(self.buf[pos..][0..gear_id.len], gear_id);
            pos += gear_id.len;

            self.buf[pos] = '/';
            pos += 1;

            @memcpy(self.buf[pos..][0..suffix.len], suffix);
            pos += suffix.len;

            self.len = pos;
            return self.buf[0..pos];
        }

        /// Get the stored topic string. Always returns a slice into
        /// self.buf — safe to call after struct copy/move.
        pub fn slice(self: *const Self) []const u8 {
            return self.buf[0..self.len];
        }
    };
}

// ============================================================================
// MQTT Client Connection
// ============================================================================

/// Generic MQTT connection for the chatgear protocol.
///
/// MqttClient must provide:
/// - `publish(topic: []const u8, payload: []const u8) !void`
/// - `subscribe(topics: []const []const u8) !void`
///
/// Downlink message handling (audio frames, commands) is done via mqtt0's
/// Mux pattern — register handlers before calling readLoop(). See port.zig.
pub fn MqttClientConn(comptime MqttClient: type) type {
    return struct {
        const Self = @This();

        mqtt: *MqttClient,

        // Pre-built topic name buffers (use tb_*.slice() for stable access)
        tb_input_audio: TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_state: TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_stats: TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_output_audio: TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_command: TopicBuilder(MAX_TOPIC_LEN) = .{},

        // Wire buffers for encoding
        stamp_buf: [wire.HEADER_SIZE + wire.MAX_OPUS_FRAME_SIZE]u8 = undefined,
        json_buf: [wire.STATS_EVENT_JSON_SIZE]u8 = undefined,

        /// Initialize a new chatgear connection.
        /// Scope is normalized to end with '/' if non-empty (matching Go behavior).
        pub fn init(mqtt: *MqttClient, config: Config) Self {
            var self = Self{ .mqtt = mqtt };

            // Normalize scope: ensure trailing '/' if non-empty
            // Matches Go: if scope != "" && !strings.HasSuffix(scope, "/") { scope += "/" }
            var scope_buf: [MAX_TOPIC_LEN]u8 = undefined;
            var scope = config.scope;
            if (scope.len > 0 and scope[scope.len - 1] != '/') {
                @memcpy(scope_buf[0..scope.len], scope);
                scope_buf[scope.len] = '/';
                scope = scope_buf[0 .. scope.len + 1];
            }

            // Build topics into internal buffers (don't cache returned slices —
            // they become dangling after struct copy. Use tb_*.slice() instead.)
            _ = self.tb_input_audio.build(scope, config.gear_id, "input_audio_stream");
            _ = self.tb_state.build(scope, config.gear_id, "state");
            _ = self.tb_stats.build(scope, config.gear_id, "stats");
            _ = self.tb_output_audio.build(scope, config.gear_id, "output_audio_stream");
            _ = self.tb_command.build(scope, config.gear_id, "command");

            return self;
        }

        /// Subscribe to downlink topics (output_audio_stream, command).
        pub fn subscribe(self: *Self) !void {
            const topics = [_][]const u8{
                self.tb_output_audio.slice(),
                self.tb_command.slice(),
            };
            try self.mqtt.subscribe(&topics);
        }

        // ====================================================================
        // Uplink: Device -> Server
        // ====================================================================

        /// Send an opus frame with timestamp.
        pub fn sendOpusFrame(self: *Self, timestamp_ms: i64, frame: []const u8) !void {
            const stamped_len = wire.stampFrame(timestamp_ms, frame, &self.stamp_buf) catch {
                return error.BufferTooSmall;
            };
            try self.mqtt.publish(self.tb_input_audio.slice(), self.stamp_buf[0..stamped_len]);
        }

        /// Send a state event.
        pub fn sendState(self: *Self, event: *const types.StateEvent) !void {
            const json_len = wire.encodeStateEvent(event, &self.json_buf) catch {
                return error.BufferTooSmall;
            };
            try self.mqtt.publish(self.tb_state.slice(), self.json_buf[0..json_len]);
        }

        /// Send a stats event.
        pub fn sendStats(self: *Self, event: *const types.StatsEvent) !void {
            const json_len = wire.encodeStatsEvent(event, &self.json_buf) catch {
                return error.BufferTooSmall;
            };
            try self.mqtt.publish(self.tb_stats.slice(), self.json_buf[0..json_len]);
        }

        // ====================================================================
        // Topic Accessors (for Mux handler registration)
        // ====================================================================

        /// Returns the output audio stream topic (downlink).
        pub fn outputAudioTopic(self: *const Self) []const u8 {
            return self.tb_output_audio.slice();
        }

        /// Returns the command topic (downlink).
        pub fn commandTopic(self: *const Self) []const u8 {
            return self.tb_command.slice();
        }
    };
}

// ============================================================================
// Tests
// ============================================================================

test "TopicBuilder with scope" {
    var builder = TopicBuilder(128){};
    const topic = builder.build("stage/", "gear-001", "input_audio_stream");
    try std.testing.expectEqualStrings("stage/device/gear-001/input_audio_stream", topic);
}

test "TopicBuilder without scope" {
    var builder = TopicBuilder(128){};
    const topic = builder.build("", "device-abc", "command");
    try std.testing.expectEqualStrings("device/device-abc/command", topic);
}

test "TopicBuilder various suffixes" {
    const Case = struct { suffix: []const u8, expected: []const u8 };
    const cases = [_]Case{
        .{ .suffix = "state", .expected = "prod/device/gear-123/state" },
        .{ .suffix = "stats", .expected = "prod/device/gear-123/stats" },
        .{ .suffix = "output_audio_stream", .expected = "prod/device/gear-123/output_audio_stream" },
        .{ .suffix = "input_audio_stream", .expected = "prod/device/gear-123/input_audio_stream" },
        .{ .suffix = "command", .expected = "prod/device/gear-123/command" },
    };

    for (cases) |tc| {
        var builder = TopicBuilder(128){};
        const topic = builder.build("prod/", "gear-123", tc.suffix);
        try std.testing.expectEqualStrings(tc.expected, topic);
    }
}

test "MqttClientConn init builds topics" {
    const MockMqtt = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) !void {}
        pub fn subscribe(_: *@This(), _: []const []const u8) !void {}
    };

    var mqtt = MockMqtt{};
    var conn = MqttClientConn(MockMqtt).init(&mqtt, .{
        .scope = "palr/cn/",
        .gear_id = "test-001",
    });

    try std.testing.expectEqualStrings("palr/cn/device/test-001/input_audio_stream", conn.tb_input_audio.slice());
    try std.testing.expectEqualStrings("palr/cn/device/test-001/state", conn.tb_state.slice());
    try std.testing.expectEqualStrings("palr/cn/device/test-001/stats", conn.tb_stats.slice());
    try std.testing.expectEqualStrings("palr/cn/device/test-001/output_audio_stream", conn.tb_output_audio.slice());
    try std.testing.expectEqualStrings("palr/cn/device/test-001/command", conn.tb_command.slice());
}

test "MqttClientConn auto-appends trailing slash to scope" {
    const MockMqtt = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) !void {}
        pub fn subscribe(_: *@This(), _: []const []const u8) !void {}
    };

    var mqtt = MockMqtt{};
    // Scope without trailing slash — should be normalized
    var conn = MqttClientConn(MockMqtt).init(&mqtt, .{
        .scope = "RyBFG6",
        .gear_id = "zig-001",
    });

    try std.testing.expectEqualStrings("RyBFG6/device/zig-001/state", conn.tb_state.slice());
    try std.testing.expectEqualStrings("RyBFG6/device/zig-001/command", conn.tb_command.slice());
}

test "MqttClientConn empty scope" {
    const MockMqtt = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) !void {}
        pub fn subscribe(_: *@This(), _: []const []const u8) !void {}
    };

    var mqtt = MockMqtt{};
    var conn = MqttClientConn(MockMqtt).init(&mqtt, .{
        .scope = "",
        .gear_id = "dev-001",
    });

    try std.testing.expectEqualStrings("device/dev-001/state", conn.tb_state.slice());
}
