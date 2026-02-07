//! Chatgear Protocol Types
//!
//! Core types for the chatgear device-server communication protocol.
//! Based on go/pkg/chatgear.

const std = @import("std");

// ============================================================================
// Device State
// ============================================================================

/// Device state enumeration
pub const State = enum(u8) {
    unknown = 0,
    shutting_down = 1,
    sleeping = 2,
    resetting = 3,
    ready = 4,
    recording = 5,
    waiting_for_response = 6,
    streaming = 7,
    calling = 8,
    interrupted = 9,

    /// Returns the string representation of the state
    pub fn toString(self: State) []const u8 {
        return switch (self) {
            .unknown => "unknown",
            .shutting_down => "shutting_down",
            .sleeping => "sleeping",
            .resetting => "resetting",
            .ready => "ready",
            .recording => "recording",
            .waiting_for_response => "waiting_for_response",
            .streaming => "streaming",
            .calling => "calling",
            .interrupted => "interrupted",
        };
    }

    /// Parse state from string
    pub fn fromString(s: []const u8) State {
        const map = std.StaticStringMap(State).initComptime(.{
            .{ "unknown", .unknown },
            .{ "shutting_down", .shutting_down },
            .{ "sleeping", .sleeping },
            .{ "resetting", .resetting },
            .{ "ready", .ready },
            .{ "recording", .recording },
            .{ "waiting_for_response", .waiting_for_response },
            .{ "streaming", .streaming },
            .{ "calling", .calling },
            .{ "interrupted", .interrupted },
        });
        return map.get(s) orelse .unknown;
    }

    /// Returns true if the device is in an active (non-idle) state
    pub fn isActive(self: State) bool {
        return switch (self) {
            .recording, .waiting_for_response, .streaming, .calling => true,
            else => false,
        };
    }

    /// Returns true if the device can start recording in this state
    pub fn canRecord(self: State) bool {
        return self == .ready or self == .streaming;
    }
};

// ============================================================================
// State Event (Uplink: Device -> Server)
// ============================================================================

/// State change cause information
pub const StateChangeCause = struct {
    calling_initiated: bool = false,
    calling_resume: bool = false,
};

/// State event sent from device to server
pub const StateEvent = struct {
    version: u8 = 1,
    time: i64, // epoch milliseconds
    state: State,
    cause: ?StateChangeCause = null,
    update_at: i64, // epoch milliseconds
};

// ============================================================================
// Commands (Downlink: Server -> Device)
// ============================================================================

/// Command type enumeration
pub const CommandType = enum {
    streaming,
    reset,
    set_volume,
    set_brightness,
    set_light_mode,
    set_wifi,
    delete_wifi,
    ota_upgrade,
    raise,
    halt,

    pub fn toString(self: CommandType) []const u8 {
        return switch (self) {
            .streaming => "streaming",
            .reset => "reset",
            .set_volume => "set_volume",
            .set_brightness => "set_brightness",
            .set_light_mode => "set_light_mode",
            .set_wifi => "set_wifi",
            .delete_wifi => "delete_wifi",
            .ota_upgrade => "ota_upgrade",
            .raise => "raise",
            .halt => "halt",
        };
    }

    pub fn fromString(s: []const u8) ?CommandType {
        const map = std.StaticStringMap(CommandType).initComptime(.{
            .{ "streaming", .streaming },
            .{ "reset", .reset },
            .{ "set_volume", .set_volume },
            .{ "set_brightness", .set_brightness },
            .{ "set_light_mode", .set_light_mode },
            .{ "set_wifi", .set_wifi },
            .{ "delete_wifi", .delete_wifi },
            .{ "ota_upgrade", .ota_upgrade },
            .{ "raise", .raise },
            .{ "halt", .halt },
        });
        return map.get(s);
    }
};

/// Reset command payload
pub const ResetPayload = struct {
    unpair: bool = false,
};

/// Raise command payload
pub const RaisePayload = struct {
    call: bool = false,
};

/// Halt command payload
pub const HaltPayload = struct {
    sleep: bool = false,
    shutdown: bool = false,
    interrupt: bool = false,
};

/// WiFi configuration payload
pub const SetWifiPayload = struct {
    ssid: []const u8,
    security: []const u8,
    password: []const u8,
};

