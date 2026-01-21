//! Common types for MQTT.

/// MQTT Quality of Service level.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
#[repr(u8)]
pub enum QoS {
    /// At most once delivery (fire and forget).
    #[default]
    AtMostOnce = 0,
    /// At least once delivery (acknowledged delivery).
    AtLeastOnce = 1,
    /// Exactly once delivery (assured delivery).
    ExactlyOnce = 2,
}

impl From<QoS> for rumqttc::QoS {
    fn from(qos: QoS) -> Self {
        match qos {
            QoS::AtMostOnce => rumqttc::QoS::AtMostOnce,
            QoS::AtLeastOnce => rumqttc::QoS::AtLeastOnce,
            QoS::ExactlyOnce => rumqttc::QoS::ExactlyOnce,
        }
    }
}

impl From<rumqttc::QoS> for QoS {
    fn from(qos: rumqttc::QoS) -> Self {
        match qos {
            rumqttc::QoS::AtMostOnce => QoS::AtMostOnce,
            rumqttc::QoS::AtLeastOnce => QoS::AtLeastOnce,
            rumqttc::QoS::ExactlyOnce => QoS::ExactlyOnce,
        }
    }
}

impl From<u8> for QoS {
    fn from(v: u8) -> Self {
        match v {
            0 => QoS::AtMostOnce,
            1 => QoS::AtLeastOnce,
            2 => QoS::ExactlyOnce,
            _ => QoS::AtMostOnce,
        }
    }
}
