//! ESP32-S3 DevKit development board configuration
//!
//! A basic development board with minimal peripherals.
//! Good for prototyping and learning.

const std = @import("std");

/// Board metadata
pub const name = "devkit";
pub const chip = "esp32s3";
pub const description = "ESP32-S3 DevKit development board";

/// GPIO pin assignments
pub const gpio = struct {
    /// Built-in RGB LED (directly addressable)
    pub const led_rgb = 48;

    /// Boot button (directly readable)
    pub const btn_boot = 0;
};

/// LED controller
pub const led = struct {
    const Self = @This();

    pub fn init() void {
        // RGB LED is directly connected to GPIO48
        // Uses RMT peripheral for WS2812 control
    }

    pub fn setColor(r: u8, g: u8, b: u8) void {
        _ = r;
        _ = g;
        _ = b;
        // TODO: implement WS2812 control via RMT
    }
};

/// Button controller
pub const button = struct {
    pub fn init() void {
        // Configure GPIO0 as input with pull-up
    }

    pub fn isPressed() bool {
        // Boot button is active low
        return false; // TODO: read GPIO0
    }
};

/// Board initialization
pub fn init() void {
    led.init();
    button.init();
}

/// High-level board interface
pub const Board = struct {
    pub const Led = led;
    pub const Button = button;

    pub fn setup() void {
        init();
    }
};
