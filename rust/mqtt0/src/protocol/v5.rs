//! MQTT 5.0 (v5) protocol implementation.
//!
//! This is a simplified implementation focusing on QoS 0 operations.
//! Properties are parsed but not all are fully supported.

#[cfg(feature = "alloc")]
use alloc::string::String;
#[cfg(feature = "alloc")]
use alloc::vec::Vec;

use bytes::Bytes;

use crate::error::{Error, Result};
use crate::types::{ConnectFlags, ConnectReasonCodeV5, PacketType, QoS};

use super::codec::{
    read_binary_slice, read_fixed_header, read_string_slice, read_u16, read_variable_int,
};

/// MQTT 5.0 packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub enum Packet {
    Connect(Connect),
    ConnAck(ConnAck),
    Publish(Publish),
    Subscribe(Subscribe),
    SubAck(SubAck),
    Unsubscribe(Unsubscribe),
    UnsubAck(UnsubAck),
    PingReq,
    PingResp,
    Disconnect(Disconnect),
}

/// CONNECT packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Connect {
    pub client_id: String,
    pub keep_alive: u16,
    pub clean_start: bool,
    pub username: Option<String>,
    pub password: Option<Vec<u8>>,
    pub will: Option<Will>,
    pub properties: Option<ConnectProperties>,
}

/// Connect properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct ConnectProperties {
    pub session_expiry_interval: Option<u32>,
    pub receive_maximum: Option<u16>,
    pub maximum_packet_size: Option<u32>,
    pub topic_alias_maximum: Option<u16>,
    pub request_response_information: Option<bool>,
    pub request_problem_information: Option<bool>,
    pub authentication_method: Option<String>,
    pub authentication_data: Option<Vec<u8>>,
}

/// Last Will and Testament.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Will {
    pub topic: String,
    pub payload: Vec<u8>,
    pub qos: QoS,
    pub retain: bool,
    pub properties: Option<WillProperties>,
}

/// Will properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct WillProperties {
    pub delay_interval: Option<u32>,
    pub payload_format_indicator: Option<bool>,
    pub message_expiry_interval: Option<u32>,
    pub content_type: Option<String>,
    pub response_topic: Option<String>,
    pub correlation_data: Option<Vec<u8>>,
}

/// CONNACK packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct ConnAck {
    pub session_present: bool,
    pub code: ConnectReasonCodeV5,
    pub properties: Option<ConnAckProperties>,
}

/// ConnAck properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct ConnAckProperties {
    pub session_expiry_interval: Option<u32>,
    pub receive_maximum: Option<u16>,
    pub maximum_qos: Option<QoS>,
    pub retain_available: Option<bool>,
    pub maximum_packet_size: Option<u32>,
    pub assigned_client_identifier: Option<String>,
    pub topic_alias_maximum: Option<u16>,
    pub reason_string: Option<String>,
    pub wildcard_subscription_available: Option<bool>,
    pub subscription_identifiers_available: Option<bool>,
    pub shared_subscription_available: Option<bool>,
    pub server_keep_alive: Option<u16>,
    pub response_information: Option<String>,
    pub server_reference: Option<String>,
    pub authentication_method: Option<String>,
    pub authentication_data: Option<Vec<u8>>,
}

/// PUBLISH packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Publish {
    pub topic: String,
    pub payload: Bytes,
    pub qos: QoS,
    pub retain: bool,
    pub dup: bool,
    pub pkid: u16,
    pub properties: Option<PublishProperties>,
}

/// Publish properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct PublishProperties {
    pub payload_format_indicator: Option<bool>,
    pub message_expiry_interval: Option<u32>,
    pub topic_alias: Option<u16>,
    pub response_topic: Option<String>,
    pub correlation_data: Option<Vec<u8>>,
    pub subscription_identifiers: Vec<u32>,
    pub content_type: Option<String>,
}

