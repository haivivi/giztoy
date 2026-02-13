// Package main provides a long-running ChatGear MQTT logger with HTTP API.
//
// Connects to the MQTT broker as a server port, records all uplink events
// (state, stats, audio) into an in-memory ring buffer (last 1000 entries),
// and exposes them via HTTP for inspection.
//
// Designed to run once in the background — survives device reboots/reflashes.
//
// Usage:
//
//	bazel build //e2e/go/chatgear/logger
//	bazel-bin/e2e/go/chatgear/logger/logger_/logger \
//	  --mqtt=mqtts://admin:xxx@mqtt.host:8883 \
//	  --namespace=RyBFG6 \
//	  --gear-id=693b0fb7839769199432f516 \
//	  --port=8899 &
//
// HTTP API:
//
//	GET /logs          — last 100 entries (JSON), newest first
//	GET /logs?n=500    — last N entries
//	GET /logs?type=state — filter by type (state|stats|audio|error|connect)
//	GET /state         — current device state + stats snapshot
//	GET /              — auto-refresh HTML page
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/go/pkg/chatgear"
)

// ============================================================================
// Flags
// ============================================================================

var (
	mqttAddr  = flag.String("mqtt", "", "MQTT broker URL (required)")
	namespace = flag.String("namespace", "", "Topic namespace/scope (required)")
	gearID    = flag.String("gear-id", "", "Device gear ID (required)")
	httpPort  = flag.Int("port", 8899, "HTTP API port")
)

// ============================================================================
// Ring Log
// ============================================================================

const maxEntries = 1000

type LogEntry struct {
	Time    string `json:"time"`              // "15:04:05.000"
	Type    string `json:"type"`              // state | stats | audio | connect | error
	Summary string `json:"summary"`           // one-line description
	Detail  any    `json:"detail,omitempty"`   // raw event data (state/stats)
	Seq     int64  `json:"seq"`               // monotonic sequence number
}

type RingLog struct {
	mu      sync.Mutex
	entries [maxEntries]LogEntry
	pos     int   // next write position
	count   int   // total written (for seq)
	total   int64 // monotonic counter
}

func (r *RingLog) Add(typ, summary string, detail any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++
	r.entries[r.pos] = LogEntry{
		Time:    time.Now().Format("15:04:05.000"),
		Type:    typ,
		Summary: summary,
		Detail:  detail,
		Seq:     r.total,
	}
	r.pos = (r.pos + 1) % maxEntries
	if r.count < maxEntries {
		r.count++
	}
}

