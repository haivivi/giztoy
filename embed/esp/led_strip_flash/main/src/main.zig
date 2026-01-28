//! LED Strip Flash - ESP Platform Entry Point
//!
//! This file is the ESP-IDF entry point. It simply imports the
//! platform-independent app and calls its run function.

const std = @import("std");
const idf = @import("esp");
const app = @import("app");

pub const std_options: std.Options = .{
    .logFn = idf.log.stdLogFn,
};

export fn app_main() void {
    app.run();
}
