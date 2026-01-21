package commands

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		panic(err)
	}
}

// WebServer serves the control panel for the simulator.
type WebServer struct {
	sim  *Simulator
	addr string
}

// NewWebServer creates a new web server.
func NewWebServer(sim *Simulator, port int) *WebServer {
	return &WebServer{
		sim:  sim,
		addr: fmt.Sprintf(":%d", port),
	}
}

// Start starts the web server in a goroutine.
func (ws *WebServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/api/stats", ws.handleStats)
	mux.HandleFunc("/api/stats/update", ws.handleUpdateStats)
	mux.HandleFunc("/api/control", ws.handleControl)
	mux.HandleFunc("/api/webrtc/offer", ws.handleWebRTCOffer)

	go func() {
		slog.Info("Web control panel starting", "addr", ws.addr)
		if err := http.ListenAndServe(ws.addr, mux); err != nil {
			slog.Error("web server error", "error", err)
		}
	}()
}

func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// StatsResponse is the JSON response for /api/stats.
type StatsResponse struct {
	// Unified device state: OFF, SLEEP, READY, RECORDING, CALLING, etc.
	State           string `json:"state"`
	WebRTCConnected bool   `json:"webrtcConnected"` // WebRTC audio connected

	// Device stats
	Battery    float64  `json:"battery"`
	Charging   bool     `json:"charging"`
	Volume     int      `json:"volume"`
	Brightness int      `json:"brightness"`
	LightMode  string   `json:"lightMode"`
	WifiSSID   string   `json:"wifiSSID"`
	WifiRSSI   float64  `json:"wifiRSSI"`
	WifiIP     string   `json:"wifiIP"`
	WifiStore  []string `json:"wifiStore"`
	Version    string   `json:"version"`
	PairWith   string   `json:"pairWith"`
	Shaking    float64  `json:"shaking"`
}

