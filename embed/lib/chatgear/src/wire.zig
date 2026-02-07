//! Chatgear Wire Format
//!
//! Encoding/decoding for the chatgear wire protocol.
//! Based on go/pkg/chatgear wire format.
//!
//! Stamped Opus Frame Wire Format:
//! +--------+------------------+------------------+
//! | Version| Timestamp (7B)   | Opus Frame Data  |
//! | (1B)   | Big-endian ms    |                  |
//! +--------+------------------+------------------+

const std = @import("std");
const types = @import("types.zig");

const StampedFrame = types.StampedFrame;

/// Wire format version
pub const VERSION: u8 = 1;

/// Header size: 1 byte version + 7 bytes timestamp
pub const HEADER_SIZE: usize = 8;

/// Maximum opus frame size (typical is ~80 bytes for voice)
pub const MAX_OPUS_FRAME_SIZE: usize = 1024;

/// Error type for wire operations
pub const WireError = error{
    BufferTooSmall,
    InvalidVersion,
    InvalidData,
};

/// Stamp an opus frame with timestamp
/// Returns the number of bytes written to output buffer
pub fn stampFrame(
    timestamp_ms: i64,
    opus_frame: []const u8,
    output: []u8,
) WireError!usize {
    const total_size = HEADER_SIZE + opus_frame.len;
    if (output.len < total_size) {
        return WireError.BufferTooSmall;
    }

    // Version byte
    output[0] = VERSION;

    // Timestamp: 7 bytes big-endian
    // Use lower 7 bytes of i64 (enough for ~2^55 ms = ~1000 years)
    const ts_u64: u64 = @bitCast(timestamp_ms);
    output[1] = @truncate(ts_u64 >> 48);
    output[2] = @truncate(ts_u64 >> 40);
    output[3] = @truncate(ts_u64 >> 32);
    output[4] = @truncate(ts_u64 >> 24);
    output[5] = @truncate(ts_u64 >> 16);
    output[6] = @truncate(ts_u64 >> 8);
    output[7] = @truncate(ts_u64);

    // Opus frame data
    @memcpy(output[HEADER_SIZE..][0..opus_frame.len], opus_frame);

    return total_size;
}

/// Unstamp an opus frame, extracting timestamp and frame data
/// The frame slice points into the input buffer (zero-copy)
pub fn unstampFrame(data: []const u8) WireError!StampedFrame {
    if (data.len < HEADER_SIZE) {
        return WireError.InvalidData;
    }

    // Check version
    const version = data[0];
    if (version != VERSION) {
        return WireError.InvalidVersion;
    }

    // Parse timestamp: 7 bytes big-endian
    const ts_u64: u64 =
        (@as(u64, data[1]) << 48) |
        (@as(u64, data[2]) << 40) |
        (@as(u64, data[3]) << 32) |
        (@as(u64, data[4]) << 24) |
        (@as(u64, data[5]) << 16) |
        (@as(u64, data[6]) << 8) |
        @as(u64, data[7]);

    const timestamp_ms: i64 = @bitCast(ts_u64);

    return StampedFrame{
        .timestamp_ms = timestamp_ms,
        .frame = data[HEADER_SIZE..],
    };
}

/// Calculate the required buffer size for stamping a frame
pub fn stampedSize(opus_frame_len: usize) usize {
    return HEADER_SIZE + opus_frame_len;
}

// ============================================================================
// JSON Encoding/Decoding for Events
// ============================================================================

/// JSON buffer size for state event
pub const STATE_EVENT_JSON_SIZE: usize = 256;

/// JSON buffer size for stats event
pub const STATS_EVENT_JSON_SIZE: usize = 1024;

/// Encode a StateEvent to JSON
/// Returns the number of bytes written
pub fn encodeStateEvent(event: *const types.StateEvent, output: []u8) !usize {
    var stream = std.io.fixedBufferStream(output);
    var writer = std.json.writeStream(stream.writer(), .{});

    try writer.beginObject();

    try writer.objectField("version");
    try writer.write(event.version);

    try writer.objectField("time");
    try writer.write(event.time);

    try writer.objectField("state");
    try writer.write(event.state.toString());

    if (event.cause) |cause| {
        try writer.objectField("cause");
        try writer.beginObject();
        if (cause.calling_initiated) {
            try writer.objectField("calling_initiated");
            try writer.write(true);
        }
        if (cause.calling_resume) {
            try writer.objectField("calling_resume");
            try writer.write(true);
        }
        try writer.endObject();
    }

    try writer.objectField("update_at");
    try writer.write(event.update_at);

    try writer.endObject();

    return stream.pos;
}

