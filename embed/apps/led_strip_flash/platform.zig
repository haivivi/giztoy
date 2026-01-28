//! Board Configuration - HAL Abstraction Layer
//!
//! This file provides platform HAL abstraction by selecting the appropriate
//! board implementation based on build options.

const hal = @import("hal");
const build_options = @import("build_options");

/// Supported board types
pub const BoardType = build_options.@"build.BoardType";

/// Currently selected board (from build options)
pub const selected_board: BoardType = build_options.board;

/// Hardware implementation for the selected board
const hw = switch (selected_board) {
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

    // HAL peripherals
    pub const rgb_leds = hal.led_strip.from(hw.led_spec);
};

/// HAL Board type with all peripherals
pub const Board = hal.Board(spec);
