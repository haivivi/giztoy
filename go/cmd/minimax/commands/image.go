package commands

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

Supports text-to-image and image-to-image generation.

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
  minimax -c myctx image generate -f image.yaml
  minimax -c myctx image generate -f image.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.ImageGenerateRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = minimax.ModelImage01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Image.Generate(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("image generation failed: %w", err)
		}

		// Output image URLs
		for i, img := range resp.Images {
			printSuccess("Image %d: %s", i+1, img.URL)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
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
  aspect_ratio: "1:1"

Examples:
  minimax -c myctx image reference -f image-ref.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.ImageReferenceRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = minimax.ModelImage01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Image.GenerateWithReference(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("image generation failed: %w", err)
		}

		// Output image URLs
		for i, img := range resp.Images {
			printSuccess("Image %d: %s", i+1, img.URL)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

func init() {
	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageReferenceCmd)
}
