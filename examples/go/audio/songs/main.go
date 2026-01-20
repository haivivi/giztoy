// Package main demonstrates the songs library with multi-voice rendering and playback.
//
// Usage:
//
//	go run main.go                  # Play all songs in loop
//	go run main.go -song=canon      # Play specific song
//	go run main.go -list            # List available songs
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/audio/portaudio"
	"github.com/haivivi/giztoy/pkg/audio/songs"
)

var (
	songID = flag.String("song", "", "Song ID to play (empty = all songs)")
	list   = flag.Bool("list", false, "List all available songs")
	volume = flag.Float64("volume", 0.5, "Volume (0.0-1.0)")
	loop   = flag.Bool("loop", true, "Loop playback")
)

func main() {
	flag.Parse()

	if *list {
		listSongs()
		return
	}

	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize PortAudio: %v\n", err)
		os.Exit(1)
	}
	defer portaudio.Terminate()

	// Setup signal handler for graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{})
	go func() {
		<-sigCh
		fmt.Println("\n\n🛑 Stopping...")
		close(stopCh)
	}()

	// Determine which songs to play
	var playlist []songs.Song
	if *songID != "" {
		song := songs.ByID(*songID)
		if song == nil {
			fmt.Fprintf(os.Stderr, "Song not found: %s\n", *songID)
			fmt.Fprintln(os.Stderr, "Use -list to see available songs")
			os.Exit(1)
		}
		playlist = []songs.Song{*song}
	} else {
		playlist = songs.All
	}

	fmt.Printf("🎵 Playlist: %d songs, Volume: %.0f%%\n", len(playlist), *volume*100)
	fmt.Println("   Press Ctrl+C to stop\n")

	// Play loop
	for {
		for i, song := range playlist {
			select {
			case <-stopCh:
				return
			default:
			}

			fmt.Printf("▶️  [%d/%d] %s (%s)\n", i+1, len(playlist), song.Name, song.ID)

			if err := playSong(&song, stopCh); err != nil {
				fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
			}

			// Brief pause between songs
			select {
			case <-stopCh:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}

		if !*loop {
			break
		}
		fmt.Println("\n🔄 Restarting playlist...\n")
	}
}

func playSong(song *songs.Song, stopCh <-chan struct{}) error {
	// Configure rendering
	opts := songs.RenderOptions{
		Format:    pcm.L16Mono16K,
		Volume:    *volume,
		Metronome: false,
		RichSound: true,
	}

	// Render to bytes (pre-generate for proper mixing)
	data, err := song.RenderBytes(opts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	// Open audio stream (Start is called internally by NewOutputStream)
	stream, err := portaudio.NewOutputStream(opts.Format, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	// Play with progress
	totalSamples := len(data) / 2
	totalDuration := opts.Format.Duration(int64(len(data)))
	chunkSize := opts.Format.BytesRate() / 10 // 100ms chunks

	played := 0
	startTime := time.Now()

	for played < len(data) {
		select {
		case <-stopCh:
			return nil
		default:
		}

		end := played + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := opts.Format.DataChunk(data[played:end])
		if err := stream.WriteChunk(chunk); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		played = end

		// Show progress
		progress := float64(played) / float64(len(data)) * 100
		elapsed := time.Since(startTime)

		fmt.Printf("\r   ⏱️  %.1f%% | %v / %v | %d samples",
			progress, elapsed.Truncate(time.Second), totalDuration.Truncate(time.Second), totalSamples)
	}

	fmt.Println(" ✓")
	return nil
}

func listSongs() {
	fmt.Println("Available songs:")
	fmt.Println()

	// Group by category
	categories := map[string][]songs.Song{
		"儿歌":   {},
		"古典钢琴": {},
		"巴赫":   {},
		"练习曲":  {},
		"音阶":   {},
		"舞曲":   {},
	}

	for _, s := range songs.All {
		switch {
		case strings.Contains(s.ID, "czerny") || strings.Contains(s.ID, "hanon") || strings.Contains(s.ID, "burgmuller"):
			categories["练习曲"] = append(categories["练习曲"], s)
		case strings.Contains(s.ID, "bach") || strings.Contains(s.ID, "canon_3voice"):
			categories["巴赫"] = append(categories["巴赫"], s)
		case strings.Contains(s.ID, "scale"):
			categories["音阶"] = append(categories["音阶"], s)
		case strings.Contains(s.ID, "waltz") || strings.Contains(s.ID, "tarantella"):
			categories["舞曲"] = append(categories["舞曲"], s)
		case strings.Contains(s.ID, "twinkle") || strings.Contains(s.ID, "happy") || strings.Contains(s.ID, "tiger") || strings.Contains(s.ID, "doll"):
			categories["儿歌"] = append(categories["儿歌"], s)
		default:
			categories["古典钢琴"] = append(categories["古典钢琴"], s)
		}
	}

	order := []string{"儿歌", "古典钢琴", "巴赫", "练习曲", "音阶", "舞曲"}
	for _, cat := range order {
		songList := categories[cat]
		if len(songList) == 0 {
			continue
		}
		fmt.Printf("【%s】\n", cat)
		for _, s := range songList {
			voices := s.ToVoices(false)
			dur := float64(s.Duration()) / 1000
			fmt.Printf("  %-20s %s (%d声部, %.0fs)\n",
				s.ID, s.Name, len(voices), dur)
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d songs\n", len(songs.All))
}
