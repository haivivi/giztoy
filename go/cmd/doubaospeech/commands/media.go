package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Media processing service",
	Long: `Media processing service.

Extract subtitles and captions from audio/video files.

Example request file (subtitle.yaml):
  media_url: https://example.com/video.mp4
  language: zh-CN
  format: srt
  enable_translation: true
  target_language: en-US`,
}

var mediaSubtitleCmd = &cobra.Command{
	Use:   "subtitle",
	Short: "Extract subtitles from media",
	Long: `Extract subtitles from audio/video files.

Supports various output formats (srt, vtt, txt).
Can optionally translate subtitles to another language.

Example request file (subtitle.yaml):
  media_url: https://example.com/video.mp4
  language: zh-CN
  format: srt
  enable_translation: true
  target_language: en-US

Examples:
  doubaospeech -c myctx media subtitle -f subtitle.yaml
  doubaospeech -c myctx media subtitle -f subtitle.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var req ds.SubtitleRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Media.ExtractSubtitle(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("subtitle extraction failed: %w", err)
		}

		printSuccess("Subtitle extraction task submitted!")
		printInfo("Task ID: %s", task.ID)
		printInfo("Use 'doubaospeech media status %s' to check status", task.ID)

		result := map[string]any{
			"task_id": task.ID,
			"status":  "submitted",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var mediaStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query media task status",
	Long: `Query the status of a media processing task.

Examples:
  doubaospeech -c myctx media status task_12345
  doubaospeech -c myctx media status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Querying task: %s", taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.Media.GetSubtitleTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query subtitle task failed: %w", err)
		}

		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)
		if status.Progress > 0 {
			printInfo("Progress: %d%%", status.Progress)
		}
		if status.Result != nil && status.Result.SubtitleURL != "" {
			printSuccess("Subtitle URL: %s", status.Result.SubtitleURL)
		}

		result := map[string]any{
			"task_id":  status.TaskID,
			"status":   status.Status,
			"progress": status.Progress,
		}
		if status.Result != nil {
			result["subtitle_url"] = status.Result.SubtitleURL
			result["duration"] = status.Result.Duration
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	mediaCmd.AddCommand(mediaSubtitleCmd)
	mediaCmd.AddCommand(mediaStatusCmd)
}
