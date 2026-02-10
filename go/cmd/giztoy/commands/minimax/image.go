package minimax

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Image generation service",
	Long: `Image generation service.

Example request file (image.yaml):
  model: image-01
  prompt: A beautiful sunset over mountains
  aspect_ratio: "16:9"
  n: 1
  prompt_optimizer: true`,
}

var imageGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate image from text",
	Long: `Generate an image from text prompt.

Examples:
  giztoy minimax image generate -f image.yaml
  giztoy minimax image generate -f image.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.ImageGenerateRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Model == "" {
			req.Model = minimax.ModelImage01
		}

		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Image.Generate(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("image generation failed: %w", err)
		}

		for i, img := range resp.Images {
			printSuccess("Image %d: %s", i+1, img.URL)
		}

		return outputResult(resp, outputFile, outputJSON)
	},
}

var imageReferenceCmd = &cobra.Command{
	Use:   "reference",
	Short: "Generate image with reference",
	Long: `Generate an image using a reference image.

Example request file (image-ref.yaml):
  model: image-01
  prompt: Same style but with different colors
  image_prompt: https://example.com/reference.jpg
  image_prompt_strength: 0.7

Examples:
  giztoy minimax image reference -f image-ref.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.ImageReferenceRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Model == "" {
			req.Model = minimax.ModelImage01
		}

		printVerbose("Model: %s", req.Model)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Image.GenerateWithReference(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("image generation failed: %w", err)
		}

		for i, img := range resp.Images {
			printSuccess("Image %d: %s", i+1, img.URL)
		}

		return outputResult(resp, outputFile, outputJSON)
	},
}

func init() {
	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageReferenceCmd)
}
