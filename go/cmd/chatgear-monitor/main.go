// ChatGear MQTT Monitor — subscribes to ALL device topics via wildcard.
//
// Usage:
//
//	cd go && go build -o /tmp/chatgear-monitor ./cmd/chatgear-monitor
//	/tmp/chatgear-monitor --gear-id 693b0fb7839769199432f516
package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/mqtt0"
)

func main() {
	mqttURL := flag.String("mqtt", "mqtts://admin:isA953Nx56EBfEu@mqtt.stage.haivivi.cn:8883", "MQTT broker URL")
	scope := flag.String("scope", "RyBFG6", "Topic scope/namespace")
	gearID := flag.String("gear-id", "693b0fb7839769199432f516", "Gear ID to monitor")
	logFile := flag.String("log", "", "Log file path (default: stdout)")
	flag.Parse()

	// Optional log to file
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatalf("open log: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	s := *scope
	if s != "" && !strings.HasSuffix(s, "/") {
		s += "/"
	}

	prefix := fmt.Sprintf("%sdevice/%s/", s, *gearID)
	wildcard := prefix + "#"

	log.Printf("ChatGear Monitor")
	log.Printf("  Scope:   %s", *scope)
	log.Printf("  Gear ID: %s", *gearID)
	log.Printf("  Topic:   %s", wildcard)
	log.Printf("---")

	// Parse URL
	u, err := url.Parse(*mqttURL)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}
	var username string
	var password []byte
	if u.User != nil {
		username = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = []byte(p)
		}
	}
	host := u.Hostname()
	port := u.Port()
	useTLS := u.Scheme == "mqtts" || u.Scheme == "ssl"
	if port == "" {
		if useTLS {
			port = "8883"
		} else {
			port = "1883"
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := fmt.Sprintf("%s:%s", host, port)
	cfg := mqtt0.ClientConfig{
		Addr:            "tcp://" + addr,
		ClientID:        fmt.Sprintf("monitor-%d", time.Now().UnixNano()%10000),
		Username:        username,
		Password:        password,
		KeepAlive:       60,
		ConnectTimeout:  30 * time.Second,
		ProtocolVersion: mqtt0.ProtocolV5,
	}

	if useTLS {
		tlsHost := host
		cfg.Dialer = func(_ context.Context, _ string, _ *tls.Config) (net.Conn, error) {
			return tls.Dial("tcp", addr, &tls.Config{ServerName: tlsHost})
		}
	}

	log.Printf("Connecting to %s (TLS=%v, v5)...", addr, useTLS)
	client, err := mqtt0.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()
	log.Printf("Connected!")

	// Subscribe to all 5 device topics explicitly
	topics := []string{
		prefix + "state",
		prefix + "stats",
		prefix + "input_audio_stream",
		prefix + "output_audio_stream",
		prefix + "command",
	}
	if err := client.Subscribe(ctx, topics...); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	log.Printf("Subscribed to %d topics under %s", len(topics), prefix)
	log.Printf("Waiting for messages...")
	log.Printf("===")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	audioUpCount := 0
	audioDownCount := 0
	lastAudioLog := time.Now()

	for {
		msg, err := client.RecvTimeout(500 * time.Millisecond)
		if err != nil {
			if !client.IsRunning() {
				return
			}
			log.Printf("recv error: %v", err)
			return
		}
		if msg == nil {
			// Periodic audio summary
			if (audioUpCount > 0 || audioDownCount > 0) && time.Since(lastAudioLog) > time.Second {
				if audioUpCount > 0 {
					log.Printf("🎤 [AUDIO UP]   %d frames/s", audioUpCount)
				}
				if audioDownCount > 0 {
					log.Printf("🔊 [AUDIO DOWN] %d frames/s", audioDownCount)
				}
				audioUpCount = 0
				audioDownCount = 0
				lastAudioLog = time.Now()
			}
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		// Extract suffix
		suffix := msg.Topic
		if strings.HasPrefix(msg.Topic, prefix) {
			suffix = msg.Topic[len(prefix):]
		}

		ts := time.Now().Format("15:04:05.000")

		switch suffix {
		case "state":
			log.Printf("[%s] 📡 STATE      %s", ts, string(msg.Payload))
		case "stats":
			log.Printf("[%s] 📊 STATS      %s", ts, string(msg.Payload))
		case "input_audio_stream":
			audioUpCount++
		case "output_audio_stream":
			audioDownCount++
		case "command":
			log.Printf("[%s] ⚡ COMMAND    %s", ts, string(msg.Payload))
		default:
			log.Printf("[%s] ❓ %s  %d bytes: %s", ts, suffix, len(msg.Payload), hex.EncodeToString(msg.Payload[:min(32, len(msg.Payload))]))
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
