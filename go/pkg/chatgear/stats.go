package chatgear

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/haivivi/giztoy/go/pkg/encoding"
	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

// StatsChanges represents changes to device statistics.
type StatsChanges struct {
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

// StatsEvent converts changes to a full stats event.
func (c *StatsChanges) StatsEvent() *StatsEvent {
	if c == nil {
		return nil
	}
	e := &StatsEvent{
		Time:          c.Time,
		Battery:       c.Battery,
		SystemVersion: c.SystemVersion,
		Volume:        c.Volume,
		Brightness:    c.Brightness,
		LightMode:     c.LightMode,
		WifiNetwork:   c.WifiNetwork,
		WifiStore:     c.WifiStore,
		ReadNFCTag:    c.ReadNFCTag,
		PairStatus:    c.PairStatus,
		Shaking:       c.Shaking,
	}
	if c.LastResetAt != nil {
		e.LastResetAt = *c.LastResetAt
	}
	return e
}

// StatsEvent contains device statistics.
type StatsEvent struct {
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
func (e *StatsEvent) Clone() *StatsEvent {
	if e == nil {
		return nil
	}
	v := *e
	v.Battery = clonePtr(e.Battery)
	v.SystemVersion = e.SystemVersion.clone()
	v.Volume = clonePtr(e.Volume)
	v.Brightness = clonePtr(e.Brightness)
	v.LightMode = clonePtr(e.LightMode)
	v.WifiNetwork = e.WifiNetwork.clone()
	v.WifiStore = e.WifiStore.clone()
	v.ReadNFCTag = e.ReadNFCTag.clone()
	v.PairStatus = clonePtr(e.PairStatus)
	v.Shaking = clonePtr(e.Shaking)
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
func (e *StatsEvent) MergeWith(other *StatsEvent) *StatsChanges {
	if other.Time.Before(e.Time) {
		return nil
	}
	e.Time = other.Time

	var diff StatsChanges

	if other.LastResetAt.After(e.LastResetAt) {
		e.LastResetAt = other.LastResetAt
		v := other.LastResetAt
		diff.LastResetAt = &v
	}

	switch {
	case other.SystemVersion == nil:
	case e.SystemVersion == nil,
		!other.SystemVersion.UpdateAt.Before(e.SystemVersion.UpdateAt):
		e.SystemVersion = other.SystemVersion
		diff.SystemVersion = other.SystemVersion.clone()
	}

	switch {
	case other.Volume == nil:
	case e.Volume == nil,
		!other.Volume.UpdateAt.Before(e.Volume.UpdateAt):
		e.Volume = other.Volume
		diff.Volume = clonePtr(other.Volume)
	}

	switch {
	case other.Brightness == nil:
	case e.Brightness == nil,
		!other.Brightness.UpdateAt.Before(e.Brightness.UpdateAt):
		e.Brightness = other.Brightness
		diff.Brightness = clonePtr(other.Brightness)
	}

	switch {
	case other.LightMode == nil:
	case e.LightMode == nil,
		!other.LightMode.UpdateAt.Before(e.LightMode.UpdateAt):
		e.LightMode = other.LightMode
		diff.LightMode = clonePtr(other.LightMode)
	}

	switch {
	case other.PairStatus == nil:
	case e.PairStatus == nil,
		!other.PairStatus.UpdateAt.Before(e.PairStatus.UpdateAt):
		e.PairStatus = other.PairStatus
		diff.PairStatus = clonePtr(other.PairStatus)
	}

	switch {
	case other.Battery == nil:
	case !other.Battery.Equal(e.Battery):
		e.Battery = other.Battery
		diff.Battery = clonePtr(other.Battery)
	}

	switch {
	case other.WifiStore == nil:
	case e.WifiStore == nil,
		!other.WifiStore.UpdateAt.Before(e.WifiStore.UpdateAt):
		e.WifiStore = other.WifiStore
		diff.WifiStore = other.WifiStore.clone()
	}

	switch {
	case other.Cellular == nil:
	case !e.Cellular.Equal(other.Cellular):
		e.Cellular = other.Cellular
		diff.Cellular = other.Cellular.clone()
	}

	switch {
	case other.WifiNetwork == nil:
	case !e.WifiNetwork.Equal(other.WifiNetwork):
		e.WifiNetwork = other.WifiNetwork
		diff.WifiNetwork = other.WifiNetwork.clone()
	}

	switch {
	case other.ReadNFCTag == nil:
	case !other.ReadNFCTag.Equal(e.ReadNFCTag):
		e.ReadNFCTag = other.ReadNFCTag
		diff.ReadNFCTag = other.ReadNFCTag.clone()
	}

	switch {
	case other.Shaking == nil:
	case e.Shaking == nil,
		!other.Shaking.Equal(e.Shaking):
		e.Shaking = other.Shaking
		diff.Shaking = clonePtr(other.Shaking)
	}

	if diff == (StatsChanges{}) {
		return nil
	}
	diff.Time = e.Time
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
