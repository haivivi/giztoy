//! ChatGear E2E Test Application — Platform Independent
//!
//! Device simulator that exercises the full chatgear protocol:
//! - Connects to MQTT broker (mqtt:// or mqtts:// with auth)
//! - Manages state/stats exclusively through ClientPort
//! - Handles ADC button events for recording and calling
//! - Receives and executes commands from server
//!
//! MQTT URL format (matches Go DialMQTT):
//!   mqtt://host:1883              — TCP, no auth
//!   mqtt://user:pass@host:1883    — TCP with auth
//!   mqtts://user:pass@host:8883   — TLS with auth
//!
//! Button behavior (Korvo-2 V3 ADC buttons):
//!   do  (VOL+): send current state
//!   re  (VOL-): send stats
//!   mi  (SET):  press-and-hold = recording, release = waiting_for_response
//!   fa  (PLAY): press = toggle calling/ready
//!   so  (MUTE): reserved
//!   la  (REC):  reserved

const std = @import("std");

const platform = @import("platform.zig");
const Board = platform.Board;
const ButtonId = platform.ButtonId;
const Audio = platform.Audio;
const hw = platform.hw;
const log = Board.log;
const esp = @import("esp");
const idf = esp.idf;

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
/// Heap-allocated, passed to spawned tasks. Eliminates global mutable state.
const AppCtx = struct {
    port: *TlsPort,
    conn: *TlsConn,
    client: *TlsMqttClient,
    encoder: *opus.Encoder,
    decoder: *opus.Decoder,
    epoch_offset: i64,
};

/// The single global pointer. Set once during MQTT connect, used by button
/// handler in the main event loop. All other tasks receive AppCtx directly.
var g_app: ?*AppCtx = null;

// ============================================================================
// Application State Machine
// ============================================================================

const AppState = enum {
    connecting_wifi,
    connecting_mqtt,
    running,
};

// ============================================================================
// Audio Hardware (initialized once, used across tasks)
// ============================================================================

var g_i2c: ?idf.I2c = null;
var g_i2s: ?idf.I2s = null;
var g_pa: ?hw.PaSwitchDriver = null;
var g_spk: ?hw.SpeakerDriver = null;

const LedDriver = @import("esp").boards.korvo2_v3.LedDriver;
var g_led: ?LedDriver = null;

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

    // Init audio hardware (I2C + I2S + ES8311 speaker + PA + LEDs)
    initAudio();

    // Play startup beep (confirms speaker works)
    playTone(440, 100);
    Board.time.sleepMs(50);
    playTone(880, 100);
    log.info("Startup beep played", .{});

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

    var port = TcpPort.init(&conn, g_epoch_offset);
    initPort(&port);
    app_state.* = .running;

    playReadyBeeps();
    setLed(false, true);

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

const alloc = hw.allocator;

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

    // Init opus codec
    log.info("Initializing opus codec...", .{});
    const opus_alloc = idf.heap.psram;
    const enc = opus_alloc.create(opus.Encoder) catch |err| {
        log.err("alloc encoder: {}", .{err});
        return;
    };
    enc.* = opus.Encoder.init(opus_alloc, Audio.sample_rate, 1, .voip) catch |err| {
        log.err("opus encoder init: {}", .{err});
        return;
    };
    enc.setBitrate(24000) catch {};
    enc.setComplexity(0) catch {};
    enc.setSignal(.voice) catch {};
    log.info("Opus encoder ready", .{});

    const dec = opus_alloc.create(opus.Decoder) catch |err| {
        log.err("alloc decoder: {}", .{err});
        return;
    };
    dec.* = opus.Decoder.init(opus_alloc, Audio.sample_rate, 1) catch |err| {
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
        .encoder = enc,
        .decoder = dec,
        .epoch_offset = g_epoch_offset,
    };
    g_app = ctx;

    // Start audio pipeline (mic capture + speaker playback tasks)
    startAudioPipeline(ctx);

    // Run automated test sequence (exercises full protocol)
    runAutoTest(ctx);

    // Enter manual button mode
    log.info("Auto test done. Manual button mode.", .{});
    app_state.* = .running;

    playReadyBeeps();
    setLed(false, true);

    // MQTT readLoop in background
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

    // MQTT keep-alive ping
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
// Audio Init + Tone Playback
// ============================================================================

