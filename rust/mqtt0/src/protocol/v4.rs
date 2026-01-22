//! MQTT 3.1.1 (v4) protocol implementation.

#[cfg(feature = "alloc")]
use alloc::string::String;
#[cfg(feature = "alloc")]
use alloc::vec::Vec;

use bytes::Bytes;

use crate::error::{Error, Result};
use crate::types::{ConnectFlags, ConnectReturnCodeV4, PacketType, QoS};

use super::codec::{
    read_binary_slice, read_fixed_header, read_string_slice, read_u16,
    variable_int_len, write_binary, write_fixed_header, write_string, write_u16,
};

/// MQTT 3.1.1 packet.
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
    Disconnect,
}

/// CONNECT packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Connect {
    pub client_id: String,
    pub keep_alive: u16,
    pub clean_session: bool,
    pub username: Option<String>,
    pub password: Option<Vec<u8>>,
    pub will: Option<Will>,
}

/// Last Will and Testament.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Will {
    pub topic: String,
    pub payload: Vec<u8>,
    pub qos: QoS,
    pub retain: bool,
}

/// CONNACK packet.
#[derive(Debug, Clone, Copy)]
pub struct ConnAck {
    pub session_present: bool,
    pub code: ConnectReturnCodeV4,
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
}

/// SUBSCRIBE packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Subscribe {
    pub pkid: u16,
    pub filters: Vec<SubscribeFilter>,
}

/// Subscribe filter.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct SubscribeFilter {
    pub path: String,
    pub qos: QoS,
}

/// SUBACK packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct SubAck {
    pub pkid: u16,
    pub return_codes: Vec<SubscribeReasonCode>,
}

/// Subscribe reason code.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubscribeReasonCode {
    Success(QoS),
    Failure,
}

impl SubscribeReasonCode {
    pub fn from_u8(value: u8) -> Self {
        match value {
            0 => SubscribeReasonCode::Success(QoS::AtMostOnce),
            1 => SubscribeReasonCode::Success(QoS::AtLeastOnce),
            2 => SubscribeReasonCode::Success(QoS::ExactlyOnce),
            _ => SubscribeReasonCode::Failure,
        }
    }

    pub fn to_u8(self) -> u8 {
        match self {
            SubscribeReasonCode::Success(QoS::AtMostOnce) => 0,
            SubscribeReasonCode::Success(QoS::AtLeastOnce) => 1,
            SubscribeReasonCode::Success(QoS::ExactlyOnce) => 2,
            SubscribeReasonCode::Failure => 0x80,
        }
    }
}

/// UNSUBSCRIBE packet.
#[cfg(feature = "alloc")]
#[derive(Debug, Clone)]
pub struct Unsubscribe {
    pub pkid: u16,
    pub topics: Vec<String>,
}

/// UNSUBACK packet.
#[derive(Debug, Clone, Copy)]
pub struct UnsubAck {
    pub pkid: u16,
}

// ============================================================================
// Packet parsing
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
            PacketType::Disconnect => Packet::Disconnect,
            _ => return Err(Error::InvalidPacketType(header.packet_type as u8)),
        };

        Ok((packet, total_len))
    }

    /// Write packet to buffer.
    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        match self {
            Packet::Connect(p) => p.write(buf),
            Packet::ConnAck(p) => p.write(buf),
            Packet::Publish(p) => p.write(buf),
            Packet::Subscribe(p) => p.write(buf),
            Packet::SubAck(p) => p.write(buf),
            Packet::Unsubscribe(p) => p.write(buf),
            Packet::UnsubAck(p) => p.write(buf),
            Packet::PingReq => write_simple_packet(buf, PacketType::PingReq),
            Packet::PingResp => write_simple_packet(buf, PacketType::PingResp),
            Packet::Disconnect => write_simple_packet(buf, PacketType::Disconnect),
        }
    }

    /// Calculate packet size.
    pub fn size(&self) -> usize {
        match self {
            Packet::Connect(p) => p.size(),
            Packet::ConnAck(_) => 4,
            Packet::Publish(p) => p.size(),
            Packet::Subscribe(p) => p.size(),
            Packet::SubAck(p) => p.size(),
            Packet::Unsubscribe(p) => p.size(),
            Packet::UnsubAck(_) => 4,
            Packet::PingReq | Packet::PingResp | Packet::Disconnect => 2,
        }
    }
}

fn write_simple_packet(buf: &mut [u8], packet_type: PacketType) -> Result<usize> {
    if buf.len() < 2 {
        return Err(Error::BufferTooSmall { required: 2, available: buf.len() });
    }
    write_fixed_header(buf, packet_type, 0, 0)
        .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() })
}

