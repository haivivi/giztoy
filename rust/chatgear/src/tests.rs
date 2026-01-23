//! Integration tests for chatgear.

use super::*;

#[test]
fn test_gear_state_roundtrip() {
    let event = GearStateEvent::new(GearState::Recording);
    let json = serde_json::to_string(&event).unwrap();
    let restored: GearStateEvent = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.state, GearState::Recording);
    assert_eq!(restored.version, 1);
}

#[test]
fn test_command_event_roundtrip() {
    let cmd = SetVolume::new(75);
    let event = SessionCommandEvent::new(&cmd);
    
    let json = serde_json::to_string(&event).unwrap();
    let restored: SessionCommandEvent = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.cmd_type, "set_volume");
    
    let parsed: SetVolume = restored.parse_payload().unwrap();
    assert_eq!(parsed.0, 75);
}

#[test]
fn test_stats_event_roundtrip() {
    let mut event = GearStatsEvent::new();
    event.battery = Some(Battery {
        percentage: 85.0,
        is_charging: true,
        voltage: 4.2,
        ..Default::default()
    });
    event.volume = Some(Volume {
        percentage: 50.0,
        update_at: giztoy_jsontime::Milli::now(),
    });
    
    let json = serde_json::to_string(&event).unwrap();
    let restored: GearStatsEvent = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.battery.as_ref().unwrap().percentage, 85.0);
    assert_eq!(restored.volume.as_ref().unwrap().percentage, 50.0);
}

#[test]
fn test_all_commands() {
    // Test all command types serialize correctly
    let commands: Vec<Box<dyn SessionCommand>> = vec![
        Box::new(Streaming::new(true)),
        Box::new(Reset::new()),
        Box::new(Raise::call()),
        Box::new(Halt::sleep()),
        Box::new(SetVolume::new(50)),
        Box::new(SetBrightness::new(80)),
        Box::new(SetLightMode::new("auto")),
        Box::new(SetWifi::new("SSID", "WPA2", "password")),
        Box::new(DeleteWifi::new("SSID")),
        Box::new(OTA::default()),
    ];
    
    let expected_types = [
        "streaming", "reset", "raise", "halt",
        "set_volume", "set_brightness", "set_light_mode",
        "set_wifi", "delete_wifi", "ota_upgrade",
    ];
    
    for (cmd, expected) in commands.iter().zip(expected_types.iter()) {
        assert_eq!(cmd.command_type(), *expected);
    }
}

#[test]
fn test_state_transitions() {
    // Test state machine logic
    assert!(GearState::Ready.can_record());
    assert!(GearState::Streaming.can_record());
    assert!(!GearState::Recording.can_record());
    assert!(!GearState::Sleeping.can_record());
    
    assert!(GearState::Recording.is_active());
    assert!(GearState::WaitingForResponse.is_active());
    assert!(GearState::Streaming.is_active());
    assert!(GearState::Calling.is_active());
    assert!(!GearState::Ready.is_active());
    assert!(!GearState::Sleeping.is_active());
}

#[test]
fn test_stats_merge_preserves_newer() {
    let mut event1 = GearStatsEvent::new();
    event1.battery = Some(Battery {
        percentage: 50.0,
        ..Default::default()
    });
    event1.volume = Some(Volume {
        percentage: 30.0,
        update_at: giztoy_jsontime::Milli::now(),
    });
    
    // Wait to ensure time difference
    std::thread::sleep(std::time::Duration::from_millis(2));
    
    let mut event2 = GearStatsEvent::new();
    event2.battery = Some(Battery {
        percentage: 75.0,
        ..Default::default()
    });
    // event2 doesn't have volume, so event1's volume should be preserved
    
    let changes = event1.merge_with(&event2);
    assert!(changes.is_some());
    
    // Battery should be updated
    assert_eq!(event1.battery.as_ref().unwrap().percentage, 75.0);
    
    // Volume should still be there (not overwritten by None)
    assert!(event1.volume.is_some());
}

#[test]
fn test_wifi_info() {
    let wifi = ConnectedWifi {
        ssid: "HomeNetwork".to_string(),
        bssid: "AA:BB:CC:DD:EE:FF".to_string(),
        ip: "192.168.1.100".to_string(),
        gateway: "192.168.1.1".to_string(),
        net_mask: "255.255.255.0".to_string(),
        rssi: -45.0,
        security: "WPA2".to_string(),
        dns: vec!["8.8.8.8".to_string(), "8.8.4.4".to_string()],
        ..Default::default()
    };
    
    let json = serde_json::to_string(&wifi).unwrap();
    let restored: ConnectedWifi = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.ssid, "HomeNetwork");
    assert_eq!(restored.dns.len(), 2);
}

#[test]
fn test_nfc_tag_data() {
    let tag = NFCTag {
        uid: "04:AB:CD:EF:12:34:56".to_string(),
        tag_type: "NTAG215".to_string(),
        raw_data: giztoy_encoding::StdBase64Data::from(vec![0x01, 0x02, 0x03, 0x04]),
        data_format: "NDEF".to_string(),
        rssi: -30.0,
        update_at: giztoy_jsontime::Milli::now(),
    };
    
    let json = serde_json::to_string(&tag).unwrap();
    let restored: NFCTag = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.uid, "04:AB:CD:EF:12:34:56");
    assert_eq!(restored.raw_data.len(), 4);
}

#[test]
fn test_ota_command() {
    let ota = OTA {
        version: "2.0.0".to_string(),
        image_url: "https://example.com/firmware.bin".to_string(),
        image_md5: "abc123".to_string(),
        components: vec![
            ComponentOTA {
                name: "bootloader".to_string(),
                version: "1.5.0".to_string(),
                image_url: "https://example.com/bootloader.bin".to_string(),
                ..Default::default()
            },
        ],
        ..Default::default()
    };
    
    let json = serde_json::to_string(&ota).unwrap();
    let restored: OTA = serde_json::from_str(&json).unwrap();
    
    assert_eq!(restored.version, "2.0.0");
    assert_eq!(restored.components.len(), 1);
    assert_eq!(restored.components[0].name, "bootloader");
}
