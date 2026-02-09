//! ChatGear E2E Test Application — Platform Independent
//!
//! Device simulator that exercises the full chatgear protocol:
//! - Connects to MQTT broker (mqtt:// or mqtts:// with auth)
//! - Sends state/stats via ClientPort periodic reporting
//! - Handles ADC button events for recording and calling
//! - Receives commands from server
//!
//! MQTT URL format (matches Go DialMQTT):
//!   mqtt://host:1883              — TCP, no auth
//!   mqtt://user:pass@host:1883    — TCP with auth
//!   mqtts://user:pass@host:8883   — TLS with auth
//!
//! Button behavior (Korvo-2 V3 ADC buttons):
//! - REC: press-and-hold = recording, release = waiting_for_response
//! - PLAY: press = toggle calling/ready
//! - VOL+/VOL-: adjust volume

const std = @import("std");

const platform = @import("platform.zig");
const Board = platform.Board;
const ButtonId = platform.ButtonId;
const hw = platform.hw;
const log = Board.log;

const chatgear = @import("chatgear");
const mqtt0 = @import("mqtt0");
const tls = @import("tls");
const dns = @import("dns");

const MqttRt = hw.MqttRt;
const FullRt = hw.FullRt;
const Socket = Board.socket;
const Crypto = hw.crypto;
const TlsClient = tls.Client(Socket, Crypto);
const TcpMqttClient = mqtt0.Client(Socket, MqttRt);
const TlsMqttClient = mqtt0.Client(TlsClient, MqttRt);
const TcpConn = chatgear.MqttClientConn(TcpMqttClient);
const TlsConn = chatgear.MqttClientConn(TlsMqttClient);
const TcpPort = chatgear.ClientPort(TcpMqttClient, FullRt);
const TlsPort = chatgear.ClientPort(TlsMqttClient, FullRt);

// ============================================================================
// MQTT URL Parser
// ============================================================================

const MqttScheme = enum { tcp, tls };

const MqttUrl = struct {
    scheme: MqttScheme = .tcp,
    host: []const u8 = "",
    port: u16 = 1883,
    username: []const u8 = "",
    password: []const u8 = "",

    /// Parse a URL like "mqtt://user:pass@host:port" or "mqtts://host:8883"
    /// All slices point into the input buffer.
    pub fn parse(url: []const u8) MqttUrl {
        var result = MqttUrl{};
        var rest = url;

        // Scheme
        if (startsWith(rest, "mqtts://")) {
            result.scheme = .tls;
            result.port = 8883;
            rest = rest["mqtts://".len..];
        } else if (startsWith(rest, "mqtt://")) {
            result.scheme = .tcp;
            result.port = 1883;
            rest = rest["mqtt://".len..];
        }
        // else: treat as host:port (no scheme)

        // Userinfo: user:pass@
        if (indexOf(rest, '@')) |at_pos| {
            const userinfo = rest[0..at_pos];
            rest = rest[at_pos + 1 ..];

            if (indexOf(userinfo, ':')) |colon_pos| {
                result.username = userinfo[0..colon_pos];
                result.password = userinfo[colon_pos + 1 ..];
            } else {
                result.username = userinfo;
            }
        }

        // Host:port
        if (indexOf(rest, ':')) |colon_pos| {
            result.host = rest[0..colon_pos];
            result.port = parseU16(rest[colon_pos + 1 ..]);
        } else {
            // Strip trailing slash or path if any
            if (indexOf(rest, '/')) |slash_pos| {
                result.host = rest[0..slash_pos];
            } else {
                result.host = rest;
            }
        }

        return result;
    }

    fn startsWith(haystack: []const u8, prefix: []const u8) bool {
        if (haystack.len < prefix.len) return false;
        return std.mem.eql(u8, haystack[0..prefix.len], prefix);
    }

    fn indexOf(haystack: []const u8, needle: u8) ?usize {
        for (haystack, 0..) |c, i| {
            if (c == needle) return i;
        }
        return null;
    }

    fn parseU16(s: []const u8) u16 {
        var val: u16 = 0;
        for (s) |c| {
            if (c >= '0' and c <= '9') {
                val = val *| 10 +| (c - '0');
            } else break;
        }
        return val;
    }
};

