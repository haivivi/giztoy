//! MQTT 5.0 Packet Encoding/Decoding
//!
//! This module provides low-level packet encoding and decoding for MQTT 5.0 protocol.
//! It does NOT use std library - all operations are manual byte manipulation.

// ============================================================================
// Error Types
// ============================================================================

pub const Error = error{
    BufferTooSmall,
    MalformedPacket,
    MalformedVariableInt,
    MalformedString,
    UnknownPacketType,
    ProtocolError,
    UnsupportedProtocolVersion,
};

// ============================================================================
// Constants
// ============================================================================

pub const protocol_name = "MQTT";
pub const protocol_version: u8 = 5; // MQTT 5.0

// ============================================================================
// Packet Types
// ============================================================================

pub const PacketType = enum(u4) {
    reserved = 0,
    connect = 1,
    connack = 2,
    publish = 3,
    puback = 4,
    pubrec = 5,
    pubrel = 6,
    pubcomp = 7,
    subscribe = 8,
    suback = 9,
    unsubscribe = 10,
    unsuback = 11,
    pingreq = 12,
    pingresp = 13,
    disconnect = 14,
    auth = 15,
};

// ============================================================================
// MQTT 5.0 Property Identifiers
// ============================================================================

pub const PropertyId = enum(u8) {
    payload_format = 0x01,
    message_expiry = 0x02,
    content_type = 0x03,
    response_topic = 0x08,
    correlation_data = 0x09,
    subscription_id = 0x0B,
    session_expiry = 0x11,
    assigned_client_id = 0x12,
    server_keep_alive = 0x13,
    auth_method = 0x15,
    auth_data = 0x16,
    request_problem_info = 0x17,
    will_delay_interval = 0x18,
    request_response_info = 0x19,
    response_info = 0x1A,
    server_reference = 0x1C,
    reason_string = 0x1F,
    receive_maximum = 0x21,
    topic_alias_maximum = 0x22,
    topic_alias = 0x23,
    maximum_qos = 0x24,
    retain_available = 0x25,
    user_property = 0x26,
    maximum_packet_size = 0x27,
    wildcard_sub_available = 0x28,
    sub_id_available = 0x29,
    shared_sub_available = 0x2A,
};

// ============================================================================
// Reason Codes (MQTT 5.0)
// ============================================================================

pub const ReasonCode = enum(u8) {
    success = 0x00,
    normal_disconnection = 0x00,
    granted_qos_0 = 0x00,
    granted_qos_1 = 0x01,
    granted_qos_2 = 0x02,
    disconnect_with_will = 0x04,
    no_matching_subscribers = 0x10,
    no_subscription_existed = 0x11,
    continue_auth = 0x18,
    re_authenticate = 0x19,
    unspecified_error = 0x80,
    malformed_packet = 0x81,
    protocol_error = 0x82,
    implementation_specific = 0x83,
    unsupported_protocol = 0x84,
    client_id_not_valid = 0x85,
    bad_username_password = 0x86,
    not_authorized = 0x87,
    server_unavailable = 0x88,
    server_busy = 0x89,
    banned = 0x8A,
    server_shutting_down = 0x8B,
    bad_auth_method = 0x8C,
    keep_alive_timeout = 0x8D,
    session_taken_over = 0x8E,
    topic_filter_invalid = 0x8F,
    topic_name_invalid = 0x90,
    packet_id_in_use = 0x91,
    packet_id_not_found = 0x92,
    receive_max_exceeded = 0x93,
    topic_alias_invalid = 0x94,
    packet_too_large = 0x95,
    message_rate_too_high = 0x96,
    quota_exceeded = 0x97,
    administrative_action = 0x98,
    payload_format_invalid = 0x99,
    retain_not_supported = 0x9A,
    qos_not_supported = 0x9B,
    use_another_server = 0x9C,
    server_moved = 0x9D,
    shared_sub_not_supported = 0x9E,
    connection_rate_exceeded = 0x9F,
    max_connect_time = 0xA0,
    sub_id_not_supported = 0xA1,
    wildcard_sub_not_supported = 0xA2,
    _,
};

// ============================================================================
// Basic Encoding/Decoding Functions
// ============================================================================