/// SUBSCRIBE packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Subscribe {
    pub pkid: u16,
    pub filters: Vec<SubscribeFilter>,
    pub properties: Option<SubscribeProperties>,
}

/// Subscribe filter.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct SubscribeFilter {
    pub path: String,
    pub qos: QoS,
    pub no_local: bool,
    pub retain_as_published: bool,
    pub retain_handling: RetainHandling,
}

/// Retain handling options.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
#[repr(u8)]
pub enum RetainHandling {
    #[default]
    SendOnSubscribe = 0,
    SendOnSubscribeIfNew = 1,
    DoNotSend = 2,
}

/// Subscribe properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct SubscribeProperties {
    pub subscription_identifier: Option<u32>,
}

/// SUBACK packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct SubAck {
    pub pkid: u16,
    pub return_codes: Vec<SubscribeReasonCode>,
    pub properties: Option<SubAckProperties>,
}

/// SubAck properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct SubAckProperties {
    pub reason_string: Option<String>,
}

/// Subscribe reason code.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubscribeReasonCode {
    Success(QoS),
    UnspecifiedError,
    ImplementationSpecificError,
    NotAuthorized,
    TopicFilterInvalid,
    PacketIdentifierInUse,
    QuotaExceeded,
    SharedSubscriptionsNotSupported,
    SubscriptionIdentifiersNotSupported,
    WildcardSubscriptionsNotSupported,
}

impl SubscribeReasonCode {
    pub fn from_u8(value: u8) -> Self {
        match value {
            0 => SubscribeReasonCode::Success(QoS::AtMostOnce),
            1 => SubscribeReasonCode::Success(QoS::AtLeastOnce),
            2 => SubscribeReasonCode::Success(QoS::ExactlyOnce),
            128 => SubscribeReasonCode::UnspecifiedError,
            131 => SubscribeReasonCode::ImplementationSpecificError,
            135 => SubscribeReasonCode::NotAuthorized,
            143 => SubscribeReasonCode::TopicFilterInvalid,
            145 => SubscribeReasonCode::PacketIdentifierInUse,
            151 => SubscribeReasonCode::QuotaExceeded,
            158 => SubscribeReasonCode::SharedSubscriptionsNotSupported,
            161 => SubscribeReasonCode::SubscriptionIdentifiersNotSupported,
            162 => SubscribeReasonCode::WildcardSubscriptionsNotSupported,
            _ => SubscribeReasonCode::UnspecifiedError,
        }
    }

    pub fn to_u8(self) -> u8 {
        match self {
            SubscribeReasonCode::Success(QoS::AtMostOnce) => 0,
            SubscribeReasonCode::Success(QoS::AtLeastOnce) => 1,
            SubscribeReasonCode::Success(QoS::ExactlyOnce) => 2,
            SubscribeReasonCode::UnspecifiedError => 128,
            SubscribeReasonCode::ImplementationSpecificError => 131,
            SubscribeReasonCode::NotAuthorized => 135,
            SubscribeReasonCode::TopicFilterInvalid => 143,
            SubscribeReasonCode::PacketIdentifierInUse => 145,
            SubscribeReasonCode::QuotaExceeded => 151,
            SubscribeReasonCode::SharedSubscriptionsNotSupported => 158,
            SubscribeReasonCode::SubscriptionIdentifiersNotSupported => 161,
            SubscribeReasonCode::WildcardSubscriptionsNotSupported => 162,
        }
    }
}

/// UNSUBSCRIBE packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Unsubscribe {
    pub pkid: u16,
    pub filters: Vec<String>,
    pub properties: Option<UnsubscribeProperties>,
}

/// Unsubscribe properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct UnsubscribeProperties {}

/// UNSUBACK packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct UnsubAck {
    pub pkid: u16,
    pub reasons: Vec<UnsubAckReason>,
    pub properties: Option<UnsubAckProperties>,
}

