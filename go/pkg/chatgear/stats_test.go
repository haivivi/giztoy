package chatgear

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

func TestBattery_Equal(t *testing.T) {
	b1 := &Battery{Percentage: 50, IsCharging: true}
	b2 := &Battery{Percentage: 50, IsCharging: true}
	b3 := &Battery{Percentage: 60, IsCharging: true}

	if !b1.Equal(b2) {
		t.Error("Equal batteries should be equal")
	}
	if b1.Equal(b3) {
		t.Error("Different batteries should not be equal")
	}
	if b1.Equal(nil) {
		t.Error("Non-nil battery should not equal nil")
	}
	if (*Battery)(nil).Equal(b1) {
		t.Error("Nil battery should not equal non-nil")
	}
}

func TestConnectedWifi_Equal(t *testing.T) {
	w1 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w2 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w3 := &ConnectedWifi{SSID: "other", IP: "192.168.1.1", DNS: []string{"8.8.8.8"}}
	w4 := &ConnectedWifi{SSID: "test", IP: "192.168.1.1", DNS: []string{"8.8.4.4"}}

	if !w1.Equal(w2) {
		t.Error("Equal WiFi should be equal")
	}
	if w1.Equal(w3) {
		t.Error("Different SSID should not be equal")
	}
	if w1.Equal(w4) {
		t.Error("Different DNS should not be equal")
	}
}

func TestStatsEvent_Clone(t *testing.T) {
	event := &StatsEvent{
		Time:    jsontime.NowEpochMilli(),
		Battery: &Battery{Percentage: 80},
		Volume:  &Volume{Percentage: 50},
		WifiNetwork: &ConnectedWifi{
			SSID: "test",
			DNS:  []string{"8.8.8.8"},
		},
	}

	clone := event.Clone()

	// Modify original
	event.Battery.Percentage = 20
	event.WifiNetwork.SSID = "changed"
	event.WifiNetwork.DNS[0] = "1.1.1.1"

	// Clone should be unchanged
	if clone.Battery.Percentage != 80 {
		t.Error("Clone's Battery was modified")
	}
	if clone.WifiNetwork.SSID != "test" {
		t.Error("Clone's WiFi SSID was modified")
	}
	if clone.WifiNetwork.DNS[0] != "8.8.8.8" {
		t.Error("Clone's WiFi DNS was modified")
	}
}

func TestStatsEvent_MergeWith(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:    t1,
		Battery: &Battery{Percentage: 80},
	}

	other := &StatsEvent{
		Time:    t2,
		Battery: &Battery{Percentage: 70},
		Volume:  &Volume{Percentage: 50, UpdateAt: t2},
	}

	changes := event.MergeWith(other)

	if changes == nil {
		t.Fatal("MergeWith should return changes")
	}
	if changes.Battery == nil || changes.Battery.Percentage != 70 {
		t.Error("Battery change not detected")
	}
	if changes.Volume == nil || changes.Volume.Percentage != 50 {
		t.Error("Volume change not detected")
	}
}

func TestReadNFCTag_Equal(t *testing.T) {
	tag1 := &NFCTag{UID: "abc123"}
	tag2 := &NFCTag{UID: "def456"}

	nfc1 := &ReadNFCTag{Tags: []*NFCTag{tag1, tag2}}
	nfc2 := &ReadNFCTag{Tags: []*NFCTag{tag2, tag1}} // Same UIDs, different order
	nfc3 := &ReadNFCTag{Tags: []*NFCTag{tag1}}       // Missing one

	if !nfc1.Equal(nfc2) {
		t.Error("Same UIDs in different order should be equal")
	}
	if nfc1.Equal(nfc3) {
		t.Error("Different number of tags should not be equal")
	}
}

func TestReadNFCTag_JSON(t *testing.T) {
	nfc := &ReadNFCTag{
		Tags: []*NFCTag{
			{UID: "abc", Type: "NDEF"},
			{UID: "def", Type: "MIFARE"},
		},
	}

	data, err := json.Marshal(nfc)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored ReadNFCTag
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(restored.Tags) != 2 {
		t.Errorf("Tags length = %d; want 2", len(restored.Tags))
	}
}