/// Encode a variable-length integer (MQTT spec)
/// Returns the number of bytes written
pub fn encodeVariableInt(buf: []u8, value: u32) Error!usize {
    var v = value;
    var i: usize = 0;

    while (true) {
        if (i >= buf.len) return Error.BufferTooSmall;

        var byte: u8 = @truncate(v & 0x7F);
        v >>= 7;

        if (v > 0) {
            byte |= 0x80; // More bytes follow
        }

        buf[i] = byte;
        i += 1;

        if (v == 0) break;
    }

    return i;
}

/// Decode a variable-length integer
/// Returns the value and number of bytes consumed
pub fn decodeVariableInt(buf: []const u8) Error!struct { value: u32, len: usize } {
    var value: u32 = 0;
    var multiplier: u32 = 1;
    var i: usize = 0;

    while (i < 4) {
        if (i >= buf.len) return Error.MalformedVariableInt;

        const byte = buf[i];
        value += @as(u32, byte & 0x7F) * multiplier;

        i += 1;

        if ((byte & 0x80) == 0) {
            return .{ .value = value, .len = i };
        }

        multiplier *= 128;
    }

    return Error.MalformedVariableInt;
}

/// Get the size of a variable-length integer encoding
pub fn variableIntSize(value: u32) usize {
    if (value < 128) return 1;
    if (value < 16384) return 2;
    if (value < 2097152) return 3;
    return 4;
}

/// Encode a 16-bit unsigned integer (big-endian)
pub fn encodeU16(buf: []u8, value: u16) Error!usize {
    if (buf.len < 2) return Error.BufferTooSmall;
    buf[0] = @truncate(value >> 8);
    buf[1] = @truncate(value & 0xFF);
    return 2;
}

/// Decode a 16-bit unsigned integer (big-endian)
pub fn decodeU16(buf: []const u8) Error!u16 {
    if (buf.len < 2) return Error.MalformedPacket;
    return (@as(u16, buf[0]) << 8) | @as(u16, buf[1]);
}

/// Encode a 32-bit unsigned integer (big-endian)
pub fn encodeU32(buf: []u8, value: u32) Error!usize {
    if (buf.len < 4) return Error.BufferTooSmall;
    buf[0] = @truncate(value >> 24);
    buf[1] = @truncate((value >> 16) & 0xFF);
    buf[2] = @truncate((value >> 8) & 0xFF);
    buf[3] = @truncate(value & 0xFF);
    return 4;
}

/// Decode a 32-bit unsigned integer (big-endian)
pub fn decodeU32(buf: []const u8) Error!u32 {
    if (buf.len < 4) return Error.MalformedPacket;
    return (@as(u32, buf[0]) << 24) |
        (@as(u32, buf[1]) << 16) |
        (@as(u32, buf[2]) << 8) |
        @as(u32, buf[3]);
}

/// Encode a UTF-8 string (2-byte length prefix + data)
pub fn encodeString(buf: []u8, str: []const u8) Error!usize {
    if (str.len > 65535) return Error.MalformedString;
    if (buf.len < 2 + str.len) return Error.BufferTooSmall;

    _ = try encodeU16(buf[0..2], @truncate(str.len));

    // Copy string data
    for (str, 0..) |c, i| {
        buf[2 + i] = c;
    }

    return 2 + str.len;
}

/// Decode a UTF-8 string
/// Returns the string slice and total bytes consumed
pub fn decodeString(buf: []const u8) Error!struct { str: []const u8, len: usize } {
    if (buf.len < 2) return Error.MalformedString;

    const str_len = try decodeU16(buf[0..2]);
    const total_len = 2 + @as(usize, str_len);

    if (buf.len < total_len) return Error.MalformedString;

    return .{
        .str = buf[2..total_len],
        .len = total_len,
    };
}

/// Encode binary data (2-byte length prefix + data)
pub fn encodeBinary(buf: []u8, data: []const u8) Error!usize {
    return encodeString(buf, data); // Same format as string
}

/// Decode binary data
pub fn decodeBinary(buf: []const u8) Error!struct { data: []const u8, len: usize } {
    const result = try decodeString(buf);
    return .{ .data = result.str, .len = result.len };
}

// ============================================================================
// Fixed Header
// ============================================================================