/// UnsubAck reason codes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum UnsubAckReason {
    Success = 0,
    NoSubscriptionExisted = 17,
    UnspecifiedError = 128,
    ImplementationSpecificError = 131,
    NotAuthorized = 135,
    TopicFilterInvalid = 143,
    PacketIdentifierInUse = 145,
}

impl UnsubAckReason {
    pub fn from_u8(value: u8) -> Self {
        match value {
            0 => UnsubAckReason::Success,
            17 => UnsubAckReason::NoSubscriptionExisted,
            131 => UnsubAckReason::ImplementationSpecificError,
            135 => UnsubAckReason::NotAuthorized,
            143 => UnsubAckReason::TopicFilterInvalid,
            145 => UnsubAckReason::PacketIdentifierInUse,
            _ => UnsubAckReason::UnspecifiedError,
        }
    }
}

/// UnsubAck properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct UnsubAckProperties {
    pub reason_string: Option<String>,
}

/// DISCONNECT packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Disconnect {
    pub reason_code: DisconnectReasonCode,
    pub properties: Option<DisconnectProperties>,
}

/// Disconnect reason codes.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
#[repr(u8)]
pub enum DisconnectReasonCode {
    #[default]
    NormalDisconnection = 0,
    DisconnectWithWillMessage = 4,
    UnspecifiedError = 128,
    MalformedPacket = 129,
    ProtocolError = 130,
    ImplementationSpecificError = 131,
    NotAuthorized = 135,
    ServerBusy = 137,
    ServerShuttingDown = 139,
    KeepAliveTimeout = 141,
    SessionTakenOver = 142,
    TopicFilterInvalid = 143,
    TopicNameInvalid = 144,
    ReceiveMaximumExceeded = 147,
    TopicAliasInvalid = 148,
    PacketTooLarge = 149,
    MessageRateTooHigh = 150,
    QuotaExceeded = 151,
    AdministrativeAction = 152,
    PayloadFormatInvalid = 153,
    RetainNotSupported = 154,
    QoSNotSupported = 155,
    UseAnotherServer = 156,
    ServerMoved = 157,
    SharedSubscriptionsNotSupported = 158,
    ConnectionRateExceeded = 159,
    MaximumConnectTime = 160,
    SubscriptionIdentifiersNotSupported = 161,
    WildcardSubscriptionsNotSupported = 162,
}

impl DisconnectReasonCode {
    pub fn from_u8(value: u8) -> Option<Self> {
        Some(match value {
            0 => Self::NormalDisconnection,
            4 => Self::DisconnectWithWillMessage,
            128 => Self::UnspecifiedError,
            129 => Self::MalformedPacket,
            130 => Self::ProtocolError,
            131 => Self::ImplementationSpecificError,
            135 => Self::NotAuthorized,
            137 => Self::ServerBusy,
            139 => Self::ServerShuttingDown,
            141 => Self::KeepAliveTimeout,
            142 => Self::SessionTakenOver,
            143 => Self::TopicFilterInvalid,
            144 => Self::TopicNameInvalid,
            147 => Self::ReceiveMaximumExceeded,
            148 => Self::TopicAliasInvalid,
            149 => Self::PacketTooLarge,
            150 => Self::MessageRateTooHigh,
            151 => Self::QuotaExceeded,
            152 => Self::AdministrativeAction,
            153 => Self::PayloadFormatInvalid,
            154 => Self::RetainNotSupported,
            155 => Self::QoSNotSupported,
            156 => Self::UseAnotherServer,
            157 => Self::ServerMoved,
            158 => Self::SharedSubscriptionsNotSupported,
            159 => Self::ConnectionRateExceeded,
            160 => Self::MaximumConnectTime,
            161 => Self::SubscriptionIdentifiersNotSupported,
            162 => Self::WildcardSubscriptionsNotSupported,
            _ => return None,
        })
    }
}

