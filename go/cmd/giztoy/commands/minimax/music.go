package minimax

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
  giztoy minimax music generate -f music.yaml -o song.mp3
  giztoy minimax music generate -f music.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.MusicRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Model == "" {
			req.Model = minimax.ModelMusic20
		}

		printVerbose("Model: %s", req.Model)
		printVerbose("Prompt: %s", req.Prompt)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		resp, err := client.Music.Generate(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("music generation failed: %w", err)
		}

		if outputFile != "" && len(resp.Audio) > 0 {
			if err := outputBytes(resp.Audio, outputFile); err != nil {
				return fmt.Errorf("failed to write audio: %w", err)
			}
			printSuccess("Music saved to: %s (%s)", outputFile, formatBytes(len(resp.Audio)))
		}

		result := map[string]any{
			"audio_size":  len(resp.Audio),
			"duration_ms": resp.Duration,
			"extra_info":  resp.ExtraInfo,
			"output_file": outputFile,
		}
		return outputResult(result, "", outputJSON)
	},
}

func init() {
	musicCmd.AddCommand(musicGenerateCmd)
}