// ============================================================================
// Configuration
// ============================================================================

var config: struct {
    wifi_ssid: []const u8,
    wifi_password: []const u8,
    mqtt: MqttUrl,
    gear_id: []const u8,
    scope: []const u8,
} = undefined;

// ============================================================================
// Application State
// ============================================================================

const AppState = enum {
    connecting_wifi,
    connecting_mqtt,
    running,
};

var volume: i32 = 50;

// ============================================================================
// Entry Point
// ============================================================================

pub fn run(env: anytype) void {
    // Parse environment
    config = .{
        .wifi_ssid = env.wifi_ssid,
        .wifi_password = env.wifi_password,
        .mqtt = MqttUrl.parse(if (@hasField(@TypeOf(env), "mqtt_url")) env.mqtt_url else "mqtt://test.mosquitto.org:1883"),
        .gear_id = if (@hasField(@TypeOf(env), "gear_id")) env.gear_id else "zig-test-001",
        .scope = if (@hasField(@TypeOf(env), "scope")) env.scope else "stage/",
    };

    log.info("==========================================", .{});
    log.info("  ChatGear E2E Test", .{});
    log.info("  Board:   {s}", .{Board.meta.id});
    log.info("  Gear ID: {s}", .{config.gear_id});
    log.info("  MQTT:    {s}://{s}:{d}", .{
        if (config.mqtt.scheme == .tls) @as([]const u8, "mqtts") else "mqtt",
        config.mqtt.host,
        config.mqtt.port,
    });
    if (config.mqtt.username.len > 0) {
        log.info("  Auth:    {s}:***", .{config.mqtt.username});
    }
    log.info("==========================================", .{});

    // Init board
    var board: Board = undefined;
    board.init() catch |err| {
        log.err("Board init failed: {}", .{err});
        return;
    };
    defer board.deinit();

    // Connect WiFi
    log.info("Connecting to WiFi: {s}", .{config.wifi_ssid});
    board.wifi.connect(config.wifi_ssid, config.wifi_password);

    var app_state: AppState = .connecting_wifi;

    // Main event loop
    while (Board.isRunning()) {
        // Process board events
        while (board.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| handleWifiEvent(wifi_event, &app_state),
                .net => |net_event| handleNetEvent(net_event, &app_state),
                .button => |btn| {
                    _ = btn;
                    // Button handling requires port, done in running state
                },
                else => {},
            }
        }

        switch (app_state) {
            .connecting_wifi => {},
            .connecting_mqtt => {
                Board.time.sleepMs(500);

                switch (config.mqtt.scheme) {
                    .tcp => connectTcp(&app_state),
                    .tls => connectTls(&app_state),
                }
            },
            .running => {
                // Main loop body is handled by spawned tasks
            },
        }

        Board.time.sleepMs(10);
    }
}

// ============================================================================
// WiFi / Net Event Handlers
// ============================================================================

fn handleWifiEvent(wifi_event: anytype, app_state: *AppState) void {
    switch (wifi_event) {
        .connected => log.info("WiFi connected (waiting for IP...)", .{}),
        .disconnected => |reason| {
            log.warn("WiFi disconnected: {}", .{reason});
            app_state.* = .connecting_wifi;
        },
        .connection_failed => |reason| {
            log.err("WiFi connection failed: {}", .{reason});
        },
        else => {},
    }
}

fn handleNetEvent(net_event: anytype, app_state: *AppState) void {
    switch (net_event) {
        .dhcp_bound, .dhcp_renewed => |info| {
            var buf: [16]u8 = undefined;
            const ip_str = std.fmt.bufPrint(&buf, "{d}.{d}.{d}.{d}", .{
                info.ip[0], info.ip[1], info.ip[2], info.ip[3],
            }) catch "?.?.?.?";
            log.info("Got IP: {s}", .{ip_str});
            app_state.* = .connecting_mqtt;
        },
        .ip_lost => {
            log.warn("IP lost", .{});
            app_state.* = .connecting_wifi;
        },
        else => {},
    }
}

// ============================================================================
// MQTT Connection (TCP)
// ============================================================================

