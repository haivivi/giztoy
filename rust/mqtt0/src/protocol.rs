//! MQTT protocol utilities.
//!
//! This module provides async read/write functions for MQTT packets,
//! supporting both MQTT 3.1.1 (v4) and MQTT 5.0 (v5).

use bytes::BytesMut;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};

use crate::error::{Error, Result};

/// Maximum packet size (1MB default).
pub const MAX_PACKET_SIZE: usize = 1024 * 1024;

// ============================================================================
// V4 (MQTT 3.1.1)
// ============================================================================

pub mod v4 {
    use super::*;
    pub use rumqttc::mqttbytes::v4::*;
    pub use rumqttc::mqttbytes::QoS;

    /// Read a single MQTT v4 packet from an async reader.
    pub async fn read_packet<R: AsyncRead + Unpin>(
        reader: &mut R,
        buf: &mut BytesMut,
        max_size: usize,
    ) -> Result<Packet> {
        loop {
            if !buf.is_empty() {
                match Packet::read(buf, max_size) {
                    Ok(packet) => return Ok(packet),
                    Err(rumqttc::mqttbytes::Error::InsufficientBytes(_)) => {}
                    Err(e) => return Err(Error::Protocol(e.to_string())),
                }
            }

            let mut tmp = [0u8; 4096];
            let n = reader.read(&mut tmp).await?;
            if n == 0 {
                return Err(Error::ConnectionClosed);
            }
            buf.extend_from_slice(&tmp[..n]);
        }
    }

    /// Write a single MQTT v4 packet to an async writer.
    pub async fn write_packet<W: AsyncWrite + Unpin>(
        writer: &mut W,
        packet: Packet,
    ) -> Result<()> {
        let mut buf = BytesMut::with_capacity(packet.size());
        packet
            .write(&mut buf, super::MAX_PACKET_SIZE)
            .map_err(|e| Error::Protocol(e.to_string()))?;
        writer.write_all(&buf).await?;
        writer.flush().await?;
        Ok(())
    }

    /// Create a CONNECT packet.
    pub fn create_connect(
        client_id: &str,
        username: Option<&str>,
        password: Option<&[u8]>,
        keep_alive: u16,
        clean_session: bool,
    ) -> Packet {
        let mut connect = Connect::new(client_id);
        connect.keep_alive = keep_alive;
        connect.clean_session = clean_session;

        if let (Some(user), Some(pass)) = (username, password) {
            let pass_str = std::str::from_utf8(pass).unwrap_or_else(|_| {
                tracing::warn!(
                    "Password contains non-UTF8 bytes for client {}, using empty string",
                    client_id
                );
                ""
            });
            connect.set_login(user, pass_str);
        }

        Packet::Connect(connect)
    }

    /// Create a CONNACK packet.
    pub fn create_connack(session_present: bool, code: ConnectReturnCode) -> Packet {
        Packet::ConnAck(ConnAck::new(code, session_present))
    }

    /// Create a PUBLISH packet (QoS 0).
    pub fn create_publish(topic: &str, payload: &[u8], retain: bool) -> Packet {
        let mut publish = Publish::new(topic, QoS::AtMostOnce, payload.to_vec());
        publish.retain = retain;
        Packet::Publish(publish)
    }

    /// Create a SUBSCRIBE packet.
    pub fn create_subscribe(pkid: u16, topics: &[&str]) -> Packet {
        let filters: Vec<SubscribeFilter> = topics
            .iter()
            .map(|t| SubscribeFilter::new(t.to_string(), QoS::AtMostOnce))
            .collect();

        let mut subscribe = Subscribe::new_many(filters);
        subscribe.pkid = pkid;
        Packet::Subscribe(subscribe)
    }

    /// Create a SUBACK packet.
    pub fn create_suback(pkid: u16, return_codes: Vec<SubscribeReasonCode>) -> Packet {
        Packet::SubAck(SubAck::new(pkid, return_codes))
    }

    /// Create an UNSUBSCRIBE packet.
    pub fn create_unsubscribe(pkid: u16, topics: &[&str]) -> Packet {
        let topics: Vec<String> = topics.iter().map(|t| t.to_string()).collect();
        Packet::Unsubscribe(Unsubscribe { pkid, topics })
    }

    /// Create an UNSUBACK packet.
    pub fn create_unsuback(pkid: u16) -> Packet {
        Packet::UnsubAck(UnsubAck::new(pkid))
    }

    /// Create a PINGREQ packet.
    pub fn create_pingreq() -> Packet {
        Packet::PingReq
    }

    /// Create a PINGRESP packet.
    pub fn create_pingresp() -> Packet {
        Packet::PingResp
    }

