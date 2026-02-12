//! Chatgear Client Port — Go-style async architecture
//!
//! High-level client port that mirrors go/pkg/chatgear/port_client.go:
//! - Channel-based queues for uplink/downlink (like Go's buffer.Buffer)
//! - Spawned tasks for periodic state/stats reporting (like Go goroutines)
//! - CancellationToken for lifecycle (like Go context.Context)
//!
//! Generic over MqttClient (transport) and Rt (runtime), so the same code
//! runs on ESP32 (RTOS tasks) and macOS (std.Thread).
//!
//! Usage:
//!   const Rt = std_impl.runtime;  // or ESP runtime
//!   var port = ClientPort(MqttClient, Rt).init(&conn);
//!   try port.startPeriodicReporting();
//!   // ... set state, receive commands via channels ...
//!   port.close();

const std = @import("std");
const types = @import("types.zig");
const wire = @import("wire.zig");
const conn_mod = @import("conn.zig");
const trait = @import("trait");
const channel = @import("channel");
const cancellation = @import("cancellation");

const CancellationToken = cancellation.CancellationToken;

/// State reporting interval: every 5 seconds.
/// Matches Go stateSendLoop ticker.
pub const STATE_INTERVAL_MS: u32 = 5_000;

/// Stats reporting base interval: every 20 seconds.
/// Matches Go statsReportLoop ticker.
pub const STATS_BASE_INTERVAL_MS: u32 = 20_000;

