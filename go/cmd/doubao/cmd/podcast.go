package cmd

import (
	"github.com/spf13/cobra"

	dsi "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

var podcastCmd = &cobra.Command{
	Use:   "podcast",
	Short: "Podcast synthesis service",
	Long: `Podcast synthesis service.

Create multi-speaker podcast audio from scripts.

Example request file (podcast.yaml):
  script:
    - speaker_id: zh_male_yangguang
      text: 大家好，欢迎收听今天的节目。
    - speaker_id: zh_female_cancan
      text: 是的，今天我们要聊的话题非常有趣。
    - speaker_id: zh_male_yangguang
      text: 让我们开始吧！
  encoding: mp3
  sample_rate: 24000`,
}

var podcastCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create podcast synthesis task",
	Long: `Create a podcast synthesis task.

Combines multiple speakers into a single audio file.
Returns a task ID for tracking.

Example request file (podcast.yaml):
  script:
    - speaker_id: zh_male_yangguang
      text: 大家好，欢迎收听今天的节目。
    - speaker_id: zh_female_cancan
      text: 是的，今天我们要聊的话题非常有趣。
  encoding: mp3
  sample_rate: 24000

Examples:
  doubao -c myctx podcast create -f podcast.yaml
  doubao -c myctx podcast create -f podcast.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.PodcastTaskRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Script segments: %d", len(req.Script))

		// TODO: Implement podcast create API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var podcastStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query podcast task status",
	Long: `Query the status of a podcast synthesis task.

Examples:
  doubao -c myctx podcast status task_12345
  doubao -c myctx podcast status task_12345 --json`,
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
			"_note":      "API call not implemented yet",
			"_context":   ctx.Name,
			"task_id":    taskID,
			"status":     "completed",
			"audio_url":  "https://example.com/podcast-output.mp3",
			"duration":   120.5,
			"created_at": "2024-01-01T10:00:00Z",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	podcastCmd.AddCommand(podcastCreateCmd)
	podcastCmd.AddCommand(podcastStatusCmd)
}
