//! ChatGear E2E Test Application — Platform Independent
//!
//! Device simulator that exercises the full chatgear protocol:
//! - Connects to MQTT broker
//! - Sends state/stats via ClientPort periodic reporting
//! - Handles ADC button events for recording and calling
//! - Receives commands from server
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

const Rt = hw.runtime;
const MqttClient = mqtt0.Client(Board.socket);
const Conn = chatgear.MqttClientConn(MqttClient);
const Port = chatgear.ClientPort(MqttClient, Rt);

// ============================================================================
// Configuration
// ============================================================================

var config: struct {
    wifi_ssid: []const u8,
    wifi_password: []const u8,
    mqtt_host: []const u8,
    mqtt_port: u16,
    mqtt_username: []const u8,
    mqtt_password: []const u8,
    gear_id: []const u8,
    scope: []const u8,
} = undefined;

fn parsePort(port_str: []const u8) u16 {
    var port: u16 = 0;
    for (port_str) |c| {
        if (c >= '0' and c <= '9') {
            port = port * 10 + (c - '0');
        }
    }
    return if (port == 0) 1883 else port;
}

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
        .mqtt_host = if (@hasField(@TypeOf(env.*), "mqtt_host")) env.mqtt_host else "test.mosquitto.org",
        .mqtt_port = if (@hasField(@TypeOf(env.*), "mqtt_port")) parsePort(env.mqtt_port) else 1883,
        .mqtt_username = if (@hasField(@TypeOf(env.*), "mqtt_username")) env.mqtt_username else "",
        .mqtt_password = if (@hasField(@TypeOf(env.*), "mqtt_password")) env.mqtt_password else "",
        .gear_id = if (@hasField(@TypeOf(env.*), "gear_id")) env.gear_id else "zig-test-001",
        .scope = if (@hasField(@TypeOf(env.*), "scope")) env.scope else "stage/",
    };

    log.info("==========================================", .{});
    log.info("  ChatGear E2E Test", .{});
    log.info("  Board: {s}", .{Board.meta.id});
    log.info("  Gear ID: {s}", .{config.gear_id});
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

    // MQTT + chatgear state (initialized after WiFi connects)
    var allocator = std.heap.page_allocator;
    var mux = mqtt0.Mux.init(allocator) catch |err| {
        log.err("Mux init failed: {}", .{err});
        return;
    };
    defer mux.deinit();

    var mqtt_client: ?MqttClient = null;
    var chatgear_conn: ?Conn = null;
    var port: ?Port = null;

    // Main event loop
    while (Board.isRunning()) {
        // Process board events
        while (board.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| {
                    switch (wifi_event) {
                        .connected => log.info("WiFi connected (waiting for IP...)", .{}),
                        .disconnected => |reason| {
                            log.warn("WiFi disconnected: {}", .{reason});
                            app_state = .connecting_wifi;
                        },
                        .connection_failed => |reason| {
                            log.err("WiFi connection failed: {}", .{reason});
                        },
                        else => {},
                    }
                },
                .net => |net_event| {
                    switch (net_event) {
                        .dhcp_bound, .dhcp_renewed => |info| {
                            var buf: [16]u8 = undefined;
                            const ip_str = std.fmt.bufPrint(&buf, "{d}.{d}.{d}.{d}", .{
                                info.ip[0], info.ip[1], info.ip[2], info.ip[3],
                            }) catch "?.?.?.?";
                            log.info("Got IP: {s}", .{ip_str});
                            app_state = .connecting_mqtt;
                        },
                        .ip_lost => {
                            log.warn("IP lost", .{});
                            app_state = .connecting_wifi;
                        },
                        else => {},
                    }
                },
                .button => |btn| {
                    if (app_state == .running) {
                        if (port) |*p| {
                            handleButton(p, btn);
                        }
                    }
                },
                else => {},
            }
        }

        // State machine
        switch (app_state) {
            .connecting_wifi => {},
            .connecting_mqtt => {
                Board.time.sleepMs(500);

                // Connect MQTT
                log.info("Connecting to MQTT: {s}:{d}", .{ config.mqtt_host, config.mqtt_port });
                var socket = Board.socket.tcp() catch |err| {
                    log.err("TCP socket failed: {}", .{err});
                    continue;
                };
                socket.connect(config.mqtt_host, config.mqtt_port) catch |err| {
                    log.err("TCP connect failed: {}", .{err});
                    socket.close();
                    Board.time.sleepMs(3000);
                    continue;
                };

                mqtt_client = MqttClient.init(&socket, &mux, .{
                    .client_id = config.gear_id,
                    .username = config.mqtt_username,
                    .password = config.mqtt_password,
                }) catch |err| {
                    log.err("MQTT connect failed: {}", .{err});
                    socket.close();
                    Board.time.sleepMs(3000);
                    continue;
                };

                // Init chatgear connection + port
                chatgear_conn = Conn.init(&mqtt_client.?, .{
                    .scope = config.scope,
                    .gear_id = config.gear_id,
                });
                chatgear_conn.?.subscribe() catch |err| {
                    log.err("MQTT subscribe failed: {}", .{err});
                    continue;
                };

                port = Port.init(&chatgear_conn.?);

                // Set initial stats (batch mode)
                port.?.beginBatch();
                port.?.setVolume(@floatFromInt(volume));
                port.?.setBattery(100, false);
                port.?.setSystemVersion("zig-e2e-0.1.0");
                port.?.endBatch();

                // Start periodic reporting
                port.?.startPeriodicReporting() catch |err| {
                    log.err("Failed to start reporting: {}", .{err});
                    continue;
                };

                port.?.setState(.ready);
                log.info("ChatGear ready! Gear ID: {s}", .{config.gear_id});
                app_state = .running;

                // Start MQTT read loop in background
                Rt.spawn("mqtt_rx", mqttReadLoopFn, @ptrCast(&mqtt_client.?), .{}) catch |err| {
                    log.err("Failed to spawn MQTT read loop: {}", .{err});
                };
            },
            .running => {
                // Process commands from server
                if (port) |*p| {
                    while (p.commands.tryRecv()) |cmd| {
                        handleCommand(p, cmd);
                    }
                }
            },
        }

        Board.time.sleepMs(10);
    }

    // Cleanup
    if (port) |*p| p.close();
}

