// dashscope demonstrates text embedding with DashScope API.
//
// Usage:
//
//	export DASHSCOPE_API_KEY=your-api-key
//	go run . "你好世界"
//	go run . -batch "你好" "世界" "恐龙"
//	go run . -model text-embedding-v3 -dim 512 "hello"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/haivivi/giztoy/go/pkg/embed"
)

func main() {
	model := flag.String("model", embed.ModelDashScopeV4, "Model name")
	dim := flag.Int("dim", 1024, "Vector dimension")
	batch := flag.Bool("batch", false, "Batch mode: embed all args at once")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: dashscope [flags] <text> [text...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY environment variable not set")
	}

	e := embed.NewDashScope(apiKey,
		embed.WithModel(*model),
		embed.WithDimension(*dim),
	)

	ctx := context.Background()

	fmt.Printf("Model:     %s\n", *model)
	fmt.Printf("Dimension: %d\n", e.Dimension())
	fmt.Println()

	if *batch {
		texts := flag.Args()
		vecs, err := e.EmbedBatch(ctx, texts)
		if err != nil {
			log.Fatalf("EmbedBatch failed: %v", err)
		}
		for i, vec := range vecs {
			printVector(texts[i], vec)
		}
	} else {
		text := flag.Arg(0)
		vec, err := e.Embed(ctx, text)
		if err != nil {
			log.Fatalf("Embed failed: %v", err)
		}
		printVector(text, vec)
	}
}

func printVector(text string, vec []float32) {
	fmt.Printf("Text: %q\n", text)
	fmt.Printf("Dims: %d\n", len(vec))
	// Print first 8 and last 4 values as preview.
	n := len(vec)
	if n <= 12 {
		fmt.Printf("Vector: %v\n", vec)
	} else {
		fmt.Printf("Vector: [%v ... %v]\n", vec[:8], vec[n-4:])
	}
	fmt.Println()
}