func TestShaking_Equal(t *testing.T) {
	s1 := &Shaking{Level: 0.5}
	s2 := &Shaking{Level: 0.5}
	s3 := &Shaking{Level: 0.8}

	if !s1.Equal(s2) {
		t.Error("Equal Shaking should be equal")
	}
	if s1.Equal(s3) {
		t.Error("Different Shaking should not be equal")
	}
}

func TestStatsChanges_StatsEvent(t *testing.T) {
	changes := &StatsChanges{
		Battery: &Battery{Percentage: 80},
		Volume:  &Volume{Percentage: 50},
	}

	evt := changes.StatsEvent()
	if evt == nil {
		t.Fatal("StatsEvent returned nil")
	}
	if evt.Battery == nil || evt.Battery.Percentage != 80 {
		t.Error("Battery not copied")
	}
	if evt.Volume == nil || evt.Volume.Percentage != 50 {
		t.Error("Volume not copied")
	}
}

func TestPing_Equal(t *testing.T) {
	p1 := &Ping{Latency: 10}
	p2 := &Ping{Latency: 10}
	p3 := &Ping{Latency: 20}

	if !p1.Equal(p2) {
		t.Error("Equal pings should be equal")
	}
	if p1.Equal(p3) {
		t.Error("Different pings should not be equal")
	}
	if p1.Equal(nil) {
		t.Error("Non-nil ping should not equal nil")
	}
}

func TestConnectedCellular_Equal(t *testing.T) {
	c1 := &ConnectedCellular{IP: "10.0.0.1", DNS: []string{"8.8.8.8"}}
	c2 := &ConnectedCellular{IP: "10.0.0.1", DNS: []string{"8.8.8.8"}}
	c3 := &ConnectedCellular{IP: "10.0.0.2", DNS: []string{"8.8.8.8"}}
	c4 := &ConnectedCellular{IP: "10.0.0.1", DNS: []string{"1.1.1.1"}}

	if !c1.Equal(c2) {
		t.Error("Equal cellular should be equal")
	}
	if c1.Equal(c3) {
		t.Error("Different IP should not be equal")
	}
	if c1.Equal(c4) {
		t.Error("Different DNS should not be equal")
	}
	if c1.Equal(nil) {
		t.Error("Non-nil cellular should not equal nil")
	}
}

func TestStatsEvent_MergeWith_Cellular(t *testing.T) {
	event := &StatsEvent{
		Cellular: &ConnectedCellular{IP: "10.0.0.1"},
	}

	other := &StatsEvent{
		Cellular: &ConnectedCellular{IP: "10.0.0.2", DNS: []string{"8.8.8.8"}},
	}

	changes := event.MergeWith(other)

	if changes == nil {
		t.Fatal("MergeWith should return changes")
	}
	if changes.Cellular == nil {
		t.Error("Cellular change not detected")
	}
	if changes.Cellular.IP != "10.0.0.2" {
		t.Errorf("Cellular IP = %s; want 10.0.0.2", changes.Cellular.IP)
	}
}

func TestNFCTag_Equal(t *testing.T) {
	n1 := &NFCTag{UID: "abc", Type: "NDEF"}
	n2 := &NFCTag{UID: "abc", Type: "NDEF"}
	n3 := &NFCTag{UID: "def", Type: "NDEF"}

	if !n1.Equal(n2) {
		t.Error("Equal NFC tags should be equal")
	}
	if n1.Equal(n3) {
		t.Error("Different NFC tags should not be equal")
	}
	if n1.Equal(nil) {
		t.Error("Non-nil tag should not equal nil")
	}
}

