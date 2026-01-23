//! Tests for encoding types.

use super::*;

// ============================================================================
// StdBase64Data tests
// ============================================================================

#[test]
fn test_base64_marshal_json() {
    let data = StdBase64Data::from(b"hello world".as_slice());
    let json = serde_json::to_string(&data).unwrap();
    assert_eq!(json, r#""aGVsbG8gd29ybGQ=""#);
}

#[test]
fn test_base64_unmarshal_json_valid() {
    let json = r#""aGVsbG8gd29ybGQ=""#;
    let data: StdBase64Data = serde_json::from_str(json).unwrap();
    assert_eq!(data.as_bytes(), b"hello world");
}

#[test]
fn test_base64_unmarshal_json_empty() {
    let json = r#""""#;
    let data: StdBase64Data = serde_json::from_str(json).unwrap();
    assert!(data.is_empty());
}

#[test]
fn test_base64_unmarshal_json_null() {
    let json = "null";
    let data: StdBase64Data = serde_json::from_str(json).unwrap();
    assert!(data.is_empty());
}

#[test]
fn test_base64_unmarshal_json_invalid() {
    let json = "123";
    let result: Result<StdBase64Data, _> = serde_json::from_str(json);
    assert!(result.is_err());
}

#[test]
fn test_base64_round_trip() {
    let original = StdBase64Data::from(b"test data for round trip".as_slice());
    let json = serde_json::to_string(&original).unwrap();
    let restored: StdBase64Data = serde_json::from_str(&json).unwrap();
    assert_eq!(original, restored);
}

#[test]
fn test_base64_string() {
    let data = StdBase64Data::from(b"hello".as_slice());
    assert_eq!(data.to_string(), "aGVsbG8=");
}

#[test]
fn test_base64_encode_decode() {
    let data = StdBase64Data::from(b"test".as_slice());
    let encoded = data.encode();
    let decoded = StdBase64Data::decode(&encoded).unwrap();
    assert_eq!(data, decoded);
}

#[test]
fn test_base64_from_conversions() {
    // From Vec<u8>
    let v = vec![1, 2, 3];
    let d1 = StdBase64Data::from(v.clone());
    assert_eq!(d1.as_bytes(), &[1, 2, 3]);

    // From &[u8]
    let d2 = StdBase64Data::from(v.as_slice());
    assert_eq!(d1, d2);

    // From array
    let d3 = StdBase64Data::from([1u8, 2, 3]);
    assert_eq!(d1, d3);

    // Into Vec<u8>
    let back: Vec<u8> = d1.into();
    assert_eq!(back, vec![1, 2, 3]);
}

#[test]
fn test_base64_methods() {
    let mut data = StdBase64Data::new(vec![1, 2, 3]);
    
    assert_eq!(data.len(), 3);
    assert!(!data.is_empty());
    
    data.as_bytes_mut().push(4);
    assert_eq!(data.len(), 4);
    
    let bytes = data.into_bytes();
    assert_eq!(bytes, vec![1, 2, 3, 4]);
}

// ============================================================================
// HexData tests
// ============================================================================

#[test]
fn test_hex_marshal_json() {
    let data = HexData::from(vec![0xde, 0xad, 0xbe, 0xef]);
    let json = serde_json::to_string(&data).unwrap();
    assert_eq!(json, r#""deadbeef""#);
}

#[test]
fn test_hex_unmarshal_json_valid() {
    let json = r#""deadbeef""#;
    let data: HexData = serde_json::from_str(json).unwrap();
    assert_eq!(data.as_bytes(), &[0xde, 0xad, 0xbe, 0xef]);
}

#[test]
fn test_hex_unmarshal_json_empty() {
    let json = r#""""#;
    let data: HexData = serde_json::from_str(json).unwrap();
    assert!(data.is_empty());
}

#[test]
fn test_hex_unmarshal_json_null() {
    let json = "null";
    let data: HexData = serde_json::from_str(json).unwrap();
    assert!(data.is_empty());
}

#[test]
fn test_hex_unmarshal_json_invalid_odd_length() {
    let json = r#""abc""#;
    let result: Result<HexData, _> = serde_json::from_str(json);
    assert!(result.is_err());
}

#[test]
fn test_hex_unmarshal_json_invalid_non_hex() {
    let json = r#""xyz123""#;
    let result: Result<HexData, _> = serde_json::from_str(json);
    assert!(result.is_err());
}

#[test]
fn test_hex_round_trip() {
    let original = HexData::from(vec![0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef]);
    let json = serde_json::to_string(&original).unwrap();
    let restored: HexData = serde_json::from_str(&json).unwrap();
    assert_eq!(original, restored);
}

#[test]
fn test_hex_string() {
    let data = HexData::from(vec![0xca, 0xfe]);
    assert_eq!(data.to_string(), "cafe");
}

#[test]
fn test_hex_encode_decode() {
    let data = HexData::from(vec![0xab, 0xcd]);
    let encoded = data.encode();
    let decoded = HexData::decode(&encoded).unwrap();
    assert_eq!(data, decoded);
}

#[test]
fn test_hex_from_conversions() {
    // From Vec<u8>
    let v = vec![0xaa, 0xbb];
    let d1 = HexData::from(v.clone());
    assert_eq!(d1.as_bytes(), &[0xaa, 0xbb]);

    // From &[u8]
    let d2 = HexData::from(v.as_slice());
    assert_eq!(d1, d2);

    // From array
    let d3 = HexData::from([0xaau8, 0xbb]);
    assert_eq!(d1, d3);

    // Into Vec<u8>
    let back: Vec<u8> = d1.into();
    assert_eq!(back, vec![0xaa, 0xbb]);
}

#[test]
fn test_hex_methods() {
    let mut data = HexData::new(vec![0x11, 0x22]);
    
    assert_eq!(data.len(), 2);
    assert!(!data.is_empty());
    
    data.as_bytes_mut().push(0x33);
    assert_eq!(data.len(), 3);
    
    let bytes = data.into_bytes();
    assert_eq!(bytes, vec![0x11, 0x22, 0x33]);
}

// ============================================================================
// Struct integration tests
// ============================================================================

#[test]
fn test_in_struct() {
    #[derive(serde::Serialize, serde::Deserialize, PartialEq, Debug)]
    struct Message {
        id: String,
        payload: StdBase64Data,
        hash: HexData,
    }

    let msg = Message {
        id: "test-123".to_string(),
        payload: StdBase64Data::from(b"hello".as_slice()),
        hash: HexData::from(vec![0xab, 0xcd]),
    };

    let json = serde_json::to_string(&msg).unwrap();
    let restored: Message = serde_json::from_str(&json).unwrap();

    assert_eq!(restored, msg);
}

#[test]
fn test_optional_in_struct() {
    #[derive(serde::Serialize, serde::Deserialize, PartialEq, Debug)]
    struct Packet {
        data: Option<StdBase64Data>,
        checksum: Option<HexData>,
    }

    // With values
    let pkt1 = Packet {
        data: Some(StdBase64Data::from(b"test".as_slice())),
        checksum: Some(HexData::from(vec![0xff])),
    };
    let json1 = serde_json::to_string(&pkt1).unwrap();
    let restored1: Packet = serde_json::from_str(&json1).unwrap();
    assert_eq!(restored1, pkt1);

    // Without values
    let pkt2 = Packet {
        data: None,
        checksum: None,
    };
    let json2 = serde_json::to_string(&pkt2).unwrap();
    let restored2: Packet = serde_json::from_str(&json2).unwrap();
    assert_eq!(restored2, pkt2);
}

// ============================================================================
// Edge cases
// ============================================================================

#[test]
fn test_base64_binary_data() {
    // Test with actual binary data (non-UTF8)
    let binary: Vec<u8> = (0..=255).collect();
    let data = StdBase64Data::from(binary.clone());
    
    let json = serde_json::to_string(&data).unwrap();
    let restored: StdBase64Data = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.as_bytes(), binary.as_slice());
}

#[test]
fn test_hex_all_values() {
    // Test all byte values
    let binary: Vec<u8> = (0..=255).collect();
    let data = HexData::from(binary.clone());
    
    let json = serde_json::to_string(&data).unwrap();
    let restored: HexData = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.as_bytes(), binary.as_slice());
}

#[test]
fn test_base64_large_data() {
    // Test with larger data
    let large: Vec<u8> = (0..10000).map(|i| (i % 256) as u8).collect();
    let data = StdBase64Data::from(large.clone());
    
    let json = serde_json::to_string(&data).unwrap();
    let restored: StdBase64Data = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.as_bytes(), large.as_slice());
}

#[test]
fn test_hex_uppercase() {
    // Hex decode should handle uppercase
    let json = r#""DEADBEEF""#;
    let data: HexData = serde_json::from_str(json).unwrap();
    assert_eq!(data.as_bytes(), &[0xde, 0xad, 0xbe, 0xef]);
}
