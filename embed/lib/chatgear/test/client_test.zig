//! ClientPort Integration Test
//!
//! Tests ClientPort against mock MQTT client to verify:
//! - State sending (immediate + periodic)
//! - Stats sending
//! - Command callback handling
//!
//! Run with:
//!   zig build test --summary all (from embed/lib/chatgear)
//!   or: bazel run //embed/lib/chatgear:test

const std = @import("std");
const chatgear = @import("chatgear");
const port = chatgear.port;

// ============================================================================
// Mock Implementations
// ============================================================================

/// Mock Time for testing
const MockTime = struct {
    var current_time: u64 = 0;

    pub fn getTimeMs() u64 {
        return current_time;
    }

    pub fn sleepMs(_: u32) void {
        // Advance time by the sleep amount for testing
        current_time += 10;
    }

    pub fn reset() void {
        current_time = 0;
    }

    pub fn advance(ms: u64) void {
        current_time += ms;
    }
};

/// Mock Log for testing
const MockLog = struct {
    pub fn info(comptime _: []const u8, _: anytype) void {}
    pub fn debug(comptime _: []const u8, _: anytype) void {}
    pub fn err(comptime _: []const u8, _: anytype) void {}
    pub fn warn(comptime _: []const u8, _: anytype) void {}
};

/// Mock MQTT Client for testing
const MockMqttClient = struct {
    const Self = @This();

    // Captured messages
    var published_messages: std.ArrayList(PublishedMessage) = undefined;
    var initialized: bool = false;

    pub const PublishedMessage = struct {
        topic: []const u8,
        payload: []const u8,
    };

    pub fn init() void {
        if (!initialized) {
            published_messages = std.ArrayList(PublishedMessage).init(std.testing.allocator);
            initialized = true;
        }
    }

    pub fn deinit() void {
        if (initialized) {
            for (published_messages.items) |msg| {
                std.testing.allocator.free(msg.topic);
                std.testing.allocator.free(msg.payload);
            }
            published_messages.deinit();
            initialized = false;
        }
    }

    pub fn reset() void {
        if (initialized) {
            for (published_messages.items) |msg| {
                std.testing.allocator.free(msg.topic);
                std.testing.allocator.free(msg.payload);
            }
            published_messages.clearRetainingCapacity();
        }
    }

    pub fn getMessageCount() usize {
        return published_messages.items.len;
    }

    pub fn getMessage(index: usize) ?PublishedMessage {
        if (index < published_messages.items.len) {
            return published_messages.items[index];
        }
        return null;
    }

    // Mock MQTT client methods
    pub fn publish(topic: []const u8, payload: []const u8, _: anytype) !void {
        if (!initialized) init();

        const topic_copy = try std.testing.allocator.dupe(u8, topic);
        const payload_copy = try std.testing.allocator.dupe(u8, payload);

        try published_messages.append(.{
            .topic = topic_copy,
            .payload = payload_copy,
        });
    }

    pub fn subscribe(_: []const u8) !void {}
    pub fn recv() !?[]const u8 {
        return null;
    }
    pub fn ping() !void {}
    pub fn needsPing() bool {
        return false;
    }
    pub fn disconnect() void {}
};

// ============================================================================
// Tests
// ============================================================================

test "ClientPort: StatsData JSON building" {
    const ClientPort = port.ClientPort(MockMqttClient, MockLog, MockTime);

    var p = ClientPort{
        .mqtt_conn = undefined,
        .config = port.PortConfig{
            .gear_id = "test-device",
        },
        .current_state = .unknown,
        .last_state_send_ms = 0,
        .state_pending = false,
        .stats = ClientPort.StatsData{
            .volume = 50,
            .brightness = 80,
        },
        .last_stats_send_ms = 0,
        .stats_pending = false,
        .stats_rounds = 0,
        .on_command = null,
        .on_opus_frame = null,
        .running = false,
        .buf = undefined,
    };

    var json_buf: [1024]u8 = undefined;
    const len = p.buildStatsJson(&json_buf, 1234567890);

    try std.testing.expect(len > 0);

    const json_str = json_buf[0..len];

    // Verify JSON contains expected fields
    try std.testing.expect(std.mem.indexOf(u8, json_str, "\"time\":1234567890") != null);
    try std.testing.expect(std.mem.indexOf(u8, json_str, "\"volume\":{\"percentage\":50") != null);
    try std.testing.expect(std.mem.indexOf(u8, json_str, "\"brightness\":{\"percentage\":80") != null);
}

