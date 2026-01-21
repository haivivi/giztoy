package commands

import (
	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

var meetingCmd = &cobra.Command{
	Use:   "meeting",
	Short: "Meeting transcription service",
	Long: `Meeting transcription service.

Transcribe meeting recordings with speaker diarization.

Example request file (meeting.yaml):
  audio_url: https://example.com/meeting.mp3
  language: zh-CN
  speaker_count: 3
  enable_speaker_diarization: true
  enable_timestamp: true`,
}

var meetingCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create meeting transcription task",
	Long: `Create a meeting transcription task.

Supports speaker diarization and timestamp generation.
Returns a task ID for tracking.

Example request file (meeting.yaml):
  audio_url: https://example.com/meeting.mp3
  language: zh-CN
  speaker_count: 3
  enable_speaker_diarization: true
  enable_timestamp: true

Examples:
  doubaospeech -c myctx meeting create -f meeting.yaml
  doubaospeech -c myctx meeting create -f meeting.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.MeetingTaskRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement meeting create API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var meetingStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query meeting task status",
	Long: `Query the status of a meeting transcription task.

Examples:
  doubaospeech -c myctx meeting status task_12345
  doubaospeech -c myctx meeting status task_12345 --json`,
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
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"task_id":  taskID,
			"status":   "completed",
			"result": map[string]any{
				"speakers": []map[string]any{
					{"speaker_id": "speaker_1", "segments": 10},
					{"speaker_id": "speaker_2", "segments": 8},
				},
				"transcript": "Meeting transcript would appear here...",
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	meetingCmd.AddCommand(meetingCreateCmd)
	meetingCmd.AddCommand(meetingStatusCmd)
}
