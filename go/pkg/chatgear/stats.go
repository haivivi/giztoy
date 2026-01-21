package chatgear

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/haivivi/giztoy/pkg/encoding"
	"github.com/haivivi/giztoy/pkg/jsontime"
)

// GearStatsChanges represents changes to device statistics.
type GearStatsChanges struct {
	Time jsontime.Milli

	LastResetAt   *jsontime.Milli
	Battery       *Battery
	SystemVersion *SystemVersion
	Volume        *Volume
	Brightness    *Brightness
	LightMode     *LightMode
	Cellular      *ConnectedCellular
	WifiNetwork   *ConnectedWifi
	WifiStore     *StoredWifiList
	ReadNFCTag    *ReadNFCTag
	PairStatus    *PairStatus
	Shaking       *Shaking
}

// GearStatsEvent converts changes to a full stats event.
func (gsc *GearStatsChanges) GearStatsEvent() *GearStatsEvent {
	if gsc == nil {
		return nil
	}
	gse := &GearStatsEvent{
		Time:          gsc.Time,
		Battery:       gsc.Battery,
		SystemVersion: gsc.SystemVersion,
		Volume:        gsc.Volume,
		Brightness:    gsc.Brightness,
		LightMode:     gsc.LightMode,
		WifiNetwork:   gsc.WifiNetwork,
		WifiStore:     gsc.WifiStore,
		ReadNFCTag:    gsc.ReadNFCTag,
		PairStatus:    gsc.PairStatus,
		Shaking:       gsc.Shaking,
	}
	if gsc.LastResetAt != nil {
		gse.LastResetAt = *gsc.LastResetAt
	}
	return gse
}

// GearStatsEvent contains device statistics.
type GearStatsEvent struct {
	Time          jsontime.Milli     `json:"time"`
	LastResetAt   jsontime.Milli     `json:"last_reset_at,omitzero"`
	Battery       *Battery           `json:"battery,omitzero"`
	SystemVersion *SystemVersion     `json:"system_version,omitzero"`
	Volume        *Volume            `json:"volume,omitzero"`
	Brightness    *Brightness        `json:"brightness,omitzero"`
	LightMode     *LightMode         `json:"light_mode,omitzero"`
	Cellular      *ConnectedCellular `json:"cellular_network,omitzero"`
	WifiNetwork   *ConnectedWifi     `json:"wifi_network,omitzero"`
	WifiStore     *StoredWifiList    `json:"wifi_store,omitzero"`
	ReadNFCTag    *ReadNFCTag        `json:"read_nfc_tag,omitzero"`
	PairStatus    *PairStatus        `json:"pair_status,omitzero"`
	Shaking       *Shaking           `json:"shaking,omitzero"`
}

// SystemVersion contains system version information.
type SystemVersion struct {
	CurrentVersion       string             `json:"current_version,omitzero"`
	InstallingPercentage float64            `json:"installing_percentage,omitzero"`
	InstallingVersion    string             `json:"installing_version,omitzero"`
	Components           []ComponentVersion `json:"components,omitzero"`
	UpdateAt             jsontime.Milli     `json:"update_at,omitzero"`
}

// ComponentVersion contains version info for a system component.
type ComponentVersion struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version,omitzero"`
}

// Brightness contains display brightness settings.
type Brightness struct {
	Percentage float64        `json:"percentage"`
	UpdateAt   jsontime.Milli `json:"update_at"`
}

// LightMode contains light mode settings.
type LightMode struct {
	Mode     string         `json:"mode"`
	UpdateAt jsontime.Milli `json:"update_at"`
}

// Volume contains audio volume settings.
type Volume struct {
	Percentage float64        `json:"percentage"`
	UpdateAt   jsontime.Milli `json:"update_at"`
}

// Ping contains network ping statistics.
type Ping struct {
	RecvAt    jsontime.Milli `json:"recv_at"`
	SendAt    jsontime.Milli `json:"send_at"`
	Timestamp jsontime.Milli `json:"timestamp"`
	Latency   float64        `json:"latency"`
}

