package commands

import (
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

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.VoiceCloneTrainRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Speaker ID: %s", req.SpeakerID)
		printVerbose("Audio samples: %d", len(req.AudioURLs))

		// TODO: Implement voice training API call
		result := map[string]any{
			"_note":      "API call not implemented yet",
			"_context":   ctx.Name,
			"request":    req,
			"speaker_id": req.SpeakerID,
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

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Querying speaker: %s", speakerID)

		// TODO: Implement status query
		result := map[string]any{
			"_note":      "API call not implemented yet",
			"_context":   ctx.Name,
			"speaker_id": speakerID,
			"status":     "ready",
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
		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement list API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"voices": []map[string]any{
				{"speaker_id": "custom_voice_1", "status": "ready", "created_at": "2024-01-01"},
				{"speaker_id": "custom_voice_2", "status": "training", "created_at": "2024-01-02"},
			},
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