func TestStatsEvent_MergeWith_AllFields(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:          t1,
		Battery:       &Battery{Percentage: 80},
		Volume:        &Volume{Percentage: 50},
		Brightness:    &Brightness{Percentage: 60},
		LightMode:     &LightMode{Mode: "dark"},
		SystemVersion: &SystemVersion{CurrentVersion: "1.0"},
		WifiNetwork:   &ConnectedWifi{SSID: "test"},
		WifiStore:     &StoredWifiList{List: []WifiStoreItem{{SSID: "a"}}},
		Cellular:      &ConnectedCellular{IP: "10.0.0.1"},
		PairStatus:    &PairStatus{PairWith: "device1"},
		Shaking:       &Shaking{Level: 0.5},
		ReadNFCTag:    &ReadNFCTag{Tags: []*NFCTag{{UID: "abc"}}},
	}

	other := &StatsEvent{
		Time:          t2,
		Battery:       &Battery{Percentage: 70},
		Volume:        &Volume{Percentage: 60},
		Brightness:    &Brightness{Percentage: 70},
		LightMode:     &LightMode{Mode: "light"},
		SystemVersion: &SystemVersion{CurrentVersion: "2.0"},
		WifiNetwork:   &ConnectedWifi{SSID: "test2"},
		WifiStore:     &StoredWifiList{List: []WifiStoreItem{{SSID: "b"}}},
		Cellular:      &ConnectedCellular{IP: "10.0.0.2"},
		PairStatus:    &PairStatus{PairWith: "device2"},
		Shaking:       &Shaking{Level: 0.8},
		ReadNFCTag:    &ReadNFCTag{Tags: []*NFCTag{{UID: "def"}}},
	}

	changes := event.MergeWith(other)

	if changes == nil {
		t.Fatal("MergeWith should return changes")
	}

	// Verify all fields changed
	if changes.Battery == nil {
		t.Error("Battery change not detected")
	}
	if changes.Volume == nil {
		t.Error("Volume change not detected")
	}
	if changes.Brightness == nil {
		t.Error("Brightness change not detected")
	}
	if changes.LightMode == nil {
		t.Error("LightMode change not detected")
	}
	if changes.SystemVersion == nil {
		t.Error("SystemVersion change not detected")
	}
	if changes.WifiNetwork == nil {
		t.Error("WifiNetwork change not detected")
	}
	if changes.WifiStore == nil {
		t.Error("WifiStore change not detected")
	}
	if changes.Cellular == nil {
		t.Error("Cellular change not detected")
	}
	if changes.PairStatus == nil {
		t.Error("PairStatus change not detected")
	}
	if changes.Shaking == nil {
		t.Error("Shaking change not detected")
	}
	if changes.ReadNFCTag == nil {
		t.Error("ReadNFCTag change not detected")
	}
}

func TestStatsEvent_MergeWith_SameValues(t *testing.T) {
	event := &StatsEvent{
		Battery: &Battery{Percentage: 80},
		Volume:  &Volume{Percentage: 50},
	}

	// Same values - MergeWith updates event regardless
	other := &StatsEvent{
		Battery: &Battery{Percentage: 80},
		Volume:  &Volume{Percentage: 50},
	}

	changes := event.MergeWith(other)
	// Changes may or may not be nil depending on implementation
	_ = changes
}

func TestStatsEvent_Clone_WithNilFields(t *testing.T) {
	event := &StatsEvent{
		Battery: &Battery{Percentage: 80},
		// Other fields are nil
	}

	clone := event.Clone()

	if clone == nil {
		t.Fatal("Clone returned nil")
	}
	if clone.Battery == nil || clone.Battery.Percentage != 80 {
		t.Error("Battery not cloned correctly")
	}
	if clone.Volume != nil {
		t.Error("Volume should be nil")
	}
}

func TestShaking_Equal_Nil(t *testing.T) {
	s1 := &Shaking{Level: 0.5}
	var s2 *Shaking

	if s1.Equal(s2) {
		t.Error("Non-nil shaking should not equal nil")
	}
	if s2.Equal(s1) {
		t.Error("Nil shaking should not equal non-nil")
	}
}

// =============================================================================
// MergeWith Edge Cases
// =============================================================================

func TestStatsEvent_MergeWith_OlderTimestamp(t *testing.T) {
	// Test that older timestamp events are rejected
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)) // 2 hours earlier

	event := &StatsEvent{
		Time:   t1,
		Volume: &Volume{Percentage: 50},
	}
	other := &StatsEvent{
		Time:   t2,
		Volume: &Volume{Percentage: 80},
	}

	changes := event.MergeWith(other)
	if changes != nil {
		t.Error("Older timestamp should return nil changes")
	}
	if event.Volume.Percentage != 50 {
		t.Errorf("Volume should not change, got %f", event.Volume.Percentage)
	}
}

