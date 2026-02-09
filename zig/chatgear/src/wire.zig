//! Chatgear Wire Format
//!
//! Encoding/decoding for the chatgear wire protocol.
//! Based on go/pkg/chatgear wire format.
//!
//! Stamped Opus Frame Wire Format (matches Go conn_mqtt.go):
//! +--------+------------------+------------------+
//! | Version| Timestamp (7B)   | Opus Frame Data  |
//! | (1B)   | Big-endian ms    |                  |
//! +--------+------------------+------------------+
//!
//! JSON Event Format (matches Go command.go / state.go):
//! - StateEvent: {"v":1,"t":123,"s":"ready","ut":123}
//! - CommandEvent: {"type":"streaming","time":123,"pld":true,"issue_at":456}
//! - StatsEvent: {"time":123,"battery":{"percentage":80},...}

const std = @import("std");
const types = @import("types.zig");

const StampedFrame = types.StampedFrame;

/// Wire format version
pub const VERSION: u8 = 1;

/// Header size: 1 byte version + 7 bytes timestamp
pub const HEADER_SIZE: usize = 8;

/// Maximum opus frame size (typical is ~80 bytes for voice)
pub const MAX_OPUS_FRAME_SIZE: usize = 1024;

/// JSON buffer size for state event
pub const STATE_EVENT_JSON_SIZE: usize = 256;

/// JSON buffer size for stats event
pub const STATS_EVENT_JSON_SIZE: usize = 1024;

/// Error type for wire operations
pub const WireError = error{
    BufferTooSmall,
    InvalidVersion,
    InvalidData,
};

// ============================================================================
// Binary Frame Encoding (Opus audio)
// ============================================================================

/// Stamp an opus frame with timestamp.
/// Returns the number of bytes written to output buffer.
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

    // Timestamp: 7 bytes big-endian (lower 7 bytes of u64)
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

