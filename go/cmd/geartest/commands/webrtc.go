package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// WebRTCBridge handles WebRTC connection with browser for audio I/O.
type WebRTCBridge struct {
	mu sync.RWMutex

	pc          *webrtc.PeerConnection
	audioTrack  *webrtc.TrackLocalStaticRTP // for sending audio to browser
	remoteTrack *webrtc.TrackRemote         // for receiving audio from browser

	// Callbacks
	onAudioReceived func(opusData []byte) // Called when audio is received from browser
	onStateChange   func(state string)    // Called when connection state changes

	connected bool
	ssrc      uint32 // Random SSRC for RTP packets
}

// NewWebRTCBridge creates a new WebRTC bridge.
func NewWebRTCBridge() *WebRTCBridge {
	return &WebRTCBridge{
		ssrc: rand.Uint32(),
	}
}

// SetOnAudioReceived sets the callback for when audio is received from browser.
func (b *WebRTCBridge) SetOnAudioReceived(fn func(opusData []byte)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onAudioReceived = fn
}

// SetOnStateChange sets the callback for connection state changes.
func (b *WebRTCBridge) SetOnStateChange(fn func(state string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStateChange = fn
}

// HandleOffer processes an SDP offer from browser and returns an answer.
func (b *WebRTCBridge) HandleOffer(offerSDP string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close existing connection if any
	if b.pc != nil {
		if err := b.pc.Close(); err != nil {
			slog.Error("WebRTC failed to close existing peer connection", "error", err)
		}
		b.pc = nil
		b.connected = false
	}

	// Create a new PeerConnection
	config := webrtc.Configuration{
		// No ICE servers needed for localhost
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return "", fmt.Errorf("create peer connection: %w", err)
	}
	b.pc = pc

	// Create audio track for sending to browser (Opus)
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"geartest-audio",
	)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create audio track: %w", err)
	}
	b.audioTrack = audioTrack

	// Add the track to the connection
	_, err = pc.AddTrack(audioTrack)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("add track: %w", err)
	}

	// Handle incoming tracks (audio from browser)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		slog.Info("WebRTC received track", "id", track.ID(), "codec", track.Codec().MimeType)

		b.mu.Lock()
		b.remoteTrack = track
		b.mu.Unlock()

		// Read RTP packets from browser
		go b.readRemoteTrack(track)
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("WebRTC connection state", "state", state.String())

		b.mu.Lock()
		b.connected = state == webrtc.PeerConnectionStateConnected
		callback := b.onStateChange
		b.mu.Unlock()

		if callback != nil {
			callback(state.String())
		}
	})

	// Handle ICE connection state
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		slog.Info("WebRTC ICE state", "state", state.String())
	})

	// Set the remote description (offer from browser)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", fmt.Errorf("set remote description: %w", err)
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create answer: %w", err)
	}

	// Set local description
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", fmt.Errorf("set local description: %w", err)
	}

	// Wait for ICE gathering to complete (for localhost, this is fast)
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	// Return the final answer with ICE candidates
	return pc.LocalDescription().SDP, nil
}

// readRemoteTrack reads RTP packets from the browser's audio track.
func (b *WebRTCBridge) readRemoteTrack(track *webrtc.TrackRemote) {
	slog.Info("WebRTC starting to read audio from browser")

	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			slog.Info("WebRTC track read ended", "error", err)
			return
		}

		b.mu.RLock()
		callback := b.onAudioReceived
		b.mu.RUnlock()

		if callback != nil {
			// RTP payload contains Opus data
			callback(rtpPacket.Payload)
		}
	}
}

// SendAudio sends Opus audio data to the browser.
func (b *WebRTCBridge) SendAudio(opusData []byte, timestamp uint32, sequenceNumber uint16) error {
	b.mu.RLock()
	track := b.audioTrack
	connected := b.connected
	b.mu.RUnlock()

	if !connected || track == nil {
		return nil // Silently drop if not connected
	}

	// Create RTP packet
	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111, // Opus payload type
			SequenceNumber: sequenceNumber,
			Timestamp:      timestamp,
			SSRC:           b.ssrc,
		},
		Payload: opusData,
	}

	return track.WriteRTP(packet)
}

// IsConnected returns true if WebRTC is connected.
func (b *WebRTCBridge) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Close closes the WebRTC connection.
func (b *WebRTCBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pc != nil {
		err := b.pc.Close()
		b.pc = nil
		b.connected = false
		return err
	}
	return nil
}

// OfferRequest is the JSON structure for WebRTC offer.
type OfferRequest struct {
	SDP string `json:"sdp"`
}

// AnswerResponse is the JSON structure for WebRTC answer.
type AnswerResponse struct {
	SDP string `json:"sdp"`
}

// ParseOfferRequest parses JSON offer request.
func ParseOfferRequest(data []byte) (*OfferRequest, error) {
	var req OfferRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// MarshalAnswerResponse creates JSON answer response.
func MarshalAnswerResponse(sdp string) ([]byte, error) {
	return json.Marshal(AnswerResponse{SDP: sdp})
}