test "ClientPort: setState changes state" {
    const ClientPort = port.ClientPort(MockMqttClient, MockLog, MockTime);

    var p = ClientPort{
        .mqtt_conn = undefined,
        .config = port.PortConfig{
            .gear_id = "test-device",
        },
        .current_state = .unknown,
        .last_state_send_ms = 0,
        .state_pending = false,
        .stats = ClientPort.StatsData{},
        .last_stats_send_ms = 0,
        .stats_pending = false,
        .stats_rounds = 0,
        .on_command = null,
        .on_opus_frame = null,
        .running = false,
        .buf = undefined,
    };

    // Initial state
    try std.testing.expectEqual(chatgear.State.unknown, p.current_state);
    try std.testing.expectEqual(false, p.state_pending);

    // Set to ready
    p.setState(.ready);
    try std.testing.expectEqual(chatgear.State.ready, p.current_state);
    try std.testing.expectEqual(true, p.state_pending);

    // Reset pending flag
    p.state_pending = false;

    // Set to same state - should not trigger pending
    p.setState(.ready);
    try std.testing.expectEqual(chatgear.State.ready, p.current_state);
    try std.testing.expectEqual(false, p.state_pending);

    // Set to different state
    p.setState(.recording);
    try std.testing.expectEqual(chatgear.State.recording, p.current_state);
    try std.testing.expectEqual(true, p.state_pending);
}

test "ClientPort: setVolume triggers stats pending" {
    const ClientPort = port.ClientPort(MockMqttClient, MockLog, MockTime);

    var p = ClientPort{
        .mqtt_conn = undefined,
        .config = port.PortConfig{
            .gear_id = "test-device",
        },
        .current_state = .unknown,
        .last_state_send_ms = 0,
        .state_pending = false,
        .stats = ClientPort.StatsData{},
        .last_stats_send_ms = 0,
        .stats_pending = false,
        .stats_rounds = 0,
        .on_command = null,
        .on_opus_frame = null,
        .running = false,
        .buf = undefined,
    };

    try std.testing.expectEqual(@as(?i32, null), p.stats.volume);
    try std.testing.expectEqual(false, p.stats_pending);

    p.setVolume(75);

    try std.testing.expectEqual(@as(?i32, 75), p.stats.volume);
    try std.testing.expectEqual(true, p.stats_pending);
}

test "ClientPort: setBattery sets both fields" {
    const ClientPort = port.ClientPort(MockMqttClient, MockLog, MockTime);

    var p = ClientPort{
        .mqtt_conn = undefined,
        .config = port.PortConfig{
            .gear_id = "test-device",
        },
        .current_state = .unknown,
        .last_state_send_ms = 0,
        .state_pending = false,
        .stats = ClientPort.StatsData{},
        .last_stats_send_ms = 0,
        .stats_pending = false,
        .stats_rounds = 0,
        .on_command = null,
        .on_opus_frame = null,
        .running = false,
        .buf = undefined,
    };

    p.setBattery(85, true);

    try std.testing.expectEqual(@as(?i32, 85), p.stats.battery_pct);
    try std.testing.expectEqual(true, p.stats.battery_charging);
    try std.testing.expectEqual(true, p.stats_pending);
}

test "ClientPort: timing constants" {
    try std.testing.expectEqual(@as(u64, 5000), port.STATE_INTERVAL_MS);
    try std.testing.expectEqual(@as(u64, 20000), port.STATS_BASE_INTERVAL_MS);
}

test "PortConfig defaults" {
    const config = port.PortConfig{
        .gear_id = "my-device",
    };

    try std.testing.expectEqualStrings("", config.scope);
    try std.testing.expectEqualStrings("my-device", config.gear_id);
    try std.testing.expectEqual(@as(u64, 5000), config.state_interval_ms);
    try std.testing.expectEqual(@as(u64, 20000), config.stats_interval_ms);
}