/// Unstamp an opus frame, extracting timestamp and frame data.
/// The frame slice points into the input buffer (zero-copy).
pub fn unstampFrame(data: []const u8) WireError!StampedFrame {
    if (data.len < HEADER_SIZE) {
        return WireError.InvalidData;
    }

    if (data[0] != VERSION) {
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

    return StampedFrame{
        .timestamp_ms = @bitCast(ts_u64),
        .frame = data[HEADER_SIZE..],
    };
}

/// Calculate the required buffer size for stamping a frame.
pub fn stampedSize(opus_frame_len: usize) usize {
    return HEADER_SIZE + opus_frame_len;
}

// ============================================================================
// JSON Encoding — State Event
// ============================================================================

/// Encode a StateEvent to JSON using fmt.bufPrint.
/// Go format: {"v":1,"t":123,"s":"ready","c":{...},"ut":123}
/// Returns the number of bytes written.
pub fn encodeStateEvent(event: *const types.StateEvent, output: []u8) !usize {
    var fbs = std.io.fixedBufferStream(output);
    const w = fbs.writer();

    try w.print("{{\"v\":{d},\"t\":{d}", .{ event.version, event.time });
    try w.writeAll(",\"s\":\"");
    try w.writeAll(event.state.toString());
    try w.writeByte('"');

    if (event.cause) |cause| {
        try w.writeAll(",\"c\":{");
        var first = true;
        if (cause.calling_initiated) {
            try w.writeAll("\"calling_initiated\":true");
            first = false;
        }
        if (cause.calling_resume) {
            if (!first) try w.writeByte(',');
            try w.writeAll("\"calling_resume\":true");
        }
        try w.writeByte('}');
    }

    try w.print(",\"ut\":{d}}}", .{event.update_at});

    return fbs.pos;
}

// ============================================================================
// JSON Encoding — Stats Event
// ============================================================================

/// Encode a StatsEvent to JSON using fmt.
/// Only encodes non-null fields (diff upload).
/// Returns the number of bytes written.
pub fn encodeStatsEvent(event: *const types.StatsEvent, output: []u8) !usize {
    var fbs = std.io.fixedBufferStream(output);
    const w = fbs.writer();

    try w.print("{{\"time\":{d}", .{event.time});

    if (event.last_reset_at != 0) {
        try w.print(",\"last_reset_at\":{d}", .{event.last_reset_at});
    }

    if (event.battery) |bat| {
        try w.print(",\"battery\":{{\"percentage\":{d},\"is_charging\":{s}}}", .{
            @as(i32, @intFromFloat(bat.percentage)),
            if (bat.is_charging) @as([]const u8, "true") else "false",
        });
    }

    if (event.system_version) |sv| {
        try w.writeAll(",\"system_version\":{\"current_version\":\"");
        try w.writeAll(sv.current_version);
        try w.writeAll("\"}");
    }

    if (event.volume) |vol| {
        try w.print(",\"volume\":{{\"percentage\":{d},\"update_at\":{d}}}", .{ @as(i32, @intFromFloat(vol.percentage)), vol.update_at });
    }

    if (event.brightness) |br| {
        try w.print(",\"brightness\":{{\"percentage\":{d},\"update_at\":{d}}}", .{ @as(i32, @intFromFloat(br.percentage)), br.update_at });
    }

    if (event.light_mode) |lm| {
        try w.print(",\"light_mode\":{{\"mode\":\"{s}\",\"update_at\":{d}}}", .{ lm.mode, lm.update_at });
    }

    if (event.wifi_network) |wifi| {
        try w.print(",\"wifi_network\":{{\"ssid\":\"{s}\",\"ip\":\"{s}\",\"rssi\":{d}}}", .{
            wifi.ssid,
            wifi.ip,
            @as(i32, @intFromFloat(wifi.rssi)),
        });
    }

    if (event.pair_status) |ps| {
        try w.print(",\"pair_status\":{{\"pair_with\":\"{s}\",\"update_at\":{d}}}", .{ ps.pair_with, ps.update_at });
    }

    if (event.shaking) |sh| {
        try w.print(",\"shaking\":{{\"level\":{d}}}", .{@as(i32, @intFromFloat(sh.level))});
    }

    try w.writeByte('}');

    return fbs.pos;
}

// ============================================================================
// JSON Parsing — Command Event
// ============================================================================

/// Parse a CommandEvent from JSON.
/// Go format: {"type":"streaming","time":123,"pld":true,"issue_at":456}
///
/// Uses simple string matching — no allocator needed.
/// Handles the most common command types for device-side parsing.
pub fn parseCommandEvent(json: []const u8, event: *types.CommandEvent) !void {
    event.time = 0;
    event.issue_at = 0;

    // Match "type":"<command_name>" to determine command type
    if (findStringValue(json, "\"type\"")) |type_str| {
        if (types.CommandType.fromString(type_str)) |ct| {
            event.cmd_type = ct;
            switch (ct) {
                .streaming => {
                    // pld is a boolean
                    event.payload = .{ .streaming = findBoolAfter(json, "\"pld\"") };
                },
                .set_volume => {
                    event.payload = .{ .set_volume = findIntAfter(json, "\"pld\"") orelse 0 };
                },
                .set_brightness => {
                    event.payload = .{ .set_brightness = findIntAfter(json, "\"pld\"") orelse 0 };
                },
                .reset => event.payload = .{ .reset = .{} },
                .raise => {
                    event.payload = .{ .raise = .{ .call = findBoolAfter(json, "\"call\"") } };
                },
                .halt => {
                    event.payload = .{ .halt = .{
                        .sleep = findBoolAfter(json, "\"sleep\""),
                        .shutdown = findBoolAfter(json, "\"shutdown\""),
                        .interrupt = findBoolAfter(json, "\"interrupt\""),
                    } };
                },
                .set_light_mode => event.payload = .{ .set_light_mode = "" },
                .set_wifi => event.payload = .{ .set_wifi = .{
                    .ssid = "",
                    .security = "",
                    .password = "",
                } },
                .delete_wifi => event.payload = .{ .delete_wifi = "" },
                .ota_upgrade => event.payload = .{ .ota_upgrade = {} },
            }
            return;
        }
    }
    return error.UnknownCommand;
}

// ============================================================================
// JSON Helpers (no-alloc string matching)
// ============================================================================

/// Find a JSON string value after a key. Returns the unquoted string slice
/// pointing into the input buffer, or null if not found.
fn findStringValue(json: []const u8, key: []const u8) ?[]const u8 {
    const key_pos = std.mem.indexOf(u8, json, key) orelse return null;
    const after_key = json[key_pos + key.len ..];

    // Skip ':'  and whitespace
    var i: usize = 0;
    while (i < after_key.len and (after_key[i] == ':' or after_key[i] == ' ')) : (i += 1) {}
    if (i >= after_key.len or after_key[i] != '"') return null;
    i += 1; // skip opening quote

    const start = i;
    while (i < after_key.len and after_key[i] != '"') : (i += 1) {}
    if (i >= after_key.len) return null;

    return after_key[start..i];
}

/// Find a boolean value (true/false) after a key in JSON.
fn findBoolAfter(json: []const u8, key: []const u8) bool {
    const key_pos = std.mem.indexOf(u8, json, key) orelse return false;
    const after_key = json[key_pos + key.len ..];

    // Skip ':' and whitespace
    var i: usize = 0;
    while (i < after_key.len and (after_key[i] == ':' or after_key[i] == ' ')) : (i += 1) {}

    if (i + 4 <= after_key.len and std.mem.eql(u8, after_key[i..][0..4], "true")) {
        return true;
    }
    return false;
}

/// Find an integer value after a key in JSON.
fn findIntAfter(json: []const u8, key: []const u8) ?i32 {
    const key_pos = std.mem.indexOf(u8, json, key) orelse return null;
    const after_key = json[key_pos + key.len ..];

    // Skip ':' and whitespace
    var i: usize = 0;
    while (i < after_key.len and (after_key[i] == ':' or after_key[i] == ' ')) : (i += 1) {}

    // Parse integer (possibly negative)
    var negative = false;
    if (i < after_key.len and after_key[i] == '-') {
        negative = true;
        i += 1;
    }

    var val: i32 = 0;
    var found = false;
    while (i < after_key.len and after_key[i] >= '0' and after_key[i] <= '9') : (i += 1) {
        val = val * 10 + @as(i32, after_key[i] - '0');
        found = true;
    }

    if (!found) return null;
    return if (negative) -val else val;
}

// ============================================================================
// Tests
// ============================================================================

test "stampFrame and unstampFrame roundtrip" {
    const timestamp: i64 = 1706745600000; // 2024-02-01 00:00:00 UTC
    const opus_data = [_]u8{ 0x48, 0x61, 0x69, 0x56, 0x69, 0x56, 0x69 };

    var buf: [64]u8 = undefined;
    const written = try stampFrame(timestamp, &opus_data, &buf);

    try std.testing.expectEqual(HEADER_SIZE + opus_data.len, written);

    const unstamped = try unstampFrame(buf[0..written]);
    try std.testing.expectEqual(timestamp, unstamped.timestamp_ms);
    try std.testing.expect(std.mem.eql(u8, unstamped.frame, &opus_data));
}

test "stampFrame buffer too small" {
    const opus_data = [_]u8{ 0x01, 0x02, 0x03 };
    var small_buf: [5]u8 = undefined;

    const result = stampFrame(0, &opus_data, &small_buf);
    try std.testing.expectError(WireError.BufferTooSmall, result);
}

test "unstampFrame invalid data (too short)" {
    const short_data = [_]u8{ 0x01, 0x02, 0x03 };
    const result = unstampFrame(&short_data);
    try std.testing.expectError(WireError.InvalidData, result);
}

test "unstampFrame invalid version" {
    const bad_version = [_]u8{ 0x99, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
    const result = unstampFrame(&bad_version);
    try std.testing.expectError(WireError.InvalidVersion, result);
}

test "stampFrame large timestamp" {
    const large_ts: i64 = 32503680000000; // ~year 3000
    const opus_data = [_]u8{0xAB};

    var buf: [64]u8 = undefined;
    const written = try stampFrame(large_ts, &opus_data, &buf);

    const unstamped = try unstampFrame(buf[0..written]);
    try std.testing.expectEqual(large_ts, unstamped.timestamp_ms);
}

test "stampedSize calculation" {
    try std.testing.expectEqual(@as(usize, 8), stampedSize(0));
    try std.testing.expectEqual(@as(usize, 88), stampedSize(80));
    try std.testing.expectEqual(@as(usize, 1032), stampedSize(1024));
}

test "encodeStateEvent basic" {
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
    try std.testing.expect(std.mem.indexOf(u8, json, "\"v\":1") != null);
}

test "encodeStatsEvent with battery" {
    const event = types.StatsEvent{
        .time = 1706745600000,
        .battery = .{ .percentage = 85, .is_charging = true },
        .volume = .{ .percentage = 50, .update_at = 1706745600000 },
    };

    var buf: [STATS_EVENT_JSON_SIZE]u8 = undefined;
    const written = try encodeStatsEvent(&event, &buf);
    const json = buf[0..written];

    try std.testing.expect(std.mem.indexOf(u8, json, "\"battery\"") != null);
    try std.testing.expect(std.mem.indexOf(u8, json, "\"volume\"") != null);
}

test "parseCommandEvent streaming true" {
    const json =
        \\{"type":"streaming","time":123,"pld":true,"issue_at":456}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.streaming, event.cmd_type);
    try std.testing.expectEqual(true, event.payload.streaming);
}

test "parseCommandEvent streaming false" {
    const json =
        \\{"type":"streaming","time":123,"pld":false,"issue_at":456}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.streaming, event.cmd_type);
    try std.testing.expectEqual(false, event.payload.streaming);
}

test "parseCommandEvent set_volume" {
    const json =
        \\{"type":"set_volume","time":123,"pld":75,"issue_at":456}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.set_volume, event.cmd_type);
    try std.testing.expectEqual(@as(i32, 75), event.payload.set_volume);
}

test "parseCommandEvent halt" {
    const json =
        \\{"type":"halt","time":123,"pld":{"sleep":true},"issue_at":456}
    ;
    var event: types.CommandEvent = undefined;
    try parseCommandEvent(json, &event);

    try std.testing.expectEqual(types.CommandType.halt, event.cmd_type);
    try std.testing.expectEqual(true, event.payload.halt.sleep);
}

test "parseCommandEvent unknown command" {
    const json =
        \\{"type":"unknown_cmd"}
    ;
    var event: types.CommandEvent = undefined;
    const result = parseCommandEvent(json, &event);
    try std.testing.expectError(error.UnknownCommand, result);
}