/// Encode the fixed header (packet type + flags + remaining length)
pub fn encodeFixedHeader(buf: []u8, packet_type: PacketType, flags: u4, remaining_len: u32) Error!usize {
    if (buf.len < 1) return Error.BufferTooSmall;

    // First byte: packet type (4 bits) + flags (4 bits)
    buf[0] = (@as(u8, @intFromEnum(packet_type)) << 4) | @as(u8, flags);

    // Remaining length (variable int)
    const var_len = try encodeVariableInt(buf[1..], remaining_len);

    return 1 + var_len;
}

/// Decode the fixed header
pub fn decodeFixedHeader(buf: []const u8) Error!struct {
    packet_type: PacketType,
    flags: u4,
    remaining_len: u32,
    header_len: usize,
} {
    if (buf.len < 2) return Error.MalformedPacket;

    const first_byte = buf[0];
    const packet_type_raw = first_byte >> 4;
    const flags: u4 = @truncate(first_byte & 0x0F);

    const packet_type = @as(PacketType, @enumFromInt(packet_type_raw));

    const var_result = try decodeVariableInt(buf[1..]);

    return .{
        .packet_type = packet_type,
        .flags = flags,
        .remaining_len = var_result.value,
        .header_len = 1 + var_result.len,
    };
}

// ============================================================================
// Properties (MQTT 5.0)
// ============================================================================

/// Properties structure for MQTT 5.0
/// Only includes properties we actually use
pub const Properties = struct {
    // Connection properties
    session_expiry: ?u32 = null,
    receive_maximum: ?u16 = null,
    maximum_packet_size: ?u32 = null,
    topic_alias_maximum: ?u16 = null,
    server_keep_alive: ?u16 = null,

    // Publish properties
    topic_alias: ?u16 = null,
    message_expiry: ?u32 = null,
    payload_format: ?u8 = null,

    // Reason string (for CONNACK, DISCONNECT, etc.)
    reason_string: ?[]const u8 = null,

    // Assigned client ID (from CONNACK)
    assigned_client_id: ?[]const u8 = null,

    /// Calculate the encoded size of properties
    pub fn encodedSize(self: *const Properties) usize {
        var size: usize = 0;

        if (self.session_expiry != null) size += 1 + 4;
        if (self.receive_maximum != null) size += 1 + 2;
        if (self.maximum_packet_size != null) size += 1 + 4;
        if (self.topic_alias_maximum != null) size += 1 + 2;
        if (self.server_keep_alive != null) size += 1 + 2;
        if (self.topic_alias != null) size += 1 + 2;
        if (self.message_expiry != null) size += 1 + 4;
        if (self.payload_format != null) size += 1 + 1;
        if (self.reason_string) |s| size += 1 + 2 + s.len;
        if (self.assigned_client_id) |s| size += 1 + 2 + s.len;

        return size;
    }
};

/// Encode properties
pub fn encodeProperties(buf: []u8, props: *const Properties) Error!usize {
    const props_size = props.encodedSize();
    const len_size = variableIntSize(@truncate(props_size));

    if (buf.len < len_size + props_size) return Error.BufferTooSmall;

    // Encode property length
    var offset = try encodeVariableInt(buf, @truncate(props_size));

    // Encode each property
    if (props.session_expiry) |v| {
        buf[offset] = @intFromEnum(PropertyId.session_expiry);
        offset += 1;
        offset += try encodeU32(buf[offset..], v);
    }

    if (props.receive_maximum) |v| {
        buf[offset] = @intFromEnum(PropertyId.receive_maximum);
        offset += 1;
        offset += try encodeU16(buf[offset..], v);
    }

    if (props.maximum_packet_size) |v| {
        buf[offset] = @intFromEnum(PropertyId.maximum_packet_size);
        offset += 1;
        offset += try encodeU32(buf[offset..], v);
    }

    if (props.topic_alias_maximum) |v| {
        buf[offset] = @intFromEnum(PropertyId.topic_alias_maximum);
        offset += 1;
        offset += try encodeU16(buf[offset..], v);
    }

    if (props.server_keep_alive) |v| {
        buf[offset] = @intFromEnum(PropertyId.server_keep_alive);
        offset += 1;
        offset += try encodeU16(buf[offset..], v);
    }

    if (props.topic_alias) |v| {
        buf[offset] = @intFromEnum(PropertyId.topic_alias);
        offset += 1;
        offset += try encodeU16(buf[offset..], v);
    }

    if (props.message_expiry) |v| {
        buf[offset] = @intFromEnum(PropertyId.message_expiry);
        offset += 1;
        offset += try encodeU32(buf[offset..], v);
    }

    if (props.payload_format) |v| {
        buf[offset] = @intFromEnum(PropertyId.payload_format);
        offset += 1;
        buf[offset] = v;
        offset += 1;
    }

    if (props.reason_string) |s| {
        buf[offset] = @intFromEnum(PropertyId.reason_string);
        offset += 1;
        offset += try encodeString(buf[offset..], s);
    }

    if (props.assigned_client_id) |s| {
        buf[offset] = @intFromEnum(PropertyId.assigned_client_id);
        offset += 1;
        offset += try encodeString(buf[offset..], s);
    }

    return offset;
}