// ConnectedCellular contains cellular network information.
type ConnectedCellular struct {
	DNS     []string `json:"dns,omitzero"`
	Gateway string   `json:"gateway,omitzero"`
	IP      string   `json:"ip,omitzero"`
	NetMask string   `json:"mask,omitzero"`
	RSSI    float64  `json:"rssi,omitzero"`
	Ping    *Ping    `json:"ping,omitzero"`
}

// ConnectedWifi contains WiFi network information.
type ConnectedWifi struct {
	BSSID    string   `json:"bssid,omitzero"`
	DNS      []string `json:"dns,omitzero"`
	Gateway  string   `json:"gateway,omitzero"`
	IP       string   `json:"ip,omitzero"`
	NetMask  string   `json:"mask,omitzero"`
	Mac      string   `json:"mac,omitzero"`
	RSSI     float64  `json:"rssi,omitzero"`
	Security string   `json:"security,omitzero"`
	SSID     string   `json:"ssid,omitzero"`
	Ping     *Ping    `json:"ping,omitzero"`
}

// WifiStoreItem contains information about a stored WiFi network.
type WifiStoreItem struct {
	LastConnectAt jsontime.Milli `json:"last_connect_at"`
	Security      string         `json:"security,omitzero"`
	SSID          string         `json:"ssid,omitzero"`
}

// StoredWifiList contains a list of stored WiFi networks.
type StoredWifiList struct {
	List     []WifiStoreItem `json:"list,omitzero"`
	UpdateAt jsontime.Milli  `json:"update_at"`
}

// Battery contains battery status information.
type Battery struct {
	CurrentCapacity  float64 `json:"current_capacity,omitzero"`
	CycleCount       float64 `json:"cycle_count,omitzero"`
	IsCharging       bool    `json:"is_charging,omitzero"`
	OriginalCapacity float64 `json:"original_capacity,omitzero"`
	Percentage       float64 `json:"percentage,omitzero"`
	Temperature      float64 `json:"temperature,omitzero"`
	Voltage          float64 `json:"voltage,omitzero"`
}

// ReadNFCTag contains NFC tag read data.
type ReadNFCTag struct {
	Tags []*NFCTag `json:"tags,omitzero"`
}

// NFCTag contains information about an NFC tag.
type NFCTag struct {
	UID        string                `json:"uid,omitzero"`
	Type       string                `json:"type,omitzero"`
	RawData    encoding.StdBase64Data `json:"raw_data,omitzero"`
	DataFormat string                `json:"data_format,omitzero"`
	RSSI       float32               `json:"rssi,omitzero"`
	UpdateAt   jsontime.Milli        `json:"update_at,omitzero"`
}

// PairStatus contains device pairing status.
type PairStatus struct {
	PairWith string         `json:"pair_with,omitzero"`
	UpdateAt jsontime.Milli `json:"update_at,omitzero"`
}

// Shaking contains device shaking detection data.
type Shaking struct {
	Level float64 `json:"level"`
}

// Clone returns a deep copy of the stats event.
func (gse *GearStatsEvent) Clone() *GearStatsEvent {
	if gse == nil {
		return nil
	}
	v := *gse
	v.Battery = clonePtr(gse.Battery)
	v.SystemVersion = gse.SystemVersion.clone()
	v.Volume = clonePtr(gse.Volume)
	v.Brightness = clonePtr(gse.Brightness)
	v.LightMode = clonePtr(gse.LightMode)
	v.WifiNetwork = gse.WifiNetwork.clone()
	v.WifiStore = gse.WifiStore.clone()
	v.ReadNFCTag = gse.ReadNFCTag.clone()
	v.PairStatus = clonePtr(gse.PairStatus)
	v.Shaking = clonePtr(gse.Shaking)
	return &v
}

func (sv *SystemVersion) clone() *SystemVersion {
	if sv == nil {
		return nil
	}
	v := *sv
	v.Components = slices.Clone(sv.Components)
	return &v
}

