//! Palgear API Client
//!
//! Zig client for Haivivi Palgear API, supporting device authentication,
//! settings, chat mode, points, and firmware management.
//!
//! Based on OpenAPI spec: https://api.haivivi.cn/palgear/v1/openapi-yaml

const std = @import("std");
const http = @import("http");

// ============================================================================
// Types (based on OpenAPI schemas)
// ============================================================================

/// Health check result from /hello endpoint
pub const HealthCheckResult = struct {
    message: []const u8,
};

/// Device authentication credentials
pub const DeviceAuth = struct {
    keyId: []const u8,
    key: []const u8,
    token: []const u8,
    expiryUnixSec: u64,
};

/// Ping result for NTP time sync
pub const PingResult = struct {
    timestamp: i64,
    recv_at: i64,
    send_at: i64,
    utc_offset_sec: i32,
};

/// Project settings
pub const ProjectSettings = struct {
    data: []const u8,
};

/// User info result
pub const InfoResult = struct {
    nickname: []const u8,
    avatar: []const u8,
    username: []const u8,
};

/// Refresh token result (for @refresh endpoint)
pub const RefreshTokenResult = struct {
    token: []const u8,
    expiryUnixSec: u64,
    gearId: []const u8,
};

/// Virtual device chat mode
pub const VirtualDeviceChatMode = enum {
    CHAT_TOPIC,
    POD,
    PODCAST,
    ALBUM,

    pub fn jsonStringify(self: VirtualDeviceChatMode, options: std.json.StringifyOptions, writer: anytype) !void {
        _ = options;
        const str = switch (self) {
            .CHAT_TOPIC => "CHAT_TOPIC",
            .POD => "POD",
            .PODCAST => "PODCAST",
            .ALBUM => "ALBUM",
        };
        try writer.print("\"{s}\"", .{str});
    }
};

/// Chat mode with optional resource ID
pub const ChatMode = struct {
    chatMode: VirtualDeviceChatMode,
    resourceId: ?[]const u8 = null,
};

/// Point record in points history
pub const PointRecord = struct {
    title: []const u8,
    amount: i32,
};

/// Points result with total and history
pub const PointsResult = struct {
    total: i32,
    list: []const PointRecord,
};

/// Points consume type
pub const ConsumePointType = enum {
    GEAR_GAME_CONSUME,

    pub fn jsonStringify(self: ConsumePointType, options: std.json.StringifyOptions, writer: anytype) !void {
        _ = options;
        const str = switch (self) {
            .GEAR_GAME_CONSUME => "GEAR_GAME_CONSUME",
        };
        try writer.print("\"{s}\"", .{str});
    }
};

/// Points consume result
pub const PointsConsumeResult = struct {
    total: i32,
    insufficient: bool = false,
};

/// Firmware information
pub const Firmware = struct {
    version: []const u8,
    imageMD5: []const u8,
    dataFileMD5: []const u8,
    imageUrl: []const u8,
    dataFileUrl: []const u8,
};

// ============================================================================
// Request DTOs
// ============================================================================

/// Setup device request
pub const SetupDeviceDto = struct {
    vid: []const u8,
    eid: []const u8,
};

/// Refresh token request
pub const RefreshTokenDto = struct {
    key: []const u8,
};

/// Refresh request (for @refresh endpoint)
pub const RefreshReqDto = struct {
    sn: []const u8,
    reqMilliSec: i64,
};

/// Bind device request
pub const BindDeviceDto = struct {
    vid: []const u8,
    uat: []const u8,
};

/// Change chat mode request
pub const ChangeChatModeDto = struct {
    chatMode: VirtualDeviceChatMode,
    resourceId: ?[]const u8 = null,
};

/// Consume points request
pub const ConsumePointDto = struct {
    type: ConsumePointType,
    amount: i32,
    description: ?[]const u8 = null,
};

// ============================================================================
// Buffer size constants
// ============================================================================

pub const BufferSize = struct {
    /// HTTP response buffer size
    pub const http_response: usize = 4096;
    /// JSON parsing scratch buffer size
    pub const json_scratch: usize = 2048;
    /// Request body buffer size
    pub const request_body: usize = 512;
};

// ============================================================================
// Errors
// ============================================================================

pub const ApiError = error{
    /// HTTP request failed
    RequestFailed,
    /// Server returned error status code
    ServerError,
    /// JSON parsing failed
    JsonParseError,
    /// Buffer too small
    BufferTooSmall,
    /// Invalid response
    InvalidResponse,
    /// Authentication required
    AuthRequired,
};

// ============================================================================
// Client
// ============================================================================

