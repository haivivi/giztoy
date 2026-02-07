const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Create the mqtt0 module (no external dependencies!)
    const mqtt0_mod = b.addModule("mqtt0", .{
        .root_source_file = b.path("src/mqtt0.zig"),
        .target = target,
        .optimize = optimize,
    });
    _ = mqtt0_mod;

    // Tests
    const test_step = b.step("test", "Run unit tests");
    const tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/mqtt0.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    const run_tests = b.addRunArtifact(tests);
    test_step.dependOn(&run_tests.step);
}