/// Decode properties
pub fn decodeProperties(buf: []const u8) Error!struct { props: Properties, len: usize } {
    if (buf.len == 0) return .{ .props = .{}, .len = 0 };

    // Decode property length
    const len_result = try decodeVariableInt(buf);
    const props_len = len_result.value;
    const header_len = len_result.len;

    if (buf.len < header_len + props_len) return Error.MalformedPacket;

    var props = Properties{};
    var offset: usize = header_len;
    const end_offset = header_len + props_len;

    while (offset < end_offset) {
        if (offset >= buf.len) return Error.MalformedPacket;

        const prop_id = buf[offset];
        offset += 1;

        switch (prop_id) {
            @intFromEnum(PropertyId.session_expiry) => {
                props.session_expiry = try decodeU32(buf[offset..]);
                offset += 4;
            },
            @intFromEnum(PropertyId.receive_maximum) => {
                props.receive_maximum = try decodeU16(buf[offset..]);
                offset += 2;
            },
            @intFromEnum(PropertyId.maximum_packet_size) => {
                props.maximum_packet_size = try decodeU32(buf[offset..]);
                offset += 4;
            },
            @intFromEnum(PropertyId.topic_alias_maximum) => {
                props.topic_alias_maximum = try decodeU16(buf[offset..]);
                offset += 2;
            },
            @intFromEnum(PropertyId.server_keep_alive) => {
                props.server_keep_alive = try decodeU16(buf[offset..]);
                offset += 2;
            },
            @intFromEnum(PropertyId.topic_alias) => {
                props.topic_alias = try decodeU16(buf[offset..]);
                offset += 2;
            },
            @intFromEnum(PropertyId.message_expiry) => {
                props.message_expiry = try decodeU32(buf[offset..]);
                offset += 4;
            },
            @intFromEnum(PropertyId.payload_format) => {
                if (offset >= buf.len) return Error.MalformedPacket;
                props.payload_format = buf[offset];
                offset += 1;
            },
            @intFromEnum(PropertyId.reason_string) => {
                const str_result = try decodeString(buf[offset..]);
                props.reason_string = str_result.str;
                offset += str_result.len;
            },
            @intFromEnum(PropertyId.assigned_client_id) => {
                const str_result = try decodeString(buf[offset..]);
                props.assigned_client_id = str_result.str;
                offset += str_result.len;
            },
            else => {
                // Skip unknown properties - we need to know their size
                // For now, return error for unknown properties
                return Error.MalformedPacket;
            },
        }
    }

    return .{ .props = props, .len = end_offset };
}

// ============================================================================
// Helper: Copy bytes
// ============================================================================

fn copyBytes(dst: []u8, src: []const u8) void {
    for (src, 0..) |b, i| {
        dst[i] = b;
    }
}

// ============================================================================
// CONNECT Packet
// ============================================================================

/// CONNECT packet configuration
pub const ConnectConfig = struct {
    client_id: []const u8,
    username: ?[]const u8 = null,
    password: ?[]const u8 = null,
    clean_start: bool = true,
    keep_alive: u16 = 60,

    // MQTT 5.0 properties
    session_expiry: ?u32 = null,
    receive_maximum: ?u16 = null,
    maximum_packet_size: ?u32 = null,
    topic_alias_maximum: ?u16 = null, // Request topic alias support

    // Will message (optional)
    will_topic: ?[]const u8 = null,
    will_payload: ?[]const u8 = null,
    will_qos: u2 = 0,
    will_retain: bool = false,
};