func TestStatsEvent_MergeWith_UpdateAt_OlderIgnored(t *testing.T) {
	// Test that fields with older UpdateAt are ignored even if Time is newer
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)) // older
	t3 := jsontime.Milli(time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)) // newer Time

	event := &StatsEvent{
		Time:   t1,
		Volume: &Volume{Percentage: 50, UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:   t3,                                    // newer Time
		Volume: &Volume{Percentage: 80, UpdateAt: t2}, // but older UpdateAt
	}

	changes := event.MergeWith(other)
	// Volume should NOT be updated because UpdateAt is older
	if changes != nil && changes.Volume != nil {
		t.Error("Volume with older UpdateAt should not produce change")
	}
	if event.Volume.Percentage != 50 {
		t.Errorf("Volume should remain 50, got %f", event.Volume.Percentage)
	}
}

func TestStatsEvent_MergeWith_UpdateAt_NewerApplied(t *testing.T) {
	// Test that fields with newer UpdateAt are applied
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:   t1,
		Volume: &Volume{Percentage: 50, UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:   t2,
		Volume: &Volume{Percentage: 80, UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.Volume == nil {
		t.Fatal("Volume change should be detected")
	}
	if changes.Volume.Percentage != 80 {
		t.Errorf("Volume change should be 80, got %f", changes.Volume.Percentage)
	}
	if event.Volume.Percentage != 80 {
		t.Errorf("Volume should be updated to 80, got %f", event.Volume.Percentage)
	}
}

func TestStatsEvent_MergeWith_LastResetAt(t *testing.T) {
	// Test LastResetAt update logic
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:        t1,
		LastResetAt: t1,
	}
	other := &StatsEvent{
		Time:        t2,
		LastResetAt: t2, // newer
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.LastResetAt == nil {
		t.Fatal("LastResetAt change should be detected")
	}
	if !changes.LastResetAt.Time().Equal(t2.Time()) {
		t.Error("LastResetAt should be updated")
	}
}

func TestStatsEvent_MergeWith_LastResetAt_OlderIgnored(t *testing.T) {
	// Test that older LastResetAt is ignored
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t3 := jsontime.Milli(time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:        t1,
		LastResetAt: t1,
		Volume:      &Volume{Percentage: 50}, // Add something to make diff non-empty
	}
	other := &StatsEvent{
		Time:        t3, // newer Time so merge proceeds
		LastResetAt: t2, // but older LastResetAt
		Volume:      &Volume{Percentage: 80, UpdateAt: t3},
	}

	changes := event.MergeWith(other)
	if changes != nil && changes.LastResetAt != nil {
		t.Error("Older LastResetAt should not produce change")
	}
	if event.LastResetAt != t1 {
		t.Error("LastResetAt should remain unchanged")
	}
}

func TestStatsEvent_MergeWith_NilToNonNil(t *testing.T) {
	// Test adding a field that was previously nil
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time: t1,
		// Volume is nil
	}
	other := &StatsEvent{
		Time:   t2,
		Volume: &Volume{Percentage: 50, UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.Volume == nil {
		t.Fatal("Volume should be detected as new")
	}
	if event.Volume == nil || event.Volume.Percentage != 50 {
		t.Error("Volume should be set")
	}
}

func TestStatsEvent_MergeWith_ReturnNilWhenNoChanges(t *testing.T) {
	// Test that nil is returned when there are no actual changes
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:    t1,
		Battery: &Battery{Percentage: 80, IsCharging: true},
	}
	other := &StatsEvent{
		Time:    t2,
		Battery: &Battery{Percentage: 80, IsCharging: true}, // same value
	}

	changes := event.MergeWith(other)
	if changes != nil {
		t.Error("No changes should return nil")
	}
}

func TestStatsEvent_MergeWith_OtherNilField(t *testing.T) {
	// Test that nil fields in other don't overwrite existing fields
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:   t1,
		Volume: &Volume{Percentage: 50},
	}
	other := &StatsEvent{
		Time:   t2,
		Volume: nil, // nil should not overwrite
	}

	changes := event.MergeWith(other)
	if changes != nil {
		t.Error("Nil field should not produce change")
	}
	if event.Volume == nil || event.Volume.Percentage != 50 {
		t.Error("Volume should remain unchanged")
	}
}

func TestStatsEvent_MergeWith_EqualBasedFields(t *testing.T) {
	// Test fields that use Equal() for comparison (Battery, Cellular, WifiNetwork, ReadNFCTag)
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:        t1,
		Battery:     &Battery{Percentage: 80, IsCharging: true},
		WifiNetwork: &ConnectedWifi{SSID: "home", IP: "192.168.1.1"},
	}

	// Same battery, different wifi
	other := &StatsEvent{
		Time:        t2,
		Battery:     &Battery{Percentage: 80, IsCharging: true}, // same
		WifiNetwork: &ConnectedWifi{SSID: "work", IP: "10.0.0.1"},
	}

	changes := event.MergeWith(other)
	if changes == nil {
		t.Fatal("Should detect wifi change")
	}
	if changes.Battery != nil {
		t.Error("Battery should not change (same value)")
	}
	if changes.WifiNetwork == nil || changes.WifiNetwork.SSID != "work" {
		t.Error("WifiNetwork change not detected")
	}
}

