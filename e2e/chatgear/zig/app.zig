//! ChatGear E2E Test Application — Platform Independent
//!
//! Device application that exercises the full chatgear protocol:
//! - Connects to MQTT broker (mqtt:// or mqtts:// with auth)
//! - Manages state/stats exclusively through ClientPort
//! - Handles ADC button events for recording and calling
//! - Captures mic audio via AudioSystem (ES7210 + AEC), opus-encodes, sends uplink
//! - Receives downlink audio, opus-decodes, plays via AudioSystem (ES8311)
//! - Receives and executes commands from server
//!
//! MQTT URL format (matches Go DialMQTT):
//!   mqtt://host:1883              — TCP, no auth
//!   mqtt://user:pass@host:1883    — TCP with auth
//!   mqtts://user:pass@host:8883   — TLS with auth
//!
//! Button behavior (ADC buttons):
//!   do  (VOL+): log current state
//!   re  (VOL-): force full stats report
//!   mi  (SET):  press = recording (from ready/waiting/streaming), release = waiting_for_response
//!   fa  (PLAY): click = toggle calling/ready (with state guards)
//!   so  (MUTE): click = cancel — any active state back to ready
//!   la  (REC):  reserved
//!
//! Command handling (from server):
//!   Matches Go geartest simulator.applyCommand() — state guards, value clamping,
//!   settings reset. See handleCommand() for details.
//!
//! Platform independence:
//!   This file does NOT import esp/idf. All hardware access goes through
//!   platform.zig which abstracts Board, AudioSystem, LedDriver, etc.

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
const ntp = @import("ntp");
const opus = @import("opus");

const MqttRt = hw.MqttRt;
const FullRt = hw.FullRt;
const Socket = Board.socket;
const Crypto = hw.crypto;
const TlsClient = tls.Client(Socket, Crypto, MqttRt);
const TcpMqttClient = mqtt0.Client(Socket, MqttRt);
const TlsMqttClient = mqtt0.Client(TlsClient, MqttRt);
const TcpConn = chatgear.MqttClientConn(TcpMqttClient);
const TlsConn = chatgear.MqttClientConn(TlsMqttClient);
const TcpPort = chatgear.ClientPort(TcpMqttClient, FullRt);
const TlsPort = chatgear.ClientPort(TlsMqttClient, FullRt);

const alloc = hw.allocator;

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
// Application Context
// ============================================================================

/// Application context — the single source of truth for all runtime state.
/// Heap-allocated, passed to spawned tasks. No global mutable hardware state.
const AppCtx = struct {
    port: *TlsPort,
    conn: *TlsConn,
    client: *TlsMqttClient,
    audio: *platform.AudioSystem,
    led: ?*platform.LedDriver,
    encoder: *opus.Encoder,
    decoder: *opus.Decoder,
    epoch_offset: i64,
};

/// The single global pointer. Set once during MQTT connect, used by button
/// handler in the main event loop. All other tasks receive AppCtx directly.
var g_app: ?*AppCtx = null;

// ============================================================================
// MQTT Downlink Handlers (mux callbacks)
// ============================================================================

/// Mux handler for downlink commands (server -> device).
/// Parses the command JSON and pushes to port's command channel.
fn onDownlinkCommand(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_app) |app| {
        var evt: chatgear.CommandEvent = undefined;
        chatgear.parseCommandEvent(msg.payload, &evt) catch |err| {
            log.err("[rx] command parse: {}", .{err});
            return;
        };
        app.port.pushCommand(evt);
    }
}

/// Mux handler for downlink audio (server -> device).
/// Unstamps the opus frame and pushes to port's downlink audio channel.
fn onDownlinkAudio(_: []const u8, msg: *const mqtt0.Message) anyerror!void {
    if (g_app) |app| {
        const f = chatgear.unstampFrame(msg.payload) catch |err| {
            log.err("[rx] audio unstamp: {}", .{err});
            return;
        };
        app.port.pushDownlinkAudio(f);
    }
}

// ============================================================================
// Application State Machine
// ============================================================================

const AppState = enum {
    connecting_wifi,
    connecting_mqtt,
    running,
};