/// Disconnect properties.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone, Default)]
pub struct DisconnectProperties {
    pub session_expiry_interval: Option<u32>,
    pub reason_string: Option<String>,
    pub server_reference: Option<String>,
}

// ============================================================================
// Property identifiers
// ============================================================================

/// MQTT 5.0 property identifiers.
#[allow(dead_code)]
mod property_id {
    pub const PAYLOAD_FORMAT_INDICATOR: u8 = 0x01;
    pub const MESSAGE_EXPIRY_INTERVAL: u8 = 0x02;
    pub const CONTENT_TYPE: u8 = 0x03;
    pub const RESPONSE_TOPIC: u8 = 0x08;
    pub const CORRELATION_DATA: u8 = 0x09;
    pub const SUBSCRIPTION_IDENTIFIER: u8 = 0x0B;
    pub const SESSION_EXPIRY_INTERVAL: u8 = 0x11;
    pub const ASSIGNED_CLIENT_IDENTIFIER: u8 = 0x12;
    pub const SERVER_KEEP_ALIVE: u8 = 0x13;
    pub const AUTHENTICATION_METHOD: u8 = 0x15;
    pub const AUTHENTICATION_DATA: u8 = 0x16;
    pub const REQUEST_PROBLEM_INFORMATION: u8 = 0x17;
    pub const WILL_DELAY_INTERVAL: u8 = 0x18;
    pub const REQUEST_RESPONSE_INFORMATION: u8 = 0x19;
    pub const RESPONSE_INFORMATION: u8 = 0x1A;
    pub const SERVER_REFERENCE: u8 = 0x1C;
    pub const REASON_STRING: u8 = 0x1F;
    pub const RECEIVE_MAXIMUM: u8 = 0x21;
    pub const TOPIC_ALIAS_MAXIMUM: u8 = 0x22;
    pub const TOPIC_ALIAS: u8 = 0x23;
    pub const MAXIMUM_QOS: u8 = 0x24;
    pub const RETAIN_AVAILABLE: u8 = 0x25;
    pub const USER_PROPERTY: u8 = 0x26;
    pub const MAXIMUM_PACKET_SIZE: u8 = 0x27;
    pub const WILDCARD_SUBSCRIPTION_AVAILABLE: u8 = 0x28;
    pub const SUBSCRIPTION_IDENTIFIER_AVAILABLE: u8 = 0x29;
    pub const SHARED_SUBSCRIPTION_AVAILABLE: u8 = 0x2A;
}

// ============================================================================
// Packet parsing (simplified for QoS 0)
// ============================================================================

#[cfg(feature = "alloc")]
impl Packet {
    /// Parse a packet from buffer.
    pub fn read(buf: &[u8], max_size: usize) -> Result<(Packet, usize)> {
        let header = read_fixed_header(buf)?;
        let total_len = header.header_length + header.remaining_length as usize;

        if total_len > max_size {
            return Err(Error::PacketTooLarge { size: total_len, max: max_size });
        }

        if buf.len() < total_len {
            return Err(Error::Incomplete { needed: total_len - buf.len() });
        }

        let payload = &buf[header.header_length..total_len];

        let packet = match header.packet_type {
            PacketType::Connect => Packet::Connect(Connect::read(payload)?),
            PacketType::ConnAck => Packet::ConnAck(ConnAck::read(payload)?),
            PacketType::Publish => Packet::Publish(Publish::read(header.flags, payload)?),
            PacketType::Subscribe => Packet::Subscribe(Subscribe::read(payload)?),
            PacketType::SubAck => Packet::SubAck(SubAck::read(payload)?),
            PacketType::Unsubscribe => Packet::Unsubscribe(Unsubscribe::read(payload)?),
            PacketType::UnsubAck => Packet::UnsubAck(UnsubAck::read(payload)?),
            PacketType::PingReq => Packet::PingReq,
            PacketType::PingResp => Packet::PingResp,
            PacketType::Disconnect => Packet::Disconnect(Disconnect::read(payload)?),
            _ => return Err(Error::InvalidPacketType(header.packet_type as u8)),
        };

        Ok((packet, total_len))
    }