// ============================================================================
// Individual packet implementations
// ============================================================================

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
        if protocol_level != 4 {
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

        // Client ID
        let (client_id, len) = read_string_slice(&buf[pos..])?;
        let client_id = client_id.to_string();
        pos += len;

        // Will
        let will = if flags.will {
            let (topic, len) = read_string_slice(&buf[pos..])?;
            pos += len;
            let (payload, len) = read_binary_slice(&buf[pos..])?;
            pos += len;
            Some(Will {
                topic: topic.to_string(),
                payload: payload.to_vec(),
                qos: flags.will_qos,
                retain: flags.will_retain,
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
            clean_session: flags.clean_session,
            username,
            password,
            will,
        })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        let remaining_len = self.remaining_length();
        let header_len = 1 + variable_int_len(remaining_len as u32);
        let total = header_len + remaining_len;

        if buf.len() < total {
            return Err(Error::BufferTooSmall { required: total, available: buf.len() });
        }

        let mut pos = write_fixed_header(buf, PacketType::Connect, 0, remaining_len as u32)
            .ok_or(Error::BufferTooSmall { required: header_len, available: buf.len() })?;

        // Protocol name
        pos += write_string(&mut buf[pos..], "MQTT")
            .ok_or(Error::BufferTooSmall { required: 6, available: buf.len() - pos })?;

        // Protocol level
        buf[pos] = 4;
        pos += 1;

        // Connect flags
        let mut flags = ConnectFlags {
            clean_session: self.clean_session,
            username: self.username.is_some(),
            password: self.password.is_some(),
            ..Default::default()
        };
        if let Some(ref will) = self.will {
            flags.will = true;
            flags.will_qos = will.qos;
            flags.will_retain = will.retain;
        }
        buf[pos] = flags.encode();
        pos += 1;

        // Keep alive
        write_u16(&mut buf[pos..], self.keep_alive)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - pos })?;
        pos += 2;

        // Client ID
        pos += write_string(&mut buf[pos..], &self.client_id)
            .ok_or(Error::BufferTooSmall { required: 2 + self.client_id.len(), available: buf.len() - pos })?;

        // Will
        if let Some(ref will) = self.will {
            pos += write_string(&mut buf[pos..], &will.topic)
                .ok_or(Error::BufferTooSmall { required: 2 + will.topic.len(), available: buf.len() - pos })?;
            pos += write_binary(&mut buf[pos..], &will.payload)
                .ok_or(Error::BufferTooSmall { required: 2 + will.payload.len(), available: buf.len() - pos })?;
        }

        // Username
        if let Some(ref username) = self.username {
            pos += write_string(&mut buf[pos..], username)
                .ok_or(Error::BufferTooSmall { required: 2 + username.len(), available: buf.len() - pos })?;
        }

        // Password
        if let Some(ref password) = self.password {
            pos += write_binary(&mut buf[pos..], password)
                .ok_or(Error::BufferTooSmall { required: 2 + password.len(), available: buf.len() - pos })?;
        }

        Ok(pos)
    }

    fn remaining_length(&self) -> usize {
        let mut len = 2 + 4 + 1 + 1 + 2; // protocol name + level + flags + keep_alive
        len += 2 + self.client_id.len();

        if let Some(ref will) = self.will {
            len += 2 + will.topic.len();
            len += 2 + will.payload.len();
        }
        if let Some(ref username) = self.username {
            len += 2 + username.len();
        }
        if let Some(ref password) = self.password {
            len += 2 + password.len();
        }

        len
    }

    pub fn size(&self) -> usize {
        let remaining = self.remaining_length();
        1 + variable_int_len(remaining as u32) + remaining
    }
}

