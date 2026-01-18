package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
)

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Voice management service",
	Long: `Voice management service.

Manage voice IDs including listing, cloning, and designing voices.`,
}

var voiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available voices",
	Long: `List all available voices.

Use --type flag to filter by voice type:
  - all: All voices (default)
  - system: System preset voices
  - voice_cloning: Custom cloned voices

Examples:
  minimax -c myctx voice list
  minimax -c myctx voice list --type system
  minimax -c myctx voice list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		voiceType, _ := cmd.Flags().GetString("type")
		if voiceType == "" {
			voiceType = string(mm.VoiceTypeAll)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Voice type: %s", voiceType)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":      "API call not implemented yet",
			"_context":   ctx.Name,
			"voice_type": voiceType,
			"voices": []map[string]any{
				{"voice_id": "female-shaonv", "name": "少女音", "type": "system"},
				{"voice_id": "male-qn-qingse", "name": "青涩青年音", "type": "system"},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var voiceCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a voice",
	Long: `Clone a voice from an audio file.

The cloned voice is temporary and will be deleted after 7 days if not used.

Example request file (voice-clone.yaml):
  file_id: uploaded-file-id
  voice_id: my-custom-voice
  model: speech-2.6-hd
  text: Hello, this is a test for voice cloning.

Examples:
  minimax -c myctx voice clone -f voice-clone.yaml
  minimax -c myctx voice clone -f voice-clone.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.VoiceCloneRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Voice ID: %s", req.VoiceID)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var voiceDesignCmd = &cobra.Command{
	Use:   "design",
	Short: "Design a new voice",
	Long: `Design a new voice from a text description.

Example request file (voice-design.yaml):
  prompt: A warm, friendly female voice with a slight accent
  preview_text: Hello, this is a preview of the designed voice.
  voice_id: my-designed-voice
  model: speech-2.6-hd

Examples:
  minimax -c myctx voice design -f voice-design.yaml
  minimax -c myctx voice design -f voice-design.yaml -o preview.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.VoiceDesignRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Prompt: %s", req.Prompt)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var voiceDeleteCmd = &cobra.Command{
	Use:   "delete <voice_id>",
	Short: "Delete a custom voice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		voiceID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Deleting voice: %s", voiceID)

		// TODO: Implement actual API call
		fmt.Printf("[Not implemented] Would delete voice: %s\n", voiceID)

		return nil
	},
}

func init() {
	voiceListCmd.Flags().String("type", "all", "Voice type filter: all, system, voice_cloning")

	voiceCmd.AddCommand(voiceListCmd)
	voiceCmd.AddCommand(voiceCloneCmd)
	voiceCmd.AddCommand(voiceDesignCmd)
	voiceCmd.AddCommand(voiceDeleteCmd)
}
