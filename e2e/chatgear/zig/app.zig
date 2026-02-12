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
const Audio = platform.Audio;
const hw = platform.hw;
const log = Board.log;
const esp = @import("esp");
const idf = esp.idf;

const chatgear = @import("chatgear");
const mqtt0 = @import("mqtt0");
const tls = @import("tls");
const dns = @import("dns");
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

// Global port pointer (set by connectTls/connectTcp, used by button handler)
var active_port: ?*TlsPort = null;

// Audio hardware (initialized in run(), used for tone playback)
var audio_speaker: ?*hw.SpeakerDriver = null;

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

    // Init audio (I2C + I2S + ES8311 speaker + PA)
    initAudio();

    // Connect WiFi
    log.info("Connecting to WiFi: {s}", .{config.wifi_ssid});
    board.wifi.connect(config.wifi_ssid, config.wifi_password);

    var app_state: AppState = .connecting_wifi;

    // Main event loop
    while (Board.isRunning()) {
        // Poll ADC buttons (pushes events to board queue)
        board.buttons.poll();

        // Process board events
        while (board.nextEvent()) |event| {
            switch (event) {
                .wifi => |wifi_event| handleWifiEvent(wifi_event, &app_state),
                .net => |net_event| handleNetEvent(net_event, &app_state),
                .button => |btn| {
                    // Play tone on click (debounced, do-re-mi-fa-so-la)
                    if (btn.action == .click) {
                        playButtonTone(@intFromEnum(btn.id));
                    }
                    // Forward to chatgear handler if port is active
                    if (active_port) |p| {
                        handleButton(p, btn);
                    }
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

const alloc = hw.allocator;

fn connectTls(app_state: *AppState) void {
    log.info("Connecting MQTT (TLS) to {s}:{d}...", .{ config.mqtt.host, config.mqtt.port });

    const ip = resolveHost(config.mqtt.host) orelse return;
    log.info("Resolved {s} -> {d}.{d}.{d}.{d}", .{ config.mqtt.host, ip[0], ip[1], ip[2], ip[3] });

    // Each component separately heap-allocated in PSRAM.
    // Avoids Zig value-copy issues with self-referential pointers.

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

    // TLS
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
    // Use gear_id as client_id — same as Go chatgear.DialMQTT
    // Go format: "chatgear-{gearID}-{rand}" but we use just gear_id for now
    client.* = TlsMqttClient.init(tls_c, mux, .{
        .client_id = config.gear_id,
        .username = config.mqtt.username,
        .password = config.mqtt.password,
        .keep_alive = 30,
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

    // Skip initial state for stability test
    log.info("(no initial state sent — testing subscribe-only)", .{});

    // Skip port for stability test

    // Init opus codec in main task (256KB stack)
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
    log.info("Opus encoder ready (16kHz mono, 24kbps)", .{});

    const dec = opus_alloc.create(opus.Decoder) catch |err| {
        log.err("alloc decoder: {}", .{err});
        return;
    };
    dec.* = opus.Decoder.init(opus_alloc, Audio.sample_rate, 1) catch |err| {
        log.err("opus decoder init: {}", .{err});
        return;
    };
    log.info("Opus decoder ready", .{});

    // Skip everything — only readLoop + ping
    enc.deinit(idf.heap.psram);
    dec.deinit(idf.heap.psram);
    app_state.* = .running;

    // MQTT readLoop in background
    FullRt.spawn("mqtt_rx", struct {
        fn run(ctx: ?*anyopaque) void {
            const c: *TlsMqttClient = @ptrCast(@alignCast(ctx));
            c.readLoop() catch |err| {
                log.err("MQTT read loop error: {}", .{err});
            };
        }
    }.run, @ptrCast(client), .{ .stack_size = 65536 }) catch |err| {
        log.err("Failed to spawn MQTT reader: {}", .{err});
    };

    // MQTT keep-alive ping every 10s
    FullRt.spawn("mqtt_ka", struct {
        fn run(ctx: ?*anyopaque) void {
            const c: *TlsMqttClient = @ptrCast(@alignCast(ctx));
            // First ping after 5s, then every 10s
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
// Audio Init + Tone Playback
// ============================================================================

var g_i2c: ?idf.I2c = null;
var g_i2s: ?idf.I2s = null;
var g_pa: ?hw.PaSwitchDriver = null;
var g_spk: ?hw.SpeakerDriver = null;

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
        .rx_channels = 1, // Mono mic (ES7210)
        .bits_per_sample = 16,
        .bclk_pin = Audio.i2s_bclk,
        .ws_pin = Audio.i2s_ws,
        .din_pin = Audio.i2s_din, // Mic input
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
    audio_speaker = &g_spk.?;
    log.info("Audio initialized (ES8311 + PA)", .{});
}

/// Play a short sine wave tone at given frequency.
/// I2S is stereo — each sample must be written twice (L+R).
fn playTone(freq: u32, duration_ms: u32) void {
    const spk = &(g_spk orelse return);
    const sr = Audio.sample_rate;
    const total_frames = (sr * duration_ms) / 1000;
    const phase_inc = @as(f32, @floatFromInt(freq)) * 2.0 * std.math.pi / @as(f32, @floatFromInt(sr));

    // 160 stereo frames = 320 i16 samples (L,R,L,R,...) = 10ms @ 16kHz
    var buf: [320]i16 = undefined;
    var phase: f32 = 0;
    var played: u32 = 0;

    while (played < total_frames) {
        const remaining = total_frames - played;
        const frames = if (remaining < 160) remaining else 160;

        // Generate stereo: duplicate mono sample to L and R
        var i: usize = 0;
        while (i < frames) : (i += 1) {
            const val: i16 = @intFromFloat(@sin(phase) * 10000.0);
            buf[i * 2] = val; // Left
            buf[i * 2 + 1] = val; // Right
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

    // Flush I2S DMA ring buffer with multiple rounds of silence
    // I2S DMA typically has 6-8 descriptors × 320 samples each
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

/// Play button-specific tone (do re mi fa so la)
fn playButtonTone(button_idx: usize) void {
    if (button_idx < button_tones.len) {
        playTone(button_tones[button_idx], 150);
    }
}

// ============================================================================
// Audio Pipeline — mic capture + opus encode/decode + speaker playback
// ============================================================================

const FRAME_MS: u32 = 20;
const FRAME_SAMPLES: usize = Audio.sample_rate * FRAME_MS / 1000; // 320 @ 16kHz
const MAX_OPUS: usize = 512;
const MIC_GAIN: i32 = 4;

var g_recording: bool = false;

const AudioCtx = struct {
    port: *TlsPort,
    encoder: *opus.Encoder,
    decoder: *opus.Decoder,
};

var g_audio_ctx: ?*AudioCtx = null;

/// Mic capture task — reads mic, opus encode, sends to chatgear uplink
fn micTaskFn(_: ?*anyopaque) void {
    const actx = g_audio_ctx orelse return;
    const port = actx.port;
    const encoder = actx.encoder;
    log.info("[mic] task started", .{});

    const i2s = &(g_i2s orelse return);
    var raw_buf: [FRAME_SAMPLES * 2]u8 = undefined; // 16-bit samples = 2 bytes each
    var pcm_buf: [FRAME_SAMPLES]i16 = undefined;
    var opus_buf: [MAX_OPUS]u8 = undefined;

    // Enable I2S RX
    i2s.enableRx() catch |err| {
        log.err("[mic] enableRx: {}", .{err});
        return;
    };

    while (true) {
        // Wait until recording
        if (!g_recording) {
            Board.time.sleepMs(10);
            continue;
        }

        // Read raw bytes from I2S
        const bytes_read = i2s.read(&raw_buf) catch {
            Board.time.sleepMs(5);
            continue;
        };
        if (bytes_read < 2) {
            Board.time.sleepMs(1);
            continue;
        }

        // Convert bytes to i16 samples (little-endian)
        const sample_count = bytes_read / 2;
        const byte_slice = raw_buf[0..bytes_read];
        const sample_slice = @as([*]const i16, @ptrCast(@alignCast(byte_slice.ptr)))[0..sample_count];
        @memcpy(pcm_buf[0..sample_count], sample_slice);
        const n = sample_count;

        // Gain
        for (&pcm_buf) |*s| {
            const v: i32 = @as(i32, s.*) * MIC_GAIN;
            s.* = @intCast(std.math.clamp(v, std.math.minInt(i16), std.math.maxInt(i16)));
        }

        // Only encode full frames
        if (n < FRAME_SAMPLES) continue;

        // Opus encode
        const encoded = encoder.encode(pcm_buf[0..FRAME_SAMPLES], FRAME_SAMPLES, &opus_buf) catch |err| {
            log.err("[mic] encode: {}", .{err});
            continue;
        };
        if (encoded.len == 0) continue;

        // Send opus frame through chatgear port
        const ts: i64 = @intCast(Board.time.getTimeMs());
        port.sendOpusFrame(ts, encoded);
    }
}

/// Downlink audio task — receives opus from MQTT, decodes, plays on speaker
fn speakerTaskFn(_: ?*anyopaque) void {
    const actx = g_audio_ctx orelse return;
    const port = actx.port;
    const decoder = actx.decoder;
    log.info("[spk] task started", .{});

    const spk = &(g_spk orelse return);
    var pcm_buf: [FRAME_SAMPLES * 2]i16 = undefined; // stereo output

    while (true) {
        // Block until we get a downlink audio frame
        const frame = port.recvDownlinkAudio() orelse return;

        // Decode opus -> mono PCM
        var mono_buf: [FRAME_SAMPLES]i16 = undefined;
        const decoded = decoder.decode(frame.frame, &mono_buf, false) catch continue;
        if (decoded.len == 0) continue;

        // Mono -> stereo (I2S is stereo)
        for (decoded, 0..) |sample, i| {
            pcm_buf[i * 2] = sample;
            pcm_buf[i * 2 + 1] = sample;
        }

        // Write to speaker
        const stereo_len = decoded.len * 2;
        var off: usize = 0;
        while (off < stereo_len) {
            const written = spk.write(pcm_buf[off..stereo_len]) catch break;
            off += written;
        }
    }
}

/// Start audio pipeline tasks (called after chatgear port is ready)
fn startAudioPipeline(port: *TlsPort, enc: *opus.Encoder, dec: *opus.Decoder) void {
    // Heap-allocate context so it outlives this function
    const ctx = alloc.create(AudioCtx) catch |err| {
        log.err("alloc audio ctx: {}", .{err});
        return;
    };
    ctx.* = .{ .port = port, .encoder = enc, .decoder = dec };
    g_audio_ctx = ctx;

    FullRt.spawn("mic", micTaskFn, null, .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn mic task: {}", .{err});
    };
    FullRt.spawn("spk", speakerTaskFn, null, .{ .stack_size = 32768 }) catch |err| {
        log.err("spawn spk task: {}", .{err});
    };
    log.info("Audio pipeline started (mic + speaker)", .{});
}

// ============================================================================
// Raw MQTT CONNECT packet builder (for debug)
// ============================================================================

fn buildMqttConnect(buf: []u8, client_id: []const u8, username: []const u8, password: []const u8) usize {
    // MQTT 3.1.1 CONNECT: fixed header + variable header + payload
    var payload_buf: [200]u8 = undefined;
    var pos: usize = 0;

    // Variable header: Protocol Name "MQTT"
    payload_buf[pos] = 0;
    payload_buf[pos + 1] = 4;
    pos += 2;
    @memcpy(payload_buf[pos..][0..4], "MQTT");
    pos += 4;

    // Protocol Level (4 = MQTT 3.1.1)
    payload_buf[pos] = 4;
    pos += 1;

    // Connect Flags: username + password + clean session
    var flags: u8 = 0x02; // clean session
    if (username.len > 0) flags |= 0x80; // username flag
    if (password.len > 0) flags |= 0x40; // password flag
    payload_buf[pos] = flags;
    pos += 1;

    // Keep Alive (60s)
    payload_buf[pos] = 0;
    payload_buf[pos + 1] = 60;
    pos += 2;

    // Payload: Client ID
    payload_buf[pos] = @truncate(client_id.len >> 8);
    payload_buf[pos + 1] = @truncate(client_id.len);
    pos += 2;
    @memcpy(payload_buf[pos..][0..client_id.len], client_id);
    pos += client_id.len;

    // Payload: Username
    if (username.len > 0) {
        payload_buf[pos] = @truncate(username.len >> 8);
        payload_buf[pos + 1] = @truncate(username.len);
        pos += 2;
        @memcpy(payload_buf[pos..][0..username.len], username);
        pos += username.len;
    }

    // Payload: Password
    if (password.len > 0) {
        payload_buf[pos] = @truncate(password.len >> 8);
        payload_buf[pos + 1] = @truncate(password.len);
        pos += 2;
        @memcpy(payload_buf[pos..][0..password.len], password);
        pos += password.len;
    }

    // Fixed header: packet type (CONNECT = 0x10) + remaining length
    buf[0] = 0x10;
    buf[1] = @truncate(pos);
    @memcpy(buf[2..][0..pos], payload_buf[0..pos]);
    return 2 + pos;
}

// ============================================================================
// Button Handling
// ============================================================================

fn handleButton(p: anytype, btn: anytype) void {
    switch (btn.id) {
        .vol_up => {
            // do: click = send state event
            if (btn.action == .click) {
                log.info("[do] send state", .{});
                p.setState(p.getState()); // re-send current state
            }
        },
        .vol_down => {
            // re: click = send stats event
            if (btn.action == .click) {
                log.info("[re] send stats", .{});
                p.endBatch(); // triggers full stats send
            }
        },
        .set => {
            // mi: press = start recording + mic capture, release = stop
            switch (btn.action) {
                .press => {
                    log.info("[mi] recording (mic on)", .{});
                    g_recording = true;
                    p.setState(.recording);
                },
                .release => {
                    log.info("[mi] waiting_for_response ({}ms)", .{btn.duration_ms});
                    g_recording = false;
                    p.setState(.waiting_for_response);
                },
                else => {},
            }
        },
        .play => {
            // fa: click = toggle calling/ready
            if (btn.action == .click) {
                const current = p.getState();
                if (current == .calling) {
                    log.info("[fa] exit calling -> ready", .{});
                    p.setState(.ready);
                } else {
                    log.info("[fa] enter calling", .{});
                    p.setState(.calling);
                }
            }
        },
        .mute => {
            // so: (reserved)
            if (btn.action == .click) {
                log.info("[so] (reserved)", .{});
            }
        },
        .rec => {
            // la: (reserved)
            if (btn.action == .click) {
                log.info("[la] (reserved)", .{});
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
