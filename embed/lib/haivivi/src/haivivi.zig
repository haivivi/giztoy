//! Haivivi SDK for Embedded Zig
//!
//! This module provides APIs for Haivivi services:
//! - palgear: Device authentication, settings, chat mode, points, firmware

pub const palgear = @import("palgear.zig");

test {
    @import("std").testing.refAllDecls(@This());
}
