const std = @import("std");
const idf = @import("esp");

const c = @cImport({
    @cInclude("sdkconfig.h");
});

const BLINK_GPIO = c.CONFIG_BLINK_GPIO;
const BUILD_TAG = "led_strip_zig_v4";

pub const std_options: std.Options = .{
    .logFn = idf.log.stdLogFn,
};

fn printMemoryStats() void {
    std.log.info("=== Heap Memory Statistics ===", .{});

    const internal = idf.heap.getInternalStats();
    std.log.info("Internal DRAM:", .{});
    std.log.info("  Total: {} bytes", .{internal.total});
    std.log.info("  Free:  {} bytes", .{internal.free});
    std.log.info("  Used:  {} bytes", .{internal.used});

    const psram = idf.heap.getPsramStats();
    if (psram.total > 0) {
        std.log.info("External PSRAM:", .{});
        std.log.info("  Total: {} bytes", .{psram.total});
        std.log.info("  Free:  {} bytes", .{psram.free});
        std.log.info("  Used:  {} bytes", .{psram.used});
    } else {
        std.log.info("External PSRAM: not available", .{});
    }
}

export fn app_main() void {
    std.log.info("==========================================", .{});
    std.log.info("  LED Strip Flash - Zig Version", .{});
    std.log.info("  Build Tag: {s}", .{BUILD_TAG});
    std.log.info("==========================================", .{});
    printMemoryStats();

    var strip = idf.LedStrip.init(
        .{ .strip_gpio_num = BLINK_GPIO, .max_leds = 1 },
        .{ .resolution_hz = 10_000_000 },
    ) catch {
        std.log.err("Failed to initialize LED strip", .{});
        return;
    };
    defer strip.deinit();

    strip.clear() catch {};

    var state: bool = false;
    while (true) {
        std.log.info("Toggling the LED {s}!", .{if (state) "ON" else "OFF"});

        if (state) {
            strip.clear() catch {};
        } else {
            strip.setPixelAndRefresh(0, 16, 16, 16) catch {};
        }
        state = !state;

        idf.delayMs(c.CONFIG_BLINK_PERIOD);
    }
}
