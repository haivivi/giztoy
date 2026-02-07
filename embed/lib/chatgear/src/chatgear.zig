//! Chatgear Protocol Client Library
//!
//! Implements the client-side of the chatgear device-server communication protocol.
//! Based on go/pkg/chatgear.
//!
//! ## Features
//!
//! - MQTT-based transport
//! - Stamped Opus audio frame encoding/decoding
//! - State and stats event reporting (uplink)
//! - Command and audio stream reception (downlink)
//!
//! ## Usage
//!
//! ```zig
//! const chatgear = @import("chatgear");
//! const mqtt0 = @import("mqtt0");
//!
//! // Create MQTT client
//! var mqtt = mqtt0.MqttClient(Socket, Log, Time).init(&socket);
//!
//! // Connect to broker
//! try mqtt.connect(&mqtt_config, &buf);
//!
//! // Create chatgear connection
//! const config = chatgear.Config{
//!     .scope = "stage/",
//!     .gear_id = "device-001",
//! };
//! var conn = chatgear.MqttClientConn(@TypeOf(mqtt), Log).init(&mqtt, &config);
//!
//! // Subscribe to downlink topics
//! try conn.subscribe(&buf);
//!
//! // Send opus frame (uplink)
//! try conn.sendOpusFrame(timestamp_ms, opus_data, &buf);
//!
//! // Receive message (downlink)
//! if (try conn.recvMessage(&buf)) |msg| {
//!     switch (msg) {
//!         .opus_frame => |frame| {
//!             // Handle audio
//!         },
//!         .command => |cmd| {
//!             // Handle command
//!         },
//!     }
//! }
//! ```

// Re-export all public types
pub const types = @import("types.zig");
pub const wire = @import("wire.zig");
pub const conn = @import("conn.zig");
pub const port = @import("port.zig");

// Main types
pub const State = types.State;
pub const StateEvent = types.StateEvent;
pub const StateChangeCause = types.StateChangeCause;

pub const CommandType = types.CommandType;
pub const CommandPayload = types.CommandPayload;
pub const CommandEvent = types.CommandEvent;
pub const ResetPayload = types.ResetPayload;
pub const RaisePayload = types.RaisePayload;
pub const HaltPayload = types.HaltPayload;
pub const SetWifiPayload = types.SetWifiPayload;

pub const StampedFrame = types.StampedFrame;
pub const Message = types.Message;
pub const MessageType = types.MessageType;

pub const StatsEvent = types.StatsEvent;
pub const Battery = types.Battery;
pub const Volume = types.Volume;
pub const Brightness = types.Brightness;
pub const LightMode = types.LightMode;
pub const ConnectedWifi = types.ConnectedWifi;
pub const SystemVersion = types.SystemVersion;

// Wire format
pub const stampFrame = wire.stampFrame;
pub const unstampFrame = wire.unstampFrame;
pub const stampedSize = wire.stampedSize;
pub const encodeStateEvent = wire.encodeStateEvent;
pub const parseCommandEvent = wire.parseCommandEvent;

pub const VERSION = wire.VERSION;
pub const HEADER_SIZE = wire.HEADER_SIZE;
pub const MAX_OPUS_FRAME_SIZE = wire.MAX_OPUS_FRAME_SIZE;
pub const STATE_EVENT_JSON_SIZE = wire.STATE_EVENT_JSON_SIZE;
pub const STATS_EVENT_JSON_SIZE = wire.STATS_EVENT_JSON_SIZE;

pub const WireError = wire.WireError;

// Connection (low-level)
pub const Config = conn.Config;
pub const ConnError = conn.ConnError;
pub const MqttClientConn = conn.MqttClientConn;
pub const TopicBuilder = conn.TopicBuilder;

// Port (high-level)
pub const PortConfig = port.PortConfig;
pub const ClientPort = port.ClientPort;
pub const STATE_INTERVAL_MS = port.STATE_INTERVAL_MS;
pub const STATS_BASE_INTERVAL_MS = port.STATS_BASE_INTERVAL_MS;
