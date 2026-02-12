//! Chatgear MQTT Server Connection
//!
//! Server-side connection layer for the chatgear protocol over MQTT.
//! Mirrors Go's conn_mqtt_server.go ListenMQTTServer mode:
//! - Uses the broker's mux handler to capture uplink messages directly
//! - Publishes downlink (audio, commands) via broker.publish()
//!
//! Generic over Broker type — works with mqtt0.Broker(Transport, Rt).
//!
//! ## Architecture
//!
//! The server conn does NOT create its own MQTT client. Instead, it
//! registers handlers on the broker's mux for uplink topics. When a
//! device client connects and publishes, the broker calls the handler
//! which pushes data into channels. The server reads from these channels.
//!
//! For downlink, the server calls broker.publish() directly — the broker
//! routes the message to the subscribed device client.

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");
const conn_mod = @import("conn.zig");

/// Server-side MQTT connection for the chatgear protocol.
///
/// BrokerType must provide:
/// - `publish(topic: []const u8, payload: []const u8) void`
///
/// Uplink messages are received via the broker's mux handler (registered
/// externally by the caller or by ServerPort). Downlink messages are sent
/// via broker.publish().
pub fn MqttServerConn(comptime BrokerType: type) type {
    return struct {
        const Self = @This();

        broker: *BrokerType,

        // Pre-built topic name buffers (reuses TopicBuilder from conn.zig)
        tb_input_audio: conn_mod.TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_state: conn_mod.TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_stats: conn_mod.TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_output_audio: conn_mod.TopicBuilder(MAX_TOPIC_LEN) = .{},
        tb_command: conn_mod.TopicBuilder(MAX_TOPIC_LEN) = .{},

        // Cached topic slices
        input_audio_topic: []const u8 = "",
        state_topic: []const u8 = "",
        stats_topic: []const u8 = "",
        output_audio_topic: []const u8 = "",
        command_topic: []const u8 = "",

        // Wire buffers for encoding downlink
        stamp_buf: [wire.HEADER_SIZE + wire.MAX_OPUS_FRAME_SIZE]u8 = undefined,
        json_buf: [wire.COMMAND_EVENT_JSON_SIZE]u8 = undefined,

        /// Initialize a new server connection.
        /// Scope is normalized to end with '/' if non-empty (matching Go behavior).
        pub fn init(broker: *BrokerType, config: conn_mod.Config) Self {
            var self = Self{ .broker = broker };

            // Normalize scope
            var scope_buf: [MAX_TOPIC_LEN]u8 = undefined;
            var scope = config.scope;
            if (scope.len > 0 and scope[scope.len - 1] != '/') {
                @memcpy(scope_buf[0..scope.len], scope);
                scope_buf[scope.len] = '/';
                scope = scope_buf[0 .. scope.len + 1];
            }

            self.input_audio_topic = self.tb_input_audio.build(scope, config.gear_id, "input_audio_stream");
            self.state_topic = self.tb_state.build(scope, config.gear_id, "state");
            self.stats_topic = self.tb_stats.build(scope, config.gear_id, "stats");
            self.output_audio_topic = self.tb_output_audio.build(scope, config.gear_id, "output_audio_stream");
            self.command_topic = self.tb_command.build(scope, config.gear_id, "command");

            return self;
        }

        // ====================================================================
        // Downlink: Server -> Device
        // ====================================================================

        /// Send an opus frame to the device (downlink audio).
        pub fn sendOpusFrame(self: *Self, timestamp_ms: i64, frame: []const u8) !void {
            const stamped_len = wire.stampFrame(timestamp_ms, frame, &self.stamp_buf) catch {
                return error.BufferTooSmall;
            };
            self.broker.publish(self.output_audio_topic, self.stamp_buf[0..stamped_len]);
        }

        /// Send a command to the device.
        pub fn issueCommand(self: *Self, event: *const types.CommandEvent) !void {
            const json_len = wire.encodeCommandEvent(event, &self.json_buf) catch {
                return error.BufferTooSmall;
            };
            self.broker.publish(self.command_topic, self.json_buf[0..json_len]);
        }

        // ====================================================================
        // Topic Accessors (for registering mux handlers)
        // ====================================================================

        /// Returns the input audio stream topic (uplink — device sends audio here).
        pub fn inputAudioTopic(self: *const Self) []const u8 {
            return self.input_audio_topic;
        }

        /// Returns the state topic (uplink — device sends state here).
        pub fn stateTopic(self: *const Self) []const u8 {
            return self.state_topic;
        }

        /// Returns the stats topic (uplink — device sends stats here).
        pub fn statsTopic(self: *const Self) []const u8 {
            return self.stats_topic;
        }

        /// Returns the output audio stream topic (downlink).
        pub fn outputAudioTopic(self: *const Self) []const u8 {
            return self.output_audio_topic;
        }

        /// Returns the command topic (downlink).
        pub fn commandTopic(self: *const Self) []const u8 {
            return self.command_topic;
        }
    };
}

const MAX_TOPIC_LEN: usize = 128;

// ============================================================================
// Tests
// ============================================================================

test "MqttServerConn builds topics correctly" {
    const MockBroker = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) void {}
    };

    var broker = MockBroker{};
    const conn = MqttServerConn(MockBroker).init(&broker, .{
        .scope = "test/",
        .gear_id = "dev-001",
    });

    try std.testing.expectEqualStrings("test/device/dev-001/input_audio_stream", conn.input_audio_topic);
    try std.testing.expectEqualStrings("test/device/dev-001/state", conn.state_topic);
    try std.testing.expectEqualStrings("test/device/dev-001/stats", conn.stats_topic);
    try std.testing.expectEqualStrings("test/device/dev-001/output_audio_stream", conn.output_audio_topic);
    try std.testing.expectEqualStrings("test/device/dev-001/command", conn.command_topic);
}

test "MqttServerConn auto-appends trailing slash to scope" {
    const MockBroker = struct {
        pub fn publish(_: *@This(), _: []const u8, _: []const u8) void {}
    };

    var broker = MockBroker{};
    const conn = MqttServerConn(MockBroker).init(&broker, .{
        .scope = "RyBFG6",
        .gear_id = "zig-001",
    });

    try std.testing.expectEqualStrings("RyBFG6/device/zig-001/state", conn.state_topic);
    try std.testing.expectEqualStrings("RyBFG6/device/zig-001/command", conn.command_topic);
}