/// Parse a CommandEvent from JSON
/// Note: This is a simplified parser - real implementation would need full JSON parsing
pub fn parseCommandEvent(json: []const u8, event: *types.CommandEvent) !void {
    // Simplified: look for known fields
    // For production, use a proper JSON parser

    // Find cmd_type
    if (std.mem.indexOf(u8, json, "\"streaming\"")) |_| {
        event.cmd_type = .streaming;
        // Look for boolean value
        if (std.mem.indexOf(u8, json, "\"payload\":true") orelse
            std.mem.indexOf(u8, json, "\"payload\": true"))
        |_| {
            event.payload = .{ .streaming = true };
        } else {
            event.payload = .{ .streaming = false };
        }
    } else if (std.mem.indexOf(u8, json, "\"reset\"")) |_| {
        event.cmd_type = .reset;
        event.payload = .{ .reset = .{} };
    } else if (std.mem.indexOf(u8, json, "\"raise\"")) |_| {
        event.cmd_type = .raise;
        event.payload = .{ .raise = .{} };
    } else if (std.mem.indexOf(u8, json, "\"halt\"")) |_| {
        event.cmd_type = .halt;
        event.payload = .{ .halt = .{} };
    } else if (std.mem.indexOf(u8, json, "\"set_volume\"")) |_| {
        event.cmd_type = .set_volume;
        event.payload = .{ .set_volume = 50 }; // Default, would parse actual value
    } else if (std.mem.indexOf(u8, json, "\"set_brightness\"")) |_| {
        event.cmd_type = .set_brightness;
        event.payload = .{ .set_brightness = 50 }; // Default
    } else {
        return error.UnknownCommand;
    }

    // Parse time fields (simplified)
    event.time = 0;
    event.issue_at = 0;
}

// ============================================================================
// Tests
// ============================================================================

test "stampFrame and unstampFrame roundtrip" {
    const timestamp: i64 = 1706745600000; // 2024-02-01 00:00:00 UTC
    const opus_data = [_]u8{ 0x48, 0x61, 0x69, 0x56, 0x69, 0x56, 0x69 };

    var buf: [64]u8 = undefined;
    const written = try stampFrame(timestamp, &opus_data, &buf);

    std.debug.assert(written == HEADER_SIZE + opus_data.len);

    const unstamped = try unstampFrame(buf[0..written]);
    std.debug.assert(unstamped.timestamp_ms == timestamp);
    std.debug.assert(std.mem.eql(u8, unstamped.frame, &opus_data));
}

test "encodeStateEvent" {
    const event = types.StateEvent{
        .version = 1,
        .time = 1706745600000,
        .state = .ready,
        .cause = null,
        .update_at = 1706745600000,
    };

    var buf: [STATE_EVENT_JSON_SIZE]u8 = undefined;
    const written = try encodeStateEvent(&event, &buf);

    const json = buf[0..written];
    try std.testing.expect(std.mem.indexOf(u8, json, "\"ready\"") != null);
    try std.testing.expect(std.mem.indexOf(u8, json, "\"version\":1") != null);
}

test "stampFrame buffer too small" {
    const opus_data = [_]u8{ 0x01, 0x02, 0x03 };
    var small_buf: [5]u8 = undefined; // Too small (need 8 + 3 = 11)

    const result = stampFrame(0, &opus_data, &small_buf);
    try std.testing.expectError(WireError.BufferTooSmall, result);
}

test "unstampFrame invalid data (too short)" {
    const short_data = [_]u8{ 0x01, 0x02, 0x03 }; // Less than HEADER_SIZE

    const result = unstampFrame(&short_data);
    try std.testing.expectError(WireError.InvalidData, result);
}

test "unstampFrame invalid version" {
    var bad_version = [_]u8{ 0x99, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 }; // Version 0x99

    const result = unstampFrame(&bad_version);
    try std.testing.expectError(WireError.InvalidVersion, result);
}

test "stampFrame large timestamp" {
    // Large timestamp: ~year 3000
    const large_ts: i64 = 32503680000000;
    const opus_data = [_]u8{0xAB};

    var buf: [64]u8 = undefined;
    const written = try stampFrame(large_ts, &opus_data, &buf);

    const unstamped = try unstampFrame(buf[0..written]);
    try std.testing.expectEqual(large_ts, unstamped.timestamp_ms);
}

test "stampedSize calculation" {
    try std.testing.expectEqual(@as(usize, 8), stampedSize(0));
    try std.testing.expectEqual(@as(usize, 88), stampedSize(80)); // Typical opus frame
    try std.testing.expectEqual(@as(usize, 1032), stampedSize(1024));
}

test "parseCommandEvent streaming true" {
    const json =
        \\{"cmd_type":"streaming","payload":true,"time":123,"issue_at":456}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.streaming, event.cmd_type);
    try std.testing.expectEqual(true, event.payload.streaming);
}

test "parseCommandEvent streaming false" {
    const json =
        \\{"cmd_type":"streaming","payload":false}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.streaming, event.cmd_type);
    try std.testing.expectEqual(false, event.payload.streaming);
}

test "parseCommandEvent reset" {
    const json =
        \\{"cmd_type":"reset"}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.reset, event.cmd_type);
}

test "parseCommandEvent raise" {
    const json =
        \\{"cmd_type":"raise"}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.raise, event.cmd_type);
}

test "parseCommandEvent halt" {
    const json =
        \\{"cmd_type":"halt"}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.halt, event.cmd_type);
}

test "parseCommandEvent unknown command" {
    const json =
        \\{"cmd_type":"unknown_cmd"}
    ;
    var event: types.CommandEvent = undefined;
    const result = parseCommandEvent(json, &event);

    try std.testing.expectError(error.UnknownCommand, result);
}