// Last returns the most recent n entries, newest first.
// If typeFilter is non-empty, only entries of that type are returned.
func (r *RingLog) Last(n int, typeFilter string) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n <= 0 || n > r.count {
		n = r.count
	}

	// Collect from newest to oldest
	result := make([]LogEntry, 0, n)
	for i := 0; i < r.count && len(result) < n; i++ {
		idx := (r.pos - 1 - i + maxEntries) % maxEntries
		e := r.entries[idx]
		if typeFilter != "" && e.Type != typeFilter {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ============================================================================
// Globals
// ============================================================================

var (
	ringLog = &RingLog{}

	// Latest known state/stats — protected by mu
	mu          sync.RWMutex
	lastState   *chatgear.StateEvent
	lastStats   *chatgear.StatsEvent
	connected   bool
	connectTime time.Time
)

func addLog(typ, summary string, detail any) {
	ringLog.Add(typ, summary, detail)
	log.Printf("[%s] %s", typ, summary)
}

// ============================================================================
// MQTT Monitor Loop
// ============================================================================

func monitorLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		addLog("connect", fmt.Sprintf("Connecting to %s (gear=%s)...", *mqttAddr, *gearID), nil)

		conn, err := chatgear.DialMQTTServer(ctx, chatgear.MQTTServerConfig{
			Addr:   *mqttAddr,
			Scope:  *namespace,
			GearID: *gearID,
		})
		if err != nil {
			addLog("error", fmt.Sprintf("MQTT connect failed: %v", err), nil)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		mu.Lock()
		connected = true
		connectTime = time.Now()
		mu.Unlock()
		addLog("connect", "MQTT connected!", nil)

		port := chatgear.NewServerPort()

		// ReadFrom + WriteTo
		done := make(chan struct{})
		go func() {
			if err := port.ReadFrom(conn); err != nil {
				addLog("error", fmt.Sprintf("ReadFrom ended: %v", err), nil)
			}
			close(done)
		}()
		go func() {
			if err := port.WriteTo(conn); err != nil {
				addLog("error", fmt.Sprintf("WriteTo ended: %v", err), nil)
			}
		}()

		// Poll loop
		pollDone := make(chan struct{})
		go func() {
			defer close(pollDone)
			var audioFrames int
			var audioBytes int
			var lastAudioLog time.Time

			for {
				data, err := port.Poll()
				if err != nil {
					addLog("error", fmt.Sprintf("Poll ended: %v", err), nil)
					return
				}

				if data.State != nil {
					mu.Lock()
					lastState = data.State
					mu.Unlock()
					addLog("state", data.State.State.String(), data.State)
				}

				if data.StatsChanges != nil {
					// Also update full stats
					if s, ok := port.Stats(); ok {
						mu.Lock()
						lastStats = s
						mu.Unlock()
					}
					addLog("stats", formatStatsChanges(data.StatsChanges), data.StatsChanges)
				}

				if data.Audio != nil {
					audioFrames++
					audioBytes += len(data.Audio.Frame)
					now := time.Now()
					if now.Sub(lastAudioLog) >= time.Second {
						addLog("audio", fmt.Sprintf("%d frames/s, %d bytes", audioFrames, audioBytes), nil)
						audioFrames = 0
						audioBytes = 0
						lastAudioLog = now
					}
				}
			}
		}()

		// Wait for disconnect
		select {
		case <-done:
		case <-ctx.Done():
			port.Close()
			conn.Close()
			return
		}

		mu.Lock()
		connected = false
		mu.Unlock()
		addLog("connect", "MQTT disconnected, will reconnect in 3s...", nil)

		port.Close()
		conn.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func formatStatsChanges(c *chatgear.StatsChanges) string {
	var parts []string
	if c.Volume != nil {
		parts = append(parts, fmt.Sprintf("volume=%d", int(c.Volume.Percentage)))
	}
	if c.Battery != nil {
		parts = append(parts, fmt.Sprintf("battery=%d%%", int(c.Battery.Percentage)))
	}
	if c.Brightness != nil {
		parts = append(parts, fmt.Sprintf("brightness=%d", int(c.Brightness.Percentage)))
	}
	if c.LightMode != nil {
		parts = append(parts, fmt.Sprintf("light=%s", c.LightMode.Mode))
	}
	if c.WifiNetwork != nil {
		parts = append(parts, fmt.Sprintf("wifi=%s", c.WifiNetwork.SSID))
	}
	if c.SystemVersion != nil {
		parts = append(parts, fmt.Sprintf("ver=%s", c.SystemVersion.CurrentVersion))
	}
	if c.PairStatus != nil {
		parts = append(parts, fmt.Sprintf("pair=%s", c.PairStatus.PairWith))
	}
	if len(parts) == 0 {
		return "stats changed"
	}
	return strings.Join(parts, ", ")
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func handleLogs(w http.ResponseWriter, r *http.Request) {
	n := 100
	if s := r.URL.Query().Get("n"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			n = v
		}
	}
	typeFilter := r.URL.Query().Get("type")

	entries := ringLog.Last(n, typeFilter)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(entries)
}

func handleState(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	snapshot := struct {
		Connected   bool                 `json:"connected"`
		ConnectTime string               `json:"connect_time,omitempty"`
		State       *chatgear.StateEvent `json:"state,omitempty"`
		Stats       *chatgear.StatsEvent `json:"stats,omitempty"`
	}{
		Connected: connected,
		State:     lastState,
		Stats:     lastStats,
	}
	if connected {
		snapshot.ConnectTime = connectTime.Format("15:04:05")
	}
	mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(snapshot)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<title>ChatGear Logger — %s</title>
<meta http-equiv="refresh" content="3">
<style>
  body { font-family: monospace; background: #1a1a2e; color: #e0e0e0; padding: 20px; }
  h1 { color: #0f3460; }
  .entry { margin: 2px 0; padding: 4px 8px; border-left: 3px solid #333; }
  .state { border-color: #00d2ff; }
  .stats { border-color: #7b2ff7; }
  .audio { border-color: #2ecc71; }
  .connect { border-color: #f39c12; }
  .error { border-color: #e74c3c; }
  .time { color: #888; }
  .type { font-weight: bold; min-width: 70px; display: inline-block; }
  .type.state { color: #00d2ff; }
  .type.stats { color: #7b2ff7; }
  .type.audio { color: #2ecc71; }
  .type.connect { color: #f39c12; }
  .type.error { color: #e74c3c; }
  pre { background: #16213e; padding: 10px; border-radius: 4px; overflow-x: auto; }
</style>
</head>
<body>
<h1>ChatGear Logger — %s</h1>
<p>Auto-refresh every 3s. <a href="/logs?n=1000" style="color:#00d2ff">JSON (all)</a> | <a href="/state" style="color:#7b2ff7">State</a></p>
<div id="logs">Loading...</div>
<script>
fetch('/logs?n=200').then(r=>r.json()).then(entries=>{
  const el = document.getElementById('logs');
  el.innerHTML = entries.map(e =>
    '<div class="entry '+e.type+'">' +
    '<span class="time">'+e.time+'</span> ' +
    '<span class="type '+e.type+'">'+e.type+'</span> ' +
    e.summary +
    (e.detail ? '<pre>'+JSON.stringify(e.detail,null,2)+'</pre>' : '') +
    '</div>'
  ).join('');
});
</script>
</body>
</html>`, *gearID, *gearID)
}

// ============================================================================
// Main
// ============================================================================

func main() {
	flag.Parse()

	if *mqttAddr == "" || *gearID == "" {
		fmt.Fprintln(os.Stderr, "Usage: logger --mqtt=URL --namespace=NS --gear-id=ID [--port=8899]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Start MQTT monitor in background
	go monitorLoop(ctx)

	// HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", handleLogs)
	mux.HandleFunc("/state", handleState)
	mux.HandleFunc("/", handleIndex)

	addr := fmt.Sprintf(":%d", *httpPort)
	server := &http.Server{Addr: addr, Handler: mux}

	addLog("connect", fmt.Sprintf("HTTP server starting on %s", addr), nil)
	log.Printf("HTTP API: http://localhost:%d/logs", *httpPort)
	log.Printf("HTML UI:  http://localhost:%d/", *httpPort)

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}