/// Encode a CONNECT packet
pub fn encodeConnect(buf: []u8, config: *const ConnectConfig) Error!usize {
    // Calculate variable header + payload size first
    var var_header_size: usize = 0;

    // Protocol name: "MQTT" (2 + 4 bytes)
    var_header_size += 2 + protocol_name.len;

    // Protocol version: 1 byte
    var_header_size += 1;

    // Connect flags: 1 byte
    var_header_size += 1;

    // Keep alive: 2 bytes
    var_header_size += 2;

    // Properties
    var props = Properties{
        .session_expiry = config.session_expiry,
        .receive_maximum = config.receive_maximum,
        .maximum_packet_size = config.maximum_packet_size,
        .topic_alias_maximum = config.topic_alias_maximum,
    };
    const props_size = props.encodedSize();
    var_header_size += variableIntSize(@truncate(props_size)) + props_size;

    // Payload
    var payload_size: usize = 0;

    // Client ID (required)
    payload_size += 2 + config.client_id.len;

    // Will properties + topic + payload (if present)
    if (config.will_topic) |topic| {
        // Will properties (empty for now)
        payload_size += 1; // Property length = 0
        payload_size += 2 + topic.len;
        if (config.will_payload) |payload| {
            payload_size += 2 + payload.len;
        } else {
            payload_size += 2; // Empty payload
        }
    }

    // Username
    if (config.username) |username| {
        payload_size += 2 + username.len;
    }

    // Password
    if (config.password) |password| {
        payload_size += 2 + password.len;
    }

    const remaining_len = var_header_size + payload_size;

    // Check buffer size
    const header_size = 1 + variableIntSize(@truncate(remaining_len));
    if (buf.len < header_size + remaining_len) return Error.BufferTooSmall;

    // Encode fixed header
    var offset = try encodeFixedHeader(buf, .connect, 0, @truncate(remaining_len));

    // Encode protocol name
    offset += try encodeString(buf[offset..], protocol_name);

    // Protocol version
    buf[offset] = protocol_version;
    offset += 1;

    // Connect flags
    var flags: u8 = 0;
    if (config.clean_start) flags |= 0x02;
    if (config.will_topic != null) {
        flags |= 0x04; // Will flag
        flags |= @as(u8, config.will_qos) << 3;
        if (config.will_retain) flags |= 0x20;
    }
    if (config.password != null) flags |= 0x40;
    if (config.username != null) flags |= 0x80;
    buf[offset] = flags;
    offset += 1;

    // Keep alive
    offset += try encodeU16(buf[offset..], config.keep_alive);

    // Properties
    offset += try encodeProperties(buf[offset..], &props);

    // Payload: Client ID
    offset += try encodeString(buf[offset..], config.client_id);

    // Will properties + topic + payload
    if (config.will_topic) |topic| {
        // Empty will properties
        buf[offset] = 0;
        offset += 1;

        offset += try encodeString(buf[offset..], topic);

        if (config.will_payload) |payload| {
            offset += try encodeBinary(buf[offset..], payload);
        } else {
            offset += try encodeU16(buf[offset..], 0);
        }
    }

    // Username
    if (config.username) |username| {
        offset += try encodeString(buf[offset..], username);
    }

    // Password
    if (config.password) |password| {
        offset += try encodeBinary(buf[offset..], password);
    }

    return offset;
}

// ============================================================================
// CONNACK Packet
// ============================================================================

/// Decoded CONNACK packet
pub const ConnAck = struct {
    session_present: bool,
    reason_code: ReasonCode,
    props: Properties,
};

/// Decode a CONNACK packet (after fixed header)
pub fn decodeConnAck(buf: []const u8) Error!ConnAck {
    if (buf.len < 2) return Error.MalformedPacket;

    // Acknowledge flags (byte 0)
    const session_present = (buf[0] & 0x01) != 0;

    // Reason code (byte 1)
    const reason_code: ReasonCode = @enumFromInt(buf[1]);

    // Properties (if present)
    var props = Properties{};
    if (buf.len > 2) {
        const props_result = try decodeProperties(buf[2..]);
        props = props_result.props;
    }

    return .{
        .session_present = session_present,
        .reason_code = reason_code,
        .props = props,
    };
}