fn resolveHost(hostname: []const u8) ?[4]u8 {
    const DnsResolver = dns.Resolver(Socket);
    var resolver = DnsResolver{
        .server = .{ 223, 5, 5, 5 }, // AliDNS
        .protocol = .udp,
    };
    return resolver.resolve(hostname) catch |err| {
        log.err("DNS resolve failed for {s}: {}", .{ hostname, err });
        return null;
    };
}

fn connectTcp(app_state: *AppState) void {
    log.info("Connecting MQTT (TCP) to {s}:{d}...", .{ config.mqtt.host, config.mqtt.port });

    const ip = resolveHost(config.mqtt.host) orelse return;
    log.info("Resolved {s} -> {d}.{d}.{d}.{d}", .{ config.mqtt.host, ip[0], ip[1], ip[2], ip[3] });

    var socket = Socket.tcp() catch |err| {
        log.err("TCP socket failed: {}", .{err});
        return;
    };
    socket.connect(ip, config.mqtt.port) catch |err| {
        log.err("TCP connect failed: {}", .{err});
        socket.close();
        Board.time.sleepMs(3000);
        return;
    };

    var mux = mqtt0.Mux(MqttRt).init(hw.allocator) catch |err| {
        log.err("Mux init failed: {}", .{err});
        socket.close();
        return;
    };

    var client = TcpMqttClient.init(&socket, &mux, .{
        .client_id = config.gear_id,
        .username = config.mqtt.username,
        .password = config.mqtt.password,
        .allocator = hw.allocator,
    }) catch |err| {
        log.err("MQTT connect failed: {}", .{err});
        socket.close();
        Board.time.sleepMs(3000);
        return;
    };

    var conn = TcpConn.init(&client, .{
        .scope = config.scope,
        .gear_id = config.gear_id,
    });
    conn.subscribe() catch |err| {
        log.err("MQTT subscribe failed: {}", .{err});
        return;
    };

    var port = TcpPort.init(&conn);
    initPort(&port);
    app_state.* = .running;

    // MQTT readLoop in background (blocks until disconnect)
    FullRt.spawn("mqtt_rx", struct {
        fn run(ctx: ?*anyopaque) void {
            const c: *TcpMqttClient = @ptrCast(@alignCast(ctx));
            c.readLoop() catch |err| {
                log.err("MQTT read loop error: {}", .{err});
            };
        }
    }.run, @ptrCast(&client), .{}) catch |err| {
        log.err("Failed to spawn MQTT reader: {}", .{err});
    };
}

// ============================================================================
// MQTT Connection (TLS)
// ============================================================================

fn connectTls(app_state: *AppState) void {
    log.info("Connecting MQTT (TLS) to {s}:{d}...", .{ config.mqtt.host, config.mqtt.port });

    const ip = resolveHost(config.mqtt.host) orelse return;
    log.info("Resolved {s} -> {d}.{d}.{d}.{d}", .{ config.mqtt.host, ip[0], ip[1], ip[2], ip[3] });

    var socket = Socket.tcp() catch |err| {
        log.err("TCP socket failed: {}", .{err});
        return;
    };

    // Set recv timeout (critical for TLS handshake)
    socket.setRecvTimeout(30000);

    socket.connect(ip, config.mqtt.port) catch |err| {
        log.err("TCP connect failed: {}", .{err});
        socket.close();
        Board.time.sleepMs(3000);
        return;
    };
    log.info("TCP connected", .{});

    // TLS handshake
    log.info("TLS handshake...", .{});
    var tls_client = TlsClient.init(&socket, .{
        .hostname = config.mqtt.host,
        .allocator = hw.allocator,
        .skip_verify = true,
        .timeout_ms = 30000,
    }) catch |err| {
        log.err("TLS init failed: {}", .{err});
        socket.close();
        Board.time.sleepMs(3000);
        return;
    };
    tls_client.connect() catch |err| {
        log.err("TLS handshake failed: {}", .{err});
        tls_client.deinit();
        Board.time.sleepMs(3000);
        return;
    };
    log.info("TLS connected!", .{});

    var mux = mqtt0.Mux(MqttRt).init(hw.allocator) catch |err| {
        log.err("Mux init failed: {}", .{err});
        return;
    };

    var client = TlsMqttClient.init(&tls_client, &mux, .{
        .client_id = config.gear_id,
        .username = config.mqtt.username,
        .password = config.mqtt.password,
        .allocator = hw.allocator,
    }) catch |err| {
        log.err("MQTT connect failed: {}", .{err});
        Board.time.sleepMs(3000);
        return;
    };

    var conn = TlsConn.init(&client, .{
        .scope = config.scope,
        .gear_id = config.gear_id,
    });
    conn.subscribe() catch |err| {
        log.err("MQTT subscribe failed: {}", .{err});
        return;
    };

    var port = TlsPort.init(&conn);
    initPort(&port);
    app_state.* = .running;

    FullRt.spawn("mqtt_rx", struct {
        fn run(ctx: ?*anyopaque) void {
            const c: *TlsMqttClient = @ptrCast(@alignCast(ctx));
            c.readLoop() catch |err| {
                log.err("MQTT read loop error: {}", .{err});
            };
        }
    }.run, @ptrCast(&client), .{}) catch |err| {
        log.err("Failed to spawn MQTT reader: {}", .{err});
    };
}