func (cc *ConnectedCellular) clone() *ConnectedCellular {
	if cc == nil {
		return nil
	}
	v := *cc
	v.DNS = slices.Clone(cc.DNS)
	v.Ping = clonePtr(cc.Ping)
	return &v
}

func (cw *ConnectedWifi) clone() *ConnectedWifi {
	if cw == nil {
		return nil
	}
	v := *cw
	v.DNS = slices.Clone(cw.DNS)
	v.Ping = clonePtr(cw.Ping)
	return &v
}

func (swl *StoredWifiList) clone() *StoredWifiList {
	if swl == nil {
		return nil
	}
	v := *swl
	v.List = slices.Clone(swl.List)
	return &v
}

func (rnt *ReadNFCTag) clone() *ReadNFCTag {
	if rnt == nil {
		return nil
	}
	v := &ReadNFCTag{
		Tags: make([]*NFCTag, 0, len(rnt.Tags)),
	}
	for _, tag := range rnt.Tags {
		v.Tags = append(v.Tags, tag.clone())
	}
	return v
}

func (nt *NFCTag) clone() *NFCTag {
	if nt == nil {
		return nil
	}
	return &NFCTag{
		UID:        nt.UID,
		Type:       nt.Type,
		RawData:    bytes.Clone(nt.RawData),
		DataFormat: nt.DataFormat,
		RSSI:       nt.RSSI,
		UpdateAt:   nt.UpdateAt,
	}
}

// MarshalJSON implements json.Marshaler.
func (rnt *ReadNFCTag) MarshalJSON() ([]byte, error) {
	return json.Marshal(rnt.Tags)
}

// UnmarshalJSON implements json.Unmarshaler.
func (rnt *ReadNFCTag) UnmarshalJSON(data []byte) error {
	*rnt = ReadNFCTag{}
	return json.Unmarshal(data, &rnt.Tags)
}

// MergeWith merges another stats event into this one.
// Returns the changes if any fields were updated.
func (gse *GearStatsEvent) MergeWith(other *GearStatsEvent) *GearStatsChanges {
	if other.Time.Before(gse.Time) {
		return nil
	}
	gse.Time = other.Time

	var diff GearStatsChanges

	if other.LastResetAt.After(gse.LastResetAt) {
		gse.LastResetAt = other.LastResetAt
		v := other.LastResetAt
		diff.LastResetAt = &v
	}

	switch {
	case other.SystemVersion == nil:
	case gse.SystemVersion == nil,
		!other.SystemVersion.UpdateAt.Before(gse.SystemVersion.UpdateAt):
		gse.SystemVersion = other.SystemVersion
		diff.SystemVersion = other.SystemVersion.clone()
	}

	switch {
	case other.Volume == nil:
	case gse.Volume == nil,
		!other.Volume.UpdateAt.Before(gse.Volume.UpdateAt):
		gse.Volume = other.Volume
		diff.Volume = clonePtr(other.Volume)
	}

	switch {
	case other.Brightness == nil:
	case gse.Brightness == nil,
		!other.Brightness.UpdateAt.Before(gse.Brightness.UpdateAt):
		gse.Brightness = other.Brightness
		diff.Brightness = clonePtr(other.Brightness)
	}

	switch {
	case other.LightMode == nil:
	case gse.LightMode == nil,
		!other.LightMode.UpdateAt.Before(gse.LightMode.UpdateAt):
		gse.LightMode = other.LightMode
		diff.LightMode = clonePtr(other.LightMode)
	}

	switch {
	case other.PairStatus == nil:
	case gse.PairStatus == nil,
		!other.PairStatus.UpdateAt.Before(gse.PairStatus.UpdateAt):
		gse.PairStatus = other.PairStatus
		diff.PairStatus = clonePtr(other.PairStatus)
	}

	switch {
	case other.Battery == nil:
	case !other.Battery.Equal(gse.Battery):
		gse.Battery = other.Battery
		diff.Battery = clonePtr(other.Battery)
	}

	switch {
	case other.WifiStore == nil:
	case gse.WifiStore == nil,
		!other.WifiStore.UpdateAt.Before(gse.WifiStore.UpdateAt):
		gse.WifiStore = other.WifiStore
		diff.WifiStore = other.WifiStore.clone()
	}

	switch {
	case other.Cellular == nil:
	case !gse.Cellular.Equal(other.Cellular):
		gse.Cellular = other.Cellular
		diff.Cellular = other.Cellular.clone()
	}

	switch {
	case other.WifiNetwork == nil:
	case !gse.WifiNetwork.Equal(other.WifiNetwork):
		gse.WifiNetwork = other.WifiNetwork
		diff.WifiNetwork = other.WifiNetwork.clone()
	}

	switch {
	case other.ReadNFCTag == nil:
	case !other.ReadNFCTag.Equal(gse.ReadNFCTag):
		gse.ReadNFCTag = other.ReadNFCTag
		diff.ReadNFCTag = other.ReadNFCTag.clone()
	}

	switch {
	case other.Shaking == nil:
	case gse.Shaking == nil,
		!other.Shaking.Equal(gse.Shaking):
		gse.Shaking = other.Shaking
		diff.Shaking = clonePtr(other.Shaking)
	}

	if diff == (GearStatsChanges{}) {
		return nil
	}
	diff.Time = gse.Time
	return &diff
}

