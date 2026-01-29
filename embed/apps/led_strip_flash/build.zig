const std = @import("std");

/// Supported board types (must match embed-zig's board names)
pub const BoardType = enum {
    korvo2_v3,
    esp32s3_devkit,
};

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Board selection option
    const board = b.option(BoardType, "board", "Target board") orelse .esp32s3_devkit;

    // Get dependencies from embed-zig
    const esp_dep = b.dependency("esp", .{
        .target = target,
        .optimize = optimize,
    });

    const hal_dep = b.dependency("hal", .{
        .target = target,
        .optimize = optimize,
    });

    const drivers_dep = b.dependency("drivers", .{
        .target = target,
        .optimize = optimize,
    });

    // Create app module
    const app_module = b.addModule("app", .{
        .root_source_file = b.path("app.zig"),
        .target = target,
        .optimize = optimize,
    });

    // Add dependencies
    app_module.addImport("esp", esp_dep.module("esp"));
    app_module.addImport("hal", hal_dep.module("hal"));
    app_module.addImport("drivers", drivers_dep.module("drivers"));

    // Board selection options
    const board_options = b.addOptions();
    board_options.addOption(BoardType, "board", board);
    app_module.addOptions("build_options", board_options);
}
