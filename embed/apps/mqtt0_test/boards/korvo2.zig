//! Korvo-2 Board Implementation
//!
//! ESP32-S3-Korvo-2 v3.1 audio development board
//!
//! Hardware:
//! - TCA9554 I2C GPIO expander with red/blue LEDs
//! - ES8311 audio codec
//! - WiFi Station mode (event-driven)
//! - BSD Sockets via LWIP
//! - Pure Zig TLS

const std = @import("std");
const esp = @import("esp");
const hal = @import("hal");
const drivers = @import("drivers");

const idf = esp.idf;
const impl = esp.impl;
const hw_params = esp.boards.korvo2_v3;

// ============================================================================
// Hardware Info
// ============================================================================

pub const Hardware = struct {
    pub const name = hw_params.name;
    pub const serial_port = hw_params.serial_port;
    pub const led_type = "tca9554";
    pub const led_count: u32 = 1;

    // I2C configuration
    pub const i2c_sda: u8 = hw_params.i2c_sda;
    pub const i2c_scl: u8 = hw_params.i2c_scl;
    pub const i2c_freq_hz: u32 = hw_params.i2c_freq_hz;
    pub const tca9554_addr: u7 = hw_params.tca9554_addr;

    // LED pins on TCA9554
    pub const led_red_pin = hw_params.led_red_pin;
    pub const led_blue_pin = hw_params.led_blue_pin;
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
// LED Driver (implements HAL LedStrip.Driver interface)
// ============================================================================

const I2c = impl.I2c;
const Tca9554 = drivers.Tca9554(*I2c);
const Pin = drivers.tca9554.Pin;
const RED_PIN = Pin.pin6;
const BLUE_PIN = Pin.pin7;

pub const LedDriver = struct {
    const Self = @This();

    i2c: I2c,
    gpio: Tca9554,
    initialized: bool = false,
    current_color: hal.Color = hal.Color.black,

    pub fn init() !Self {
        var self = Self{
            .i2c = undefined,
            .gpio = undefined,
        };

        // Initialize I2C bus
        self.i2c = try I2c.init(.{
            .sda = Hardware.i2c_sda,
            .scl = Hardware.i2c_scl,
            .freq_hz = Hardware.i2c_freq_hz,
        });
        errdefer self.i2c.deinit();

        // Initialize TCA9554 driver
        self.gpio = Tca9554.init(&self.i2c, Hardware.tca9554_addr);

        // Configure LED pins as outputs (active low, off initially)
        try self.gpio.configureOutput(RED_PIN, .high);
        try self.gpio.configureOutput(BLUE_PIN, .high);

        self.initialized = true;
        std.log.info("Korvo2 LedDriver: TCA9554 @ 0x{x} initialized", .{Hardware.tca9554_addr});

        return self;
    }

    pub fn deinit(self: *Self) void {
        if (self.initialized) {
            self.gpio.write(RED_PIN, .high) catch {};
            self.gpio.write(BLUE_PIN, .high) catch {};
            self.i2c.deinit();
            self.initialized = false;
        }
    }

    pub fn setPixel(self: *Self, index: u32, color: hal.Color) void {
        if (index > 0) return;

        self.current_color = color;

        const brightness = @max(color.r, @max(color.g, color.b));
        const threshold: u8 = 30;

        var red_on = false;
        var blue_on = false;

        if (brightness >= threshold) {
            if (color.r > color.b + 50) {
                red_on = true;
            } else if (color.b > color.r + 50) {
                blue_on = true;
            } else if (color.g > color.r and color.g > color.b) {
                blue_on = true;
            } else {
                red_on = true;
                blue_on = true;
            }
        }

        self.gpio.write(RED_PIN, if (red_on) .low else .high) catch {};
        self.gpio.write(BLUE_PIN, if (blue_on) .low else .high) catch {};
    }

    pub fn getPixelCount(_: *Self) u32 {
        return 1;
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