// Equal methods for comparison

func (s *Shaking) Equal(other *Shaking) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Level == other.Level
}

func (b *Battery) Equal(other *Battery) bool {
	if b == nil || other == nil {
		return b == other
	}
	return *b == *other
}

func (p *Ping) Equal(other *Ping) bool {
	if p == nil || other == nil {
		return p == other
	}
	return *p == *other
}

func (cc *ConnectedCellular) Equal(other *ConnectedCellular) bool {
	if cc == nil || other == nil {
		return cc == other
	}
	ok := cc.Gateway == other.Gateway &&
		cc.IP == other.IP &&
		cc.NetMask == other.NetMask &&
		cc.RSSI == other.RSSI &&
		cc.Ping.Equal(other.Ping)
	if !ok {
		return false
	}
	if len(cc.DNS) != len(other.DNS) {
		return false
	}
	for i, v := range cc.DNS {
		if v != other.DNS[i] {
			return false
		}
	}
	return true
}

func (cw *ConnectedWifi) Equal(other *ConnectedWifi) bool {
	if cw == nil || other == nil {
		return cw == other
	}
	ok := cw.BSSID == other.BSSID &&
		cw.Gateway == other.Gateway &&
		cw.IP == other.IP &&
		cw.Mac == other.Mac &&
		cw.NetMask == other.NetMask &&
		cw.RSSI == other.RSSI &&
		cw.Security == other.Security &&
		cw.SSID == other.SSID &&
		cw.Ping.Equal(other.Ping)
	if !ok {
		return false
	}
	if len(cw.DNS) != len(other.DNS) {
		return false
	}
	for i, v := range cw.DNS {
		if v != other.DNS[i] {
			return false
		}
	}
	return true
}

func (nt *NFCTag) Equal(other *NFCTag) bool {
	if nt == nil || other == nil {
		return nt == other
	}
	return nt.UID == other.UID
}

func (rnt *ReadNFCTag) Equal(other *ReadNFCTag) bool {
	if rnt == nil || other == nil {
		return rnt == other
	}

	rntUids := make(map[string]struct{})
	for _, tag := range rnt.Tags {
		rntUids[tag.UID] = struct{}{}
	}

	otherUids := make(map[string]struct{})
	for _, tag := range other.Tags {
		otherUids[tag.UID] = struct{}{}
	}

	if len(rntUids) != len(otherUids) {
		return false
	}

	for uid := range rntUids {
		if _, ok := otherUids[uid]; !ok {
			return false
		}
	}

	return true
}

// clonePtr creates a shallow copy of a pointer value.
func clonePtr[T any](v *T) *T {
	if v == nil {
		return nil
	}
	v2 := *v
	return &v2
}
