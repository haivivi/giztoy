package doubao

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
	Long: `Meeting transcription with speaker diarization.

Example request file (meeting.yaml):
  audio_url: https://example.com/meeting.mp3
  language: zh-CN
  speaker_count: 3
  enable_speaker_diarization: true`,
}

var meetingCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create meeting transcription task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.MeetingTaskRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Meeting.CreateTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("meeting task creation failed: %w", err)
		}

		printSuccess("Meeting transcription task submitted!")
		printInfo("Task ID: %s", task.ID)

		return outputResult(map[string]any{
			"task_id": task.ID,
			"status":  "submitted",
		}, outputFile, outputJSON)
	},
}

var meetingStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query meeting task status",
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

		status, err := client.Meeting.GetTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query meeting task failed: %w", err)
		}

		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)
		if status.Progress > 0 {
			printInfo("Progress: %d%%", status.Progress)
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
		return outputResult(result, outputFile, outputJSON)
	},
}

func init() {
	meetingCmd.AddCommand(meetingCreateCmd)
	meetingCmd.AddCommand(meetingStatusCmd)
}