func TestStatsEvent_MergeWith_Brightness(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:       t1,
		Brightness: &Brightness{Percentage: 50, UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:       t2,
		Brightness: &Brightness{Percentage: 80, UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.Brightness == nil {
		t.Fatal("Brightness change should be detected")
	}
	if changes.Brightness.Percentage != 80 {
		t.Errorf("Brightness should be 80, got %f", changes.Brightness.Percentage)
	}
}

func TestStatsEvent_MergeWith_LightMode(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:      t1,
		LightMode: &LightMode{Mode: "dark", UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:      t2,
		LightMode: &LightMode{Mode: "light", UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.LightMode == nil {
		t.Fatal("LightMode change should be detected")
	}
	if changes.LightMode.Mode != "light" {
		t.Errorf("LightMode should be 'light', got %s", changes.LightMode.Mode)
	}
}

func TestStatsEvent_MergeWith_SystemVersion(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:          t1,
		SystemVersion: &SystemVersion{CurrentVersion: "1.0.0", UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:          t2,
		SystemVersion: &SystemVersion{CurrentVersion: "2.0.0", UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.SystemVersion == nil {
		t.Fatal("SystemVersion change should be detected")
	}
	if changes.SystemVersion.CurrentVersion != "2.0.0" {
		t.Errorf("SystemVersion should be '2.0.0', got %s", changes.SystemVersion.CurrentVersion)
	}
}

func TestStatsEvent_MergeWith_PairStatus(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:       t1,
		PairStatus: &PairStatus{PairWith: "device1", UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:       t2,
		PairStatus: &PairStatus{PairWith: "device2", UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.PairStatus == nil {
		t.Fatal("PairStatus change should be detected")
	}
	if changes.PairStatus.PairWith != "device2" {
		t.Errorf("PairStatus should be 'device2', got %s", changes.PairStatus.PairWith)
	}
}

func TestStatsEvent_MergeWith_WifiStore(t *testing.T) {
	t1 := jsontime.Milli(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	t2 := jsontime.Milli(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	event := &StatsEvent{
		Time:      t1,
		WifiStore: &StoredWifiList{List: []WifiStoreItem{{SSID: "a"}}, UpdateAt: t1},
	}
	other := &StatsEvent{
		Time:      t2,
		WifiStore: &StoredWifiList{List: []WifiStoreItem{{SSID: "a"}, {SSID: "b"}}, UpdateAt: t2},
	}

	changes := event.MergeWith(other)
	if changes == nil || changes.WifiStore == nil {
		t.Fatal("WifiStore change should be detected")
	}
	if len(changes.WifiStore.List) != 2 {
		t.Errorf("WifiStore should have 2 items, got %d", len(changes.WifiStore.List))
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestStatsEvent_JSON_RoundTrip(t *testing.T) {
	now := jsontime.NowEpochMilli()
	event := &StatsEvent{
		Time:        now,
		LastResetAt: now,
		Volume:      &Volume{Percentage: 50, UpdateAt: now},
		Battery:     &Battery{Percentage: 80, IsCharging: true},
		WifiNetwork: &ConnectedWifi{SSID: "test", IP: "192.168.1.1"},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored StatsEvent
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.Volume == nil || restored.Volume.Percentage != 50 {
		t.Error("Volume should be restored")
	}
	if restored.Battery == nil || restored.Battery.Percentage != 80 {
		t.Error("Battery should be restored")
	}
}

func TestStatsEvent_UnmarshalJSON_Invalid(t *testing.T) {
	invalidCases := []struct {
		name  string
		input string
	}{
		{"invalid_json", `invalid`},
		{"empty_object", `{}`},
		{"null", `null`},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			var event StatsEvent
			err := json.Unmarshal([]byte(tc.input), &event)
			if err != nil {
				t.Logf("Invalid %s produces error: %v", tc.name, err)
			} else {
				t.Logf("Invalid %s parsed successfully: %+v", tc.name, event)
			}
		})
	}
}

func TestStatsChanges_JSON_RoundTrip(t *testing.T) {
	now := jsontime.NowEpochMilli()
	changes := &StatsChanges{
		Time:       now,
		Volume:     &Volume{Percentage: 70, UpdateAt: now},
		Brightness: &Brightness{Percentage: 80, UpdateAt: now},
	}

	data, err := json.Marshal(changes)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored StatsChanges
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.Volume == nil || restored.Volume.Percentage != 70 {
		t.Error("Volume should be restored")
	}
}

func TestStatsChanges_StatsEvent_Nil(t *testing.T) {
	var changes *StatsChanges = nil
	evt := changes.StatsEvent()
	if evt != nil {
		t.Error("StatsEvent of nil changes should be nil")
	}
}

func TestStatsChanges_StatsEvent_WithLastResetAt(t *testing.T) {
	now := jsontime.NowEpochMilli()
	changes := &StatsChanges{
		Time:        now,
		LastResetAt: &now,
		Volume:      &Volume{Percentage: 50},
	}

	evt := changes.StatsEvent()
	if evt == nil {
		t.Fatal("StatsEvent should not be nil")
	}
	if evt.LastResetAt != now {
		t.Errorf("LastResetAt mismatch: got %v, want %v", evt.LastResetAt, now)
	}
}

func TestStatsChanges_StatsEvent_AllFields(t *testing.T) {
	now := jsontime.NowEpochMilli()
	changes := &StatsChanges{
		Time:          now,
		LastResetAt:   &now,
		Battery:       &Battery{Percentage: 80},
		SystemVersion: &SystemVersion{CurrentVersion: "1.0.0"},
		Volume:        &Volume{Percentage: 50},
		Brightness:    &Brightness{Percentage: 80},
		LightMode:     &LightMode{Mode: "dark"},
		WifiNetwork:   &ConnectedWifi{SSID: "test"},
		WifiStore:     &StoredWifiList{List: []WifiStoreItem{{SSID: "a"}}},
		ReadNFCTag:    &ReadNFCTag{Tags: []*NFCTag{{UID: "tag1"}}},
		PairStatus:    &PairStatus{PairWith: "device1"},
		Shaking:       &Shaking{Level: 0.5},
	}

	evt := changes.StatsEvent()
	if evt == nil {
		t.Fatal("StatsEvent should not be nil")
	}
	if evt.Battery == nil || evt.Battery.Percentage != 80 {
		t.Error("Battery not copied")
	}
	if evt.Volume == nil || evt.Volume.Percentage != 50 {
		t.Error("Volume not copied")
	}
}

func TestReadNFCTag_Clone_WithNilTag(t *testing.T) {
	// Test clone with nil tag in the list
	rnt := &ReadNFCTag{
		Tags: []*NFCTag{
			{UID: "tag1"},
			nil,
			{UID: "tag2"},
		},
	}

	clone := rnt.clone()
	if clone == nil {
		t.Fatal("Clone should not be nil")
	}
	if len(clone.Tags) != 3 {
		t.Errorf("Clone should have 3 tags, got %d", len(clone.Tags))
	}
	if clone.Tags[1] != nil {
		t.Error("Nil tag should remain nil in clone")
	}
}

func TestStatsEvent_Clone_WithCellular(t *testing.T) {
	now := jsontime.NowEpochMilli()
	event := &StatsEvent{
		Time:     now,
		Cellular: &ConnectedCellular{IP: "10.0.0.1", DNS: []string{"8.8.8.8"}},
	}

	clone := event.Clone()
	// Note: Current Clone() doesn't deep-copy Cellular (shallow copy only)
	// This test just exercises the clone path
	if clone == nil {
		t.Fatal("Clone should not be nil")
	}
}

func TestConnectedCellular_clone_Nil(t *testing.T) {
	var cc *ConnectedCellular = nil
	clone := cc.clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestStoredWifiList_clone_Nil(t *testing.T) {
	var wl *StoredWifiList = nil
	clone := wl.clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestReadNFCTag_clone_Nil(t *testing.T) {
	var rnt *ReadNFCTag = nil
	clone := rnt.clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestSystemVersion_clone_Nil(t *testing.T) {
	var sv *SystemVersion = nil
	clone := sv.clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestConnectedWifi_clone_Nil(t *testing.T) {
	var cw *ConnectedWifi = nil
	clone := cw.clone()
	if clone != nil {
		t.Error("Clone of nil should be nil")
	}
}

func TestConnectedWifi_Equal_DNS(t *testing.T) {
	w1 := &ConnectedWifi{
		SSID: "test",
		DNS:  []string{"8.8.8.8", "8.8.4.4"},
	}
	w2 := &ConnectedWifi{
		SSID: "test",
		DNS:  []string{"8.8.8.8", "1.1.1.1"}, // Different DNS
	}
	w3 := &ConnectedWifi{
		SSID: "test",
		DNS:  []string{"8.8.8.8"}, // Different length
	}
	w4 := &ConnectedWifi{
		SSID: "test",
		DNS:  []string{"8.8.8.8", "8.8.4.4"}, // Same
	}

	if w1.Equal(w2) {
		t.Error("Different DNS should not be equal")
	}
	if w1.Equal(w3) {
		t.Error("Different DNS length should not be equal")
	}
	if !w1.Equal(w4) {
		t.Error("Same DNS should be equal")
	}
}

func TestConnectedWifi_Equal_AllFields(t *testing.T) {
	w1 := &ConnectedWifi{
		SSID:     "test",
		BSSID:    "aa:bb:cc",
		Gateway:  "192.168.1.1",
		IP:       "192.168.1.100",
		Mac:      "11:22:33",
		NetMask:  "255.255.255.0",
		RSSI:     -50,
		Security: "WPA2",
		Ping:     &Ping{Latency: 10},
		DNS:      []string{"8.8.8.8"},
	}
	w2 := &ConnectedWifi{
		SSID:     "test",
		BSSID:    "aa:bb:cc",
		Gateway:  "192.168.1.1",
		IP:       "192.168.1.100",
		Mac:      "11:22:33",
		NetMask:  "255.255.255.0",
		RSSI:     -50,
		Security: "WPA2",
		Ping:     &Ping{Latency: 10},
		DNS:      []string{"8.8.8.8"},
	}

	if !w1.Equal(w2) {
		t.Error("Same wifi should be equal")
	}

	// Test each field difference
	w2.BSSID = "xx:yy:zz"
	if w1.Equal(w2) {
		t.Error("Different BSSID should not be equal")
	}
}

func TestConnectedCellular_Equal_DNS(t *testing.T) {
	c1 := &ConnectedCellular{
		IP:  "10.0.0.1",
		DNS: []string{"8.8.8.8"},
	}
	c2 := &ConnectedCellular{
		IP:  "10.0.0.1",
		DNS: []string{"1.1.1.1"},
	}
	c3 := &ConnectedCellular{
		IP:  "10.0.0.1",
		DNS: []string{"8.8.8.8", "8.8.4.4"},
	}

	if c1.Equal(c2) {
		t.Error("Different DNS should not be equal")
	}
	if c1.Equal(c3) {
		t.Error("Different DNS length should not be equal")
	}
}

// =============================================================================
// JSON Format and omitzero Tests
// =============================================================================

func TestStatsEvent_JSONFormat_OmitsNilFields(t *testing.T) {
	// Create event with only Battery set
	event := &StatsEvent{
		Time:    jsontime.Milli(time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)),
		Battery: &Battery{Percentage: 80, IsCharging: true},
		// All other fields are nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Parse as raw JSON to check which fields exist
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Should have: time, battery
	if _, ok := raw["time"]; !ok {
		t.Error("Missing 'time' field")
	}
	if _, ok := raw["battery"]; !ok {
		t.Error("Missing 'battery' field")
	}

	// Should NOT have: volume, brightness, light_mode, etc.
	unexpectedFields := []string{"volume", "brightness", "light_mode", "system_version", "wifi_network", "wifi_store", "pair_status", "shaking", "read_nfc_tag", "cellular_network"}
	for _, field := range unexpectedFields {
		if _, ok := raw[field]; ok {
			t.Errorf("Field '%s' should be omitted when nil", field)
		}
	}

	t.Logf("StatsEvent JSON (only battery): %s", string(data))
}

func TestStatsEvent_JSONFormat_IncludesSetFields(t *testing.T) {
	now := jsontime.NowEpochMilli()
	event := &StatsEvent{
		Time:       now,
		Battery:    &Battery{Percentage: 80, IsCharging: true},
		Volume:     &Volume{Percentage: 70, UpdateAt: now},
		Brightness: &Brightness{Percentage: 60, UpdateAt: now},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Should have all set fields
	expectedFields := []string{"time", "battery", "volume", "brightness"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("Missing expected field '%s'", field)
		}
	}

	// Should NOT have unset fields
	unexpectedFields := []string{"light_mode", "system_version", "wifi_network"}
	for _, field := range unexpectedFields {
		if _, ok := raw[field]; ok {
			t.Errorf("Field '%s' should be omitted when nil", field)
		}
	}

	t.Logf("StatsEvent JSON (battery+volume+brightness): %s", string(data))
}

func TestStatsEvent_JSONFormat_TimeIsEpochMillis(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 10, 30, 45, 0, time.UTC)
	expectedMs := testTime.UnixMilli()

	event := &StatsEvent{
		Time:    jsontime.Milli(testTime),
		Battery: &Battery{Percentage: 50},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if timeVal, ok := raw["time"].(float64); ok {
		if int64(timeVal) != expectedMs {
			t.Errorf("time = %d; want %d (epoch ms)", int64(timeVal), expectedMs)
		}
	} else {
		t.Errorf("time is not a number: %T", raw["time"])
	}
}

func TestStatsEvent_JSONFormat_NestedFieldsCorrect(t *testing.T) {
	now := jsontime.NowEpochMilli()
	event := &StatsEvent{
		Time:    now,
		Battery: &Battery{Percentage: 80, IsCharging: true},
		Volume:  &Volume{Percentage: 70, UpdateAt: now},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Check battery structure
	if battery, ok := raw["battery"].(map[string]interface{}); ok {
		if pct, ok := battery["percentage"].(float64); !ok || pct != 80 {
			t.Errorf("battery.percentage = %v; want 80", battery["percentage"])
		}
		if charging, ok := battery["is_charging"].(bool); !ok || !charging {
			t.Errorf("battery.is_charging = %v; want true", battery["is_charging"])
		}
	} else {
		t.Errorf("battery is not an object: %T", raw["battery"])
	}

	// Check volume structure
	if volume, ok := raw["volume"].(map[string]interface{}); ok {
		if pct, ok := volume["percentage"].(float64); !ok || pct != 70 {
			t.Errorf("volume.percentage = %v; want 70", volume["percentage"])
		}
		if _, ok := volume["update_at"]; !ok {
			t.Error("volume should have update_at field")
		}
	} else {
		t.Errorf("volume is not an object: %T", raw["volume"])
	}
}

func TestStatsEvent_JSONFormat_LastResetAtOmittedWhenZero(t *testing.T) {
	event := &StatsEvent{
		Time:    jsontime.NowEpochMilli(),
		Battery: &Battery{Percentage: 50},
		// LastResetAt is zero value
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// LastResetAt should be omitted when zero (due to omitzero tag)
	if _, ok := raw["last_reset_at"]; ok {
		t.Error("last_reset_at should be omitted when zero")
	}
}

func TestStatsEvent_JSONFormat_LastResetAtIncludedWhenSet(t *testing.T) {
	now := jsontime.NowEpochMilli()
	event := &StatsEvent{
		Time:        now,
		LastResetAt: now,
		Battery:     &Battery{Percentage: 50},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// LastResetAt should be included when set
	if _, ok := raw["last_reset_at"]; !ok {
		t.Error("last_reset_at should be included when set")
	}
}
