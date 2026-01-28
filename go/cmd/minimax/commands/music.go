package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/minimax"
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

		var req minimax.MusicRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		if req.Model == "" {
			req.Model = minimax.ModelMusic20
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		// Create API client
		client := createClient(ctx)

		// Call API - music generation can take a while
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		resp, err := client.Music.Generate(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("music generation failed: %w", err)
		}

		// Output audio to file if specified
		outputPath := getOutputFile()
		if outputPath != "" && len(resp.Audio) > 0 {
			if err := outputBytes(resp.Audio, outputPath); err != nil {
				return fmt.Errorf("failed to write audio file: %w", err)
			}
			printSuccess("Music saved to: %s (%s)", outputPath, formatBytes(len(resp.Audio)))
		}

		// Output result
		result := map[string]any{
			"audio_size":  len(resp.Audio),
			"duration_ms": resp.Duration,
			"extra_info":  resp.ExtraInfo,
			"output_file": outputPath,
		}

		return outputResult(result, "", isJSONOutput())
	},
}

func init() {
	musicCmd.AddCommand(musicGenerateCmd)
}