// ============================================================================
// PUBLISH Packet
// ============================================================================

/// PUBLISH packet options
pub const PublishOptions = struct {
    topic: []const u8,
    payload: []const u8,
    retain: bool = false,
    qos: u2 = 0, // QoS 0 only for this implementation
    dup: bool = false,

    // MQTT 5.0 properties
    topic_alias: ?u16 = null, // Use topic alias instead of full topic
    message_expiry: ?u32 = null,
    payload_format: ?u8 = null, // 0 = binary, 1 = UTF-8
};

/// Encode a PUBLISH packet
/// When topic_alias is set and topic is empty, uses alias only (saves bandwidth)
pub fn encodePublish(buf: []u8, opts: *const PublishOptions) Error!usize {
    // Calculate variable header size
    var var_header_size: usize = 0;

    // Topic name
    var_header_size += 2 + opts.topic.len;

    // Packet ID (only for QoS > 0, but we only support QoS 0)
    // No packet ID for QoS 0

    // Properties
    var props = Properties{
        .topic_alias = opts.topic_alias,
        .message_expiry = opts.message_expiry,
        .payload_format = opts.payload_format,
    };
    const props_size = props.encodedSize();
    var_header_size += variableIntSize(@truncate(props_size)) + props_size;

    // Payload
    const remaining_len = var_header_size + opts.payload.len;

    // Fixed header flags
    var flags: u4 = 0;
    if (opts.dup) flags |= 0x08;
    // QoS bits (we only support 0)
    if (opts.retain) flags |= 0x01;

    // Check buffer size
    const header_size = 1 + variableIntSize(@truncate(remaining_len));
    if (buf.len < header_size + remaining_len) return Error.BufferTooSmall;

    // Encode fixed header
    var offset = try encodeFixedHeader(buf, .publish, flags, @truncate(remaining_len));

    // Topic name
    offset += try encodeString(buf[offset..], opts.topic);

    // Properties
    offset += try encodeProperties(buf[offset..], &props);

    // Payload
    copyBytes(buf[offset..], opts.payload);
    offset += opts.payload.len;

    return offset;
}

/// Decoded PUBLISH packet
pub const Publish = struct {
    topic: []const u8,
    payload: []const u8,
    retain: bool,
    qos: u2,
    dup: bool,
    packet_id: ?u16, // Only present for QoS > 0
    props: Properties,
};

/// Decode a PUBLISH packet (after fixed header)
pub fn decodePublish(buf: []const u8, flags: u4, remaining_len: u32) Error!Publish {
    const dup = (flags & 0x08) != 0;
    const qos: u2 = @truncate((flags >> 1) & 0x03);
    const retain = (flags & 0x01) != 0;

    var offset: usize = 0;

    // Topic name
    const topic_result = try decodeString(buf[offset..]);
    const topic = topic_result.str;
    offset += topic_result.len;

    // Packet ID (only for QoS > 0)
    var packet_id: ?u16 = null;
    if (qos > 0) {
        packet_id = try decodeU16(buf[offset..]);
        offset += 2;
    }

    // Properties
    const props_result = try decodeProperties(buf[offset..]);
    offset += props_result.len;

    // Payload is the rest
    const payload_len = remaining_len - @as(u32, @truncate(offset));
    const payload = buf[offset .. offset + payload_len];

    return .{
        .topic = topic,
        .payload = payload,
        .retain = retain,
        .qos = qos,
        .dup = dup,
        .packet_id = packet_id,
        .props = props_result.props,
    };
}

// ============================================================================
// SUBSCRIBE Packet
// ============================================================================

/// Subscription options for MQTT 5.0
pub const SubscriptionOptions = struct {
    qos: u2 = 0, // We only support QoS 0
    no_local: bool = false,
    retain_as_published: bool = false,
    retain_handling: u2 = 0,
};

/// Encode a SUBSCRIBE packet
/// topics is an array of topic filters
pub fn encodeSubscribe(buf: []u8, packet_id: u16, topics: []const []const u8) Error!usize {
    return encodeSubscribeWithOptions(buf, packet_id, topics, .{});
}

