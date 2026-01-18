package cmd

import (
	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
)

var videoCmd = &cobra.Command{
	Use:   "video",
	Short: "Video generation service",
	Long: `Video generation service.

Supports text-to-video (T2V), image-to-video (I2V), and other generation modes.

Example T2V request file (t2v.yaml):
  model: MiniMax-Hailuo-2.3
  prompt: A cat playing with a ball in a sunny garden
  duration: 6
  resolution: 1080P

Example I2V request file (i2v.yaml):
  model: I2V-01
  prompt: The cat starts running
  first_frame_image: https://example.com/cat.jpg`,
}

var videoT2VCmd = &cobra.Command{
	Use:   "t2v",
	Short: "Create text-to-video task",
	Long: `Create a text-to-video generation task.

Examples:
  minimax -c myctx video t2v -f t2v.yaml
  minimax -c myctx video t2v -f t2v.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.TextToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = mm.ModelHailuo23
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoI2VCmd = &cobra.Command{
	Use:   "i2v",
	Short: "Create image-to-video task",
	Long: `Create an image-to-video generation task.

Examples:
  minimax -c myctx video i2v -f i2v.yaml
  minimax -c myctx video i2v -f i2v.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.ImageToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = mm.ModelI2V01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoFrameCmd = &cobra.Command{
	Use:   "frame",
	Short: "Create frame-to-video task",
	Long: `Create a frame-to-video generation task (first and last frame).

Example request file (frame.yaml):
  model: MiniMax-Hailuo-2.3
  prompt: Smooth transition between frames
  first_frame_image: https://example.com/start.jpg
  last_frame_image: https://example.com/end.jpg

Examples:
  minimax -c myctx video frame -f frame.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.FrameToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Check video generation task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Task ID: %s", taskID)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"task_id":  taskID,
			"status":   "placeholder",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	videoCmd.AddCommand(videoT2VCmd)
	videoCmd.AddCommand(videoI2VCmd)
	videoCmd.AddCommand(videoFrameCmd)
	videoCmd.AddCommand(videoStatusCmd)
}