    /// Create a DISCONNECT packet.
    pub fn create_disconnect() -> Packet {
        Packet::Disconnect
    }
}

// ============================================================================
// V5 (MQTT 5.0)
// ============================================================================

pub mod v5 {
    use super::*;
    // rumqttc exposes v5 types under rumqttc::v5::mqttbytes::v5
    pub use rumqttc::v5::mqttbytes::v5::{
        ConnAck, Connect, ConnectProperties, ConnectReturnCode, Disconnect,
        DisconnectReasonCode, Filter, Login, Packet, PingReq, PingResp, Publish,
        SubAck, Subscribe, UnsubAck, UnsubAckReason, Unsubscribe,
    };
    pub use rumqttc::v5::mqttbytes::QoS;

    // Re-export SubscribeReasonCode explicitly to avoid ambiguity
    pub type SubscribeReasonCode = rumqttc::v5::mqttbytes::v5::SubscribeReasonCode;

    /// Read a single MQTT v5 packet from an async reader.
    pub async fn read_packet<R: AsyncRead + Unpin>(
        reader: &mut R,
        buf: &mut BytesMut,
        max_size: usize,
    ) -> Result<Packet> {
        let max_size_opt = Some(max_size as u32);
        loop {
            if !buf.is_empty() {
                match Packet::read(buf, max_size_opt) {
                    Ok(packet) => return Ok(packet),
                    Err(rumqttc::v5::mqttbytes::Error::InsufficientBytes(_)) => {}
                    Err(e) => return Err(Error::Protocol(e.to_string())),
                }
            }

            let mut tmp = [0u8; 4096];
            let n = reader.read(&mut tmp).await?;
            if n == 0 {
                return Err(Error::ConnectionClosed);
            }
            buf.extend_from_slice(&tmp[..n]);
        }
    }

    /// Write a single MQTT v5 packet to an async writer.
    pub async fn write_packet<W: AsyncWrite + Unpin>(
        writer: &mut W,
        packet: Packet,
    ) -> Result<()> {
        let max_size_opt = Some(super::MAX_PACKET_SIZE as u32);
        let mut buf = BytesMut::with_capacity(packet.size());
        packet
            .write(&mut buf, max_size_opt)
            .map_err(|e| Error::Protocol(e.to_string()))?;
        writer.write_all(&buf).await?;
        writer.flush().await?;
        Ok(())
    }

    /// Create a CONNECT packet.
    ///
    /// `session_expiry` is the Session Expiry Interval in seconds:
    /// - None: use broker default
    /// - Some(0): session ends immediately on disconnect
    /// - Some(n): session persists for n seconds after disconnect
    /// - Some(0xFFFFFFFF): session never expires
    pub fn create_connect(
        client_id: &str,
        username: Option<&str>,
        password: Option<&[u8]>,
        keep_alive: u16,
        clean_start: bool,
        session_expiry: Option<u32>,
    ) -> Packet {
        // Build Connect struct directly since Connect::new doesn't exist in v5
        let mut connect = Connect {
            keep_alive,
            client_id: client_id.to_string(),
            clean_start,
            properties: None,
        };

        // Set login if credentials provided
        let login = match (username, password) {
            (Some(user), Some(pass)) => {
                let pass_str = std::str::from_utf8(pass).unwrap_or_else(|_| {
                    tracing::warn!(
                        "Password contains non-UTF8 bytes for client {}, using empty string",
                        client_id
                    );
                    ""
                });
                Some(Login {
                    username: user.to_string(),
                    password: pass_str.to_string(),
                })
            }
            _ => None,
        };

        // Set session expiry interval
        if session_expiry.is_some() {
            let mut props = ConnectProperties::new();
            props.session_expiry_interval = session_expiry;
            connect.properties = Some(props);
        }

        Packet::Connect(connect, None, login)
    }

    /// Create a CONNACK packet.
    pub fn create_connack(session_present: bool, code: ConnectReturnCode) -> Packet {
        Packet::ConnAck(ConnAck {
            session_present,
            code,
            properties: None,
        })
    }

    /// Create a PUBLISH packet (QoS 0).
    pub fn create_publish(topic: &str, payload: &[u8], retain: bool) -> Packet {
        Packet::Publish(Publish {
            dup: false,
            qos: QoS::AtMostOnce,
            retain,
            topic: bytes::Bytes::copy_from_slice(topic.as_bytes()),
            pkid: 0,
            payload: bytes::Bytes::copy_from_slice(payload),
            properties: None,
        })
    }