/// Palgear API Client
///
/// Generic over the HTTP client type to support different platforms.
/// Use with embed-zig's http.ClientFull for ESP32.
pub fn Client(comptime HttpClient: type) type {
    return struct {
        /// HTTP client instance
        http_client: *HttpClient,
        /// API base URL (e.g., "https://api.haivivi.cn")
        base_url: []const u8,
        /// Authentication token (JWT)
        token: ?[]const u8 = null,

        const Self = @This();

        /// Initialize client with HTTP client and base URL
        pub fn init(http_client: *HttpClient, base_url: []const u8) Self {
            return .{
                .http_client = http_client,
                .base_url = base_url,
            };
        }

        /// Set authentication token
        pub fn setToken(self: *Self, token: []const u8) void {
            self.token = token;
        }

        /// Clear authentication token
        pub fn clearToken(self: *Self) void {
            self.token = null;
        }

        // ====================================================================
        // API Methods
        // ====================================================================

        /// GET /palgear/v1/hello - Health check
        pub fn hello(self: *Self, buf: []u8) ApiError!HealthCheckResult {
            const resp = self.http_client.get(
                self.buildUrl("/palgear/v1/hello"),
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(HealthCheckResult, resp.body()) catch error.JsonParseError;
        }

        /// GET /palgear/v1/ping - NTP time sync
        pub fn ping(self: *Self, timestamp: ?i64, buf: []u8) ApiError!PingResult {
            var url_buf: [256]u8 = undefined;
            const url = if (timestamp) |ts|
                std.fmt.bufPrint(&url_buf, "{s}/palgear/v1/ping?timestamp={d}", .{ self.base_url, ts }) catch return error.BufferTooSmall
            else
                self.buildUrl("/palgear/v1/ping");

            const resp = self.http_client.get(url, buf) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(PingResult, resp.body()) catch error.JsonParseError;
        }

        /// POST /palgear/v1/refresh-token - Refresh device token
        pub fn refreshToken(self: *Self, key: []const u8, buf: []u8) ApiError!DeviceAuth {
            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(RefreshTokenDto{ .key = key }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/refresh-token"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(DeviceAuth, resp.body()) catch error.JsonParseError;
        }

        /// POST /palgear/v1/setup - Setup device (requires bearer token)
        pub fn setup(self: *Self, vid: []const u8, eid: []const u8, bearer_token: []const u8, buf: []u8) ApiError!DeviceAuth {
            _ = bearer_token; // TODO: Add Authorization header support
            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(SetupDeviceDto{ .vid = vid, .eid = eid }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/setup"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(DeviceAuth, resp.body()) catch error.JsonParseError;
        }

        /// GET /palgear/v1/settings - Get project settings (requires auth)
        pub fn getSettings(self: *Self, buf: []u8) ApiError!ProjectSettings {
            if (self.token == null) return error.AuthRequired;

            const resp = self.http_client.get(
                self.buildUrl("/palgear/v1/settings"),
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(ProjectSettings, resp.body()) catch error.JsonParseError;
        }

        /// GET /palgear/v1/info - Get user info (requires auth)
        pub fn getInfo(self: *Self, buf: []u8) ApiError!InfoResult {
            if (self.token == null) return error.AuthRequired;

            const resp = self.http_client.get(
                self.buildUrl("/palgear/v1/info"),
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(InfoResult, resp.body()) catch error.JsonParseError;
        }

        /// POST /palgear/v1/@refresh - Refresh authentication
        pub fn refresh(self: *Self, sn: []const u8, req_milli_sec: i64, buf: []u8) ApiError!RefreshTokenResult {
            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(RefreshReqDto{ .sn = sn, .reqMilliSec = req_milli_sec }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/@refresh"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(RefreshTokenResult, resp.body()) catch error.JsonParseError;
        }

        /// POST /palgear/v1/@bind - Bind device
        pub fn bind(self: *Self, vid: []const u8, uat: []const u8, buf: []u8) ApiError!void {
            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(BindDeviceDto{ .vid = vid, .uat = uat }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/@bind"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (resp.status_code != 204 and !resp.isSuccess()) return error.ServerError;
        }

        /// POST /palgear/v1/@unbind - Unbind device (requires auth)
        pub fn unbind(self: *Self, buf: []u8) ApiError!void {
            if (self.token == null) return error.AuthRequired;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/@unbind"),
                null,
                null,
                buf,
            ) catch return error.RequestFailed;

            if (resp.status_code != 204 and !resp.isSuccess()) return error.ServerError;
        }

        /// GET /palgear/v1/chat-mode - Get chat mode (requires auth)
        pub fn getChatMode(self: *Self, buf: []u8) ApiError!ChatMode {
            if (self.token == null) return error.AuthRequired;

            const resp = self.http_client.get(
                self.buildUrl("/palgear/v1/chat-mode"),
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(ChatMode, resp.body()) catch error.JsonParseError;
        }

        /// PATCH /palgear/v1/chat-mode - Change chat mode (requires auth)
        pub fn changeChatMode(self: *Self, chat_mode: VirtualDeviceChatMode, resource_id: ?[]const u8, buf: []u8) ApiError!ChatMode {
            if (self.token == null) return error.AuthRequired;

            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(ChangeChatModeDto{
                .chatMode = chat_mode,
                .resourceId = resource_id,
            }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .PATCH,
                self.buildUrl("/palgear/v1/chat-mode"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(ChatMode, resp.body()) catch error.JsonParseError;
        }

        /// GET /palgear/v1/points - Get points (requires auth)
        pub fn getPoints(self: *Self, buf: []u8) ApiError!PointsResult {
            if (self.token == null) return error.AuthRequired;

            const resp = self.http_client.get(
                self.buildUrl("/palgear/v1/points"),
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(PointsResult, resp.body()) catch error.JsonParseError;
        }

        /// POST /palgear/v1/points/@consume - Consume points (requires auth)
        pub fn consumePoints(self: *Self, point_type: ConsumePointType, amount: i32, description: ?[]const u8, buf: []u8) ApiError!PointsConsumeResult {
            if (self.token == null) return error.AuthRequired;

            var req_buf: [BufferSize.request_body]u8 = undefined;
            const body = stringifyJson(ConsumePointDto{
                .type = point_type,
                .amount = amount,
                .description = description,
            }, &req_buf) catch return error.BufferTooSmall;

            const resp = self.http_client.request(
                .POST,
                self.buildUrl("/palgear/v1/points/@consume"),
                body,
                "application/json",
                buf,
            ) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(PointsConsumeResult, resp.body()) catch error.JsonParseError;
        }

        /// GET /palgear/v1/firmwares/@latest - Get latest firmware (requires auth)
        pub fn getLatestFirmware(self: *Self, sn: ?[]const u8, buf: []u8) ApiError!Firmware {
            if (self.token == null) return error.AuthRequired;

            var url_buf: [256]u8 = undefined;
            const url = if (sn) |s|
                std.fmt.bufPrint(&url_buf, "{s}/palgear/v1/firmwares/@latest?sn={s}", .{ self.base_url, s }) catch return error.BufferTooSmall
            else
                self.buildUrl("/palgear/v1/firmwares/@latest");

            const resp = self.http_client.get(url, buf) catch return error.RequestFailed;

            if (!resp.isSuccess()) return error.ServerError;
            return parseJson(Firmware, resp.body()) catch error.JsonParseError;
        }

        // ====================================================================
        // Internal helpers
        // ====================================================================

        /// URL buffer for building full URLs (thread-local to avoid conflicts)
        threadlocal var url_buffer: [512]u8 = undefined;

        fn buildUrl(self: *const Self, path: []const u8) []const u8 {
            const result = std.fmt.bufPrint(&url_buffer, "{s}{s}", .{ self.base_url, path }) catch {
                // Fallback to just path if buffer overflow (shouldn't happen)
                return path;
            };
            return result;
        }
    };
}

// ============================================================================
// JSON utilities
// ============================================================================

/// Parse JSON data into a struct using fixed buffer allocator
fn parseJson(comptime T: type, data: []const u8) !T {
    var scratch: [BufferSize.json_scratch]u8 = undefined;
    var fba = std.heap.FixedBufferAllocator.init(&scratch);
    const parsed = try std.json.parseFromSlice(T, fba.allocator(), data, .{
        .ignore_unknown_fields = true,
    });
    return parsed.value;
}

/// Stringify a struct to JSON into a fixed buffer
fn stringifyJson(value: anytype, buf: []u8) ![]const u8 {
    var fbs = std.io.fixedBufferStream(buf);
    try std.json.stringify(value, .{}, fbs.writer());
    return fbs.getWritten();
}

// ============================================================================
// Tests
// ============================================================================

test "parseJson - HealthCheckResult" {
    const json = "{\"message\":\"Hello, World!\"}";
    const result = try parseJson(HealthCheckResult, json);
    try std.testing.expectEqualStrings("Hello, World!", result.message);
}

test "parseJson - DeviceAuth" {
    const json =
        \\{"keyId":"key123","key":"secret","token":"jwt.token.here","expiryUnixSec":1700000000}
    ;
    const result = try parseJson(DeviceAuth, json);
    try std.testing.expectEqualStrings("key123", result.keyId);
    try std.testing.expectEqualStrings("secret", result.key);
    try std.testing.expectEqual(@as(u64, 1700000000), result.expiryUnixSec);
}

test "parseJson - PingResult" {
    const json =
        \\{"timestamp":1700000000000,"recv_at":1700000000100,"send_at":1700000000050,"utc_offset_sec":28800}
    ;
    const result = try parseJson(PingResult, json);
    try std.testing.expectEqual(@as(i64, 1700000000000), result.timestamp);
    try std.testing.expectEqual(@as(i32, 28800), result.utc_offset_sec);
}

test "stringifyJson - RefreshTokenDto" {
    var buf: [256]u8 = undefined;
    const json = try stringifyJson(RefreshTokenDto{ .key = "my-key" }, &buf);
    try std.testing.expectEqualStrings("{\"key\":\"my-key\"}", json);
}

test "stringifyJson - SetupDeviceDto" {
    var buf: [256]u8 = undefined;
    const json = try stringifyJson(SetupDeviceDto{ .vid = "vid123", .eid = "eid456" }, &buf);
    try std.testing.expectEqualStrings("{\"vid\":\"vid123\",\"eid\":\"eid456\"}", json);
}