impl ConnAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 - buf.len() });
        }

        let session_present = buf[0] & 0x01 != 0;
        let code = ConnectReturnCodeV4::from_u8(buf[1])
            .ok_or(Error::Protocol(crate::error::ProtocolError::MalformedPacket))?;

        Ok(ConnAck { session_present, code })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        if buf.len() < 4 {
            return Err(Error::BufferTooSmall { required: 4, available: buf.len() });
        }

        write_fixed_header(buf, PacketType::ConnAck, 0, 2)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() })?;

        buf[2] = if self.session_present { 0x01 } else { 0x00 };
        buf[3] = self.code as u8;

        Ok(4)
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

        let payload = Bytes::copy_from_slice(&buf[pos..]);

        Ok(Publish { topic, payload, qos, retain, dup, pkid })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        let remaining_len = self.remaining_length();
        let header_len = 1 + variable_int_len(remaining_len as u32);
        let total = header_len + remaining_len;

        if buf.len() < total {
            return Err(Error::BufferTooSmall { required: total, available: buf.len() });
        }

        let mut flags = (self.qos as u8) << 1;
        if self.dup {
            flags |= 0x08;
        }
        if self.retain {
            flags |= 0x01;
        }

        let mut pos = write_fixed_header(buf, PacketType::Publish, flags, remaining_len as u32)
            .ok_or(Error::BufferTooSmall { required: header_len, available: buf.len() })?;

        pos += write_string(&mut buf[pos..], &self.topic)
            .ok_or(Error::BufferTooSmall { required: 2 + self.topic.len(), available: buf.len() - pos })?;

        if self.qos != QoS::AtMostOnce {
            write_u16(&mut buf[pos..], self.pkid)
                .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - pos })?;
            pos += 2;
        }

        buf[pos..pos + self.payload.len()].copy_from_slice(&self.payload);
        pos += self.payload.len();

        Ok(pos)
    }

    fn remaining_length(&self) -> usize {
        let mut len = 2 + self.topic.len() + self.payload.len();
        if self.qos != QoS::AtMostOnce {
            len += 2;
        }
        len
    }

    pub fn size(&self) -> usize {
        let remaining = self.remaining_length();
        1 + variable_int_len(remaining as u32) + remaining
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

        let mut filters = Vec::new();
        while pos < buf.len() {
            let (path, len) = read_string_slice(&buf[pos..])?;
            pos += len;

            if pos >= buf.len() {
                return Err(Error::Incomplete { needed: 1 });
            }
            let qos = QoS::from_u8(buf[pos] & 0x03).ok_or(Error::InvalidQoS(buf[pos]))?;
            pos += 1;

            filters.push(SubscribeFilter { path: path.to_string(), qos });
        }

        Ok(Subscribe { pkid, filters })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        let remaining_len = self.remaining_length();
        let header_len = 1 + variable_int_len(remaining_len as u32);
        let total = header_len + remaining_len;

        if buf.len() < total {
            return Err(Error::BufferTooSmall { required: total, available: buf.len() });
        }

        // Subscribe has fixed flags of 0x02
        let mut pos = write_fixed_header(buf, PacketType::Subscribe, 0x02, remaining_len as u32)
            .ok_or(Error::BufferTooSmall { required: header_len, available: buf.len() })?;

        write_u16(&mut buf[pos..], self.pkid)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - pos })?;
        pos += 2;

        for filter in &self.filters {
            pos += write_string(&mut buf[pos..], &filter.path)
                .ok_or(Error::BufferTooSmall { required: 2 + filter.path.len(), available: buf.len() - pos })?;
            buf[pos] = filter.qos as u8;
            pos += 1;
        }

        Ok(pos)
    }

    fn remaining_length(&self) -> usize {
        let mut len = 2; // pkid
        for filter in &self.filters {
            len += 2 + filter.path.len() + 1;
        }
        len
    }

    pub fn size(&self) -> usize {
        let remaining = self.remaining_length();
        1 + variable_int_len(remaining as u32) + remaining
    }
}

#[cfg(feature = "alloc")]
impl SubAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        let return_codes: Vec<_> = buf[2..].iter().map(|&b| SubscribeReasonCode::from_u8(b)).collect();

        Ok(SubAck { pkid, return_codes })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        let remaining_len = 2 + self.return_codes.len();
        let header_len = 1 + variable_int_len(remaining_len as u32);
        let total = header_len + remaining_len;

        if buf.len() < total {
            return Err(Error::BufferTooSmall { required: total, available: buf.len() });
        }

        let mut pos = write_fixed_header(buf, PacketType::SubAck, 0, remaining_len as u32)
            .ok_or(Error::BufferTooSmall { required: header_len, available: buf.len() })?;

        write_u16(&mut buf[pos..], self.pkid)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - pos })?;
        pos += 2;

        for code in &self.return_codes {
            buf[pos] = code.to_u8();
            pos += 1;
        }

        Ok(pos)
    }

    pub fn size(&self) -> usize {
        let remaining = 2 + self.return_codes.len();
        1 + variable_int_len(remaining as u32) + remaining
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

        let mut topics = Vec::new();
        while pos < buf.len() {
            let (topic, len) = read_string_slice(&buf[pos..])?;
            topics.push(topic.to_string());
            pos += len;
        }

        Ok(Unsubscribe { pkid, topics })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        let remaining_len = self.remaining_length();
        let header_len = 1 + variable_int_len(remaining_len as u32);
        let total = header_len + remaining_len;

        if buf.len() < total {
            return Err(Error::BufferTooSmall { required: total, available: buf.len() });
        }

        // Unsubscribe has fixed flags of 0x02
        let mut pos = write_fixed_header(buf, PacketType::Unsubscribe, 0x02, remaining_len as u32)
            .ok_or(Error::BufferTooSmall { required: header_len, available: buf.len() })?;

        write_u16(&mut buf[pos..], self.pkid)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - pos })?;
        pos += 2;

        for topic in &self.topics {
            pos += write_string(&mut buf[pos..], topic)
                .ok_or(Error::BufferTooSmall { required: 2 + topic.len(), available: buf.len() - pos })?;
        }

        Ok(pos)
    }

    fn remaining_length(&self) -> usize {
        let mut len = 2; // pkid
        for topic in &self.topics {
            len += 2 + topic.len();
        }
        len
    }

    pub fn size(&self) -> usize {
        let remaining = self.remaining_length();
        1 + variable_int_len(remaining as u32) + remaining
    }
}

