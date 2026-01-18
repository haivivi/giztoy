package cmd

import (
	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
)

var musicCmd = &cobra.Command{
	Use:   "music",
	Short: "Music generation service",
	Long: `Music generation service.

Generate music with vocals based on prompts and lyrics.

Example request file (music.yaml):
  model: music-2.0
  prompt: Pop music, upbeat, suitable for summer
  lyrics: |
    [Verse]
    Walking down the street
    Feeling the summer heat
    [Chorus]
    Life is beautiful
    Every day is wonderful
  sample_rate: 44100
  format: mp3`,
}

var musicGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate music",
	Long: `Generate music with vocals.

Examples:
  minimax -c myctx music generate -f music.yaml -o song.mp3
  minimax -c myctx music generate -f music.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req mm.MusicRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = mm.ModelMusic20
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
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

func init() {
	musicCmd.AddCommand(musicGenerateCmd)
}
