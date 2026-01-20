//! Binary protocol for Realtime and Podcast V3 services.
//!
//! Protocol format:
//! - Header (4 bytes):
//!   - (4bits) version + (4bits) header_size
//!   - (4bits) message_type + (4bits) message_type_flags
//!   - (4bits) serialization + (4bits) compression
//!   - (8bits) reserved
//!
//! - Payload:
//!   - [optional] sequence (4 bytes)
//!   - [optional] event (4 bytes)
//!   - [optional] session_id (4 bytes len + data)
//!   - payload_size (4 bytes) + payload_data

use std::io::{Cursor, Read, Write};

use bytes::{Buf, BufMut, BytesMut};
use flate2::read::GzDecoder;
use flate2::write::GzEncoder;
use flate2::Compression;

use crate::error::{Error, Result};

// ================== Protocol Constants ==================

/// Protocol version.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ProtocolVersion {
    #[default]
    V1 = 0b0001,
}

/// Message type.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum MessageType {
    #[default]
    FullClient = 0b0001,
    AudioOnlyClient = 0b0010,
    FullServer = 0b1001,
    AudioOnlyServer = 0b1011,
    FrontEndResult = 0b1100,
    Error = 0b1111,
}

impl From<u8> for MessageType {
    fn from(v: u8) -> Self {
        match v {
            0b0001 => MessageType::FullClient,
            0b0010 => MessageType::AudioOnlyClient,
            0b1001 => MessageType::FullServer,
            0b1011 => MessageType::AudioOnlyServer,
            0b1100 => MessageType::FrontEndResult,
            0b1111 => MessageType::Error,
            _ => MessageType::FullClient,
        }
    }
}

/// Message type flags.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum MessageFlags {
    #[default]
    NoSequence = 0b0000,
    PosSequence = 0b0001,
    NegSequence = 0b0010,
    NegWithSeq = 0b0011,
    WithEvent = 0b0100,
}

impl From<u8> for MessageFlags {
    fn from(v: u8) -> Self {
        match v {
            0b0000 => MessageFlags::NoSequence,
            0b0001 => MessageFlags::PosSequence,
            0b0010 => MessageFlags::NegSequence,
            0b0011 => MessageFlags::NegWithSeq,
            0b0100 => MessageFlags::WithEvent,
            _ => MessageFlags::NoSequence,
        }
    }
}

/// Serialization type.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum SerializationType {
    #[default]
    None = 0b0000,
    Json = 0b0001,
    Thrift = 0b0011,
}

impl From<u8> for SerializationType {
    fn from(v: u8) -> Self {
        match v {
            0b0000 => SerializationType::None,
            0b0001 => SerializationType::Json,
            0b0011 => SerializationType::Thrift,
            _ => SerializationType::None,
        }
    }
}

/// Compression type.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum CompressionType {
    #[default]
    None = 0b0000,
    Gzip = 0b0001,
}

impl From<u8> for CompressionType {
    fn from(v: u8) -> Self {
        match v {
            0b0000 => CompressionType::None,
            0b0001 => CompressionType::Gzip,
            _ => CompressionType::None,
        }
    }
}

/// Protocol event types.
pub mod events {
    pub const SESSION_START: i32 = 1;
    pub const SESSION_FINISH: i32 = 2;
    pub const CONNECTION_STARTED: i32 = 50;
    pub const CONNECTION_FAILED: i32 = 51;
    pub const CONNECTION_FINISHED: i32 = 52;
    
    // Realtime events
    pub const ASR_STARTED: i32 = 100;
    pub const ASR_FINISHED: i32 = 101;
    pub const TTS_STARTED: i32 = 102;
    pub const TTS_FINISHED: i32 = 103;
    pub const AUDIO_RECEIVED: i32 = 104;
}

// ================== Protocol Message ==================

