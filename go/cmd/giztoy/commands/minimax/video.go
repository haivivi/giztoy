package minimax

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var videoCmd = &cobra.Command{
	Use:   "video",
	Short: "Video generation service",
	Long: `Video generation service.

Supports text-to-video (T2V), image-to-video (I2V), and frame-to-video generation.

Example T2V request file (t2v.yaml):
  model: MiniMax-Hailuo-2.3
  prompt: A cat playing with a ball in a sunny garden
  duration: 6
  resolution: 1080P`,
}

var videoT2VCmd = &cobra.Command{
	Use:   "t2v",
	Short: "Create text-to-video task",
	Long: `Create a text-to-video generation task.

Use --wait to wait for the task to complete and optionally download the result.

Examples:
  giztoy minimax video t2v -f t2v.yaml
  giztoy minimax video t2v -f t2v.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.TextToVideoRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Model == "" {
			req.Model = minimax.ModelHailuo23
		}

		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateTextToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}
		printSuccess("Video task created: %s", task.ID)

		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		return outputResult(map[string]any{"task_id": task.ID, "status": "created"}, outputFile, outputJSON)
	},
}

var videoI2VCmd = &cobra.Command{
	Use:   "i2v",
	Short: "Create image-to-video task",
	Long: `Create an image-to-video generation task.

Examples:
  giztoy minimax video i2v -f i2v.yaml
  giztoy minimax video i2v -f i2v.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.ImageToVideoRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Model == "" {
			req.Model = minimax.ModelI2V01
		}

		printVerbose("Model: %s", req.Model)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateImageToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}
		printSuccess("Video task created: %s", task.ID)

		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		return outputResult(map[string]any{"task_id": task.ID, "status": "created"}, outputFile, outputJSON)
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
  giztoy minimax video frame -f frame.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.FrameToVideoRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Model: %s", req.Model)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateFrameToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}
		printSuccess("Video task created: %s", task.ID)

		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		return outputResult(map[string]any{"task_id": task.ID, "status": "created"}, outputFile, outputJSON)
	},
}

var videoStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Check video generation task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		printVerbose("Task ID: %s", taskID)

		client, err := createClient()
		if err != nil {
			return err
		}

		task := client.NewVideoTask(taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := task.Status(reqCtx)
		if err != nil {
			return fmt.Errorf("get task status failed: %w", err)
		}

		return outputResult(map[string]any{"task_id": taskID, "status": status}, outputFile, outputJSON)
	},
}

var videoWaitCmd = &cobra.Command{
	Use:   "wait <task_id>",
	Short: "Wait for video generation task to complete",
	Long: `Wait for a video generation task to complete and optionally download the video.

Examples:
  giztoy minimax video wait task-123
  giztoy minimax video wait task-123 -o output.mp4`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		printVerbose("Waiting for task: %s", taskID)

		client, err := createClient()
		if err != nil {
			return err
		}

		return waitAndDownloadVideo(client, taskID)
	},
}

// waitAndDownloadVideo waits for a video task to complete and downloads the result.
func waitAndDownloadVideo(client *minimax.Client, taskID string) error {
	task := client.NewVideoTask(taskID)

	printInfo("Waiting for video task %s to complete...", taskID)

	reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := task.Wait(reqCtx)
	if err != nil {
		return fmt.Errorf("wait for task failed: %w", err)
	}

	printSuccess("Video task completed: %s", result.FileID)

	if outputFile != "" && result.DownloadURL != "" {
		printInfo("Downloading video to %s...", outputFile)
		if err := downloadFile(result.DownloadURL, outputFile); err != nil {
			return err
		}
		return nil
	}

	output := map[string]any{
		"task_id":      taskID,
		"status":       "Success",
		"file_id":      result.FileID,
		"download_url": result.DownloadURL,
	}
	return outputResult(output, "", outputJSON)
}

func init() {
	videoT2VCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")
	videoI2VCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")
	videoFrameCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")

	videoCmd.AddCommand(videoT2VCmd)
	videoCmd.AddCommand(videoI2VCmd)
	videoCmd.AddCommand(videoFrameCmd)
	videoCmd.AddCommand(videoStatusCmd)
	videoCmd.AddCommand(videoWaitCmd)
}