    /// Create a PUBLISH packet with topic alias (QoS 0, MQTT 5.0).
    ///
    /// Used for testing topic alias functionality:
    /// - `topic`: The topic string (can be empty when using existing alias)
    /// - `topic_alias`: The topic alias value (1-65535, 0 is invalid per MQTT spec)
    pub fn create_publish_with_alias(
        topic: &str,
        payload: &[u8],
        retain: bool,
        topic_alias: u16,
    ) -> Packet {
        use rumqttc::v5::mqttbytes::v5::PublishProperties;
        let mut props = PublishProperties::default();
        props.topic_alias = Some(topic_alias);
        Packet::Publish(Publish {
            dup: false,
            qos: QoS::AtMostOnce,
            retain,
            topic: bytes::Bytes::copy_from_slice(topic.as_bytes()),
            pkid: 0,
            payload: bytes::Bytes::copy_from_slice(payload),
            properties: Some(props),
        })
    }

    /// Create a SUBSCRIBE packet.
    pub fn create_subscribe(pkid: u16, topics: &[&str]) -> Packet {
        let filters: Vec<Filter> = topics
            .iter()
            .map(|t| Filter {
                path: t.to_string(),
                qos: QoS::AtMostOnce,
                nolocal: false,
                preserve_retain: false,
                retain_forward_rule: rumqttc::v5::mqttbytes::v5::RetainForwardRule::OnEverySubscribe,
            })
            .collect();

        Packet::Subscribe(Subscribe {
            pkid,
            filters,
            properties: None,
        })
    }

    /// Create a SUBACK packet.
    pub fn create_suback(pkid: u16, return_codes: Vec<SubscribeReasonCode>) -> Packet {
        Packet::SubAck(SubAck {
            pkid,
            return_codes,
            properties: None,
        })
    }

    /// Create an UNSUBSCRIBE packet.
    pub fn create_unsubscribe(pkid: u16, topics: &[&str]) -> Packet {
        let filters: Vec<String> = topics.iter().map(|t| t.to_string()).collect();
        Packet::Unsubscribe(Unsubscribe {
            pkid,
            filters,
            properties: None,
        })
    }

    /// Create an UNSUBACK packet.
    pub fn create_unsuback(pkid: u16) -> Packet {
        Packet::UnsubAck(UnsubAck {
            pkid,
            reasons: vec![UnsubAckReason::Success],
            properties: None,
        })
    }

    /// Create a PINGREQ packet.
    pub fn create_pingreq() -> Packet {
        Packet::PingReq(PingReq)
    }

    /// Create a PINGRESP packet.
    pub fn create_pingresp() -> Packet {
        Packet::PingResp(PingResp)
    }

    /// Create a DISCONNECT packet.
    pub fn create_disconnect() -> Packet {
        Packet::Disconnect(Disconnect {
            reason_code: DisconnectReasonCode::NormalDisconnection,
            properties: None,
        })
    }
}

// Legacy API aliases are no longer needed - use v4:: or v5:: module directly

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_create_connect_v4() {
        let packet = v4::create_connect("client-1", Some("user"), Some(b"pass"), 60, true);
        if let v4::Packet::Connect(connect) = packet {
            assert_eq!(connect.client_id, "client-1");
            assert_eq!(connect.keep_alive, 60);
            assert!(connect.clean_session);
        } else {
            panic!("Expected Connect packet");
        }
    }

    #[test]
    fn test_create_connect_v5() {
        let packet = v5::create_connect(
            "client-1",
            Some("user"),
            Some(b"pass"),
            60,
            true,
            Some(3600), // 1 hour session expiry
        );
        if let v5::Packet::Connect(connect, _, _) = packet {
            assert_eq!(connect.client_id, "client-1");
            assert_eq!(connect.keep_alive, 60);
            assert!(connect.clean_start);
            assert_eq!(
                connect.properties.as_ref().unwrap().session_expiry_interval,
                Some(3600)
            );
        } else {
            panic!("Expected Connect packet");
        }
    }

    #[test]
    fn test_create_publish_v4() {
        let packet = v4::create_publish("test/topic", b"hello", false);
        if let v4::Packet::Publish(publish) = packet {
            assert_eq!(publish.topic, "test/topic");
            assert_eq!(publish.payload.as_ref(), b"hello");
            assert!(!publish.retain);
        } else {
            panic!("Expected Publish packet");
        }
    }

    #[test]
    fn test_create_publish_v5() {
        let packet = v5::create_publish("test/topic", b"hello", false);
        if let v5::Packet::Publish(publish) = packet {
            assert_eq!(publish.topic.as_ref(), b"test/topic");
            assert_eq!(publish.payload.as_ref(), b"hello");
            assert!(!publish.retain);
        } else {
            panic!("Expected Publish packet");
        }
    }
}
