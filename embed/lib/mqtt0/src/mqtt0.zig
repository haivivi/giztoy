//! mqtt0 - Lightweight MQTT 5.0 Client Library
//!
//! A minimal MQTT 5.0 client implementation for embedded systems.
//! Features:
//! - QoS 0 only (fire and forget)
//! - Topic Alias support for bandwidth reduction
//! - Zero dynamic allocation (uses caller-provided buffers)
//! - Generic over socket type (works with TCP or TLS)
//!
//! ## Usage
//!
//! ```zig
//! const mqtt0 = @import("mqtt0");
//!
//! // Create client with your socket and platform types
//! const Client = mqtt0.MqttClient(TlsSocket, Log, Time);
//!
//! var client = Client.init(&socket);
//! var buf: [4096]u8 = undefined;
//!
//! // Connect
//! try client.connect(&.{
//!     .client_id = "my-device",
//!     .keep_alive = 60,
//!     .topic_alias_maximum = 16, // Request topic alias support
//! }, &buf);
//!
//! // Subscribe
//! try client.subscribe(&.{"device/+/command"}, &buf);
//!
//! // Publish (auto-uses topic alias after first publish)
//! try client.publish("device/123/state", payload, &buf);
//!
//! // Receive messages
//! if (try client.recvMessage(&buf)) |msg| {
//!     // Handle message
//! }
//!
//! // Disconnect
//! client.disconnect(&buf);
//! ```

// Re-export client types
pub const MqttClient = @import("client.zig").MqttClient;
pub const TopicAliasManager = @import("client.zig").TopicAliasManager;
pub const Message = @import("client.zig").Message;
pub const ClientError = @import("client.zig").ClientError;
pub const ConnectConfig = @import("client.zig").ConnectConfig;
pub const PublishOptions = @import("client.zig").PublishOptions;
pub const ReasonCode = @import("client.zig").ReasonCode;
pub const Properties = @import("client.zig").Properties;

// Re-export packet types for advanced usage
pub const packet = @import("packet.zig");
pub const PacketType = packet.PacketType;
pub const PropertyId = packet.PropertyId;