/// Binary protocol message.
#[derive(Debug, Clone, Default)]
pub struct Message {
    /// Message type.
    pub msg_type: MessageType,
    /// Message flags.
    pub flags: MessageFlags,
    /// Event type (if flags include WithEvent).
    pub event: i32,
    /// Session ID.
    pub session_id: String,
    /// Connect ID.
    pub connect_id: String,
    /// Sequence number.
    pub sequence: i32,
    /// Error code (for error messages).
    pub error_code: u32,
    /// Payload data.
    pub payload: Vec<u8>,
}

impl Message {
    /// Creates a new message.
    pub fn new() -> Self {
        Self::default()
    }

    /// Creates an audio-only client message.
    pub fn audio_only(session_id: &str, audio: Vec<u8>, event: i32) -> Self {
        Self {
            msg_type: MessageType::AudioOnlyClient,
            flags: MessageFlags::WithEvent,
            event,
            session_id: session_id.to_string(),
            payload: audio,
            ..Default::default()
        }
    }

    /// Returns true if this is an audio-only message.
    pub fn is_audio_only(&self) -> bool {
        matches!(
            self.msg_type,
            MessageType::AudioOnlyServer | MessageType::AudioOnlyClient
        )
    }

    /// Returns true if this is an error message.
    pub fn is_error(&self) -> bool {
        self.msg_type == MessageType::Error
    }

    /// Returns true if this is a frontend result message.
    pub fn is_frontend(&self) -> bool {
        self.msg_type == MessageType::FrontEndResult
    }

    /// Returns true if flags include WithEvent.
    pub fn has_event(&self) -> bool {
        self.flags == MessageFlags::WithEvent
    }
}

// ================== Binary Protocol ==================

/// Binary protocol encoder/decoder.
#[derive(Debug, Clone)]
pub struct BinaryProtocol {
    version: ProtocolVersion,
    header_size: u8,
    compression: CompressionType,
    serialization: SerializationType,
}

impl Default for BinaryProtocol {
    fn default() -> Self {
        Self::new()
    }
}

impl BinaryProtocol {
    /// Creates a new binary protocol handler.
    pub fn new() -> Self {
        Self {
            version: ProtocolVersion::V1,
            header_size: 1, // 4 bytes
            compression: CompressionType::None,
            serialization: SerializationType::Json,
        }
    }

    /// Sets compression type.
    pub fn set_compression(&mut self, compression: CompressionType) {
        self.compression = compression;
    }

    /// Sets serialization type.
    pub fn set_serialization(&mut self, serialization: SerializationType) {
        self.serialization = serialization;
    }

    /// Marshals a message to bytes.
    pub fn marshal(&self, msg: &Message) -> Result<Vec<u8>> {
        let mut buf = BytesMut::with_capacity(256);

        // Header (4 bytes)
        buf.put_u8((self.version as u8) << 4 | self.header_size);
        buf.put_u8((msg.msg_type as u8) << 4 | (msg.flags as u8));
        buf.put_u8((self.serialization as u8) << 4 | (self.compression as u8));
        buf.put_u8(0x00); // reserved

        // Sequence (if needed)
        if msg.flags == MessageFlags::PosSequence || msg.flags == MessageFlags::NegSequence {
            buf.put_i32(msg.sequence);
        }

        // Event (if needed)
        if msg.flags == MessageFlags::WithEvent {
            buf.put_i32(msg.event);

            // Session ID (for non-connection events)
            if !is_connection_event(msg.event) {
                buf.put_u32(msg.session_id.len() as u32);
                buf.put_slice(msg.session_id.as_bytes());
            }
        }

        // Payload
        let payload = if self.compression == CompressionType::Gzip && !msg.payload.is_empty() {
            gzip_compress(&msg.payload)?
        } else {
            msg.payload.clone()
        };

        buf.put_u32(payload.len() as u32);
        buf.put_slice(&payload);

        Ok(buf.to_vec())
    }

