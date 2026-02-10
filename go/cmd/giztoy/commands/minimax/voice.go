package minimax

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Voice management service",
	Long:  `Voice management: listing, cloning, designing, and deleting voices.`,
}

var voiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available voices",
	Long: `List all available voices.

Use --type to filter: all, system, voice_cloning, voice_generation.

Examples:
  giztoy minimax voice list
  giztoy minimax voice list --type system --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		voiceType, _ := cmd.Flags().GetString("type")
		if voiceType == "" {
			voiceType = string(minimax.VoiceTypeAll)
		}

		printVerbose("Voice type: %s", voiceType)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.Voice.List(reqCtx, minimax.VoiceType(voiceType))
		if err != nil {
			return fmt.Errorf("list voices failed: %w", err)
		}

		return outputResult(resp, outputFile, outputJSON)
	},
}

var voiceCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a voice",
	Long: `Clone a voice from an audio file.

Example request file (voice-clone.yaml):
  file_id: uploaded-file-id
  voice_id: my-custom-voice
  model: speech-2.6-hd
  text: Hello, this is a test for voice cloning.

Examples:
  giztoy minimax voice clone -f voice-clone.yaml
  giztoy minimax voice clone -f voice-clone.yaml -o demo.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.VoiceCloneRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Voice ID: %s", req.VoiceID)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.Clone(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice clone failed: %w", err)
		}

		if outputFile != "" && len(resp.DemoAudio) > 0 {
			if err := outputBytes(resp.DemoAudio, outputFile); err != nil {
				return fmt.Errorf("failed to write demo audio: %w", err)
			}
			printVerbose("Demo audio saved to: %s", outputFile)
		}

		result := map[string]any{
			"voice_id":        resp.VoiceID,
			"demo_audio_size": len(resp.DemoAudio),
		}
		return outputResult(result, "", outputJSON)
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

Examples:
  giztoy minimax voice design -f voice-design.yaml -o preview.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		var req minimax.VoiceDesignRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Prompt: %s", req.Prompt)

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.Design(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("voice design failed: %w", err)
		}

		if outputFile != "" && len(resp.DemoAudio) > 0 {
			if err := outputBytes(resp.DemoAudio, outputFile); err != nil {
				return fmt.Errorf("failed to write demo audio: %w", err)
			}
			printVerbose("Demo audio saved to: %s", outputFile)
		}

		result := map[string]any{
			"voice_id":        resp.VoiceID,
			"demo_audio_size": len(resp.DemoAudio),
		}
		return outputResult(result, "", outputJSON)
	},
}

var voiceDeleteCmd = &cobra.Command{
	Use:   "delete <voice_id>",
	Short: "Delete a custom voice",
	Long: `Delete a custom voice (cloned or designed).

Use --type to specify: voice_cloning (default) or voice_generation.

Examples:
  giztoy minimax voice delete my-cloned-voice
  giztoy minimax voice delete my-designed-voice --type voice_generation`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		voiceID := args[0]
		voiceType, _ := cmd.Flags().GetString("type")
		if voiceType == "" {
			voiceType = string(minimax.VoiceTypeCloning)
		}

		printVerbose("Deleting voice: %s (type: %s)", voiceID, voiceType)

		client, err := createClient()
		if err != nil {
			return err
		}

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
  giztoy minimax voice upload audio.mp3
  giztoy minimax voice upload audio.mp3 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		}

		printVerbose("File: %s (%s)", filePath, formatBytes(int(info.Size())))

		client, err := createClient()
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Voice.UploadCloneAudio(reqCtx, file, info.Name())
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		printSuccess("File uploaded: %s", resp.FileID)
		return outputResult(resp, outputFile, outputJSON)
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
