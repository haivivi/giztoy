//! MQTT 5.0 Client Implementation
//!
//! Provides a lightweight MQTT 5.0 client with Topic Alias support.
//! Uses trait interfaces for socket, time, and logging.

const packet = @import("packet.zig");

// Re-export commonly used types
pub const ConnectConfig = packet.ConnectConfig;
pub const PublishOptions = packet.PublishOptions;
pub const ReasonCode = packet.ReasonCode;
pub const Properties = packet.Properties;
pub const Error = packet.Error;

// ============================================================================
// Topic Alias Manager
// ============================================================================

/// Simple hash function for topic strings (FNV-1a)
fn hashTopic(topic: []const u8) u32 {
    var hash: u32 = 2166136261; // FNV offset basis
    for (topic) |byte| {
        hash ^= byte;
        hash *%= 16777619; // FNV prime
    }
    return hash;
}

/// Topic Alias Manager for client-to-server direction
/// Manages a fixed-size mapping table from topic hash to alias
pub fn TopicAliasManager(comptime max_aliases: u16) type {
    return struct {
        const Self = @This();

        const Entry = struct {
            topic_hash: u32,
            alias: u16,
            used: bool,
        };

        entries: [max_aliases]Entry,
        count: u16,
        server_max: u16, // Maximum aliases supported by server

        /// Initialize the manager
        pub fn init() Self {
            var self = Self{
                .entries = undefined,
                .count = 0,
                .server_max = 0,
            };
            // Initialize all entries as unused
            for (&self.entries) |*entry| {
                entry.used = false;
                entry.topic_hash = 0;
                entry.alias = 0;
            }
            return self;
        }

        /// Set the server's maximum topic alias (from CONNACK)
        pub fn setServerMax(self: *Self, max: u16) void {
            self.server_max = max;
        }

        /// Reset all aliases (e.g., on reconnect)
        pub fn reset(self: *Self) void {
            for (&self.entries) |*entry| {
                entry.used = false;
            }
            self.count = 0;
        }

        /// Get existing alias for a topic, or null if not found
        pub fn get(self: *const Self, topic: []const u8) ?u16 {
            if (self.server_max == 0) return null;

            const hash = hashTopic(topic);
            for (self.entries[0..self.count]) |entry| {
                if (entry.used and entry.topic_hash == hash) {
                    return entry.alias;
                }
            }
            return null;
        }

        /// Get or create an alias for a topic
        /// Returns: .{ alias, is_new }
        /// - If alias exists: returns existing alias, is_new = false
        /// - If new alias created: returns new alias, is_new = true
        /// - If no more aliases available: returns null
        pub fn getOrCreate(self: *Self, topic: []const u8) ?struct { alias: u16, is_new: bool } {
            if (self.server_max == 0) return null;

            const hash = hashTopic(topic);

            // Check if already exists
            for (self.entries[0..self.count]) |entry| {
                if (entry.used and entry.topic_hash == hash) {
                    return .{ .alias = entry.alias, .is_new = false };
                }
            }

            // Check if we can create a new one
            if (self.count >= self.server_max or self.count >= max_aliases) {
                return null;
            }

            // Create new alias (1-based)
            const new_alias = self.count + 1;
            self.entries[self.count] = .{
                .topic_hash = hash,
                .alias = new_alias,
                .used = true,
            };
            self.count += 1;

            return .{ .alias = new_alias, .is_new = true };
        }

        /// Check if topic alias is enabled
        pub fn isEnabled(self: *const Self) bool {
            return self.server_max > 0;
        }
    };
}

// ============================================================================
// Client Error Types
// ============================================================================

pub const ClientError = error{
    // Packet errors
    BufferTooSmall,
    MalformedPacket,
    MalformedVariableInt,
    MalformedString,
    UnknownPacketType,
    ProtocolError,
    UnsupportedProtocolVersion,

    // Connection errors
    ConnectionFailed,
    ConnectionClosed,
    ConnectionRefused,
    Timeout,
    SendFailed,
    RecvFailed,

    // MQTT errors
    NotConnected,
    SubscribeFailed,
    UnexpectedPacket,
};