fn initAudio() void {
    log.info("Initializing audio...", .{});

    g_i2c = idf.I2c.init(.{
        .sda = Audio.i2c_sda,
        .scl = Audio.i2c_scl,
        .freq_hz = 400_000,
    }) catch |err| {
        log.err("I2C init failed: {}", .{err});
        return;
    };

    g_i2s = idf.I2s.init(.{
        .port = Audio.i2s_port,
        .sample_rate = Audio.sample_rate,
        .rx_channels = 1,
        .bits_per_sample = 16,
        .bclk_pin = Audio.i2s_bclk,
        .ws_pin = Audio.i2s_ws,
        .din_pin = Audio.i2s_din,
        .dout_pin = Audio.i2s_dout,
        .mclk_pin = Audio.i2s_mclk,
    }) catch |err| {
        log.err("I2S init failed: {}", .{err});
        return;
    };

    g_spk = hw.SpeakerDriver.init() catch |err| {
        log.err("Speaker init failed: {}", .{err});
        return;
    };
    g_spk.?.initWithShared(&g_i2c.?, &g_i2s.?) catch |err| {
        log.err("Speaker shared init failed: {}", .{err});
        return;
    };

    g_pa = hw.PaSwitchDriver.init(&g_i2c.?) catch |err| {
        log.err("PA init failed: {}", .{err});
        return;
    };
    g_pa.?.on() catch |err| {
        log.warn("PA enable failed: {}", .{err});
    };

    g_spk.?.setVolume(200) catch {};
    log.info("Audio initialized (ES8311 + PA)", .{});

    // Init LED driver (TCA9554 via same I2C bus)
    g_led = LedDriver.init(&g_i2c.?) catch |err| {
        log.warn("LED init failed: {} (continuing without LED)", .{err});
        g_led = null;
        return;
    };
    if (g_led) |*led| {
        led.setBlue(true);
        Board.time.sleepMs(200);
        led.off();
    }
    log.info("LED initialized (TCA9554 red+blue)", .{});
}

/// Play "ready" notification: ascending beeps
fn playReadyBeeps() void {
    playTone(523, 100); // C5
    Board.time.sleepMs(80);
    playTone(659, 100); // E5
    Board.time.sleepMs(80);
    playTone(784, 150); // G5
    Board.time.sleepMs(50);
    playTone(1047, 200); // C6
}

/// Set LED state.
fn setLed(red: bool, blue: bool) void {
    if (g_led) |*led| {
        led.setRed(red);
        led.setBlue(blue);
    }
}

