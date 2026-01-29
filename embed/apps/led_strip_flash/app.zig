//! LED Strip Flash - Platform Independent App
//!
//! Simple LED strip flash demo.

const hal = @import("hal");
const platform = @import("platform.zig");

const Board = platform.Board;
const log = Board.log;

const BUILD_TAG = "giztoy_led_strip_flash_v1";

fn printBoardInfo() void {
    log.info("==========================================", .{});
    log.info("LED Strip Flash - giztoy", .{});
    log.info("Build Tag: {s}", .{BUILD_TAG});
    log.info("==========================================", .{});
    log.info("Board:     {s}", .{Board.meta.id});
    log.info("Build:     -DZIG_BOARD={s}", .{@tagName(platform.selected_board)});
    log.info("==========================================", .{});
}

pub fn run(_: anytype) void {
    printBoardInfo();

    // Initialize board (in-place to preserve driver pointers)
    var board: Board = undefined;
    board.init() catch |err| {
        log.err("Failed to initialize board: {}", .{err});
        return;
    };
    defer board.deinit();

    log.info("Board initialized", .{});

    // Flash the LED
    var state: bool = false;
    const brightness: u8 = 32;

    log.info("Starting flash loop (1 second interval)", .{});

    while (true) {
        state = !state;

        if (state) {
            board.rgb_leds.setColor(hal.Color.rgb(brightness, brightness, brightness));
        } else {
            board.rgb_leds.clear();
        }
        board.rgb_leds.refresh();

        log.info("LED: {s}", .{if (state) "ON" else "OFF"});
        Board.time.sleepMs(1000);
    }
}