impl UnsubAck {
    pub fn read(buf: &[u8]) -> Result<Self> {
        if buf.len() < 2 {
            return Err(Error::Incomplete { needed: 2 });
        }

        let pkid = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })?;
        Ok(UnsubAck { pkid })
    }

    pub fn write(&self, buf: &mut [u8]) -> Result<usize> {
        if buf.len() < 4 {
            return Err(Error::BufferTooSmall { required: 4, available: buf.len() });
        }

        write_fixed_header(buf, PacketType::UnsubAck, 0, 2)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() })?;

        write_u16(&mut buf[2..], self.pkid)
            .ok_or(Error::BufferTooSmall { required: 2, available: buf.len() - 2 })?;

        Ok(4)
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
    clean_session: bool,
) -> Packet {
    Packet::Connect(Connect {
        client_id: client_id.to_string(),
        keep_alive,
        clean_session,
        username: username.map(|s| s.to_string()),
        password: password.map(|p| p.to_vec()),
        will: None,
    })
}

/// Create a CONNACK packet.
pub fn create_connack(session_present: bool, code: ConnectReturnCodeV4) -> ConnAck {
    ConnAck { session_present, code }
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
    })
}

#[cfg(feature = "alloc")]
/// Create a SUBSCRIBE packet.
pub fn create_subscribe(pkid: u16, topics: &[&str]) -> Packet {
    let filters = topics.iter().map(|t| SubscribeFilter {
        path: t.to_string(),
        qos: QoS::AtMostOnce,
    }).collect();

    Packet::Subscribe(Subscribe { pkid, filters })
}

#[cfg(feature = "alloc")]
/// Create a SUBACK packet.
pub fn create_suback(pkid: u16, return_codes: Vec<SubscribeReasonCode>) -> Packet {
    Packet::SubAck(SubAck { pkid, return_codes })
}

#[cfg(feature = "alloc")]
/// Create an UNSUBSCRIBE packet.
pub fn create_unsubscribe(pkid: u16, topics: &[&str]) -> Packet {
    let topics = topics.iter().map(|t| t.to_string()).collect();
    Packet::Unsubscribe(Unsubscribe { pkid, topics })
}

/// Create an UNSUBACK packet.
pub fn create_unsuback(pkid: u16) -> UnsubAck {
    UnsubAck { pkid }
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
    Packet::Disconnect
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_connack_roundtrip() {
        let connack = ConnAck {
            session_present: true,
            code: ConnectReturnCodeV4::Success,
        };

        let mut buf = [0u8; 10];
        let written = connack.write(&mut buf).unwrap();
        assert_eq!(written, 4);

        let header = read_fixed_header(&buf).unwrap();
        assert_eq!(header.packet_type, PacketType::ConnAck);

        let parsed = ConnAck::read(&buf[header.header_length..]).unwrap();
        assert_eq!(parsed.session_present, true);
        assert_eq!(parsed.code, ConnectReturnCodeV4::Success);
    }

    #[cfg(feature = "alloc")]
    #[test]
    fn test_publish_roundtrip() {
        let publish = Publish {
            topic: "test/topic".to_string(),
            payload: Bytes::from_static(b"hello"),
            qos: QoS::AtMostOnce,
            retain: false,
            dup: false,
            pkid: 0,
        };

        let mut buf = vec![0u8; publish.size()];
        let written = Packet::Publish(publish.clone()).write(&mut buf).unwrap();

        let (packet, consumed) = Packet::read(&buf, 1024).unwrap();
        assert_eq!(written, consumed);

        if let Packet::Publish(p) = packet {
            assert_eq!(p.topic, "test/topic");
            assert_eq!(p.payload.as_ref(), b"hello");
        } else {
            panic!("Expected Publish packet");
        }
    }
}