/// Play a short sine wave tone at given frequency.
fn playTone(freq: u32, duration_ms: u32) void {
    const spk = &(g_spk orelse return);
    const sr = Audio.sample_rate;
    const total_frames = (sr * duration_ms) / 1000;
    const phase_inc = @as(f32, @floatFromInt(freq)) * 2.0 * std.math.pi / @as(f32, @floatFromInt(sr));

    var buf: [320]i16 = undefined; // 160 stereo frames
    var phase: f32 = 0;
    var played: u32 = 0;

    while (played < total_frames) {
        const remaining = total_frames - played;
        const frames = if (remaining < 160) remaining else 160;

        var i: usize = 0;
        while (i < frames) : (i += 1) {
            const val: i16 = @intFromFloat(@sin(phase) * 10000.0);
            buf[i * 2] = val;
            buf[i * 2 + 1] = val;
            phase += phase_inc;
            if (phase >= 2.0 * std.math.pi) phase -= 2.0 * std.math.pi;
        }

        const samples = frames * 2;
        var off: usize = 0;
        while (off < samples) {
            const written = spk.write(buf[off..samples]) catch return;
            off += written;
        }
        played += @intCast(frames);
    }

    // Flush I2S DMA with silence
    @memset(&buf, 0);
    var flush: u32 = 0;
    while (flush < 10) : (flush += 1) {
        var s_off: usize = 0;
        while (s_off < buf.len) {
            const written = spk.write(buf[s_off..]) catch break;
            s_off += written;
        }
    }
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
    const sleep = Board.time.sleepMs;
    const i2s = &(g_i2s orelse return);

    // [0s] Startup beep
    playTone(440, 100);
    sleep(50);
    playTone(880, 100);
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

    // [6s] state=recording + mic 3 seconds (through port)
    sleep(2000);
    port.setState(.recording);
    log.info("[TEST] STATE -> recording, mic 3s...", .{});
    {
        var pcm_buf: [FRAME_SAMPLES * 2]u8 = undefined;
        var opus_buf: [MAX_OPUS]u8 = undefined;
        const start = Board.time.getTimeMs();
        var frame_count: u32 = 0;

        while (Board.time.getTimeMs() - start < 3000) {
            const bytes_read = i2s.read(&pcm_buf) catch {
                sleep(5);
                continue;
            };
            if (bytes_read < FRAME_SAMPLES * 2) {
                sleep(1);
                continue;
            }

            const sample_slice = @as([*]const i16, @ptrCast(@alignCast(pcm_buf[0 .. FRAME_SAMPLES * 2].ptr)))[0..FRAME_SAMPLES];

            var gained: [FRAME_SAMPLES]i16 = undefined;
            for (sample_slice, 0..) |s, idx| {
                const v: i32 = @as(i32, s) * MIC_GAIN;
                gained[idx] = @intCast(std.math.clamp(v, std.math.minInt(i16), std.math.maxInt(i16)));
            }

            const encoded = ctx.encoder.encode(&gained, FRAME_SAMPLES, &opus_buf) catch continue;
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
    playTone(880, 80);
    sleep(50);
    playTone(880, 80);
    sleep(50);
    playTone(880, 80);
    log.info("[TEST] End beep", .{});

    log.info("========== AUTO TEST DONE ==========", .{});
}

// ============================================================================
// Audio Pipeline — mic capture + speaker playback
// ============================================================================

const FRAME_MS: u32 = 20;
const FRAME_SAMPLES: usize = Audio.sample_rate * FRAME_MS / 1000; // 320 @ 16kHz
const MAX_OPUS: usize = 512;
const MIC_GAIN: i32 = 4;

/// Mic capture task — reads I2S, opus-encodes, sends to chatgear uplink.
/// Recording is gated by port.getState() == .recording — no global flag.
fn micTaskFn(arg: ?*anyopaque) void {
    const ctx: *AppCtx = @ptrCast(@alignCast(arg));
    const port = ctx.port;
    const encoder = ctx.encoder;
    log.info("[mic] task started", .{});

    const i2s = &(g_i2s orelse return);
    var raw_buf: [FRAME_SAMPLES * 2]u8 = undefined;
    var pcm_buf: [FRAME_SAMPLES]i16 = undefined;
    var opus_buf: [MAX_OPUS]u8 = undefined;

    i2s.enableRx() catch |err| {
        log.err("[mic] enableRx: {}", .{err});
        return;
    };

    while (true) {
        // Check port state — only record when state is .recording
        if (port.getState() != .recording) {
            Board.time.sleepMs(10);
            continue;
        }

        const bytes_read = i2s.read(&raw_buf) catch {
            Board.time.sleepMs(5);
            continue;
        };
        if (bytes_read < 2) {
            Board.time.sleepMs(1);
            continue;
        }

        const sample_count = bytes_read / 2;
        const byte_slice = raw_buf[0..bytes_read];
        const sample_slice = @as([*]const i16, @ptrCast(@alignCast(byte_slice.ptr)))[0..sample_count];
        @memcpy(pcm_buf[0..sample_count], sample_slice);

        // Apply gain
        for (pcm_buf[0..sample_count]) |*s| {
            const v: i32 = @as(i32, s.*) * MIC_GAIN;
            s.* = @intCast(std.math.clamp(v, std.math.minInt(i16), std.math.maxInt(i16)));
        }

        if (sample_count < FRAME_SAMPLES) continue;

        const encoded = encoder.encode(pcm_buf[0..FRAME_SAMPLES], FRAME_SAMPLES, &opus_buf) catch |err| {
            log.err("[mic] encode: {}", .{err});
            continue;
        };
        if (encoded.len == 0) continue;

        // Send through port — StampedFrame.init copies data, so opus_buf
        // can be reused immediately without data races.
        const ts: i64 = ctx.epoch_offset + @as(i64, @intCast(Board.time.getTimeMs()));
        port.sendOpusFrame(ts, encoded);
    }
}

/// Downlink audio task — receives opus from server, decodes, plays on speaker.
fn speakerTaskFn(arg: ?*anyopaque) void {
    const ctx: *AppCtx = @ptrCast(@alignCast(arg));
    const port = ctx.port;
    const decoder = ctx.decoder;
    log.info("[spk] task started", .{});

    const spk = &(g_spk orelse return);
    var pcm_buf: [FRAME_SAMPLES * 2]i16 = undefined;

    while (true) {
        const sf = port.recvDownlinkAudio() orelse return;

        var mono_buf: [FRAME_SAMPLES]i16 = undefined;
        const decoded = decoder.decode(sf.frame(), &mono_buf, false) catch continue;
        if (decoded.len == 0) continue;

        // Mono -> stereo
        for (decoded, 0..) |sample, i| {
            pcm_buf[i * 2] = sample;
            pcm_buf[i * 2 + 1] = sample;
        }

        const stereo_len = decoded.len * 2;
        var off: usize = 0;
        while (off < stereo_len) {
            const written = spk.write(pcm_buf[off..stereo_len]) catch break;
            off += written;
        }
    }
}

/// Start audio pipeline tasks.
fn startAudioPipeline(ctx: *AppCtx) void {
    FullRt.spawn("mic", micTaskFn, @ptrCast(ctx), .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn mic task: {}", .{err});
    };
    FullRt.spawn("spk", speakerTaskFn, @ptrCast(ctx), .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn spk task: {}", .{err});
    };
    log.info("Audio pipeline started (mic + speaker)", .{});
}

// ============================================================================
// Button Handling
// ============================================================================

/// Handle button events through the ClientPort API.
/// All state transitions go through port.setState() which:
/// 1. Updates the port's internal state
/// 2. Queues a StateEvent for the periodic tx loop to send
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
            playTone(button_tones[idx], 80);
        }
    }

    switch (btn.id) {
        .vol_up => {
            // do: click = force-send current state (for debugging)
            if (btn.action == .click) {
                // Re-setting the same state won't queue because ClientPort
                // deduplicates. Use a trick: set to a different state then back.
                // Or just log — the periodic reporting already sends every 5s.
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
            switch (btn.action) {
                .press => {
                    port.setState(.recording);
                    log.info("[mi] -> recording", .{});
                },
                .release => {
                    port.setState(.waiting_for_response);
                    log.info("[mi] -> waiting_for_response", .{});
                },
                else => {},
            }
        },
        .play => {
            // fa: click = toggle calling/ready
            if (btn.action == .click) {
                if (port.getState() == .calling) {
                    port.setState(.ready);
                    log.info("[fa] -> ready", .{});
                } else {
                    port.setState(.calling);
                    log.info("[fa] -> calling", .{});
                }
            }
        },
        .mute => {
            if (btn.action == .click) log.info("[so] (reserved)", .{});
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
/// Called from the command receive task (spawned by startCommandHandler).
fn handleCommand(ctx: *AppCtx, cmd: chatgear.CommandEvent) void {
    const port = ctx.port;
    log.info("Command: {s}", .{cmd.cmd_type.toString()});

    switch (cmd.payload) {
        .streaming => |enabled| {
            if (enabled) port.setState(.streaming) else port.setState(.ready);
        },
        .set_volume => |vol| {
            port.setVolume(@floatFromInt(vol));
        },
        .set_brightness => |br| {
            port.setBrightness(@floatFromInt(br));
        },
        .halt => |h| {
            if (h.sleep) port.setState(.sleeping) else if (h.shutdown) port.setState(.shutting_down) else if (h.interrupt) port.setState(.interrupted);
        },
        .reset => port.setState(.resetting),
        .raise => |r| {
            if (r.call) port.setState(.calling);
        },
        else => {},
    }
}