    /// Calculate packet size.
    pub fn size(&self) -> usize {
        match self {
            Packet::Publish(p) => p.size(),
            Packet::PingReq | Packet::PingResp => 2,
            Packet::Disconnect(_) => 4, // Simplified
            _ => 64, // Placeholder for other packets
        }
    }
}

#[cfg(feature = "alloc")]
impl Connect {
    pub fn read(buf: &[u8]) -> Result<Self> {
        let mut pos = 0;

        // Protocol name
        let (name, len) = read_string_slice(buf)?;
        if name != "MQTT" {
            return Err(Error::Protocol(crate::error::ProtocolError::MalformedPacket));
        }
        pos += len;

        // Protocol level
        if buf.len() < pos + 1 {
            return Err(Error::Incomplete { needed: 1 });
        }
        let protocol_level = buf[pos];
        if protocol_level != 5 {
            return Err(Error::InvalidProtocolVersion(protocol_level));
        }
        pos += 1;

        // Connect flags
        if buf.len() < pos + 1 {
            return Err(Error::Incomplete { needed: 1 });
        }
        let flags = ConnectFlags::decode(buf[pos])
            .ok_or(Error::Protocol(crate::error::ProtocolError::InvalidConnectFlags))?;
        pos += 1;

        // Keep alive
        let keep_alive = read_u16(&buf[pos..]).ok_or(Error::Incomplete { needed: 2 })?;
        pos += 2;

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size;

        // Skip properties for now (simplified)
        let properties = if prop_len > 0 {
            let mut props = ConnectProperties::default();
            let prop_end = pos + prop_len as usize;
            while pos < prop_end && pos < buf.len() {
                let prop_id = buf[pos];
                pos += 1;
                match prop_id {
                    property_id::SESSION_EXPIRY_INTERVAL => {
                        if buf.len() < pos + 4 {
                            break;
                        }
                        props.session_expiry_interval = Some(u32::from_be_bytes([
                            buf[pos], buf[pos + 1], buf[pos + 2], buf[pos + 3]
                        ]));
                        pos += 4;
                    }
                    _ => {
                        // Skip unknown properties
                        // Skip unknown properties
                        break;
                    }
                }
            }
            pos = prop_end.min(buf.len());
            Some(props)
        } else {
            None
        };

        // Client ID
        let (client_id, len) = read_string_slice(&buf[pos..])?;
        let client_id = client_id.to_string();
        pos += len;

        // Will (simplified - skip for now)
        let will = if flags.will {
            // Skip will properties
            let (will_prop_len, will_prop_len_size) = read_variable_int(&buf[pos..])
                .ok_or(Error::Incomplete { needed: 1 })?;
            pos += will_prop_len_size + will_prop_len as usize;

            let (topic, len) = read_string_slice(&buf[pos..])?;
            pos += len;
            let (payload, len) = read_binary_slice(&buf[pos..])?;
            pos += len;

            Some(Will {
                topic: topic.to_string(),
                payload: payload.to_vec(),
                qos: flags.will_qos,
                retain: flags.will_retain,
                properties: None,
            })
        } else {
            None
        };

        // Username
        let username = if flags.username {
            let (u, len) = read_string_slice(&buf[pos..])?;
            pos += len;
            Some(u.to_string())
        } else {
            None
        };

        // Password
        let password = if flags.password {
            let (p, len) = read_binary_slice(&buf[pos..])?;
            let _ = pos + len;
            Some(p.to_vec())
        } else {
            None
        };

        Ok(Connect {
            client_id,
            keep_alive,
            clean_start: flags.clean_session,
            username,
            password,
            will,
            properties,
        })
    }
}

