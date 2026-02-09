//! Platform Configuration — ChatGear E2E Test

const hal = @import("hal");
const build_options = @import("build_options");

/// Hardware implementation (board-specific)
pub const hw = switch (build_options.board) {
    .korvo2_v3 => @import("esp/korvo2_v3.zig"),
};

/// Button IDs for ADC button group (Korvo-2 V3)
pub const ButtonId = enum(u8) {
    vol_up = 0,
    vol_down = 1,
    set = 2,
    play = 3,
    mute = 4,
    rec = 5,

    pub fn name(self: @This()) []const u8 {
        return switch (self) {
            .vol_up => "VOL+",
            .vol_down => "VOL-",
            .set => "SET",
            .play => "PLAY",
            .mute => "MUTE",
            .rec => "REC",
        };
    }
};

const OuterButtonId = ButtonId;

const spec = struct {
    pub const meta = .{ .id = hw.Hardware.name };

    // Button ID type (required for button_group)
    pub const ButtonId = OuterButtonId;

    // Required primitives
    pub const rtc = hal.rtc.reader.from(hw.rtc_spec);
    pub const log = hw.log;
    pub const time = hw.time;
    pub const isRunning = hw.isRunning;

    // WiFi HAL peripheral (802.11 layer events)
    pub const wifi = hal.wifi.from(hw.wifi_spec);

    // Net HAL peripheral (IP events)
    pub const net = hal.net.from(hw.net_spec);

    // Socket trait (for MQTT/TCP)
    pub const socket = hw.socket;

    // ADC buttons
    pub const buttons = hal.button_group.from(hw.button_group_spec, OuterButtonId);
};

pub const Board = hal.Board(spec);
