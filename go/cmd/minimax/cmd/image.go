package cmd

import (
	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
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

		var req mm.ImageGenerateRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = mm.ModelImage01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
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

		var req mm.ImageReferenceRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = mm.ModelImage01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageReferenceCmd)
}
