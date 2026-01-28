// image_gen demonstrates image generation with MiniMax API.
//
// Usage:
//
//	export MINIMAX_API_KEY=your-api-key
//	go run . "A beautiful sunset over mountains"
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
	model := flag.String("model", minimax.ModelImage01, "Model name")
	aspectRatio := flag.String("ratio", minimax.AspectRatio16x9, "Aspect ratio: 1:1, 16:9, 9:16, 4:3, etc.")
	n := flag.Int("n", 1, "Number of images to generate (1-9)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: image_gen [flags] <prompt>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable not set")
	}

	client := minimax.NewClient(apiKey)
	ctx := context.Background()

	prompt := flag.Arg(0)

	fmt.Printf("Generating %d image(s)...\n", *n)
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Printf("Aspect ratio: %s\n", *aspectRatio)

	resp, err := client.Image.Generate(ctx, &minimax.ImageGenerateRequest{
		Model:       *model,
		Prompt:      prompt,
		AspectRatio: *aspectRatio,
		N:           *n,
	})
	if err != nil {
		log.Fatalf("Image generation failed: %v", err)
	}

	fmt.Printf("\n✓ Generated %d image(s):\n", len(resp.Images))
	for i, img := range resp.Images {
		fmt.Printf("  [%d] %s\n", i+1, img.URL)
	}
}
