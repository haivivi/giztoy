package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/minimax"
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

		voiceType, err := cmd.Flags().GetString("type")
		if err != nil {
			return fmt.Errorf("failed to read 'type' flag: %w", err)
		}
		if voiceType == "" {
			voiceType = string(minimax.VoiceTypeAll)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Voice type: %s", voiceType)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.Voice.List(reqCtx, minimax.VoiceType(voiceType))
		if err != nil {
			return fmt.Errorf("list voices failed: %w", err)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
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

		var req minimax.VoiceCloneRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Voice ID: %s", req.VoiceID)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.Clone(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice clone failed: %w", err)
		}

		// Save demo audio if output file specified and audio available
		outputPath := getOutputFile()
		if outputPath != "" && len(resp.DemoAudio) > 0 {
			if err := outputBytes(resp.DemoAudio, outputPath); err != nil {
				return fmt.Errorf("failed to write demo audio: %w", err)
			}
			printVerbose("Demo audio saved to: %s", outputPath)
		}

		result := map[string]any{
			"voice_id":        resp.VoiceID,
			"demo_audio_size": len(resp.DemoAudio),
		}

		return outputResult(result, "", isJSONOutput())
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

		var req minimax.VoiceDesignRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Prompt: %s", req.Prompt)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.Design(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice design failed: %w", err)
		}

		// Save demo audio if output file specified and audio available
		outputPath := getOutputFile()
		if outputPath != "" && len(resp.DemoAudio) > 0 {
			if err := outputBytes(resp.DemoAudio, outputPath); err != nil {
				return fmt.Errorf("failed to write demo audio: %w", err)
			}
			printVerbose("Demo audio saved to: %s", outputPath)
		}

		result := map[string]any{
			"voice_id":        resp.VoiceID,
			"demo_audio_size": len(resp.DemoAudio),
		}

		return outputResult(result, "", isJSONOutput())
	},
}

var voiceDeleteCmd = &cobra.Command{
	Use:   "delete <voice_id>",
	Short: "Delete a custom voice",
	Long: `Delete a custom voice (cloned or designed).

Use --type flag to specify the voice type:
  - voice_cloning: Cloned voices (default)
  - voice_generation: Designed voices

Examples:
  minimax -c myctx voice delete my-cloned-voice
  minimax -c myctx voice delete my-designed-voice --type voice_generation`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		voiceID := args[0]

		voiceType, _ := cmd.Flags().GetString("type")
		if voiceType == "" {
			voiceType = string(minimax.VoiceTypeCloning)
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Deleting voice: %s (type: %s)", voiceID, voiceType)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := client.Voice.Delete(reqCtx, voiceID, minimax.VoiceType(voiceType)); err != nil {
			return fmt.Errorf("delete voice failed: %w", err)
		}

		printSuccess("Voice deleted: %s", voiceID)

		return nil
	},
}

var voiceUploadCmd = &cobra.Command{
	Use:   "upload <audio_file>",
	Short: "Upload audio file for voice cloning",
	Long: `Upload an audio file to be used for voice cloning.

Returns a file_id that can be used in voice clone request.

Examples:
  minimax -c myctx voice upload audio.mp3
  minimax -c myctx voice upload audio.mp3 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		// Open file
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("File: %s (%s)", filePath, formatBytes(int(info.Size())))

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.UploadCloneAudio(reqCtx, file, info.Name())
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		printSuccess("File uploaded: %s", resp.FileID)

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

func init() {
	voiceListCmd.Flags().String("type", "all", "Voice type filter: all, system, voice_cloning, voice_generation")
	voiceDeleteCmd.Flags().String("type", "voice_cloning", "Voice type: voice_cloning, voice_generation")

	voiceCmd.AddCommand(voiceListCmd)
	voiceCmd.AddCommand(voiceCloneCmd)
	voiceCmd.AddCommand(voiceDesignCmd)
	voiceCmd.AddCommand(voiceDeleteCmd)
	voiceCmd.AddCommand(voiceUploadCmd)
}
