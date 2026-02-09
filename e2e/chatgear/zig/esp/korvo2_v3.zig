//! Korvo-2 V3 Board Implementation for ChatGear E2E Test
//!
//! Hardware:
//! - WiFi Station mode (event-driven)
//! - BSD Sockets via LWIP (for MQTT TCP)
//! - 6 ADC buttons on ADC1 Channel 4
//! - I2S mic + speaker (Korvo-2 codec)

const std = @import("std");
const esp = @import("esp");
const hal = @import("hal");

const idf = esp.idf;
const impl = esp.impl;
const board = esp.boards.korvo2_v3;

// ============================================================================
// Hardware Info
// ============================================================================

pub const Hardware = struct {
    pub const name = board.name;
    pub const serial_port = board.serial_port;
    pub const adc_channel: idf.adc.AdcChannel = @enumFromInt(board.adc_channel);
};

// ============================================================================
// Socket Implementation (for MQTT TCP)
// ============================================================================

pub const socket = idf.socket.Socket;

// ============================================================================
// Runtime (for Channel, WaitGroup, Spawner)
// ============================================================================

pub const runtime = impl.runtime;

// ============================================================================
// RTC Driver
// ============================================================================

pub const RtcDriver = board.RtcDriver;

// ============================================================================
// WiFi + Net Drivers (Event-Driven)
// ============================================================================

pub const WifiDriver = impl.wifi.WifiDriver;
pub const NetDriver = impl.net.NetDriver;

// ============================================================================
// ADC Button Group Driver
// ============================================================================

pub const AdcReader = struct {
    const Self = @This();

    adc_unit: ?idf.adc.AdcOneshot = null,
    initialized: bool = false,

    pub fn init() !Self {
        var self = Self{};
        self.adc_unit = try idf.adc.AdcOneshot.init(.adc1);
        errdefer {
            if (self.adc_unit) |*unit| unit.deinit();
        }
        try self.adc_unit.?.configChannel(Hardware.adc_channel, .{
            .atten = .db_12,
            .bitwidth = .bits_12,
        });
        self.initialized = true;
        return self;
    }

    pub fn deinit(self: *Self) void {
        if (self.adc_unit) |*unit| {
            unit.deinit();
            self.adc_unit = null;
        }
        self.initialized = false;
    }

    pub fn readRaw(self: *Self) u16 {
        if (self.adc_unit) |unit| {
            const raw = unit.read(Hardware.adc_channel) catch return 4095;
            return if (raw > 0) @intCast(raw) else 4095;
        }
        return 4095;
    }
};

// ============================================================================
// HAL Specs
// ============================================================================

pub const rtc_spec = struct {
    pub const Driver = RtcDriver;
    pub const meta = .{ .id = "rtc" };
};

pub const wifi_spec = impl.wifi.wifi_spec;
pub const net_spec = impl.net.net_spec;

/// Button group spec with ADC ranges.
/// Calibrated for Korvo-2 V3.1 board.
pub const button_group_spec = struct {
    pub const Driver = AdcReader;

    /// ADC value ranges (12-bit raw values)
    pub const ranges = &[_]hal.button_group.Range{
        .{ .id = 0, .min = 250, .max = 600 }, // vol_up
        .{ .id = 1, .min = 750, .max = 1100 }, // vol_down
        .{ .id = 2, .min = 1110, .max = 1500 }, // set
        .{ .id = 3, .min = 1510, .max = 2100 }, // play
        .{ .id = 4, .min = 2110, .max = 2550 }, // mute
        .{ .id = 5, .min = 2650, .max = 3100 }, // rec
    };

    pub const ref_value: u16 = 4095;
    pub const ref_tolerance: u16 = 500;

    pub const meta = .{ .id = "buttons.adc" };
};

// ============================================================================
// Platform Primitives
// ============================================================================

pub const log = std.log.scoped(.chatgear);
pub const time = board.time;

pub fn isRunning() bool {
    return board.isRunning();
}