/// Command payload union
pub const CommandPayload = union(CommandType) {
    streaming: bool,
    reset: ResetPayload,
    set_volume: i32,
    set_brightness: i32,
    set_light_mode: []const u8,
    set_wifi: SetWifiPayload,
    delete_wifi: []const u8,
    ota_upgrade: void, // Complex, handle separately
    raise: RaisePayload,
    halt: HaltPayload,
};

/// Command event received from server
pub const CommandEvent = struct {
    cmd_type: CommandType,
    time: i64, // epoch milliseconds
    payload: CommandPayload,
    issue_at: i64, // epoch milliseconds
};

// ============================================================================
// Stamped Opus Frame
// ============================================================================

/// Stamped opus frame for audio streaming
pub const StampedFrame = struct {
    timestamp_ms: i64,
    frame: []const u8,
};

// ============================================================================
// Stats Event (Uplink: Device -> Server) - Simplified
// ============================================================================

/// Battery status
pub const Battery = struct {
    percentage: f32 = 0,
    is_charging: bool = false,
    voltage: f32 = 0,
    temperature: f32 = 0,
};

/// Volume settings
pub const Volume = struct {
    percentage: f32 = 0,
    update_at: i64 = 0,
};

/// Brightness settings
pub const Brightness = struct {
    percentage: f32 = 0,
    update_at: i64 = 0,
};

/// Light mode settings
pub const LightMode = struct {
    mode: []const u8 = "",
    update_at: i64 = 0,
};

/// Connected WiFi information
pub const ConnectedWifi = struct {
    ssid: []const u8 = "",
    ip: []const u8 = "",
    rssi: f32 = 0,
};

/// System version information
pub const SystemVersion = struct {
    current_version: []const u8 = "",
    update_at: i64 = 0,
};

/// Stats event - simplified for embedded
pub const StatsEvent = struct {
    time: i64 = 0,
    last_reset_at: i64 = 0,
    battery: ?Battery = null,
    system_version: ?SystemVersion = null,
    volume: ?Volume = null,
    brightness: ?Brightness = null,
    light_mode: ?LightMode = null,
    wifi_network: ?ConnectedWifi = null,
};

// ============================================================================
// Message Types for Receive
// ============================================================================

/// Message type enumeration
pub const MessageType = enum {
    opus_frame,
    command,
};

/// Message received from server (downlink)
pub const Message = union(MessageType) {
    opus_frame: StampedFrame,
    command: CommandEvent,
};

// ============================================================================
// Tests
// ============================================================================

test "State.toString and fromString roundtrip" {
    const states = [_]State{
        .unknown, .shutting_down, .sleeping, .resetting, .ready,
        .recording, .waiting_for_response, .streaming, .calling, .interrupted,
    };

    for (states) |state| {
        const str = state.toString();
        const parsed = State.fromString(str);
        try std.testing.expectEqual(state, parsed);
    }
}

test "State.fromString unknown input returns unknown" {
    try std.testing.expectEqual(State.unknown, State.fromString("invalid_state"));
    try std.testing.expectEqual(State.unknown, State.fromString(""));
}

test "State.isActive" {
    // Active states
    try std.testing.expect(State.recording.isActive());
    try std.testing.expect(State.waiting_for_response.isActive());
    try std.testing.expect(State.streaming.isActive());
    try std.testing.expect(State.calling.isActive());

    // Inactive states
    try std.testing.expect(!State.unknown.isActive());
    try std.testing.expect(!State.ready.isActive());
    try std.testing.expect(!State.sleeping.isActive());
    try std.testing.expect(!State.interrupted.isActive());
}

test "State.canRecord" {
    try std.testing.expect(State.ready.canRecord());
    try std.testing.expect(State.streaming.canRecord());

    try std.testing.expect(!State.recording.canRecord());
    try std.testing.expect(!State.sleeping.canRecord());
    try std.testing.expect(!State.unknown.canRecord());
}

test "CommandType.toString and fromString roundtrip" {
    const cmd_types = [_]CommandType{
        .streaming, .reset, .set_volume, .set_brightness,
        .set_light_mode, .set_wifi, .delete_wifi, .ota_upgrade, .raise, .halt,
    };

    for (cmd_types) |cmd| {
        const str = cmd.toString();
        const parsed = CommandType.fromString(str);
        try std.testing.expectEqual(cmd, parsed.?);
    }
}

test "CommandType.fromString unknown input returns null" {
    try std.testing.expectEqual(@as(?CommandType, null), CommandType.fromString("invalid_cmd"));
    try std.testing.expectEqual(@as(?CommandType, null), CommandType.fromString(""));
}