// ============================================================================
// MQTT Read Loop (background task)
// ============================================================================

fn mqttReadLoopFn(ctx: ?*anyopaque) void {
    const client: *MqttClient = @ptrCast(@alignCast(ctx));
    client.readLoop() catch |err| {
        log.err("MQTT read loop error: {}", .{err});
    };
}

// ============================================================================
// Button Handling
// ============================================================================

fn handleButton(p: *Port, btn: anytype) void {
    const btn_name = btn.id.name();

    switch (btn.id) {
        .rec => {
            // REC button: press-and-hold = recording, release = waiting_for_response
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
            // PLAY button: click = toggle calling/ready
            switch (btn.action) {
                .click => {
                    const current = p.getState();
                    if (current == .calling) {
                        log.info("[{s}] CLICK -> exit calling", .{btn_name});
                        p.setState(.ready);
                    } else {
                        log.info("[{s}] CLICK -> enter calling", .{btn_name});
                        p.setState(.calling);
                    }
                },
                else => {},
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

fn handleCommand(p: *Port, cmd: chatgear.CommandEvent) void {
    log.info("Command: {s}", .{cmd.cmd_type.toString()});

    switch (cmd.payload) {
        .streaming => |enabled| {
            if (enabled) {
                p.setState(.streaming);
            } else {
                p.setState(.ready);
            }
        },
        .set_volume => |vol| {
            volume = vol;
            p.setVolume(@floatFromInt(vol));
            log.info("Volume set to {}", .{vol});
        },
        .set_brightness => |br| {
            p.setBrightness(@floatFromInt(br));
            log.info("Brightness set to {}", .{br});
        },
        .halt => |h| {
            if (h.sleep) {
                p.setState(.sleeping);
            } else if (h.shutdown) {
                p.setState(.shutting_down);
            } else if (h.interrupt) {
                p.setState(.interrupted);
            }
        },
        .reset => {
            p.setState(.resetting);
        },
        .raise => |r| {
            if (r.call) {
                p.setState(.calling);
            }
        },
        else => {},
    }
}
