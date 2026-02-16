// gen_reference generates reference inference outputs from Go
// for cross-language validation with Rust.
//
// Usage: go run ./testdata/compat/inference/gen_reference.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/pkg/ncnn"
	"github.com/haivivi/giztoy/go/pkg/onnx"
)

type ReferenceOutput struct {
	Engine    string    `json:"engine"`
	Model     string    `json:"model"`
	InputW    int       `json:"input_w"`
	InputH    int       `json:"input_h"`
	Embedding []float32 `json:"embedding"`
}

func main() {
	// Same input as Go tests: data[i] = float32(i%100) * 0.01
	T := 40
	data := make([]float32, T*80)
	for i := range data {
		data[i] = float32(i%100) * 0.01
	}

	var outputs []ReferenceOutput

	// ncnn speaker
	{
		net, err := ncnn.LoadModel(ncnn.ModelSpeakerERes2Net)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ncnn LoadModel: %v\n", err)
			os.Exit(1)
		}
		defer net.Close()

		input, err := ncnn.NewMat2D(80, T, data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ncnn NewMat2D: %v\n", err)
			os.Exit(1)
		}
		defer input.Close()

		ex, err := net.NewExtractor()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ncnn NewExtractor: %v\n", err)
			os.Exit(1)
		}
		defer ex.Close()

		ex.SetInput("in0", input)
		output, err := ex.Extract("out0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ncnn Extract: %v\n", err)
			os.Exit(1)
		}
		defer output.Close()

		emb := output.FloatData()
		outputs = append(outputs, ReferenceOutput{
			Engine:    "ncnn",
			Model:     "speaker-eres2net",
			InputW:    80,
			InputH:    T,
			Embedding: emb,
		})
		fmt.Printf("ncnn speaker: %d dims, first 5: %v\n", len(emb), emb[:5])
	}

	// onnx speaker
	{
		env, err := onnx.NewEnv("gen")
		if err != nil {
			fmt.Fprintf(os.Stderr, "onnx NewEnv: %v\n", err)
			os.Exit(1)
		}
		defer env.Close()

		session, err := onnx.LoadModel(env, onnx.ModelSpeakerERes2Net)
		if err != nil {
			fmt.Fprintf(os.Stderr, "onnx LoadModel: %v\n", err)
			os.Exit(1)
		}
		defer session.Close()

		input, err := onnx.NewTensor([]int64{1, int64(T), 80}, data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "onnx NewTensor: %v\n", err)
			os.Exit(1)
		}
		defer input.Close()

		outs, err := session.Run([]string{"x"}, []*onnx.Tensor{input}, []string{"embedding"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "onnx Run: %v\n", err)
			os.Exit(1)
		}
		defer outs[0].Close()

		emb, err := outs[0].FloatData()
		if err != nil {
			fmt.Fprintf(os.Stderr, "onnx FloatData: %v\n", err)
			os.Exit(1)
		}
		outputs = append(outputs, ReferenceOutput{
			Engine:    "onnx",
			Model:     "speaker-eres2net",
			InputW:    80,
			InputH:    T,
			Embedding: emb,
		})
		fmt.Printf("onnx speaker: %d dims, first 5: %v\n", len(emb), emb[:5])
	}

	f, err := os.Create("testdata/compat/inference/reference.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(outputs); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote testdata/compat/inference/reference.json")
}
