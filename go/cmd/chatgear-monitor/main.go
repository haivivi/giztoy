// ChatGear MQTT Monitor — subscribes to device uplink topics and prints messages.
//
// Usage:
//
//	cd go && go run ../e2e/chatgear/monitor
package main

import (
	"context"
	"crypto/tls"
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
	gearID := flag.String("gear-id", "zig-test-001", "Gear ID to monitor")
	flag.Parse()

	s := *scope
	if s != "" && !strings.HasSuffix(s, "/") {
		s += "/"
	}

	prefix := fmt.Sprintf("%sdevice/%s/", s, *gearID)

	log.Printf("ChatGear Monitor")
	log.Printf("  Scope:   %s", *scope)
	log.Printf("  Gear ID: %s", *gearID)
	log.Printf("  Topics:  %s{state,stats,input_audio_stream}", prefix)
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
		Addr:            "tcp://" + addr, // dialer won't be used
		ClientID:        fmt.Sprintf("monitor-%d", time.Now().UnixNano()%10000),
		Username:        username,
		Password:        password,
		KeepAlive:       60,
		ConnectTimeout:  30 * time.Second,
		ProtocolVersion: mqtt0.ProtocolV5,
	}

	// Custom dialer for TLS with correct ServerName
	if useTLS {
		tlsHost := host
		cfg.Dialer = func(_ context.Context, _ string, _ *tls.Config) (net.Conn, error) {
			return tls.Dial("tcp", addr, &tls.Config{ServerName: tlsHost})
		}
	}

	log.Printf("Connecting to %s (TLS=%v)...", addr, useTLS)
	client, err := mqtt0.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()
	log.Printf("Connected!")

	// Subscribe to device uplink topics
	stateTopic := prefix + "state"
	statsTopic := prefix + "stats"
	audioTopic := prefix + "input_audio_stream"

	if err := client.Subscribe(ctx, stateTopic, statsTopic, audioTopic); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	log.Printf("Subscribed! Waiting for messages...")
	log.Printf("---")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	audioCount := 0
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
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		switch msg.Topic {
		case stateTopic:
			log.Printf("📡 [STATE]  %s", string(msg.Payload))
		case statsTopic:
			log.Printf("📊 [STATS]  %s", string(msg.Payload))
		case audioTopic:
			audioCount++
			if time.Since(lastAudioLog) > time.Second {
				log.Printf("🎤 [AUDIO]  %d frames/s (%d bytes)", audioCount, len(msg.Payload))
				audioCount = 0
				lastAudioLog = time.Now()
			}
		default:
			log.Printf("❓ [%s] %d bytes", msg.Topic, len(msg.Payload))
		}
	}
}
