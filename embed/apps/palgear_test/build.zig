const std = @import("std");

/// Supported board types (must match embed-zig's board names)
pub const BoardType = enum {
    korvo2_v3,
    esp32s3_devkit,
};

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Board selection option (only build-time option needed)
    const board = b.option(BoardType, "board", "Target board") orelse .korvo2_v3;

    // Get dependencies from embed-zig (injected by Bazel esp_zig_app rule)
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

    const dns_dep = b.dependency("dns", .{
        .target = target,
        .optimize = optimize,
    });

    const tls_dep = b.dependency("tls", .{
        .target = target,
        .optimize = optimize,
    });

    const http_dep = b.dependency("http", .{
        .target = target,
        .optimize = optimize,
    });

    const ntp_dep = b.dependency("ntp", .{
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
    app_module.addImport("dns", dns_dep.module("dns"));
    app_module.addImport("tls", tls_dep.module("tls"));
    app_module.addImport("http", http_dep.module("http"));
    app_module.addImport("ntp", ntp_dep.module("ntp"));

    // Build options (board selection only - config comes from env via Bazel)
    const build_options = b.addOptions();
    build_options.addOption(BoardType, "board", board);
    app_module.addOptions("build_options", build_options);
}