/// Encode a SUBSCRIBE packet with options
pub fn encodeSubscribeWithOptions(buf: []u8, packet_id: u16, topics: []const []const u8, opts: SubscriptionOptions) Error!usize {
    // Calculate variable header size
    var var_header_size: usize = 0;

    // Packet ID
    var_header_size += 2;

    // Properties (empty for now)
    var_header_size += 1; // Property length = 0

    // Topic filters
    var payload_size: usize = 0;
    for (topics) |topic| {
        payload_size += 2 + topic.len; // String
        payload_size += 1; // Subscription options
    }

    const remaining_len = var_header_size + payload_size;

    // Fixed header flags for SUBSCRIBE must be 0x02
    const flags: u4 = 0x02;

    // Check buffer size
    const header_size = 1 + variableIntSize(@truncate(remaining_len));
    if (buf.len < header_size + remaining_len) return Error.BufferTooSmall;

    // Encode fixed header
    var offset = try encodeFixedHeader(buf, .subscribe, flags, @truncate(remaining_len));

    // Packet ID
    offset += try encodeU16(buf[offset..], packet_id);

    // Properties (empty)
    buf[offset] = 0;
    offset += 1;

    // Topic filters
    for (topics) |topic| {
        offset += try encodeString(buf[offset..], topic);

        // Subscription options byte
        var sub_opts: u8 = opts.qos;
        if (opts.no_local) sub_opts |= 0x04;
        if (opts.retain_as_published) sub_opts |= 0x08;
        sub_opts |= @as(u8, opts.retain_handling) << 4;
        buf[offset] = sub_opts;
        offset += 1;
    }

    return offset;
}

// ============================================================================
// SUBACK Packet
// ============================================================================

/// Maximum number of reason codes we can handle
pub const max_suback_reason_codes = 32;

/// Decoded SUBACK packet
pub const SubAck = struct {
    packet_id: u16,
    reason_codes: [max_suback_reason_codes]ReasonCode,
    reason_code_count: usize,
    props: Properties,
};

/// Decode a SUBACK packet (after fixed header)
pub fn decodeSubAck(buf: []const u8, remaining_len: u32) Error!SubAck {
    if (buf.len < 3) return Error.MalformedPacket;

    var offset: usize = 0;

    // Packet ID
    const packet_id = try decodeU16(buf[offset..]);
    offset += 2;

    // Properties
    const props_result = try decodeProperties(buf[offset..]);
    offset += props_result.len;

    // Reason codes
    var result = SubAck{
        .packet_id = packet_id,
        .reason_codes = undefined,
        .reason_code_count = 0,
        .props = props_result.props,
    };

    const reason_codes_len = remaining_len - @as(u32, @truncate(offset));
    var i: usize = 0;
    while (i < reason_codes_len and i < max_suback_reason_codes) : (i += 1) {
        if (offset + i >= buf.len) break;
        result.reason_codes[i] = @enumFromInt(buf[offset + i]);
        result.reason_code_count += 1;
    }

    return result;
}

// ============================================================================
// UNSUBSCRIBE Packet
// ============================================================================

/// Encode an UNSUBSCRIBE packet
pub fn encodeUnsubscribe(buf: []u8, packet_id: u16, topics: []const []const u8) Error!usize {
    // Calculate variable header size
    var var_header_size: usize = 0;

    // Packet ID
    var_header_size += 2;

    // Properties (empty for now)
    var_header_size += 1; // Property length = 0

    // Topic filters
    var payload_size: usize = 0;
    for (topics) |topic| {
        payload_size += 2 + topic.len; // String
    }

    const remaining_len = var_header_size + payload_size;

    // Fixed header flags for UNSUBSCRIBE must be 0x02
    const flags: u4 = 0x02;

    // Check buffer size
    const header_size = 1 + variableIntSize(@truncate(remaining_len));
    if (buf.len < header_size + remaining_len) return Error.BufferTooSmall;

    // Encode fixed header
    var offset = try encodeFixedHeader(buf, .unsubscribe, flags, @truncate(remaining_len));

    // Packet ID
    offset += try encodeU16(buf[offset..], packet_id);

    // Properties (empty)
    buf[offset] = 0;
    offset += 1;

    // Topic filters
    for (topics) |topic| {
        offset += try encodeString(buf[offset..], topic);
    }

    return offset;
}

