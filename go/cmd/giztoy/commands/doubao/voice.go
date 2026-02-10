package doubao

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Voice cloning service",
	Long: `Voice cloning service.

Train custom voice models from audio samples.

Example request file (voice-train.yaml):
  speaker_id: my_custom_voice
  audio_urls:
    - https://example.com/sample1.wav
  language: zh-CN`,
}

var voiceTrainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train a custom voice",
	Long: `Train a custom voice model from audio samples.

Examples:
  giztoy doubao voice train -f voice-train.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.VoiceCloneTrainRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Speaker ID: %s", req.SpeakerID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		task, err := client.VoiceClone.Train(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice clone training failed: %w", err)
		}

		printSuccess("Voice clone training submitted!")
		printInfo("Speaker ID: %s", task.ID)

		return outputResult(map[string]any{
			"speaker_id": task.ID,
			"status":     "training",
		}, outputFile, outputJSON)
	},
}

var voiceStatusCmd = &cobra.Command{
	Use:   "status <speaker_id>",
	Short: "Query voice training status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		speakerID := args[0]

		client, err := createClient()
		if err != nil {
			return err
		}

		printVerbose("Querying speaker: %s", speakerID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.VoiceClone.GetStatus(reqCtx, speakerID)
		if err != nil {
			return fmt.Errorf("query voice clone status failed: %w", err)
		}

		printInfo("Speaker ID: %s", status.SpeakerID)
		printInfo("Status: %s", status.Status)

		result := map[string]any{
			"speaker_id": status.SpeakerID,
			"status":     status.Status,
		}
		if status.DemoAudio != "" {
			result["demo_audio"] = status.DemoAudio
		}
		return outputResult(result, outputFile, outputJSON)
	},
}

var voiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trained voices",
	Long: `List all trained custom voices.

Requires console credentials (console_ak, console_sk) in config.

Examples:
  giztoy doubao voice list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}

		console, err := createConsole()
		if err != nil {
			return fmt.Errorf("console credentials required for listing voices: %w", err)
		}

		if svc.AppID == "" {
			return fmt.Errorf("app_id required for listing voices")
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := console.ListVoiceCloneStatus(reqCtx, &ds.ListVoiceCloneStatusRequest{
			AppID: svc.AppID,
		})
		if err != nil {
			return fmt.Errorf("list voice clone status failed: %w", err)
		}

		printInfo("Total voices: %d", resp.Total)

		var voices []map[string]any
		for _, s := range resp.Statuses {
			v := map[string]any{
				"speaker_id":  s.SpeakerID,
				"state":       s.State,
				"version":     s.Version,
				"resource_id": s.ResourceID,
				"create_time": s.CreateTime,
				"expire_time": s.ExpireTime,
			}
			if s.Alias != "" {
				v["alias"] = s.Alias
			}
			voices = append(voices, v)
		}

		return outputResult(map[string]any{
			"total":  resp.Total,
			"voices": voices,
		}, outputFile, outputJSON)
	},
}

func init() {
	voiceCmd.AddCommand(voiceTrainCmd)
	voiceCmd.AddCommand(voiceStatusCmd)
	voiceCmd.AddCommand(voiceListCmd)
}
