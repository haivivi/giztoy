//! Additional tests for the MQTT library.
//!
//! Note: Integration tests that require a running broker are in the examples.
//! These tests focus on unit testing individual components.

#[cfg(test)]
mod additional_tests {
    use crate::{QoS, ServeMux};
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::sync::Arc;

    #[test]
    fn test_serve_mux_multiple_handlers() {
        let mux = ServeMux::new();
        let counter1 = Arc::new(AtomicUsize::new(0));
        let counter2 = Arc::new(AtomicUsize::new(0));

        let c1 = counter1.clone();
        mux.handle_func("test/+/data", move |_msg| {
            c1.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
        .unwrap();

        let c2 = counter2.clone();
        mux.handle_func("test/device/data", move |_msg| {
            c2.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
        .unwrap();

        // test/device/data should match exact pattern first
        let msg = crate::serve_mux::Message::new("test/device/data", "payload");
        mux.handle_message(&msg).unwrap();

        // Exact match takes priority
        assert_eq!(counter2.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn test_qos_from_u8() {
        assert_eq!(QoS::from(0u8), QoS::AtMostOnce);
        assert_eq!(QoS::from(1u8), QoS::AtLeastOnce);
        assert_eq!(QoS::from(2u8), QoS::ExactlyOnce);
        assert_eq!(QoS::from(255u8), QoS::AtMostOnce); // Invalid defaults to 0
    }

    #[test]
    fn test_message_payload_str() {
        let msg = crate::serve_mux::Message::new("topic", "hello");
        assert_eq!(msg.payload_str(), Some("hello"));

        // Binary payload
        let msg = crate::serve_mux::Message {
            topic: "topic".to_string(),
            payload: bytes::Bytes::from(vec![0xFF, 0xFE]),
            qos: 0,
            retain: false,
            packet_id: None,
            user_properties: Vec::new(),
            client_id: None,
        };
        assert!(msg.payload_str().is_none());
    }

    #[test]
    fn test_serve_mux_debug() {
        let mux = ServeMux::new();
        mux.handle_func("test/+/data", |_| Ok(())).unwrap();
        mux.handle_func("device/#", |_| Ok(())).unwrap();

        let debug_str = format!("{:?}", mux);
        assert!(debug_str.contains("ServeMux"));
    }

    #[test]
    fn test_error_display() {
        let err = crate::Error::ServerClosed;
        assert_eq!(format!("{}", err), "mqtt: server closed");

        let err = crate::Error::NoHandlerFound("test/topic".to_string());
        assert!(format!("{}", err).contains("test/topic"));
    }
}