func (ws *WebServer) handleStats(w http.ResponseWriter, r *http.Request) {
	bat, charging := ws.sim.Battery()
	ssid, rssi := ws.sim.Wifi()

	// Unified state: if powered off or sleeping, use power state; otherwise use gear state
	var state string
	powerState := ws.sim.PowerState()
	switch powerState {
	case PowerOff:
		state = "OFF"
	case PowerDeepSleep:
		state = "SLEEP"
	default:
		state = ws.sim.State().String() // ready, recording, calling, etc.
	}

	resp := StatsResponse{
		State:           state,
		WebRTCConnected: ws.sim.WebRTC().IsConnected(),
		Battery:         bat,
		Charging:        charging,
		Volume:          ws.sim.Volume(),
		Brightness:      ws.sim.Brightness(),
		LightMode:       ws.sim.LightMode(),
		WifiSSID:        ssid,
		WifiRSSI:        rssi,
		WifiIP:          ws.sim.WifiIP(),
		WifiStore:       ws.sim.WifiStore(),
		Version:         ws.sim.SystemVersion(),
		PairWith:        ws.sim.PairStatus(),
		Shaking:         ws.sim.Shaking(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode stats response", "error", err)
	}
}

func (ws *WebServer) handleUpdateStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON body
	var req struct {
		Field string `json:"field"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	field := req.Field
	value := req.Value

	// Skip empty fields (likely browser bug or stale event)
	if field == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ignored"})
		return
	}

	var msg string
	switch field {
	case "volume":
		v, err := strconv.Atoi(value)
		if err != nil {
			http.Error(w, "Invalid value for volume", http.StatusBadRequest)
			return
		}
		ws.sim.SetVolume(v)
		msg = fmt.Sprintf("Volume set to %d%%", ws.sim.Volume())

	case "brightness":
		v, err := strconv.Atoi(value)
		if err != nil {
			http.Error(w, "Invalid value for brightness", http.StatusBadRequest)
			return
		}
		ws.sim.SetBrightness(v)
		msg = fmt.Sprintf("Brightness set to %d%%", ws.sim.Brightness())

	case "lightMode":
		ws.sim.SetLightMode(value)
		msg = fmt.Sprintf("Light mode set to %s", value)

	case "battery":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			http.Error(w, "Invalid value for battery", http.StatusBadRequest)
			return
		}
		_, charging := ws.sim.Battery()
		ws.sim.SetBattery(v, charging)
		msg = fmt.Sprintf("Battery set to %.0f%%", v)

	case "charging":
		charging := value == "true"
		bat, _ := ws.sim.Battery()
		ws.sim.SetBattery(bat, charging)
		if charging {
			msg = "Charging enabled"
		} else {
			msg = "Charging disabled"
		}

	case "wifi":
		// value format: "ssid,rssi" or empty to disconnect
		if value == "" {
			ws.sim.SetWifi("", 0)
			msg = "WiFi disconnected"
		} else {
			parts := strings.SplitN(value, ",", 2)
			ssid := parts[0]
			rssi := -50.0
			if len(parts) > 1 {
				if parsed, err := strconv.ParseFloat(parts[1], 64); err != nil {
					slog.Error("invalid RSSI value", "value", parts[1], "error", err)
				} else {
					rssi = parsed
				}
			}
			ws.sim.SetWifi(ssid, rssi)
			msg = fmt.Sprintf("Connected to %s", ssid)
		}

	case "version":
		ws.sim.SetVersion(value)
		msg = fmt.Sprintf("Version set to %s", value)

	case "pairWith":
		ws.sim.SetPairStatus(value)
		if value == "" {
			msg = "Unpaired"
		} else {
			msg = fmt.Sprintf("Paired with %s", value)
		}

	case "shaking":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			http.Error(w, "Invalid value for shaking", http.StatusBadRequest)
			return
		}
		ws.sim.SetShaking(v)
		msg = fmt.Sprintf("Shaking set to %.0f", v)

	case "addWifi":
		// value is SSID to add
		ws.sim.AddWifiStore(value)
		msg = fmt.Sprintf("Added WiFi: %s", value)

	case "removeWifi":
		// value is SSID to remove
		ws.sim.RemoveWifiStore(value)
		msg = fmt.Sprintf("Removed WiFi: %s", value)

	default:
		http.Error(w, "Unknown field", http.StatusBadRequest)
		return
	}

	// Trigger stats report after any change
	ws.sim.SendStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

// handleControl handles power and state control commands.
func (ws *WebServer) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var msg string
	var err error

	switch req.Action {
	case "powerOn":
		err = ws.sim.PowerOn()
		if err != nil {
			msg = fmt.Sprintf("Power on failed: %v", err)
		} else {
			msg = "Device powered on"
		}

	case "powerOff":
		ws.sim.PowerOff()
		msg = "Device powered off"

	case "deepSleep":
		ws.sim.DeepSleep()
		msg = "Device in deep sleep"

	case "reset":
		ws.sim.Reset()
		msg = "Device reset"

	case "startRecording":
		ws.sim.StartRecording()
		msg = "Recording started"

	case "endRecording":
		ws.sim.EndRecording()
		msg = "Recording ended"

	case "startCalling":
		ws.sim.StartCalling()
		msg = "Call started"

	case "endCalling":
		ws.sim.EndCalling()
		msg = "Call ended"

	case "cancel":
		ws.sim.Cancel()
		msg = "Action cancelled"

	default:
		http.Error(w, "Unknown action: "+req.Action, http.StatusBadRequest)
		return
	}

	slog.Info("control action", "action", req.Action, "result", msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

// handleWebRTCOffer handles WebRTC signaling (SDP offer/answer exchange).
func (ws *WebServer) handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the offer SDP
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("WebRTC read body error", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	offerReq, err := ParseOfferRequest(body)
	if err != nil {
		slog.Error("WebRTC parse offer error", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("WebRTC received SDP offer, processing...")

	// Handle the offer and get answer
	answerSDP, err := ws.sim.WebRTC().HandleOffer(offerReq.SDP)
	if err != nil {
		slog.Error("WebRTC handle offer error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("WebRTC sending SDP answer")

	// Send answer
	answerJSON, err := MarshalAnswerResponse(answerSDP)
	if err != nil {
		slog.Error("WebRTC marshal answer error", "error", err)
		http.Error(w, "failed to marshal answer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(answerJSON); err != nil {
		slog.Error("WebRTC write answer error", "error", err)
	}
}