/// Epoch offset: epoch_ms - uptime_ms. Set by NTP sync.
var g_epoch_offset: i64 = 0;

// Note frequencies for button tones (C4 major scale = do re mi fa so la)
const button_tones = [6]u32{
    262, // VOL+ = C4 (do)
    294, // VOL- = D4 (re)
    330, // SET  = E4 (mi)
    349, // PLAY = F4 (fa)
    392, // MUTE = G4 (so)
    440, // REC  = A4 (la)
};

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

    // Init board (WiFi + net + buttons)
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
        // Poll ADC buttons
        board.buttons.poll();

        // Process board events
        while (board.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| handleWifiEvent(wifi_event, &app_state),
                .net => |net_event| handleNetEvent(net_event, &app_state),
                .button => |btn| {
                    if (btn.action == .press) {
                        log.info("[BTN] {s} PRESS", .{btn.id.name()});
                    } else if (btn.action == .click) {
                        log.info("[BTN] {s} CLICK", .{btn.id.name()});
                    } else if (btn.action == .release) {
                        log.info("[BTN] {s} RELEASE", .{btn.id.name()});
                    }
                    if (g_app) |app| {
                        handleButton(app, btn);
                    }
                },
                else => {},
            }
        }

        switch (app_state) {
            .connecting_wifi => {},
            .connecting_mqtt => {
                Board.time.sleepMs(500);

                // NTP time sync (once, before first MQTT connect)
                if (g_epoch_offset == 0) {
                    syncNtp();
                }

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
// DNS Resolution
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

// ============================================================================
// NTP Time Sync
// ============================================================================

fn syncNtp() void {
    const NtpClient = ntp.Client(Socket);
    var client = NtpClient{ .timeout_ms = 5000 };
    const local_time: i64 = @intCast(Board.time.getTimeMs());

    if (client.getTimeRace(local_time)) |epoch_ms| {
        g_epoch_offset = epoch_ms - local_time;
        log.info("NTP sync OK: offset={d}ms", .{g_epoch_offset});
    } else |err| {
        log.err("NTP sync failed: {}, using offset=0", .{err});
    }
}

// ============================================================================
// Audio System Initialization
// ============================================================================

/// Shared I2C bus — initialized once, used by AudioSystem + LedDriver + PA.
/// Must outlive all users (heap-allocated).
var g_i2c: ?*platform.I2c = null;

/// Initialize shared I2C bus, AudioSystem, LED, and PA.
/// Returns the AudioSystem and LedDriver instances (heap-allocated).
fn initAudioSystem() ?struct { audio: *platform.AudioSystem, led: ?*platform.LedDriver } {
    // I2C bus (shared)
    if (g_i2c == null) {
        const i2c = alloc.create(platform.I2c) catch |err| {
            log.err("alloc I2C: {}", .{err});
            return null;
        };
        i2c.* = platform.I2c.init(.{
            .sda = hw.Hardware.i2c_sda,
            .scl = hw.Hardware.i2c_scl,
            .freq_hz = 400_000,
        }) catch |err| {
            log.err("I2C init failed: {}", .{err});
            return null;
        };
        g_i2c = i2c;
    }
    const i2c = g_i2c.?;

    // AudioSystem (ES7210 ADC + ES8311 DAC + I2S duplex + AEC)
    log.info("Initializing AudioSystem...", .{});
    const audio = alloc.create(platform.AudioSystem) catch |err| {
        log.err("alloc AudioSystem: {}", .{err});
        return null;
    };
    audio.* = platform.AudioSystem.init(i2c) catch |err| {
        log.err("AudioSystem init failed: {}", .{err});
        return null;
    };
    log.info("AudioSystem ready (ES7210+ES8311+AEC)", .{});

    // PA (power amplifier for speaker)
    var pa = platform.PaSwitchDriver.init(i2c) catch |err| {
        log.warn("PA init failed: {}", .{err});
        return .{ .audio = audio, .led = null };
    };
    pa.on() catch |err| {
        log.warn("PA enable failed: {}", .{err});
    };

    // LED driver (optional, non-fatal)
    const led = alloc.create(platform.LedDriver) catch {
        return .{ .audio = audio, .led = null };
    };
    led.* = platform.LedDriver.init(i2c) catch |err| {
        log.warn("LED init failed: {} (continuing without LED)", .{err});
        return .{ .audio = audio, .led = null };
    };
    led.setBlue(true);
    Board.time.sleepMs(200);
    led.off();
    log.info("LED initialized", .{});

    return .{ .audio = audio, .led = led };
}

// ============================================================================
// Tone Playback (via AudioSystem)
// ============================================================================

/// Play a short sine wave tone at given frequency via AudioSystem.
fn playTone(audio: *platform.AudioSystem, freq: u32, duration_ms: u32) void {
    const sr = platform.sample_rate;
    const total_frames = (sr * duration_ms) / 1000;
    const phase_inc = @as(f32, @floatFromInt(freq)) * 2.0 * std.math.pi / @as(f32, @floatFromInt(sr));

    var buf: [256]i16 = undefined;
    var phase: f32 = 0;
    var played: u32 = 0;

    while (played < total_frames) {
        const remaining = total_frames - played;
        const chunk = if (remaining < 256) remaining else 256;

        var i: usize = 0;
        while (i < chunk) : (i += 1) {
            buf[i] = @intFromFloat(@sin(phase) * 10000.0);
            phase += phase_inc;
            if (phase >= 2.0 * std.math.pi) phase -= 2.0 * std.math.pi;
        }

        // writeSpeaker takes mono i16, converts to stereo i32 internally
        _ = audio.writeSpeaker(buf[0..chunk]) catch return;
        played += @intCast(chunk);
    }
}

/// Play "ready" notification: ascending beeps
fn playReadyBeeps(audio: *platform.AudioSystem) void {
    playTone(audio, 523, 100); // C5
    Board.time.sleepMs(80);
    playTone(audio, 659, 100); // E5
    Board.time.sleepMs(80);
    playTone(audio, 784, 150); // G5
    Board.time.sleepMs(50);
    playTone(audio, 1047, 200); // C6
}

/// Set LED state.
fn setLed(red: bool, blue: bool) void {
    if (g_app) |app| {
        if (app.led) |led| {
            led.setRed(red);
            led.setBlue(blue);
        }
    }
}

// ============================================================================
// MQTT Connection (TCP)
// ============================================================================

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

    var mux = mqtt0.Mux(MqttRt).init(alloc) catch |err| {
        log.err("Mux init failed: {}", .{err});
        socket.close();
        return;
    };

    var client = TcpMqttClient.init(&socket, &mux, .{
        .client_id = config.gear_id,
        .username = config.mqtt.username,
        .password = config.mqtt.password,
        .allocator = alloc,
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

    var port = TcpPort.init(&conn, g_epoch_offset);
    initPort(&port);
    app_state.* = .running;

    // MQTT readLoop in background
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

    // Each component heap-allocated in PSRAM to avoid Zig value-copy issues.

    const sock = alloc.create(Socket) catch |err| {
        log.err("alloc socket: {}", .{err});
        return;
    };
    sock.* = Socket.tcp() catch |err| {
        log.err("TCP socket failed: {}", .{err});
        return;
    };
    sock.setRecvTimeout(30000);
    sock.connect(ip, config.mqtt.port) catch |err| {
        log.err("TCP connect failed: {}", .{err});
        sock.close();
        Board.time.sleepMs(3000);
        return;
    };
    log.info("TCP connected", .{});

    // TLS handshake
    log.info("TLS handshake...", .{});
    const tls_c = alloc.create(TlsClient) catch |err| {
        log.err("alloc tls: {}", .{err});
        return;
    };
    tls_c.* = TlsClient.init(sock, .{
        .hostname = config.mqtt.host,
        .allocator = alloc,
        .skip_verify = true,
        .timeout_ms = 30000,
    }) catch |err| {
        log.err("TLS init failed: {}", .{err});
        sock.close();
        Board.time.sleepMs(3000);
        return;
    };
    tls_c.connect() catch |err| {
        log.err("TLS handshake failed: {}", .{err});
        tls_c.deinit();
        Board.time.sleepMs(3000);
        return;
    };
    log.info("TLS connected!", .{});

    // MQTT mux
    const mux = alloc.create(mqtt0.Mux(MqttRt)) catch |err| {
        log.err("alloc mux: {}", .{err});
        return;
    };
    mux.* = mqtt0.Mux(MqttRt).init(alloc) catch |err| {
        log.err("Mux init failed: {}", .{err});
        return;
    };

    // MQTT client
    log.info("MQTT connecting...", .{});
    const client = alloc.create(TlsMqttClient) catch |err| {
        log.err("alloc client: {}", .{err});
        return;
    };
    client.* = TlsMqttClient.init(tls_c, mux, .{
        .client_id = config.gear_id,
        .username = config.mqtt.username,
        .password = config.mqtt.password,
        .keep_alive = 30,
        .protocol_version = .v5,
        .allocator = alloc,
    }) catch |err| {
        log.err("MQTT connect failed: {}", .{err});
        Board.time.sleepMs(3000);
        return;
    };
    log.info("MQTT connected!", .{});

    // ChatGear connection
    const conn = alloc.create(TlsConn) catch |err| {
        log.err("alloc conn: {}", .{err});
        return;
    };
    conn.* = TlsConn.init(client, .{
        .scope = config.scope,
        .gear_id = config.gear_id,
    });
    // Register downlink mux handlers (before readLoop starts, after conn has topics)
    mux.handleFn(conn.commandTopic(), onDownlinkCommand) catch |err| {
        log.err("register command handler: {}", .{err});
        return;
    };
    mux.handleFn(conn.outputAudioTopic(), onDownlinkAudio) catch |err| {
        log.err("register audio handler: {}", .{err});
        return;
    };
    log.info("Registered downlink handlers", .{});

    conn.subscribe() catch |err| {
        log.err("MQTT subscribe failed: {}", .{err});
        return;
    };
    log.info("MQTT subscribed to downlink topics", .{});

    // ChatGear port — the single source of truth for device state
    const port = alloc.create(TlsPort) catch |err| {
        log.err("alloc port: {}", .{err});
        return;
    };
    port.* = TlsPort.init(conn, g_epoch_offset);

    // Initialize port state (battery, volume, version) and start periodic reporting
    initPort(port);

    // Initialize audio system (ES7210 + ES8311 + I2S + AEC + PA + LED)
    const hw_init = initAudioSystem() orelse {
        log.err("Audio system init failed — continuing without audio", .{});
        // Still allow state/stats/command, just no audio
        const ctx = alloc.create(AppCtx) catch return;
        ctx.* = .{
            .port = port,
            .conn = conn,
            .client = client,
            .audio = undefined,
            .led = null,
            .encoder = undefined,
            .decoder = undefined,
            .epoch_offset = g_epoch_offset,
        };
        g_app = ctx;
        app_state.* = .running;
        spawnMqttTasks(client);
        return;
    };

    // Init opus codec
    log.info("Initializing opus codec...", .{});
    const enc = alloc.create(opus.Encoder) catch |err| {
        log.err("alloc encoder: {}", .{err});
        return;
    };
    enc.* = opus.Encoder.init(alloc, platform.sample_rate, 1, .voip) catch |err| {
        log.err("opus encoder init: {}", .{err});
        return;
    };
    enc.setBitrate(24000) catch {};
    enc.setComplexity(0) catch {};
    enc.setSignal(.voice) catch {};
    log.info("Opus encoder ready", .{});

    const dec = alloc.create(opus.Decoder) catch |err| {
        log.err("alloc decoder: {}", .{err});
        return;
    };
    dec.* = opus.Decoder.init(alloc, platform.sample_rate, 1) catch |err| {
        log.err("opus decoder init: {}", .{err});
        return;
    };
    log.info("Opus decoder ready", .{});

    // Build application context — single struct shared by all tasks
    const ctx = alloc.create(AppCtx) catch |err| {
        log.err("alloc AppCtx: {}", .{err});
        return;
    };
    ctx.* = .{
        .port = port,
        .conn = conn,
        .client = client,
        .audio = hw_init.audio,
        .led = hw_init.led,
        .encoder = enc,
        .decoder = dec,
        .epoch_offset = g_epoch_offset,
    };
    g_app = ctx;

    // Startup tone (confirms speaker works)
    playTone(hw_init.audio, 440, 100);
    Board.time.sleepMs(50);
    playTone(hw_init.audio, 880, 100);
    log.info("Startup tone played", .{});

    // Start audio pipeline (mic capture + speaker playback + command handler)
    startAudioPipeline(ctx);

    // Run automated test sequence (exercises full protocol)
    runAutoTest(ctx);

    // Enter manual button mode
    log.info("Auto test done. Manual button mode.", .{});
    app_state.* = .running;

    playReadyBeeps(hw_init.audio);
    setLed(false, true);

    // MQTT readLoop + keepalive in background
    spawnMqttTasks(client);
}

/// Spawn MQTT background tasks (readLoop + keepalive).
fn spawnMqttTasks(client: *TlsMqttClient) void {
    FullRt.spawn("mqtt_rx", struct {
        fn run(arg: ?*anyopaque) void {
            const c: *TlsMqttClient = @ptrCast(@alignCast(arg));
            c.readLoop() catch |err| {
                log.err("MQTT read loop error: {}", .{err});
            };
        }
    }.run, @ptrCast(client), .{ .stack_size = 65536 }) catch |err| {
        log.err("Failed to spawn MQTT reader: {}", .{err});
    };

    FullRt.spawn("mqtt_ka", struct {
        fn run(arg: ?*anyopaque) void {
            const c: *TlsMqttClient = @ptrCast(@alignCast(arg));
            hw.MqttRt.Time.sleepMs(5_000);
            while (true) {
                c.ping() catch |err| {
                    log.err("MQTT ping failed: {}", .{err});
                    return;
                };
                log.info("MQTT ping OK", .{});
                hw.MqttRt.Time.sleepMs(10_000);
            }
        }
    }.run, @ptrCast(client), .{ .stack_size = 65536 }) catch |err| {
        log.err("Failed to spawn MQTT keepalive: {}", .{err});
    };
}

// ============================================================================
// Port Initialization (shared between TCP and TLS)
// ============================================================================

/// Initialize the ClientPort with device info and start periodic reporting.
/// Matches Go: BeginBatch + Set* + EndBatch + StartPeriodicReporting + SetState.
fn initPort(p: anytype) void {
    p.beginBatch();
    p.setVolume(50);
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
// Automated Test Sequence
// ============================================================================

/// Exercise the full chatgear protocol via the port API.
/// All state/stats changes go through ClientPort — matching how the
/// Go client works. No direct conn access.
fn runAutoTest(ctx: *AppCtx) void {
    log.info("========== AUTO TEST START ==========", .{});

    const port = ctx.port;
    const audio = ctx.audio;
    const sleep = Board.time.sleepMs;

    // [0s] Startup beep
    playTone(audio, 440, 100);
    sleep(50);
    playTone(audio, 880, 100);
    log.info("[TEST] Startup beep", .{});

    // [2s] state=ready (already set by initPort, but verify by re-setting)
    sleep(2000);
    port.setState(.ready);
    log.info("[TEST] STATE -> ready", .{});

    // [4s] stats update (via port API)
    sleep(2000);
    port.setBattery(100, false);
    port.setVolume(50);
    log.info("[TEST] STATS -> bat=100 vol=50", .{});

    // [6s] state=recording + mic 3 seconds (through AudioSystem)
    sleep(2000);
    port.setState(.recording);
    log.info("[TEST] STATE -> recording, mic 3s...", .{});
    {
        const frame_size = audio.getFrameSize();
        var pcm_buf: [512]i16 = undefined;
        var opus_buf: [MAX_OPUS]u8 = undefined;
        const start = Board.time.getTimeMs();
        var frame_count: u32 = 0;

        while (Board.time.getTimeMs() - start < 3000) {
            const samples = audio.readMic(pcm_buf[0..frame_size]) catch {
                sleep(5);
                continue;
            };
            if (samples < frame_size) {
                sleep(1);
                continue;
            }

            const encoded = ctx.encoder.encode(pcm_buf[0..samples], @intCast(samples), &opus_buf) catch continue;
            if (encoded.len == 0) continue;

            // Send through port — data is copied into owned StampedFrame
            const ts: i64 = ctx.epoch_offset + @as(i64, @intCast(Board.time.getTimeMs()));
            port.sendOpusFrame(ts, encoded);
            frame_count += 1;
        }
        log.info("[TEST] Recorded {d} opus frames", .{frame_count});
    }

    // [9s] state=waiting_for_response
    port.setState(.waiting_for_response);
    log.info("[TEST] STATE -> waiting_for_response", .{});

    // [11s] state=calling
    sleep(2000);
    port.setState(.calling);
    log.info("[TEST] STATE -> calling", .{});

    // [13s] state=ready
    sleep(2000);
    port.setState(.ready);
    log.info("[TEST] STATE -> ready", .{});

    // [15s] state=streaming
    sleep(2000);
    port.setState(.streaming);
    log.info("[TEST] STATE -> streaming", .{});

    // [17s] state=ready
    sleep(2000);
    port.setState(.ready);
    log.info("[TEST] STATE -> ready", .{});

    // [19s] stats update
    sleep(2000);
    port.setBattery(85, false);
    port.setVolume(70);
    log.info("[TEST] STATS -> bat=85 vol=70", .{});

    // [21s] End beep
    sleep(2000);
    playTone(audio, 880, 80);
    sleep(50);
    playTone(audio, 880, 80);
    sleep(50);
    playTone(audio, 880, 80);
    log.info("[TEST] End beep", .{});

    log.info("========== AUTO TEST DONE ==========", .{});
}

// ============================================================================
// Audio Pipeline — mic capture + speaker playback
// ============================================================================

const FRAME_MS: u32 = 20;
const MAX_OPUS: usize = 512;

/// Mic capture task — reads mic via AudioSystem (ES7210 + AEC), opus-encodes,
/// sends to chatgear uplink. Recording is gated by port.getState() == .recording.
fn micTaskFn(arg: ?*anyopaque) void {
    const ctx: *AppCtx = @ptrCast(@alignCast(arg));
    const port = ctx.port;
    const audio = ctx.audio;
    const encoder = ctx.encoder;
    const frame_size = audio.getFrameSize(); // AEC frame size (~256 samples)
    log.info("[mic] task started (frame_size={d})", .{frame_size});

    var pcm_buf: [512]i16 = undefined;
    var opus_buf: [MAX_OPUS]u8 = undefined;
    var frame_count: u32 = 0;
    var last_log_ms: u64 = 0;

    while (true) {
        // Only capture when state is .recording
        if (port.getState() != .recording) {
            // Log summary when recording stops
            if (frame_count > 0) {
                log.info("[mic] sent {d} frames", .{frame_count});
                frame_count = 0;
            }
            Board.time.sleepMs(10);
            continue;
        }

        // readMic: I2S read + ES7210 decode + AEC echo cancellation
        // Returns clean mono i16 PCM, no manual gain needed
        const samples = audio.readMic(pcm_buf[0..frame_size]) catch |err| {
            log.err("[mic] readMic: {}", .{err});
            Board.time.sleepMs(5);
            continue;
        };
        if (samples == 0) continue;

        const encoded = encoder.encode(pcm_buf[0..samples], @intCast(samples), &opus_buf) catch |err| {
            log.err("[mic] encode: {}", .{err});
            continue;
        };
        if (encoded.len == 0) continue;

        // Send through port — StampedFrame.init copies data
        const ts: i64 = ctx.epoch_offset + @as(i64, @intCast(Board.time.getTimeMs()));
        port.sendOpusFrame(ts, encoded);
        frame_count += 1;

        // Periodic log (every 5 seconds)
        const now = Board.time.getTimeMs();
        if (now - last_log_ms >= 5000) {
            log.info("[mic] {d} frames sent", .{frame_count});
            last_log_ms = now;
        }
    }
}

/// Downlink audio task — receives opus from server, decodes, plays via AudioSystem.
fn speakerTaskFn(arg: ?*anyopaque) void {
    const ctx: *AppCtx = @ptrCast(@alignCast(arg));
    const port = ctx.port;
    const audio = ctx.audio;
    const decoder = ctx.decoder;
    log.info("[spk] task started", .{});

    var mono_buf: [512]i16 = undefined;

    while (true) {
        const sf = port.recvDownlinkAudio() orelse return;

        const decoded = decoder.decode(sf.frame(), &mono_buf, false) catch continue;
        if (decoded.len == 0) continue;

        // writeSpeaker takes mono i16, converts to stereo i32 internally
        _ = audio.writeSpeaker(decoded) catch continue;
    }
}

/// Command handler task — drains port.recvCommand() and dispatches.
/// Matches Go: go s.handleCommands() in simulator.Start().
fn commandHandlerTaskFn(arg: ?*anyopaque) void {
    const ctx: *AppCtx = @ptrCast(@alignCast(arg));
    log.info("[cmd] handler task started", .{});

    while (ctx.port.recvCommand()) |cmd| {
        handleCommand(ctx, cmd);
    }

    log.info("[cmd] handler task stopped", .{});
}

/// Start audio pipeline + command handler tasks.
fn startAudioPipeline(ctx: *AppCtx) void {
    FullRt.spawn("mic", micTaskFn, @ptrCast(ctx), .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn mic task: {}", .{err});
    };
    FullRt.spawn("spk", speakerTaskFn, @ptrCast(ctx), .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn spk task: {}", .{err});
    };
    FullRt.spawn("cmd", commandHandlerTaskFn, @ptrCast(ctx), .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn cmd task: {}", .{err});
    };
    log.info("Pipeline started (mic + speaker + command handler)", .{});
}

// ============================================================================
// Button Handling
// ============================================================================

/// Handle button events through the ClientPort API.
/// State transitions match Go geartest's StartRecording/EndRecording/
/// StartCalling/EndCalling/Cancel with proper state guards.
///
/// All state transitions go through port.setState() which:
/// 1. Updates the port's internal state (dedup: no-op if same state)
/// 2. Queues a StateEvent for the uplink tx loop to send
/// 3. Mic task sees the new state via port.getState()
fn handleButton(ctx: *AppCtx, btn: anytype) void {
    const port = ctx.port;

    // LED feedback
    if (btn.action == .press) {
        setLed(true, false);
    } else if (btn.action == .release) {
        setLed(false, true);
    }

    // Button tone on press/click
    if (btn.action == .press or btn.action == .click) {
        const idx: usize = @intFromEnum(btn.id);
        if (idx < button_tones.len) {
            playTone(ctx.audio, button_tones[idx], 80);
        }
    }

    switch (btn.id) {
        .vol_up => {
            // do: click = log current state (periodic reporting sends every 5s)
            if (btn.action == .click) {
                log.info("[do] State: {s}", .{port.getState().toString()});
            }
        },
        .vol_down => {
            // re: click = force full stats report
            if (btn.action == .click) {
                port.beginBatch();
                port.endBatch(); // sends full stats
                log.info("[re] STATS sent", .{});
            }
        },
        .set => {
            // mi: press = start recording, release = stop recording
            // Go StartRecording(): valid from ready, waiting_for_response, streaming
            // Go EndRecording(): valid from recording only
            switch (btn.action) {
                .press => {
                    const st = port.getState();
                    if (st == .ready or st == .waiting_for_response or st == .streaming) {
                        port.setState(.recording);
                        log.info("-> STATE: recording", .{});
                    } else {
                        log.info("[mi] ignored (state={s})", .{st.toString()});
                    }
                },
                .release => {
                    if (port.getState() == .recording) {
                        port.setState(.waiting_for_response);
                        log.info("-> STATE: waiting_for_response", .{});
                    }
                },
                else => {},
            }
        },
        .play => {
            // fa: click = toggle calling/ready
            // Go StartCalling(): valid from ready only
            // Go EndCalling(): valid from calling only
            if (btn.action == .click) {
                const st = port.getState();
                if (st == .calling) {
                    port.setState(.ready);
                    log.info("-> STATE: ready", .{});
                } else if (st == .ready) {
                    port.setState(.calling);
                    log.info("-> STATE: calling", .{});
                } else {
                    log.info("[fa] ignored (state={s})", .{st.toString()});
                }
            }
        },
        .mute => {
            // so: click = cancel/interrupt — return to ready
            // Go Cancel(): valid from any state except ready
            if (btn.action == .click) {
                const st = port.getState();
                if (st != .ready) {
                    port.setState(.ready);
                    log.info("-> STATE: ready (cancel from {s})", .{st.toString()});
                } else {
                    log.info("[so] already ready", .{});
                }
            }
        },
        .rec => {
            if (btn.action == .click) log.info("[la] (reserved)", .{});
        },
    }
}

// ============================================================================
// Command Handling (from server)
// ============================================================================

/// Handle commands received from the server via MQTT.
/// Matches Go geartest simulator.applyCommand() exactly.
///
/// Key behaviors copied from Go:
/// - streaming: only transition if in correct state
/// - halt: interrupt returns to ready; sleep/shutdown returns to ready
/// - reset: resets all settings to defaults via port API
/// - raise: only start calling from ready state
/// - set_volume/brightness: clamp 0-100
fn handleCommand(ctx: *AppCtx, cmd: chatgear.CommandEvent) void {
    const port = ctx.port;
    log.info("[cmd] {s}", .{cmd.cmd_type.toString()});

    switch (cmd.payload) {
        .streaming => |enabled| {
            // Go: if true and state==WaitingForResponse -> Streaming
            //     if false and state==Streaming -> Ready
            if (enabled) {
                if (port.getState() == .waiting_for_response) {
                    port.setState(.streaming);
                    log.info("[cmd] streaming ON -> streaming", .{});
                }
            } else {
                if (port.getState() == .streaming) {
                    port.setState(.ready);
                    log.info("[cmd] streaming OFF -> ready", .{});
                }
            }
        },

        .set_volume => |vol| {
            // Go: clamp 0-100, port.SetVolume(v)
            const v = std.math.clamp(vol, 0, 100);
            port.setVolume(@floatFromInt(v));
            log.info("[cmd] set_volume={d}", .{v});
        },

        .set_brightness => |br| {
            // Go: clamp 0-100, port.SetBrightness(b)
            const b = std.math.clamp(br, 0, 100);
            port.setBrightness(@floatFromInt(b));
            log.info("[cmd] set_brightness={d}", .{b});
        },

        .set_light_mode => |mode| {
            // Go: port.SetLightMode(mode)
            port.setLightMode(mode);
            log.info("[cmd] set_light_mode={s}", .{mode});
        },

        .set_wifi => |w| {
            // Go: port.SetWifiNetwork(&ConnectedWifi{SSID, RSSI, IP, Gateway})
            port.setWifiNetwork(w.ssid, "", -50);
            log.info("[cmd] set_wifi ssid={s}", .{w.ssid});
        },

        .delete_wifi => |ssid| {
            // Go: disconnect if current SSID matches, remove from store
            log.info("[cmd] delete_wifi ssid={s}", .{ssid});
        },

        .halt => |h| {
            // Go: interrupt -> if Streaming/Recording/WaitingForResponse -> Ready
            //     sleep/shutdown -> Ready
            if (h.interrupt) {
                const st = port.getState();
                if (st == .streaming or st == .recording or st == .waiting_for_response) {
                    port.setState(.ready);
                    log.info("[cmd] halt interrupt -> ready (was {s})", .{st.toString()});
                }
            } else if (h.sleep or h.shutdown) {
                port.setState(.ready);
                log.info("[cmd] halt sleep/shutdown -> ready", .{});
            }
        },

        .reset => |r| {
            // Go: reset all settings to defaults via port API
            port.setVolume(100);
            port.setBrightness(100);
            port.setLightMode("auto");
            if (r.unpair) {
                port.setWifiNetwork("", "", 0);
                port.setPairStatus("");
            }
            log.info("[cmd] reset (unpair={})", .{r.unpair});
        },

        .raise => |r| {
            // Go: if call and state==Ready -> Calling
            if (r.call) {
                if (port.getState() == .ready) {
                    port.setState(.calling);
                    log.info("[cmd] raise call -> calling", .{});
                }
            }
        },

        .ota_upgrade => {
            log.info("[cmd] ota_upgrade (acknowledged)", .{});
        },
    }
}