    /// Unmarshals bytes to a message.
    pub fn unmarshal(&self, data: &[u8]) -> Result<Message> {
        if data.len() < 4 {
            return Err(Error::Other(format!(
                "data too short: {} bytes",
                data.len()
            )));
        }

        let mut cursor = Cursor::new(data);

        // Read header
        let version_and_size = cursor.get_u8();
        let type_and_flags = cursor.get_u8();
        let ser_and_comp = cursor.get_u8();
        let _reserved = cursor.get_u8();

        let mut msg = Message {
            msg_type: MessageType::from(type_and_flags >> 4),
            flags: MessageFlags::from(type_and_flags & 0x0f),
            ..Default::default()
        };

        let compression = CompressionType::from(ser_and_comp & 0x0f);

        // Header size (in 4-byte units)
        let header_size = (version_and_size & 0x0f) as usize;
        if header_size > 1 {
            // Skip additional header bytes
            cursor.advance((header_size - 1) * 4);
        }

        // Read sequence if present
        if msg.flags == MessageFlags::PosSequence || msg.flags == MessageFlags::NegSequence {
            msg.sequence = cursor.get_i32();
        }

        // Read event if present
        if msg.flags == MessageFlags::WithEvent {
            msg.event = cursor.get_i32();

            // Read session ID (for non-connection events)
            if !is_connection_event(msg.event) {
                let session_id_len = cursor.get_u32() as usize;
                if session_id_len > 0 {
                    let mut session_id_bytes = vec![0u8; session_id_len];
                    cursor.copy_to_slice(&mut session_id_bytes);
                    msg.session_id = String::from_utf8_lossy(&session_id_bytes).to_string();
                }
            }

            // Read connect ID for connection events
            if is_connection_event(msg.event) {
                let connect_id_len = cursor.get_u32() as usize;
                if connect_id_len > 0 {
                    let mut connect_id_bytes = vec![0u8; connect_id_len];
                    cursor.copy_to_slice(&mut connect_id_bytes);
                    msg.connect_id = String::from_utf8_lossy(&connect_id_bytes).to_string();
                }
            }
        }

        // Read error code for error messages
        if msg.msg_type == MessageType::Error {
            msg.error_code = cursor.get_u32();
        }

        // Read payload
        let payload_size = cursor.get_u32() as usize;
        if payload_size > 0 {
            let mut payload = vec![0u8; payload_size];
            cursor.copy_to_slice(&mut payload);

            // Decompress if needed
            if compression == CompressionType::Gzip {
                payload = gzip_decompress(&payload)?;
            }

            msg.payload = payload;
        }

        Ok(msg)
    }
}

/// Returns true if the event is a connection-level event.
fn is_connection_event(event: i32) -> bool {
    matches!(
        event,
        events::SESSION_START
            | events::SESSION_FINISH
            | events::CONNECTION_STARTED
            | events::CONNECTION_FAILED
            | events::CONNECTION_FINISHED
    )
}

/// Gzip compress data.
fn gzip_compress(data: &[u8]) -> Result<Vec<u8>> {
    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder
        .write_all(data)
        .map_err(|e| Error::Other(format!("gzip compress: {}", e)))?;
    encoder
        .finish()
        .map_err(|e| Error::Other(format!("gzip finish: {}", e)))
}

/// Gzip decompress data.
fn gzip_decompress(data: &[u8]) -> Result<Vec<u8>> {
    let mut decoder = GzDecoder::new(data);
    let mut decompressed = Vec::new();
    decoder
        .read_to_end(&mut decompressed)
        .map_err(|e| Error::Other(format!("gzip decompress: {}", e)))?;
    Ok(decompressed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_marshal_unmarshal() {
        let proto = BinaryProtocol::new();

        let msg = Message {
            msg_type: MessageType::AudioOnlyClient,
            flags: MessageFlags::WithEvent,
            event: events::AUDIO_RECEIVED,
            session_id: "test-session".to_string(),
            payload: vec![1, 2, 3, 4, 5],
            ..Default::default()
        };

        let data = proto.marshal(&msg).unwrap();
        let decoded = proto.unmarshal(&data).unwrap();

        assert_eq!(decoded.msg_type, msg.msg_type);
        assert_eq!(decoded.flags, msg.flags);
        assert_eq!(decoded.event, msg.event);
        assert_eq!(decoded.session_id, msg.session_id);
        assert_eq!(decoded.payload, msg.payload);
    }
}