// ============================================================================
// Port Initialization (shared between TCP and TLS)
// ============================================================================

fn initPort(p: anytype) void {
    p.beginBatch();
    p.setVolume(@floatFromInt(volume));
    p.setBattery(100, false);
    p.setSystemVersion("zig-e2e-0.1.0");
    p.endBatch();

    p.startPeriodicReporting() catch |err| {
        log.err("Failed to start reporting: {}", .{err});
        return;
    };

    p.setState(.ready);
    log.info("ChatGear ready! Gear ID: {s}", .{config.gear_id});
}

// ============================================================================
// Button Handling
// ============================================================================

fn handleButton(p: anytype, btn: anytype) void {
    const btn_name = btn.id.name();

    switch (btn.id) {
        .rec => {
            // REC: press-and-hold = recording, release = waiting_for_response
            switch (btn.action) {
                .press => {
                    log.info("[{s}] PRESSED -> recording", .{btn_name});
                    p.setState(.recording);
                },
                .release => {
                    log.info("[{s}] RELEASED ({}ms) -> waiting_for_response", .{ btn_name, btn.duration_ms });
                    p.setState(.waiting_for_response);
                },
                else => {},
            }
        },
        .play => {
            // PLAY: click = toggle calling/ready
            if (btn.action == .click) {
                const current = p.getState();
                if (current == .calling) {
                    log.info("[{s}] CLICK -> exit calling", .{btn_name});
                    p.setState(.ready);
                } else {
                    log.info("[{s}] CLICK -> enter calling", .{btn_name});
                    p.setState(.calling);
                }
            }
        },
        .vol_up => {
            if (btn.action == .click or btn.action == .long_press) {
                volume = @min(volume + 10, 100);
                log.info("[{s}] volume -> {}", .{ btn_name, volume });
                p.setVolume(@floatFromInt(volume));
            }
        },
        .vol_down => {
            if (btn.action == .click or btn.action == .long_press) {
                volume = @max(volume - 10, 0);
                log.info("[{s}] volume -> {}", .{ btn_name, volume });
                p.setVolume(@floatFromInt(volume));
            }
        },
        else => {
            if (btn.action == .click) {
                log.info("[{s}] CLICK", .{btn_name});
            }
        },
    }
}

// ============================================================================
// Command Handling
// ============================================================================

fn handleCommand(p: anytype, cmd: chatgear.CommandEvent) void {
    log.info("Command: {s}", .{cmd.cmd_type.toString()});

    switch (cmd.payload) {
        .streaming => |enabled| {
            if (enabled) p.setState(.streaming) else p.setState(.ready);
        },
        .set_volume => |vol| {
            volume = vol;
            p.setVolume(@floatFromInt(vol));
        },
        .set_brightness => |br| {
            p.setBrightness(@floatFromInt(br));
        },
        .halt => |h| {
            if (h.sleep) p.setState(.sleeping) else if (h.shutdown) p.setState(.shutting_down) else if (h.interrupt) p.setState(.interrupted);
        },
        .reset => p.setState(.resetting),
        .raise => |r| {
            if (r.call) p.setState(.calling);
        },
        else => {},
    }
}
