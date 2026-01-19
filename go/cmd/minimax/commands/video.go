package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/minimax"
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

Use --wait to wait for the task to complete and optionally download the result.

Examples:
  minimax -c myctx video t2v -f t2v.yaml
  minimax -c myctx video t2v -f t2v.yaml --wait
  minimax -c myctx video t2v -f t2v.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.TextToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = minimax.ModelHailuo23
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateTextToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}

		printSuccess("Video task created: %s", task.ID)

		// Check if --wait flag is set
		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		result := map[string]any{
			"task_id": task.ID,
			"status":  "created",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoI2VCmd = &cobra.Command{
	Use:   "i2v",
	Short: "Create image-to-video task",
	Long: `Create an image-to-video generation task.

Use --wait to wait for the task to complete and optionally download the result.

Examples:
  minimax -c myctx video i2v -f i2v.yaml
  minimax -c myctx video i2v -f i2v.yaml --wait
  minimax -c myctx video i2v -f i2v.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.ImageToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = minimax.ModelI2V01
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateImageToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}

		printSuccess("Video task created: %s", task.ID)

		// Check if --wait flag is set
		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		result := map[string]any{
			"task_id": task.ID,
			"status":  "created",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoFrameCmd = &cobra.Command{
	Use:   "frame",
	Short: "Create frame-to-video task",
	Long: `Create a frame-to-video generation task (first and last frame).

Use --wait to wait for the task to complete and optionally download the result.

Example request file (frame.yaml):
  model: MiniMax-Hailuo-2.3
  prompt: Smooth transition between frames
  first_frame_image: https://example.com/start.jpg
  last_frame_image: https://example.com/end.jpg

Examples:
  minimax -c myctx video frame -f frame.yaml
  minimax -c myctx video frame -f frame.yaml --wait -o output.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.FrameToVideoRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Video.CreateFrameToVideo(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create video task failed: %w", err)
		}

		printSuccess("Video task created: %s", task.ID)

		// Check if --wait flag is set
		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			return waitAndDownloadVideo(client, task.ID)
		}

		result := map[string]any{
			"task_id": task.ID,
			"status":  "created",
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

		// Create API client
		client := createClient(ctx)

		// Create a task to check status
		task := client.NewVideoTask(taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := task.Status(reqCtx)
		if err != nil {
			return fmt.Errorf("get task status failed: %w", err)
		}

		result := map[string]any{
			"task_id": taskID,
			"status":  status,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var videoWaitCmd = &cobra.Command{
	Use:   "wait <task_id>",
	Short: "Wait for video generation task to complete",
	Long: `Wait for a video generation task to complete and optionally download the video.

Examples:
  minimax -c myctx video wait task-123
  minimax -c myctx video wait task-123 --json
  minimax -c myctx video wait task-123 -o output.mp4`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Waiting for task: %s", taskID)

		// Create API client
		client := createClient(ctx)

		// Create task and wait
		task := client.NewVideoTask(taskID)

		printInfo("Waiting for video task %s to complete...", taskID)

		// Wait for completion (up to 30 minutes)
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		result, err := task.Wait(reqCtx)
		if err != nil {
			return fmt.Errorf("wait for task failed: %w", err)
		}

		printSuccess("Video task completed: %s", result.FileID)

		// Download video if output file is specified and download URL is available
		outputFile := getOutputFile()
		if outputFile != "" && result.DownloadURL != "" {
			printInfo("Downloading video to %s...", outputFile)

			if err := downloadFile(result.DownloadURL, outputFile); err != nil {
				return err
			}
		}

		output := map[string]any{
			"task_id":      taskID,
			"status":       "Success",
			"file_id":      result.FileID,
			"download_url": result.DownloadURL,
		}

		// If output file is specified and not JSON mode, don't output JSON
		if outputFile != "" && !isJSONOutput() {
			return nil
		}

		return outputResult(output, "", isJSONOutput())
	},
}

// waitAndDownloadVideo waits for a video task to complete and downloads the result.
func waitAndDownloadVideo(client *minimax.Client, taskID string) error {
	task := client.NewVideoTask(taskID)

	printInfo("Waiting for video task %s to complete...", taskID)

	// Wait for completion (up to 30 minutes)
	reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := task.Wait(reqCtx)
	if err != nil {
		return fmt.Errorf("wait for task failed: %w", err)
	}

	printSuccess("Video task completed: %s", result.FileID)

	// Download video if output file is specified and download URL is available
	outputFile := getOutputFile()
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

	return outputResult(output, "", isJSONOutput())
}

// downloadFile downloads a file from URL with timeout and proper error handling.
func downloadFile(url, outputFile string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Minute, // Allow up to 10 minutes for large video downloads
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}

	// Copy content and track bytes written
	written, copyErr := io.Copy(out, resp.Body)

	// Always try to close the file
	closeErr := out.Close()

	// Handle errors - prefer copyErr over closeErr
	if copyErr != nil {
		os.Remove(outputFile) // Clean up partial file
		return fmt.Errorf("write file failed: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(outputFile) // Clean up on close error
		return fmt.Errorf("close file failed: %w", closeErr)
	}

	printSuccess("File saved to %s (%s)", outputFile, formatBytes(int(written)))
	return nil
}

func init() {
	// Add --wait flag to video generation commands
	videoT2VCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")
	videoI2VCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")
	videoFrameCmd.Flags().Bool("wait", false, "Wait for task to complete and download result")

	videoCmd.AddCommand(videoT2VCmd)
	videoCmd.AddCommand(videoI2VCmd)
	videoCmd.AddCommand(videoFrameCmd)
	videoCmd.AddCommand(videoStatusCmd)
	videoCmd.AddCommand(videoWaitCmd)
}
