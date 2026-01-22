//! Common types for mqtt0.
//!
//! This module provides types that work in both `std` and `no_std` environments.

#[cfg(feature = "alloc")]
use alloc::string::String;

use bytes::Bytes;
use core::fmt;

/// MQTT protocol version.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
#[repr(u8)]
pub enum ProtocolVersion {
    /// MQTT 3.1.1 (protocol level 4)
    #[default]
    V4 = 4,
    /// MQTT 5.0 (protocol level 5)
    V5 = 5,
}

impl ProtocolVersion {
    /// Create from protocol level byte.
    pub const fn from_level(level: u8) -> Option<Self> {
        match level {
            4 => Some(ProtocolVersion::V4),
            5 => Some(ProtocolVersion::V5),
            _ => None,
        }
    }

    /// Get protocol level byte.
    pub const fn level(self) -> u8 {
        self as u8
    }

    /// Get protocol name.
    pub const fn name(self) -> &'static str {
        match self {
            ProtocolVersion::V4 => "MQTT",
            ProtocolVersion::V5 => "MQTT",
        }
    }
}

impl fmt::Display for ProtocolVersion {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ProtocolVersion::V4 => write!(f, "MQTT 3.1.1"),
            ProtocolVersion::V5 => write!(f, "MQTT 5.0"),
        }
    }
}

/// Quality of Service level.
///
/// This crate only supports QoS 0, but we keep the enum for API consistency.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
#[repr(u8)]
pub enum QoS {
    /// At most once delivery (fire and forget).
    #[default]
    AtMostOnce = 0,
    /// At least once delivery.
    AtLeastOnce = 1,
    /// Exactly once delivery.
    ExactlyOnce = 2,
}

impl QoS {
    /// Create from u8 value.
    pub const fn from_u8(value: u8) -> Option<Self> {
        match value {
            0 => Some(QoS::AtMostOnce),
            1 => Some(QoS::AtLeastOnce),
            2 => Some(QoS::ExactlyOnce),
            _ => None,
        }
    }
}

/// MQTT message.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Message {
    /// Topic name.
    pub topic: String,
    /// Message payload.
    pub payload: Bytes,
    /// Retain flag.
    pub retain: bool,
}

#[cfg(feature = "alloc")]
impl Message {
    /// Create a new message.
    pub fn new(topic: impl Into<String>, payload: impl Into<Bytes>) -> Self {
        Self {
            topic: topic.into(),
            payload: payload.into(),
            retain: false,
        }
    }

    /// Set retain flag.
    pub fn with_retain(mut self, retain: bool) -> Self {
        self.retain = retain;
        self
    }
}

/// Fixed header of an MQTT packet.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FixedHeader {
    /// Packet type (upper 4 bits of first byte).
    pub packet_type: PacketType,
    /// Flags (lower 4 bits of first byte).
    pub flags: u8,
    /// Remaining length.
    pub remaining_length: u32,
    /// Total header length (1 byte type + variable length encoding).
    pub header_length: usize,
}

/// MQTT packet types.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum PacketType {
    Connect = 1,
    ConnAck = 2,
    Publish = 3,
    PubAck = 4,
    PubRec = 5,
    PubRel = 6,
    PubComp = 7,
    Subscribe = 8,
    SubAck = 9,
    Unsubscribe = 10,
    UnsubAck = 11,
    PingReq = 12,
    PingResp = 13,
    Disconnect = 14,
    Auth = 15, // MQTT 5.0 only
}

impl PacketType {
    /// Create from u8 value.
    pub const fn from_u8(value: u8) -> Option<Self> {
        match value {
            1 => Some(PacketType::Connect),
            2 => Some(PacketType::ConnAck),
            3 => Some(PacketType::Publish),
            4 => Some(PacketType::PubAck),
            5 => Some(PacketType::PubRec),
            6 => Some(PacketType::PubRel),
            7 => Some(PacketType::PubComp),
            8 => Some(PacketType::Subscribe),
            9 => Some(PacketType::SubAck),
            10 => Some(PacketType::Unsubscribe),
            11 => Some(PacketType::UnsubAck),
            12 => Some(PacketType::PingReq),
            13 => Some(PacketType::PingResp),
            14 => Some(PacketType::Disconnect),
            15 => Some(PacketType::Auth),
            _ => None,
        }
    }
}