/// ClientPort — bidirectional audio/command port for device-side communication.
///
/// Mirrors go/pkg/chatgear/port_client.go with Go-style async:
/// - 5 channels for uplink/downlink queues
/// - Spawned tasks for periodic reporting
/// - CancellationToken for shutdown
///
/// MqttClient: any type with publish()/subscribe() (see conn.zig)
/// Rt: runtime providing Mutex, Condition, spawn (see trait.sync, trait.spawner)
pub fn ClientPort(comptime MqttClient: type, comptime Rt: type) type {
    // Validate Runtime at comptime
    comptime {
        _ = trait.sync.Mutex(Rt.Mutex);
        _ = trait.sync.Condition(Rt.Condition, Rt.Mutex);
        trait.spawner.from(Rt);
    }

    const Time = trait.time.from(Rt);
    const Conn = conn_mod.MqttClientConn(MqttClient);

    // Channel types
    const AudioCh = channel.Channel(types.StampedFrame, 256, Rt);
    const CommandCh = channel.Channel(types.CommandEvent, 32, Rt);
    const StateCh = channel.Channel(types.StateEvent, 32, Rt);
    const StatsCh = channel.Channel(types.StatsEvent, 32, Rt);

    return struct {
        const Self = @This();

        // Connection
        conn: *Conn,

        // Downlink channels (server -> device)
        downlink_audio: AudioCh,
        commands: CommandCh,

        // Uplink channels (device -> server)
        uplink_audio: AudioCh,
        uplink_state: StateCh,
        uplink_stats: StatsCh,

        // Lifecycle
        token: CancellationToken,

        // Internal state (protected by mutex)
        mutex: Rt.Mutex,
        state: types.State,
        stats: types.StatsEvent,
        stats_pending: ?types.StatsEvent,
        batch_mode: bool,

        /// Epoch offset: added to Time.getTimeMs() (uptime) to get Unix epoch ms.
        /// Set to 0 if device has NTP. Otherwise set to approximate epoch base.
        epoch_offset: i64,

        /// Initialize a new ClientPort.
        /// `epoch_offset_ms`: offset added to uptime to produce Unix epoch timestamps.
        ///   Pass 0 if device has NTP time. Otherwise pass approximate epoch base
        ///   (e.g., 1770900000000 for ~2026-02-11).
        pub fn init(connection: *Conn, epoch_offset_ms: i64) Self {
            return Self{
                .conn = connection,
                .downlink_audio = AudioCh.init(),
                .commands = CommandCh.init(),
                .uplink_audio = AudioCh.init(),
                .uplink_state = StateCh.init(),
                .uplink_stats = StatsCh.init(),
                .token = CancellationToken.init(),
                .mutex = Rt.Mutex.init(),
                .state = .ready,
                .stats = .{},
                .stats_pending = null,
                .batch_mode = false,
                .epoch_offset = epoch_offset_ms,
            };
        }

        /// Release resources.
        pub fn deinit(self: *Self) void {
            self.downlink_audio.deinit();
            self.commands.deinit();
            self.uplink_audio.deinit();
            self.uplink_state.deinit();
            self.uplink_stats.deinit();
            self.mutex.deinit();
        }

        /// Get current time as Unix epoch milliseconds.
        fn epochNow(self: *const Self) i64 {
            return self.epoch_offset + @as(i64, @intCast(Time.getTimeMs()));
        }

        // ====================================================================
        // Lifecycle
        // ====================================================================

        /// Stack size for spawned tasks. TLS encryption needs large stacks.
        const TASK_STACK_SIZE: u32 = 65536;

        /// Spawn options for chatgear background tasks.
        const task_opts: Rt.Options = .{ .stack_size = TASK_STACK_SIZE };

        /// Start background tasks for periodic state/stats reporting
        /// and uplink transmission. Matches Go StartPeriodicReporting +
        /// WriteTo goroutines.
        pub fn startPeriodicReporting(self: *Self) !void {
            try Rt.spawn("cg_state", stateSendLoopFn, @ptrCast(self), task_opts);
            try Rt.spawn("cg_stats", statsReportLoopFn, @ptrCast(self), task_opts);
            try Rt.spawn("cg_tx_audio", txAudioLoopFn, @ptrCast(self), task_opts);
            try Rt.spawn("cg_tx_state", txStateLoopFn, @ptrCast(self), task_opts);
            try Rt.spawn("cg_tx_stats", txStatsLoopFn, @ptrCast(self), task_opts);
        }

        /// Cancel all background tasks and close channels.
        pub fn close(self: *Self) void {
            self.token.cancel();
            self.downlink_audio.close();
            self.commands.close();
            self.uplink_audio.close();
            self.uplink_state.close();
            self.uplink_stats.close();
        }

        // ====================================================================
        // Downlink: push received messages to channels
        // (Called from mqtt0 Mux handlers)
        // ====================================================================

        /// Push a received opus frame to the downlink audio channel.
        /// Called from the mqtt0 Mux handler for output_audio_stream.
        pub fn pushDownlinkAudio(self: *Self, frame: types.StampedFrame) void {
            self.downlink_audio.trySend(frame) catch {};
        }

        /// Push a received command to the command channel.
        /// Called from the mqtt0 Mux handler for command topic.
        pub fn pushCommand(self: *Self, cmd: types.CommandEvent) void {
            self.commands.trySend(cmd) catch {};
        }

        /// Receive the next command (blocking).
        /// Returns null when port is closed.
        pub fn recvCommand(self: *Self) ?types.CommandEvent {
            return self.commands.recv();
        }

        /// Receive the next downlink audio frame (blocking).
        /// Returns null when port is closed.
        pub fn recvDownlinkAudio(self: *Self) ?types.StampedFrame {
            return self.downlink_audio.recv();
        }

        // ====================================================================
        // Uplink: queue data for transmission
        // ====================================================================

        /// Queue an opus frame for upload.
        /// Data is copied into the StampedFrame's inline buffer, so the
        /// caller's buffer can be reused immediately after this returns.
        pub fn sendOpusFrame(self: *Self, timestamp_ms: i64, data: []const u8) void {
            self.uplink_audio.trySend(types.StampedFrame.init(timestamp_ms, data)) catch {};
        }

        // ====================================================================
        // State Management
        // ====================================================================

        /// Set the device state and queue a state event for upload.
        /// Matches Go ClientPort.SetState().
        pub fn setState(self: *Self, s: types.State) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            if (self.state == s) return;
            self.state = s;

            const now = self.epochNow();
            const evt = types.StateEvent{
                .version = 1,
                .time = now,
                .state = s,
                .update_at = now,
            };
            self.uplink_state.trySend(evt) catch {};
        }

        /// Get the current state.
        pub fn getState(self: *Self) types.State {
            self.mutex.lock();
            defer self.mutex.unlock();
            return self.state;
        }

        // ====================================================================
        // Stats Management (two-layer: storage + pending diff)
        // ====================================================================

        /// Begin batch mode — Set* methods won't queue updates until endBatch().
        /// Matches Go ClientPort.BeginBatch().
        pub fn beginBatch(self: *Self) void {
            self.mutex.lock();
            defer self.mutex.unlock();
            self.batch_mode = true;
        }

        /// End batch mode and queue one full stats update.
        /// Matches Go ClientPort.EndBatch().
        pub fn endBatch(self: *Self) void {
            self.mutex.lock();
            defer self.mutex.unlock();
            self.batch_mode = false;
            self.queueFullStats();
        }

        /// Set volume and queue a stats diff update.
        pub fn setVolume(self: *Self, volume: f32) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            const now = self.epochNow();
            if (self.stats.volume == null) self.stats.volume = .{};
            self.stats.volume.?.percentage = volume;
            self.stats.volume.?.update_at = now;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.volume = .{ .percentage = volume, .update_at = now };
            self.flushPending();
        }

        /// Set brightness and queue a stats diff update.
        pub fn setBrightness(self: *Self, brightness: f32) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            const now = self.epochNow();
            if (self.stats.brightness == null) self.stats.brightness = .{};
            self.stats.brightness.?.percentage = brightness;
            self.stats.brightness.?.update_at = now;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.brightness = .{ .percentage = brightness, .update_at = now };
            self.flushPending();
        }

        /// Set battery status and queue a stats diff update.
        pub fn setBattery(self: *Self, pct: f32, is_charging: bool) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            if (self.stats.battery == null) self.stats.battery = .{};
            self.stats.battery.?.percentage = pct;
            self.stats.battery.?.is_charging = is_charging;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.battery = .{ .percentage = pct, .is_charging = is_charging };
            self.flushPending();
        }

        /// Set light mode and queue a stats diff update.
        /// Matches Go ClientPort.SetLightMode().
        pub fn setLightMode(self: *Self, mode: []const u8) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            const now = self.epochNow();
            if (self.stats.light_mode == null) self.stats.light_mode = .{};
            self.stats.light_mode.?.mode = mode;
            self.stats.light_mode.?.update_at = now;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.light_mode = .{ .mode = mode, .update_at = now };
            self.flushPending();
        }

        /// Set WiFi network info and queue a stats diff update.
        pub fn setWifiNetwork(self: *Self, ssid: []const u8, ip: []const u8, rssi: f32) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            self.stats.wifi_network = .{ .ssid = ssid, .ip = ip, .rssi = rssi };

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.wifi_network = .{ .ssid = ssid, .ip = ip, .rssi = rssi };
            self.flushPending();
        }

        /// Set system version and queue a stats diff update.
        pub fn setSystemVersion(self: *Self, version: []const u8) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            if (self.stats.system_version == null) self.stats.system_version = .{};
            self.stats.system_version.?.current_version = version;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.system_version = .{ .current_version = version };
            self.flushPending();
        }

        /// Set pair status and queue a stats diff update.
        pub fn setPairStatus(self: *Self, pair_with: []const u8) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            const now = self.epochNow();
            if (self.stats.pair_status == null) self.stats.pair_status = .{};
            self.stats.pair_status.?.pair_with = pair_with;
            self.stats.pair_status.?.update_at = now;

            if (self.batch_mode) return;
            self.ensurePending();
            self.stats_pending.?.pair_status = .{ .pair_with = pair_with, .update_at = now };
            self.flushPending();
        }

        // ====================================================================
        // Internal: Stats helpers (must hold mutex)
        // ====================================================================

        fn ensurePending(self: *Self) void {
            if (self.stats_pending == null) {
                self.stats_pending = .{};
            }
        }

        fn flushPending(self: *Self) void {
            if (self.stats_pending) |pending| {
                var evt = pending;
                evt.time = self.epochNow();
                self.uplink_stats.trySend(evt) catch {};
                self.stats_pending = null;
            }
        }

        fn queueFullStats(self: *Self) void {
            var evt = self.stats;
            evt.time = self.epochNow();
            self.uplink_stats.trySend(evt) catch {};
        }

        // ====================================================================
        // Background Tasks (spawned via Rt.spawn)
        // ====================================================================

        /// State send loop — every 5s, send current state unconditionally.
        /// Matches Go stateSendLoop.
        fn stateSendLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            while (!self.token.isCancelled()) {
                Time.sleepMs(STATE_INTERVAL_MS);
                if (self.token.isCancelled()) return;

                self.mutex.lock();
                const current_state = self.state;
                self.mutex.unlock();

                const now = self.epochNow();
                const evt = types.StateEvent{
                    .version = 1,
                    .time = now,
                    .state = current_state,
                    .update_at = now,
                };
                self.uplink_state.trySend(evt) catch {};
            }
        }

        /// Stats report loop — tiered periodic reporting.
        /// Matches Go statsReportLoop:
        /// - Every 60s  (20s * 3): battery, volume, brightness, light_mode, sys_ver, wifi, pair_status
        /// - Every 120s (20s * 6): shaking
        /// - Every 600s (20s * 30): (reserved)
        fn statsReportLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            var rounds: u32 = 0;

            while (!self.token.isCancelled()) {
                Time.sleepMs(STATS_BASE_INTERVAL_MS);
                if (self.token.isCancelled()) return;

                rounds += 1;
                self.sendPeriodicStats(rounds);
            }
        }

        fn sendPeriodicStats(self: *Self, rounds: u32) void {
            self.mutex.lock();
            defer self.mutex.unlock();

            self.ensurePending();
            var has_fields = false;

            switch (rounds % 3) {
                0 => {
                    // Every 60s: battery, volume, brightness, light_mode, sys_ver, wifi, pair_status
                    if (self.stats.battery) |b| {
                        self.stats_pending.?.battery = b;
                        has_fields = true;
                    }
                    if (self.stats.volume) |v| {
                        self.stats_pending.?.volume = v;
                        has_fields = true;
                    }
                    if (self.stats.brightness) |b| {
                        self.stats_pending.?.brightness = b;
                        has_fields = true;
                    }
                    if (self.stats.light_mode) |lm| {
                        self.stats_pending.?.light_mode = lm;
                        has_fields = true;
                    }
                    if (self.stats.system_version) |sv| {
                        self.stats_pending.?.system_version = sv;
                        has_fields = true;
                    }
                    if (self.stats.wifi_network) |w| {
                        self.stats_pending.?.wifi_network = w;
                        has_fields = true;
                    }
                    if (self.stats.pair_status) |ps| {
                        self.stats_pending.?.pair_status = ps;
                        has_fields = true;
                    }
                },
                1 => {
                    // Every 120s (rounds % 6 == 1): shaking
                    if (rounds % 6 != 1) return;
                    if (self.stats.shaking) |sh| {
                        self.stats_pending.?.shaking = sh;
                        has_fields = true;
                    }
                },
                else => {
                    // Every 600s (rounds % 30 == 2): reserved
                    if (rounds % 30 != 2) return;
                },
            }

            if (has_fields) {
                self.flushPending();
            } else {
                self.stats_pending = null;
            }
        }

        // ====================================================================
        // Transmission loops — drain channels and send via conn
        // ====================================================================

        fn txAudioLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            while (self.uplink_audio.recv()) |f| {
                self.conn.sendOpusFrame(f.timestamp_ms, f.frame()) catch {};
            }
        }

        fn txStateLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            while (self.uplink_state.recv()) |evt| {
                var event = evt;
                self.conn.sendState(&event) catch {};
            }
        }

        fn txStatsLoopFn(ctx: ?*anyopaque) void {
            const self: *Self = @ptrCast(@alignCast(ctx));
            while (self.uplink_stats.recv()) |evt| {
                var event = evt;
                self.conn.sendStats(&event) catch {};
            }
        }
    };
}