// ============================================================================
// PINGREQ Packet
// ============================================================================

/// Encode a PINGREQ packet
pub fn encodePingReq(buf: []u8) Error!usize {
    if (buf.len < 2) return Error.BufferTooSmall;

    // PINGREQ has no variable header or payload
    buf[0] = (@as(u8, @intFromEnum(PacketType.pingreq)) << 4);
    buf[1] = 0; // Remaining length = 0

    return 2;
}

// ============================================================================
// PINGRESP Packet
// ============================================================================

// PINGRESP is decoded by checking packet type, no additional data

// ============================================================================
// DISCONNECT Packet
// ============================================================================

/// Encode a DISCONNECT packet
pub fn encodeDisconnect(buf: []u8, reason_code: ReasonCode) Error!usize {
    // If reason code is 0 (normal) and no properties, send minimal packet
    if (reason_code == .success or reason_code == .normal_disconnection) {
        if (buf.len < 2) return Error.BufferTooSmall;
        buf[0] = (@as(u8, @intFromEnum(PacketType.disconnect)) << 4);
        buf[1] = 0; // Remaining length = 0
        return 2;
    }

    // With reason code
    if (buf.len < 4) return Error.BufferTooSmall;

    buf[0] = (@as(u8, @intFromEnum(PacketType.disconnect)) << 4);
    buf[1] = 2; // Remaining length = 2 (reason code + empty properties)
    buf[2] = @intFromEnum(reason_code);
    buf[3] = 0; // Empty properties

    return 4;
}

/// Decoded DISCONNECT packet
pub const Disconnect = struct {
    reason_code: ReasonCode,
    props: Properties,
};

/// Decode a DISCONNECT packet (after fixed header)
pub fn decodeDisconnect(buf: []const u8, remaining_len: u32) Error!Disconnect {
    // Empty disconnect = normal disconnection
    if (remaining_len == 0) {
        return .{
            .reason_code = .normal_disconnection,
            .props = .{},
        };
    }

    if (buf.len < 1) return Error.MalformedPacket;

    // Reason code
    const reason_code: ReasonCode = @enumFromInt(buf[0]);

    // Properties (if present)
    var props = Properties{};
    if (remaining_len > 1 and buf.len > 1) {
        const props_result = try decodeProperties(buf[1..]);
        props = props_result.props;
    }

    return .{
        .reason_code = reason_code,
        .props = props,
    };
}

// ============================================================================
// Generic Packet Decoder
// ============================================================================

/// Decoded packet union
pub const DecodedPacket = union(PacketType) {
    reserved: void,
    connect: void, // We don't decode CONNECT (we're a client)
    connack: ConnAck,
    publish: Publish,
    puback: void, // Not used for QoS 0
    pubrec: void,
    pubrel: void,
    pubcomp: void,
    subscribe: void, // We don't decode SUBSCRIBE (we're a client)
    suback: SubAck,
    unsubscribe: void,
    unsuback: void, // TODO: implement if needed
    pingreq: void,
    pingresp: void,
    disconnect: Disconnect,
    auth: void,
};

/// Decode any packet from buffer
/// Returns the decoded packet and total bytes consumed
pub fn decodePacket(buf: []const u8) Error!struct { packet: DecodedPacket, len: usize } {
    // Decode fixed header
    const header = try decodeFixedHeader(buf);
    const total_len = header.header_len + header.remaining_len;

    if (buf.len < total_len) return Error.MalformedPacket;

    const payload = buf[header.header_len..total_len];

    const packet: DecodedPacket = switch (header.packet_type) {
        .connack => .{ .connack = try decodeConnAck(payload) },
        .publish => .{ .publish = try decodePublish(payload, header.flags, header.remaining_len) },
        .suback => .{ .suback = try decodeSubAck(payload, header.remaining_len) },
        .pingresp => .{ .pingresp = {} },
        .disconnect => .{ .disconnect = try decodeDisconnect(payload, header.remaining_len) },
        else => return Error.UnknownPacketType,
    };

    return .{ .packet = packet, .len = total_len };
}
