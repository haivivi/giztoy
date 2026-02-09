//! Chatgear Protocol Client Library
//!
//! Implements the client-side of the chatgear device-server communication
//! protocol. Based on go/pkg/chatgear.
//!
//! ## Architecture
//!
//! - **types**: Protocol types (State, Command, Stats, StampedFrame)
//! - **wire**: Binary frame encoding + JSON event encoding/decoding
//! - **conn**: MQTT connection layer (generic over MqttClient)
//! - **port**: ClientPort with Go-style async (Channel + Spawner)
//!
//! ## Usage
//!
//! ```zig
//! const chatgear = @import("chatgear");
//! const mqtt0 = @import("mqtt0");
//! const std_impl = @import("std_impl");
//! const Rt = std_impl.runtime;
//!
//! // Create MQTT client
//! var mux = try mqtt0.Mux.init(allocator);
//! var client = try mqtt0.Client(Socket).init(&socket, &mux, .{...});
//!
//! // Create chatgear connection
//! var conn = chatgear.MqttClientConn(@TypeOf(client)).init(&client, .{
//!     .scope = "palr/cn/",
//!     .gear_id = "device-001",
//! });
//! try conn.subscribe();
//!
//! // Create client port (Go-style async)
//! var port = chatgear.ClientPort(@TypeOf(client), Rt).init(&conn);
//! defer port.close();
//!
//! try port.startPeriodicReporting();
//! port.setState(.ready);
//!
//! // Process commands
//! while (port.recvCommand()) |cmd| {
//!     switch (cmd.payload) {
//!         .streaming => |enabled| { ... },
//!         .set_volume => |vol| port.setVolume(vol),
//!         .halt => |h| if (h.sleep) port.setState(.sleeping),
//!         else => {},
//!     }
//! }
//! ```

// Re-export sub-modules
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
pub const PairStatus = types.PairStatus;
pub const Shaking = types.Shaking;

// Wire format
pub const stampFrame = wire.stampFrame;
pub const unstampFrame = wire.unstampFrame;
pub const stampedSize = wire.stampedSize;
pub const encodeStateEvent = wire.encodeStateEvent;
pub const encodeStatsEvent = wire.encodeStatsEvent;
pub const parseCommandEvent = wire.parseCommandEvent;

pub const VERSION = wire.VERSION;
pub const HEADER_SIZE = wire.HEADER_SIZE;
pub const MAX_OPUS_FRAME_SIZE = wire.MAX_OPUS_FRAME_SIZE;
pub const STATE_EVENT_JSON_SIZE = wire.STATE_EVENT_JSON_SIZE;
pub const STATS_EVENT_JSON_SIZE = wire.STATS_EVENT_JSON_SIZE;
pub const WireError = wire.WireError;

// Connection
pub const Config = conn.Config;
pub const MqttClientConn = conn.MqttClientConn;
pub const TopicBuilder = conn.TopicBuilder;

// Port
pub const ClientPort = port.ClientPort;
pub const STATE_INTERVAL_MS = port.STATE_INTERVAL_MS;
pub const STATS_BASE_INTERVAL_MS = port.STATS_BASE_INTERVAL_MS;

// Run all tests
test {
    _ = types;
    _ = wire;
    _ = conn;
    // port tests require runtime (Channel, Spawner) — tested via zig_test with deps
}
