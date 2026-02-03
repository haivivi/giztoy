//! Board Configuration - HAL Abstraction Layer (Event-Driven)
//!
//! This file provides platform HAL abstraction by selecting the appropriate
//! board implementation based on build options.

const std = @import("std");
const hal = @import("hal");
const esp = @import("esp");
const build_options = @import("build_options");

/// Supported board types (derived from build options)
pub const BoardType = @TypeOf(build_options.board);

/// Currently selected board (from build options)
pub const selected_board: BoardType = build_options.board;

/// Hardware implementation for the selected board
pub const hw = switch (selected_board) {
    .korvo2_v3 => @import("boards/korvo2.zig"),
    .esp32s3_devkit => @import("boards/devkit.zig"),
};

/// Board specification for HAL
const spec = struct {
    pub const meta = .{ .id = hw.Hardware.name };

    // Required primitives
    pub const rtc = hal.rtc.reader.from(hw.rtc_spec);
    pub const log = hw.log;
    pub const time = hw.time;
    pub const isRunning = hw.isRunning;

    // HAL peripherals
    pub const rgb_leds = hal.led_strip.from(hw.led_spec);

    // WiFi HAL peripheral (802.11 layer events)
    pub const wifi = hal.wifi.from(hw.wifi_spec);

    // Net HAL peripheral (IP events, DNS)
    pub const net = hal.net.from(hw.net_spec);

    // Socket trait (for DNS resolver and TLS)
    pub const socket = hw.socket;

    // Crypto suite (mbedTLS-based, for TLS and signing)
    pub const crypto = hw.crypto;

    // Raw net impl for convenience functions
    pub const net_impl = hw.net;
};

/// HAL Board type with all peripherals
pub const Board = hal.Board(spec);
