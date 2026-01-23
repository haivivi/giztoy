//! Standard Base64 encoding type.

use base64::{engine::general_purpose::STANDARD, Engine};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::fmt;
use std::ops::{Deref, DerefMut};

/// A byte slice that serializes to/from standard Base64 in JSON.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]
pub struct StdBase64Data(Vec<u8>);

impl StdBase64Data {
    /// Creates a new StdBase64Data from bytes.
    pub fn new(data: Vec<u8>) -> Self {
        Self(data)
    }

    /// Creates a new empty StdBase64Data.
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

    /// Encodes the data to a Base64 string.
    pub fn encode(&self) -> String {
        STANDARD.encode(&self.0)
    }

    /// Decodes a Base64 string into StdBase64Data.
    pub fn decode(s: &str) -> Result<Self, base64::DecodeError> {
        Ok(Self(STANDARD.decode(s)?))
    }
}

impl fmt::Display for StdBase64Data {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.encode())
    }
}

impl Serialize for StdBase64Data {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.encode())
    }
}

impl<'de> Deserialize<'de> for StdBase64Data {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct Base64Visitor;

        impl<'de> serde::de::Visitor<'de> for Base64Visitor {
            type Value = StdBase64Data;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str("a base64-encoded string")
            }

            fn visit_str<E: serde::de::Error>(self, v: &str) -> Result<Self::Value, E> {
                StdBase64Data::decode(v).map_err(serde::de::Error::custom)
            }

            fn visit_unit<E: serde::de::Error>(self) -> Result<Self::Value, E> {
                Ok(StdBase64Data::empty())
            }

            fn visit_none<E: serde::de::Error>(self) -> Result<Self::Value, E> {
                Ok(StdBase64Data::empty())
            }
        }

        deserializer.deserialize_any(Base64Visitor)
    }
}

impl Deref for StdBase64Data {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl DerefMut for StdBase64Data {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

impl From<Vec<u8>> for StdBase64Data {
    fn from(data: Vec<u8>) -> Self {
        Self(data)
    }
}

impl From<&[u8]> for StdBase64Data {
    fn from(data: &[u8]) -> Self {
        Self(data.to_vec())
    }
}

impl<const N: usize> From<[u8; N]> for StdBase64Data {
    fn from(data: [u8; N]) -> Self {
        Self(data.to_vec())
    }
}

impl From<StdBase64Data> for Vec<u8> {
    fn from(data: StdBase64Data) -> Self {
        data.0
    }
}

impl AsRef<[u8]> for StdBase64Data {
    fn as_ref(&self) -> &[u8] {
        &self.0
    }
}