// ============================================================================
// Received Message
// ============================================================================

/// A received MQTT message
pub const Message = struct {
    topic: []const u8,
    payload: []const u8,
    retain: bool,
};

// ============================================================================
// MQTT Client
// ============================================================================

/// MQTT 5.0 Client
/// Generic over Socket type - works with raw TCP or TLS
pub fn MqttClient(comptime Socket: type, comptime Log: type, comptime Time: type) type {
    return struct {
        const Self = @This();

        // Maximum topic aliases we maintain
        const MaxTopicAliases = 32;

        socket: *Socket,
        connected: bool,
        next_packet_id: u16,
        keep_alive_ms: u32,
        last_activity_ms: u64,

        // Topic alias management
        topic_aliases: TopicAliasManager(MaxTopicAliases),

        // Server-to-client topic alias table (for received messages)
        // Maps alias -> topic (stored as hash for simplicity)
        recv_topic_aliases: [MaxTopicAliases]struct {
            alias: u16,
            topic: [256]u8,
            topic_len: usize,
            used: bool,
        },

        /// Initialize a new client
        pub fn init(socket: *Socket) Self {
            var self = Self{
                .socket = socket,
                .connected = false,
                .next_packet_id = 1,
                .keep_alive_ms = 60000,
                .last_activity_ms = 0,
                .topic_aliases = TopicAliasManager(MaxTopicAliases).init(),
                .recv_topic_aliases = undefined,
            };

            // Initialize receive alias table
            for (&self.recv_topic_aliases) |*entry| {
                entry.used = false;
                entry.alias = 0;
                entry.topic_len = 0;
            }

            return self;
        }

        /// Connect to the MQTT broker
        pub fn connect(self: *Self, config: *const ConnectConfig, buf: []u8) ClientError!void {
            // Encode CONNECT packet
            const connect_len = packet.encodeConnect(buf, config) catch |e| return mapPacketError(e);

            // Send CONNECT
            self.send(buf[0..connect_len]) catch return ClientError.SendFailed;

            // Receive CONNACK
            const recv_len = self.recv(buf) catch return ClientError.RecvFailed;
            if (recv_len == 0) return ClientError.ConnectionClosed;

            // Decode response
            const result = packet.decodePacket(buf[0..recv_len]) catch |e| return mapPacketError(e);

            switch (result.packet) {
                .connack => |connack| {
                    if (connack.reason_code != .success) {
                        Log.err("CONNACK failed: reason={d}", .{@intFromEnum(connack.reason_code)});
                        return ClientError.ConnectionRefused;
                    }

                    // Store topic alias maximum from server
                    if (connack.props.topic_alias_maximum) |max| {
                        self.topic_aliases.setServerMax(max);
                        Log.info("Server supports topic alias max: {d}", .{max});
                    }

                    // Store keep alive if server specified
                    if (connack.props.server_keep_alive) |ka| {
                        self.keep_alive_ms = @as(u32, ka) * 1000;
                    } else {
                        self.keep_alive_ms = @as(u32, config.keep_alive) * 1000;
                    }

                    self.connected = true;
                    self.last_activity_ms = Time.getTimeMs();
                    Log.info("Connected to MQTT broker", .{});
                },
                else => return ClientError.UnexpectedPacket,
            }
        }

        /// Publish a message
        /// Automatically uses topic alias if available to save bandwidth
        pub fn publish(self: *Self, topic: []const u8, payload: []const u8, buf: []u8) ClientError!void {
            if (!self.connected) return ClientError.NotConnected;

            var opts = PublishOptions{
                .topic = topic,
                .payload = payload,
            };

            // Try to use topic alias
            if (self.topic_aliases.getOrCreate(topic)) |alias_result| {
                opts.topic_alias = alias_result.alias;
                if (!alias_result.is_new) {
                    // Alias already established, send empty topic
                    opts.topic = "";
                }
            }

            const pub_len = packet.encodePublish(buf, &opts) catch |e| return mapPacketError(e);
            self.send(buf[0..pub_len]) catch return ClientError.SendFailed;
            self.last_activity_ms = Time.getTimeMs();
        }

        /// Publish with explicit options (for retain, etc.)
        pub fn publishWithOptions(self: *Self, opts: *const PublishOptions, buf: []u8) ClientError!void {
            if (!self.connected) return ClientError.NotConnected;

            const pub_len = packet.encodePublish(buf, opts) catch |e| return mapPacketError(e);
            self.send(buf[0..pub_len]) catch return ClientError.SendFailed;
            self.last_activity_ms = Time.getTimeMs();
        }

        /// Subscribe to topics
        pub fn subscribe(self: *Self, topics: []const []const u8, buf: []u8) ClientError!void {
            if (!self.connected) return ClientError.NotConnected;

            const pkt_id = self.nextPacketId();
            const sub_len = packet.encodeSubscribe(buf, pkt_id, topics) catch |e| return mapPacketError(e);

            self.send(buf[0..sub_len]) catch return ClientError.SendFailed;

            // Wait for SUBACK
            const recv_len = self.recv(buf) catch return ClientError.RecvFailed;
            if (recv_len == 0) return ClientError.ConnectionClosed;

            const result = packet.decodePacket(buf[0..recv_len]) catch |e| return mapPacketError(e);

            switch (result.packet) {
                .suback => |suback| {
                    if (suback.packet_id != pkt_id) {
                        return ClientError.ProtocolError;
                    }

                    // Check all reason codes
                    var i: usize = 0;
                    while (i < suback.reason_code_count) : (i += 1) {
                        const code = suback.reason_codes[i];
                        if (@intFromEnum(code) >= 0x80) {
                            Log.err("Subscribe failed for topic {d}: reason={d}", .{ i, @intFromEnum(code) });
                            return ClientError.SubscribeFailed;
                        }
                    }

                    self.last_activity_ms = Time.getTimeMs();
                    Log.info("Subscribed to {d} topics", .{topics.len});
                },
                else => return ClientError.UnexpectedPacket,
            }
        }

        /// Unsubscribe from topics
        pub fn unsubscribe(self: *Self, topics: []const []const u8, buf: []u8) ClientError!void {
            if (!self.connected) return ClientError.NotConnected;

            const pkt_id = self.nextPacketId();
            const unsub_len = packet.encodeUnsubscribe(buf, pkt_id, topics) catch |e| return mapPacketError(e);

            self.send(buf[0..unsub_len]) catch return ClientError.SendFailed;
            self.last_activity_ms = Time.getTimeMs();

            // Note: We don't wait for UNSUBACK in this simple implementation
        }

        /// Receive a message (non-blocking if socket supports it)
        /// Returns null if no message available
        pub fn recvMessage(self: *Self, buf: []u8) ClientError!?Message {
            if (!self.connected) return ClientError.NotConnected;

            const recv_len = self.recv(buf) catch |e| {
                // Timeout is not an error for non-blocking recv
                if (e == error.Timeout) return null;
                return ClientError.RecvFailed;
            };

            if (recv_len == 0) {
                self.connected = false;
                return ClientError.ConnectionClosed;
            }

            const result = packet.decodePacket(buf[0..recv_len]) catch |e| return mapPacketError(e);

            switch (result.packet) {
                .publish => |pub_pkt| {
                    var topic = pub_pkt.topic;

                    // Handle topic alias (server-to-client)
                    if (pub_pkt.props.topic_alias) |alias| {
                        if (topic.len > 0) {
                            // New alias mapping
                            self.storeRecvTopicAlias(alias, topic);
                        } else {
                            // Use existing alias
                            topic = self.getRecvTopicAlias(alias) orelse return ClientError.ProtocolError;
                        }
                    }

                    self.last_activity_ms = Time.getTimeMs();

                    return Message{
                        .topic = topic,
                        .payload = pub_pkt.payload,
                        .retain = pub_pkt.retain,
                    };
                },
                .pingresp => {
                    self.last_activity_ms = Time.getTimeMs();
                    return null; // Not a message
                },
                .disconnect => |disc| {
                    Log.warn("Server disconnected: reason={d}", .{@intFromEnum(disc.reason_code)});
                    self.connected = false;
                    return ClientError.ConnectionClosed;
                },
                else => return null,
            }
        }

        /// Send PINGREQ to keep connection alive
        pub fn ping(self: *Self, buf: []u8) ClientError!void {
            if (!self.connected) return ClientError.NotConnected;

            const ping_len = packet.encodePingReq(buf) catch |e| return mapPacketError(e);
            self.send(buf[0..ping_len]) catch return ClientError.SendFailed;
            self.last_activity_ms = Time.getTimeMs();
        }

        /// Check if keepalive ping is needed
        pub fn needsPing(self: *const Self) bool {
            if (!self.connected) return false;
            const now = Time.getTimeMs();
            const elapsed = now - self.last_activity_ms;
            // Send ping at half the keepalive interval
            return elapsed >= self.keep_alive_ms / 2;
        }

        /// Disconnect from the broker
        pub fn disconnect(self: *Self, buf: []u8) void {
            if (!self.connected) return;

            const disc_len = packet.encodeDisconnect(buf, .normal_disconnection) catch return;
            self.send(buf[0..disc_len]) catch {};

            self.connected = false;
            self.topic_aliases.reset();
            Log.info("Disconnected from MQTT broker", .{});
        }

        /// Check if connected
        pub fn isConnected(self: *const Self) bool {
            return self.connected;
        }

        /// Reset for reconnection
        pub fn resetForReconnect(self: *Self) void {
            self.connected = false;
            self.topic_aliases.reset();
            // Reset receive alias table
            for (&self.recv_topic_aliases) |*entry| {
                entry.used = false;
            }
        }

        // ====================================================================
        // Private helpers
        // ====================================================================

        fn nextPacketId(self: *Self) u16 {
            const id = self.next_packet_id;
            self.next_packet_id +%= 1;
            if (self.next_packet_id == 0) self.next_packet_id = 1;
            return id;
        }

        fn send(self: *Self, data: []const u8) !void {
            var sent: usize = 0;
            while (sent < data.len) {
                const n = self.socket.send(data[sent..]) catch return error.SendFailed;
                if (n == 0) return error.SendFailed;
                sent += n;
            }
        }

        fn recv(self: *Self, buf: []u8) !usize {
            return self.socket.recv(buf) catch |e| {
                // Propagate Timeout error for non-blocking recv support
                if (e == error.Timeout) return error.Timeout;
                return error.RecvFailed;
            };
        }

        fn storeRecvTopicAlias(self: *Self, alias: u16, topic: []const u8) void {
            if (alias == 0 or alias > MaxTopicAliases) return;
            const idx = alias - 1;

            self.recv_topic_aliases[idx].alias = alias;
            self.recv_topic_aliases[idx].used = true;

            const copy_len = if (topic.len > 256) 256 else topic.len;
            for (topic[0..copy_len], 0..) |c, i| {
                self.recv_topic_aliases[idx].topic[i] = c;
            }
            self.recv_topic_aliases[idx].topic_len = copy_len;
        }

        fn getRecvTopicAlias(self: *const Self, alias: u16) ?[]const u8 {
            if (alias == 0 or alias > MaxTopicAliases) return null;
            const idx = alias - 1;

            if (!self.recv_topic_aliases[idx].used) return null;
            return self.recv_topic_aliases[idx].topic[0..self.recv_topic_aliases[idx].topic_len];
        }

        fn mapPacketError(e: packet.Error) ClientError {
            return switch (e) {
                packet.Error.BufferTooSmall => ClientError.BufferTooSmall,
                packet.Error.MalformedPacket => ClientError.MalformedPacket,
                packet.Error.MalformedVariableInt => ClientError.MalformedVariableInt,
                packet.Error.MalformedString => ClientError.MalformedString,
                packet.Error.UnknownPacketType => ClientError.UnknownPacketType,
                packet.Error.ProtocolError => ClientError.ProtocolError,
                packet.Error.UnsupportedProtocolVersion => ClientError.UnsupportedProtocolVersion,
            };
        }
    };
}
