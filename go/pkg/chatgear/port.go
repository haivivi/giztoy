package chatgear

import (
	"context"

	"github.com/haivivi/giztoy/go/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
)

// =============================================================================
// Port Interfaces (Client/Server symmetric design)
// =============================================================================

// ClientPortTx represents the transmit side of a client port (client to server).
// It sends audio frames, state events, and stats events to the server.
type ClientPortTx interface {
	// SendOpusFrames sends a stamped opus frame to the server.
	SendOpusFrames(stampedOpusFrame []byte) error

	// SendState sends a state event to the server.
	SendState(state *GearStateEvent) error

	// SendStats sends a stats event to the server.
	SendStats(stats *GearStatsEvent) error
}

// ClientPortRx represents the receive side of a client port (server to client).
// It receives audio frames and commands from the server.
type ClientPortRx interface {
	opusrt.FrameReader

	// Commands returns a channel that receives commands from the server.
	Commands() <-chan *SessionCommandEvent
}

// ServerPortTx represents the transmit side of a server port (server to client).
// It sends audio frames and commands to the client.
type ServerPortTx interface {
	// --- Audio Output ---

	// NewBackgroundTrack creates a new background audio track.
	NewBackgroundTrack() (pcm.Track, *pcm.TrackCtrl, error)

	// NewForegroundTrack creates a new foreground audio track.
	NewForegroundTrack() (pcm.Track, *pcm.TrackCtrl, error)

	// NewOverlayTrack creates a new overlay audio track.
	NewOverlayTrack() (pcm.Track, *pcm.TrackCtrl, error)

	// BackgroundTrackCtrl returns the current background track controller.
	BackgroundTrackCtrl() *pcm.TrackCtrl

	// ForegroundTrackCtrl returns the current foreground track controller.
	ForegroundTrackCtrl() *pcm.TrackCtrl

	// OverlayTrackCtrl returns the current overlay track controller.
	OverlayTrackCtrl() *pcm.TrackCtrl

	// Interrupt stops all output tracks immediately.
	Interrupt()

	// --- Device Commands ---

	// SetVolume sets the volume of the device.
	SetVolume(volume int) error

	// SetLightMode sets the light mode of the device.
	SetLightMode(mode string) error

	// SetBrightness sets the brightness of the device.
	SetBrightness(brightness int) error

	// SetWifi sets the WiFi network of the device.
	SetWifi(ssid, password string) error

	// DeleteWifi deletes a stored WiFi network.
	DeleteWifi(ssid string) error

	// Reset resets the device.
	Reset() error

	// Unpair unpairs the device.
	Unpair() error

	// Sleep puts the device to sleep.
	Sleep() error

	// Shutdown shuts down the device.
	Shutdown() error

	// RaiseCall raises a call on the device.
	RaiseCall() error

	// UpgradeFirmware initiates an OTA firmware upgrade.
	UpgradeFirmware(ota OTA) error
}

// ServerPortRx represents the receive side of a server port (client to server).
// It receives audio frames, state events, and stats changes from the client.
type ServerPortRx interface {
	opusrt.FrameReader

	// Context returns the port's context.
	Context() context.Context

	// StateEvents returns a channel that receives state events from the client.
	StateEvents() <-chan *GearStateEvent

	// StatsChanges returns a channel that receives stats change events from the client.
	StatsChanges() <-chan *GearStatsChanges

	// GearState returns the current gear state.
	GearState() (*GearStateEvent, bool)

	// GearStats returns the current gear stats.
	GearStats() (*GearStatsEvent, bool)

	// Volume returns the current volume percentage.
	Volume() (int, bool)

	// LightMode returns the current light mode.
	LightMode() (string, bool)

	// Brightness returns the current brightness percentage.
	Brightness() (int, bool)

	// WifiNetwork returns the current connected WiFi network.
	WifiNetwork() (*ConnectedWifi, bool)

	// WifiStore returns the stored WiFi list.
	WifiStore() (*StoredWifiList, bool)

	// Battery returns the current battery status.
	Battery() (pct int, isCharging bool, ok bool)

	// SystemVersion returns the current system version.
	SystemVersion() (string, bool)

	// Cellular returns the current cellular network.
	Cellular() (*ConnectedCellular, bool)

	// PairStatus returns the current pair status.
	PairStatus() (string, bool)

	// ReadNFCTag returns the last read NFC tags.
	ReadNFCTag() (*ReadNFCTag, bool)

	// Shaking returns the current shaking level.
	Shaking() (float64, bool)
}