/// CONNECT packet flags.
#[derive(Debug, Clone, Copy, Default)]
pub struct ConnectFlags {
    pub clean_session: bool,  // v4: clean session, v5: clean start
    pub will: bool,
    pub will_qos: QoS,
    pub will_retain: bool,
    pub password: bool,
    pub username: bool,
}

impl ConnectFlags {
    /// Encode flags to a byte.
    pub const fn encode(self) -> u8 {
        let mut flags = 0u8;
        if self.clean_session {
            flags |= 0b0000_0010;
        }
        if self.will {
            flags |= 0b0000_0100;
        }
        flags |= (self.will_qos as u8) << 3;
        if self.will_retain {
            flags |= 0b0010_0000;
        }
        if self.password {
            flags |= 0b0100_0000;
        }
        if self.username {
            flags |= 0b1000_0000;
        }
        flags
    }

    /// Decode flags from a byte.
    pub const fn decode(byte: u8) -> Option<Self> {
        // Reserved bit (bit 0) must be 0
        if byte & 0b0000_0001 != 0 {
            return None;
        }

        let will_qos = (byte >> 3) & 0b11;
        let qos = match QoS::from_u8(will_qos) {
            Some(q) => q,
            None => return None,
        };

        Some(ConnectFlags {
            clean_session: byte & 0b0000_0010 != 0,
            will: byte & 0b0000_0100 != 0,
            will_qos: qos,
            will_retain: byte & 0b0010_0000 != 0,
            password: byte & 0b0100_0000 != 0,
            username: byte & 0b1000_0000 != 0,
        })
    }
}

/// CONNACK return codes for MQTT 3.1.1.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum ConnectReturnCodeV4 {
    Success = 0,
    UnacceptableProtocolVersion = 1,
    IdentifierRejected = 2,
    ServerUnavailable = 3,
    BadUsernamePassword = 4,
    NotAuthorized = 5,
}

impl ConnectReturnCodeV4 {
    pub const fn from_u8(value: u8) -> Option<Self> {
        match value {
            0 => Some(Self::Success),
            1 => Some(Self::UnacceptableProtocolVersion),
            2 => Some(Self::IdentifierRejected),
            3 => Some(Self::ServerUnavailable),
            4 => Some(Self::BadUsernamePassword),
            5 => Some(Self::NotAuthorized),
            _ => None,
        }
    }
}

/// CONNACK reason codes for MQTT 5.0.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum ConnectReasonCodeV5 {
    Success = 0,
    UnspecifiedError = 128,
    MalformedPacket = 129,
    ProtocolError = 130,
    ImplementationSpecificError = 131,
    UnsupportedProtocolVersion = 132,
    ClientIdentifierNotValid = 133,
    BadUserNameOrPassword = 134,
    NotAuthorized = 135,
    ServerUnavailable = 136,
    ServerBusy = 137,
    Banned = 138,
    BadAuthenticationMethod = 140,
    TopicNameInvalid = 144,
    PacketTooLarge = 149,
    QuotaExceeded = 151,
    PayloadFormatInvalid = 153,
    RetainNotSupported = 154,
    QoSNotSupported = 155,
    UseAnotherServer = 156,
    ServerMoved = 157,
    ConnectionRateExceeded = 159,
}

impl ConnectReasonCodeV5 {
    pub const fn from_u8(value: u8) -> Option<Self> {
        match value {
            0 => Some(Self::Success),
            128 => Some(Self::UnspecifiedError),
            129 => Some(Self::MalformedPacket),
            130 => Some(Self::ProtocolError),
            131 => Some(Self::ImplementationSpecificError),
            132 => Some(Self::UnsupportedProtocolVersion),
            133 => Some(Self::ClientIdentifierNotValid),
            134 => Some(Self::BadUserNameOrPassword),
            135 => Some(Self::NotAuthorized),
            136 => Some(Self::ServerUnavailable),
            137 => Some(Self::ServerBusy),
            138 => Some(Self::Banned),
            140 => Some(Self::BadAuthenticationMethod),
            144 => Some(Self::TopicNameInvalid),
            149 => Some(Self::PacketTooLarge),
            151 => Some(Self::QuotaExceeded),
            153 => Some(Self::PayloadFormatInvalid),
            154 => Some(Self::RetainNotSupported),
            155 => Some(Self::QoSNotSupported),
            156 => Some(Self::UseAnotherServer),
            157 => Some(Self::ServerMoved),
            159 => Some(Self::ConnectionRateExceeded),
            _ => None,
        }
    }
}
