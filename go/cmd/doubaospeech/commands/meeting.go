package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
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

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var req ds.MeetingTaskRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Meeting.CreateTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("meeting task creation failed: %w", err)
		}

		printSuccess("Meeting transcription task submitted!")
		printInfo("Task ID: %s", task.ID)
		printInfo("Use 'doubaospeech meeting status %s' to check status", task.ID)

		result := map[string]any{
			"task_id": task.ID,
			"status":  "submitted",
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

		status, err := client.Meeting.GetTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query meeting task failed: %w", err)
		}

		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)
		if status.Progress > 0 {
			printInfo("Progress: %d%%", status.Progress)
		}
		if status.Result != nil {
			printSuccess("Transcript: %s", status.Result.Text)
		}

		result := map[string]any{
			"task_id":  status.TaskID,
			"status":   status.Status,
			"progress": status.Progress,
		}
		if status.Result != nil {
			result["text"] = status.Result.Text
			result["duration"] = status.Result.Duration
			result["segments"] = status.Result.Segments
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	meetingCmd.AddCommand(meetingCreateCmd)
	meetingCmd.AddCommand(meetingStatusCmd)
}
