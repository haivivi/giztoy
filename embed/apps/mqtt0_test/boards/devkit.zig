//! ESP32-S3 DevKit Board Implementation
//!
//! Generic ESP32-S3 development board without I2C peripherals.
//! Uses no-op LED driver to avoid I2C errors.

const std = @import("std");
const esp = @import("esp");
const hal = @import("hal");

const idf = esp.idf;
const impl = esp.impl;
const hw_params = esp.boards.esp32s3_devkit;

// ============================================================================
// Hardware Info
// ============================================================================

pub const Hardware = struct {
    pub const name = hw_params.name;
    pub const serial_port = hw_params.serial_port;
    pub const led_type = "none";
    pub const led_count: u32 = 0;
};

// ============================================================================
// Socket Implementation (from ESP IDF)
// ============================================================================

pub const socket = idf.socket.Socket;

// ============================================================================
// Crypto Implementation (mbedTLS-based, hardware accelerated)
// ============================================================================

pub const crypto = impl.crypto.Suite;

// ============================================================================
// Network Interface Manager (implements hal.net)
// ============================================================================

pub const net = impl.net;

// ============================================================================
// RTC Driver (required by hal.Board)
// ============================================================================

pub const RtcDriver = struct {
    const Self = @This();

    pub fn init() !Self {
        return .{};
    }

    pub fn deinit(_: *Self) void {}

    pub fn uptime(_: *Self) u64 {
        return idf.time.nowMs();
    }

    pub fn nowMs(_: *Self) ?i64 {
        return null;
    }
};

// ============================================================================
// WiFi Driver (Event-Driven - uses ESP impl module)
// ============================================================================

pub const WifiDriver = impl.wifi.WifiDriver;

// ============================================================================
// Net Driver (for IP events and DNS)
// ============================================================================

pub const NetDriver = impl.net.NetDriver;

// ============================================================================
// LED Driver (No-op for DevKit - no I2C LEDs)
// ============================================================================

pub const LedDriver = struct {
    const Self = @This();

    pub fn init() !Self {
        std.log.info("DevKit: No LED driver (no I2C peripherals)", .{});
        return .{};
    }

    pub fn deinit(_: *Self) void {}

    pub fn setPixel(_: *Self, _: u32, _: hal.Color) void {}

    pub fn getPixelCount(_: *Self) u32 {
        return 0;
    }

    pub fn refresh(_: *Self) void {}
};

// ============================================================================
// HAL Specs
// ============================================================================

pub const rtc_spec = struct {
    pub const Driver = RtcDriver;
    pub const meta = .{ .id = "rtc" };
};

pub const led_spec = struct {
    pub const Driver = LedDriver;
    pub const meta = .{ .id = "led.main" };
};

pub const wifi_spec = impl.wifi.wifi_spec;
pub const net_spec = impl.net.net_spec;

// ============================================================================
// Platform Primitives
// ============================================================================

pub const log = std.log.scoped(.app);

pub const time = struct {
    pub fn sleepMs(ms: u32) void {
        idf.time.sleepMs(ms);
    }

    pub fn getTimeMs() u64 {
        return idf.time.nowMs();
    }
};

pub fn isRunning() bool {
    return true;
}

// ============================================================================
// Memory and Debug Helpers
// ============================================================================

pub const allocator = idf.heap.psram;

pub fn debugMemoryUsage(label: []const u8) void {
    const internal = idf.heap.getInternalStats();
    const psram = idf.heap.getPsramStats();
    log.info("[MEM:{s}] IRAM: {d}KB free | PSRAM: {d}KB free", .{
        label,
        internal.free / 1024,
        psram.free / 1024,
    });
}

pub fn debugStackUsage(label: []const u8, stack_size: usize) void {
    const stack = idf.heap.getTaskStackStats(null, stack_size);
    log.info("[STACK:{s}] used: {d}KB / {d}KB (high water: {d} bytes free)", .{
        label,
        stack.used / 1024,
        stack_size / 1024,
        stack.high_water,
    });
}

