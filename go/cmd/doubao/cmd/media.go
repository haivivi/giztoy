package cmd

import (
	"github.com/spf13/cobra"

	dsi "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
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
  doubao -c myctx media subtitle -f subtitle.yaml
  doubao -c myctx media subtitle -f subtitle.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.SubtitleRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement subtitle extraction API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var mediaStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query media task status",
	Long: `Query the status of a media processing task.

Examples:
  doubao -c myctx media status task_12345
  doubao -c myctx media status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Querying task: %s", taskID)

		// TODO: Implement task status query
		result := map[string]any{
			"_note":        "API call not implemented yet",
			"_context":     ctx.Name,
			"task_id":      taskID,
			"status":       "completed",
			"subtitle_url": "https://example.com/output.srt",
			"format":       "srt",
			"segments":     42,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	mediaCmd.AddCommand(mediaSubtitleCmd)
	mediaCmd.AddCommand(mediaStatusCmd)
}
