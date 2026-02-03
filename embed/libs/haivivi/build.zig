const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Get dependencies
    const http_dep = b.dependency("http", .{
        .target = target,
        .optimize = optimize,
    });

    const trait_dep = b.dependency("trait", .{
        .target = target,
        .optimize = optimize,
    });

    // Create the haivivi module
    const haivivi_mod = b.addModule("haivivi", .{
        .root_source_file = b.path("src/haivivi.zig"),
        .target = target,
        .optimize = optimize,
    });
    haivivi_mod.addImport("http", http_dep.module("http"));
    haivivi_mod.addImport("trait", trait_dep.module("trait"));

    // Tests
    const test_step = b.step("test", "Run unit tests");
    const tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/haivivi.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    tests.root_module.addImport("http", http_dep.module("http"));
    tests.root_module.addImport("trait", trait_dep.module("trait"));
    const run_tests = b.addRunArtifact(tests);
    test_step.dependOn(&run_tests.step);
}