#[cfg(feature = "alloc")]
impl ConnAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 - buf.len() });
        }

        let session_present = buf[0] & 0x01 != 0;
        let code = ConnectReasonCodeV5::from_u8(buf[1])
            .ok_or(Error::Protocol(crate::error::ProtocolError::MalformedPacket))?;

        // Skip properties for now
        Ok(ConnAck {
            session_present,
            code,
            properties: None,
        })
    }
}

#[cfg(feature = "alloc")]
impl Publish {
    pub fn read(flags: u8, buf: &[u8]) -> Result<Self> {
        let dup = flags & 0x08 != 0;
        let qos = QoS::from_u8((flags >> 1) & 0x03).ok_or(Error::InvalidQoS((flags >> 1) & 0x03))?;
        let retain = flags & 0x01 != 0;

        let mut pos = 0;

        let (topic, len) = read_string_slice(buf)?;
        let topic = topic.to_string();
        pos += len;

        let pkid = if qos != QoS::AtMostOnce {
            let id = read_u16(&buf[pos..]).ok_or(Error::Incomplete { needed: 2 })?;
            pos += 2;
            id
        } else {
            0
        };

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size + prop_len as usize;

        let payload = Bytes::copy_from_slice(&buf[pos..]);

        Ok(Publish {
            topic,
            payload,
            qos,
            retain,
            dup,
            pkid,
            properties: None,
        })
    }

    pub fn size(&self) -> usize {
        // Simplified size calculation
        2 + self.topic.len() + 1 + self.payload.len() + 4
    }
}

#[cfg(feature = "alloc")]
impl Subscribe {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        let mut pos = 2;

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size + prop_len as usize;

        let mut filters = Vec::new();
        while pos < buf.len() {
            let (path, len) = read_string_slice(&buf[pos..])?;
            pos += len;

            if pos >= buf.len() {
                return Err(Error::Incomplete { needed: 1 });
            }
            let options = buf[pos];
            let qos = QoS::from_u8(options & 0x03).ok_or(Error::InvalidQoS(options))?;
            let no_local = options & 0x04 != 0;
            let retain_as_published = options & 0x08 != 0;
            let retain_handling = match (options >> 4) & 0x03 {
                0 => RetainHandling::SendOnSubscribe,
                1 => RetainHandling::SendOnSubscribeIfNew,
                2 => RetainHandling::DoNotSend,
                _ => RetainHandling::SendOnSubscribe,
            };
            pos += 1;

            filters.push(SubscribeFilter {
                path: path.to_string(),
                qos,
                no_local,
                retain_as_published,
                retain_handling,
            });
        }

        Ok(Subscribe { pkid, filters, properties: None })
    }
}

#[cfg(feature = "alloc")]
impl SubAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        let mut pos = 2;

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size + prop_len as usize;

        let return_codes: Vec<_> = buf[pos..].iter().map(|&b| SubscribeReasonCode::from_u8(b)).collect();

        Ok(SubAck { pkid, return_codes, properties: None })
    }
}

#[cfg(feature = "alloc")]
impl Unsubscribe {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        let mut pos = 2;

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size + prop_len as usize;

        let mut filters = Vec::new();
        while pos < buf.len() {
            let (topic, len) = read_string_slice(&buf[pos..])?;
            filters.push(topic.to_string());
            pos += len;
        }

        Ok(Unsubscribe { pkid, filters, properties: None })
    }
}

#[cfg(feature = "alloc")]
impl UnsubAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        let mut pos = 2;

        // Properties length
        let (prop_len, prop_len_size) = read_variable_int(&buf[pos..])
            .ok_or(Error::Incomplete { needed: 1 })?;
        pos += prop_len_size + prop_len as usize;

        let reasons: Vec<_> = buf[pos..].iter().map(|&b| UnsubAckReason::from_u8(b)).collect();

        Ok(UnsubAck { pkid, reasons, properties: None })
    }
}

