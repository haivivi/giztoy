package doubao

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
	Long: `Extract subtitles and captions from audio/video files.

Example request file (subtitle.yaml):
  media_url: https://example.com/video.mp4
  language: zh-CN
  format: srt`,
}

var mediaSubtitleCmd = &cobra.Command{
	Use:   "subtitle",
	Short: "Extract subtitles from media",
	Long: `Extract subtitles from audio/video files.

Examples:
  giztoy doubao media subtitle -f subtitle.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.SubtitleRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Media.ExtractSubtitle(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("subtitle extraction failed: %w", err)
		}

		printSuccess("Subtitle extraction task submitted!")
		printInfo("Task ID: %s", task.ID)

		return outputResult(map[string]any{
			"task_id": task.ID,
			"status":  "submitted",
		}, outputFile, outputJSON)
	},
}

var mediaStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query media task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		client, err := createClient()
		if err != nil {
			return err
		}

		printVerbose("Querying task: %s", taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.Media.GetSubtitleTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query subtitle task failed: %w", err)
		}

		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)

		result := map[string]any{
			"task_id":  status.TaskID,
			"status":   status.Status,
			"progress": status.Progress,
		}
		if status.Result != nil {
			result["subtitle_url"] = status.Result.SubtitleURL
			result["duration"] = status.Result.Duration
		}
		return outputResult(result, outputFile, outputJSON)
	},
}

func init() {
	mediaCmd.AddCommand(mediaSubtitleCmd)
	mediaCmd.AddCommand(mediaStatusCmd)
}
