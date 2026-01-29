//! ESP32-S3 DevKit Board Implementation
//!
//! A basic development board with minimal peripherals.
//! Good for prototyping and learning.
//!
//! Hardware:
//! - WS2812 RGB LED on GPIO48

const std = @import("std");
const idf = @import("esp");
const hal = @import("hal");

// Platform primitives
pub const log = std.log.scoped(.app);

pub const time = struct {
    pub fn sleepMs(ms: u32) void {
        idf.sal.time.sleepMs(ms);
    }

    pub fn getTimeMs() u64 {
        return idf.nowMs();
    }
};

pub fn isRunning() bool {
    return true;
}

// Hardware parameters from embed-zig boards
const hw_params = idf.boards.esp32s3_devkit;

// ============================================================================
// Hardware Info
// ============================================================================

pub const Hardware = struct {
    pub const name = "esp32s3_devkit";
    pub const serial_port = hw_params.serial_port;
    pub const led_type = "ws2812";
    pub const led_count: u32 = 1;

    // LED GPIO (using led_strip_gpio from embed-zig board definition)
    pub const led_gpio: u8 = hw_params.led_strip_gpio;
};

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
        return idf.nowMs();
    }

    pub fn nowMs(_: *Self) ?i64 {
        return null;
    }
};

// ============================================================================
// LED Driver (WS2812 via RMT)
// ============================================================================

pub const LedDriver = struct {
    const Self = @This();

    strip: idf.LedStrip,
    initialized: bool = false,
    current_color: hal.Color = hal.Color.black,

    pub fn init() !Self {
        var self = Self{
            .strip = undefined,
        };

        self.strip = try idf.LedStrip.init(
            .{ .strip_gpio_num = Hardware.led_gpio, .max_leds = Hardware.led_count },
            .{ .resolution_hz = 10_000_000 },
        );

        self.initialized = true;
        std.log.info("DevKit LedDriver: WS2812 on GPIO{} initialized", .{Hardware.led_gpio});

        return self;
    }

    pub fn deinit(self: *Self) void {
        if (self.initialized) {
            self.strip.clear() catch |err| log.warn("failed to clear strip on deinit: {any}", .{err});
            self.strip.deinit();
            self.initialized = false;
        }
    }

    pub fn setPixel(self: *Self, index: u32, color: hal.Color) void {
        if (index >= Hardware.led_count) return;
        self.current_color = color;
        self.strip.setPixel(index, color.r, color.g, color.b) catch |err| log.err("failed to set pixel: {any}", .{err});
    }

    pub fn getPixelCount(_: *Self) u32 {
        return Hardware.led_count;
    }

    pub fn refresh(self: *Self) void {
        self.strip.refresh() catch |err| log.err("failed to refresh strip: {any}", .{err});
    }
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