#[cfg(feature = "alloc")]
impl Disconnect {
    pub fn read(buf: &[u8]) -> Result<Self> {
        let reason_code = if buf.is_empty() {
            DisconnectReasonCode::NormalDisconnection
        } else {
            DisconnectReasonCode::from_u8(buf[0])
                .ok_or(Error::Protocol(crate::error::ProtocolError::MalformedPacket))?
        };

        Ok(Disconnect {
            reason_code,
            properties: None,
        })
    }
}

// ============================================================================
// Helper functions for creating packets
// ============================================================================

#[cfg(feature = "alloc")]
/// Create a CONNECT packet.
pub fn create_connect(
    client_id: &str,
    username: Option<&str>,
    password: Option<&[u8]>,
    keep_alive: u16,
    clean_start: bool,
    session_expiry: Option<u32>,
) -> Packet {
    let properties = session_expiry.map(|s| ConnectProperties {
        session_expiry_interval: Some(s),
        ..Default::default()
    });

    Packet::Connect(Connect {
        client_id: client_id.to_string(),
        keep_alive,
        clean_start,
        username: username.map(|s| s.to_string()),
        password: password.map(|p| p.to_vec()),
        will: None,
        properties,
    })
}

#[cfg(feature = "alloc")]
/// Create a CONNACK packet.
pub fn create_connack(session_present: bool, code: ConnectReasonCodeV5) -> Packet {
    Packet::ConnAck(ConnAck {
        session_present,
        code,
        properties: None,
    })
}

#[cfg(feature = "alloc")]
/// Create a PUBLISH packet (QoS 0).
pub fn create_publish(topic: &str, payload: &[u8], retain: bool) -> Packet {
    Packet::Publish(Publish {
        topic: topic.to_string(),
        payload: Bytes::copy_from_slice(payload),
        qos: QoS::AtMostOnce,
        retain,
        dup: false,
        pkid: 0,
        properties: None,
    })
}

#[cfg(feature = "alloc")]
/// Create a SUBSCRIBE packet.
pub fn create_subscribe(pkid: u16, topics: &[&str]) -> Packet {
    let filters = topics.iter().map(|t| SubscribeFilter {
        path: t.to_string(),
        qos: QoS::AtMostOnce,
        no_local: false,
        retain_as_published: false,
        retain_handling: RetainHandling::SendOnSubscribe,
    }).collect();

    Packet::Subscribe(Subscribe { pkid, filters, properties: None })
}

#[cfg(feature = "alloc")]
/// Create a SUBACK packet.
pub fn create_suback(pkid: u16, return_codes: Vec<SubscribeReasonCode>) -> Packet {
    Packet::SubAck(SubAck { pkid, return_codes, properties: None })
}

#[cfg(feature = "alloc")]
/// Create an UNSUBSCRIBE packet.
pub fn create_unsubscribe(pkid: u16, topics: &[&str]) -> Packet {
    let filters = topics.iter().map(|t| t.to_string()).collect();
    Packet::Unsubscribe(Unsubscribe { pkid, filters, properties: None })
}

#[cfg(feature = "alloc")]
/// Create an UNSUBACK packet.
pub fn create_unsuback(pkid: u16) -> Packet {
    Packet::UnsubAck(UnsubAck {
        pkid,
        reasons: vec![UnsubAckReason::Success],
        properties: None,
    })
}

#[cfg(feature = "alloc")]
/// Create a PINGREQ packet.
pub fn create_pingreq() -> Packet {
    Packet::PingReq
}

#[cfg(feature = "alloc")]
/// Create a PINGRESP packet.
pub fn create_pingresp() -> Packet {
    Packet::PingResp
}

#[cfg(feature = "alloc")]
/// Create a DISCONNECT packet.
pub fn create_disconnect() -> Packet {
    Packet::Disconnect(Disconnect {
        reason_code: DisconnectReasonCode::NormalDisconnection,
        properties: None,
    })
}
