//! Hexadecimal encoding type.

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::fmt;
use std::ops::{Deref, DerefMut};

/// A byte slice that serializes to/from hexadecimal in JSON.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]
pub struct HexData(Vec<u8>);

impl HexData {
    /// Creates a new HexData from bytes.
    pub fn new(data: Vec<u8>) -> Self {
        Self(data)
    }

    /// Creates a new empty HexData.
    pub fn empty() -> Self {
        Self(Vec::new())
    }

    /// Returns the underlying bytes.
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }

    /// Returns the underlying bytes as a mutable reference.
    pub fn as_bytes_mut(&mut self) -> &mut Vec<u8> {
        &mut self.0
    }

    /// Consumes self and returns the underlying bytes.
    pub fn into_bytes(self) -> Vec<u8> {
        self.0
    }

    /// Returns true if the data is empty.
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    /// Returns the length of the data.
    pub fn len(&self) -> usize {
        self.0.len()
    }

    /// Encodes the data to a hex string.
    pub fn encode(&self) -> String {
        hex::encode(&self.0)
    }

    /// Decodes a hex string into HexData.
    pub fn decode(s: &str) -> Result<Self, hex::FromHexError> {
        Ok(Self(hex::decode(s)?))
    }
}

impl fmt::Display for HexData {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.encode())
    }
}

impl Serialize for HexData {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.encode())
    }
}

impl<'de> Deserialize<'de> for HexData {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct HexVisitor;

        impl<'de> serde::de::Visitor<'de> for HexVisitor {
            type Value = HexData;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str("a hex-encoded string")
            }

            fn visit_str<E: serde::de::Error>(self, v: &str) -> Result<Self::Value, E> {
                HexData::decode(v).map_err(serde::de::Error::custom)
            }

            fn visit_unit<E: serde::de::Error>(self) -> Result<Self::Value, E> {
                Ok(HexData::empty())
            }

            fn visit_none<E: serde::de::Error>(self) -> Result<Self::Value, E> {
                Ok(HexData::empty())
            }
        }

        deserializer.deserialize_any(HexVisitor)
    }
}

impl Deref for HexData {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl DerefMut for HexData {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

impl From<Vec<u8>> for HexData {
    fn from(data: Vec<u8>) -> Self {
        Self(data)
    }
}

impl From<&[u8]> for HexData {
    fn from(data: &[u8]) -> Self {
        Self(data.to_vec())
    }
}

impl<const N: usize> From<[u8; N]> for HexData {
    fn from(data: [u8; N]) -> Self {
        Self(data.to_vec())
    }
}

impl From<HexData> for Vec<u8> {
    fn from(data: HexData) -> Self {
        data.0
    }
}

impl AsRef<[u8]> for HexData {
    fn as_ref(&self) -> &[u8] {
        &self.0
    }
}
