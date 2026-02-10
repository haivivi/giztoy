// music_gen demonstrates music generation with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . -o output.mp3 \
//	  -prompt "Pop music, happy mood, suitable for morning" \
//	  -lyrics "[Verse]\nHello world\nIt's a beautiful day\n[Chorus]\nLet's celebrate"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

func main() {
	prompt := flag.String("prompt", "", "Music style description (10-300 chars)")
	lyrics := flag.String("lyrics", "", "Song lyrics (10-600 chars)")
	output := flag.String("o", "music.mp3", "Output file path")
	flag.Parse()

	if *prompt == "" || *lyrics == "" {
		fmt.Fprintln(os.Stderr, "Usage: music_gen -prompt <style> -lyrics <lyrics> [-o output.mp3]")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, `  go run . -prompt "Pop music, happy" -lyrics "[Verse]\nHello\n[Chorus]\nWorld"`)
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable not set")
	}

	client := minimax.NewClient(apiKey)
	ctx := context.Background()

	fmt.Println("Generating music...")
	fmt.Printf("Prompt: %s\n", *prompt)
	fmt.Printf("Lyrics: %s\n", *lyrics)

	resp, err := client.Music.Generate(ctx, &minimax.MusicRequest{
		Model:  minimax.ModelMusic20,
		Prompt: *prompt,
		Lyrics: *lyrics,
		Format: "mp3",
	})
	if err != nil {
		log.Fatalf("Music generation failed: %v", err)
	}

	if err := os.WriteFile(*output, resp.Audio, 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Printf("\n✓ Saved to %s\n", *output)
	fmt.Printf("Duration: %d ms\n", resp.Duration)
	if resp.ExtraInfo != nil {
		fmt.Printf("Size: %d bytes\n", resp.ExtraInfo.AudioSize)
	}
}
