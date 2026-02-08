package commands

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
    - https://example.com/sample2.wav
  language: zh-CN
  model_type: standard`,
}

var voiceTrainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train a custom voice",
	Long: `Train a custom voice model from audio samples.

The audio samples should be clear recordings of the target speaker.
Training may take several minutes to complete.

Example request file (voice-train.yaml):
  speaker_id: my_custom_voice
  audio_urls:
    - https://example.com/sample1.wav
    - https://example.com/sample2.wav
  language: zh-CN
  model_type: standard

Examples:
  doubaospeech -c myctx voice train -f voice-train.yaml
  doubaospeech -c myctx voice train -f voice-train.yaml --json`,
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

		var req ds.VoiceCloneTrainRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Speaker ID: %s", req.SpeakerID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		task, err := client.VoiceClone.Train(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice clone training failed: %w", err)
		}

		printSuccess("Voice clone training submitted!")
		printInfo("Speaker ID: %s", task.ID)
		printInfo("Use 'doubaospeech voice status %s' to check status", task.ID)

		result := map[string]any{
			"speaker_id": task.ID,
			"status":     "training",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var voiceStatusCmd = &cobra.Command{
	Use:   "status <speaker_id>",
	Short: "Query voice training status",
	Long: `Query the status of voice training.

Examples:
  doubaospeech -c myctx voice status my_custom_voice
  doubaospeech -c myctx voice status my_custom_voice --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		speakerID := args[0]

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Querying speaker: %s", speakerID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.VoiceClone.GetStatus(reqCtx, speakerID)
		if err != nil {
			return fmt.Errorf("query voice clone status failed: %w", err)
		}

		printInfo("Speaker ID: %s", status.SpeakerID)
		printInfo("Status: %s", status.Status)
		if status.DemoAudio != "" {
			printInfo("Demo audio available")
		}

		result := map[string]any{
			"speaker_id": status.SpeakerID,
			"status":     status.Status,
		}
		if status.DemoAudio != "" {
			result["demo_audio"] = status.DemoAudio
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var voiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trained voices",
	Long: `List all trained custom voices.

Examples:
  doubaospeech -c myctx voice list
  doubaospeech -c myctx voice list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		console, err := createConsole(cliCtx)
		if err != nil {
			return fmt.Errorf("console credentials required for listing voices: %w", err)
		}

		printVerbose("Using context: %s", cliCtx.Name)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if cliCtx.Client == nil {
			return fmt.Errorf("client credentials required: --app-id is needed for listing voices")
		}

		resp, err := console.ListVoiceCloneStatus(reqCtx, &ds.ListVoiceCloneStatusRequest{
			AppID: cliCtx.Client.AppID,
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

		result := map[string]any{
			"total":  resp.Total,
			"voices": voices,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// Note: Voice Clone delete is not supported because Voice IDs are purchased resources.
// They expire at their ExpireTime but cannot be manually deleted.

func init() {
	voiceCmd.AddCommand(voiceTrainCmd)
	voiceCmd.AddCommand(voiceStatusCmd)
	voiceCmd.AddCommand(voiceListCmd)
}
