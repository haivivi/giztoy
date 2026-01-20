package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

var translationCmd = &cobra.Command{
	Use:   "translation",
	Short: "Simultaneous translation service",
	Long: `Simultaneous translation service.

Real-time speech-to-speech translation.

Example config file (translation.yaml):
  source_language: zh-CN
  target_language: en-US
  audio_config:
    format: pcm
    sample_rate: 16000
    channel: 1
    bits: 16
  enable_tts: true
  tts_voice: en_female_sweet`,
}

var translationStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream simultaneous translation",
	Long: `Stream simultaneous translation.

Send audio in the source language and receive
translated audio in the target language.

Example config file (translation.yaml):
  source_language: zh-CN
  target_language: en-US
  enable_tts: true
  tts_voice: en_female_sweet

Examples:
  doubao -c myctx translation stream -f translation.yaml --audio input.pcm -o output.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, err := cmd.Flags().GetString("audio")
		if err != nil {
			return fmt.Errorf("failed to read 'audio' flag: %w", err)
		}

		outputPath := getOutputFile()

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.TranslationConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Source language: %s", req.SourceLanguage)
		printVerbose("Target language: %s", req.TargetLanguage)

		// TODO: Implement streaming translation
		fmt.Println("[Streaming translation not implemented yet]")
		if audioFile != "" {
			fmt.Printf("Would stream audio from %s\n", audioFile)
		}
		if outputPath != "" {
			fmt.Printf("Would output translated audio to %s\n", outputPath)
		}

		return nil
	},
}

var translationInteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive translation mode",
	Long: `Start an interactive translation session.

This mode captures audio from your microphone and plays
translated audio through your speakers.

Examples:
  doubao -c myctx translation interactive -f translation.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.TranslationConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Source language: %s", req.SourceLanguage)
		printVerbose("Target language: %s", req.TargetLanguage)

		// TODO: Implement interactive mode
		fmt.Println("[Interactive translation not implemented yet]")
		fmt.Println("Would start interactive translation...")
		fmt.Println("Press Ctrl+C to exit")

		return nil
	},
}

func init() {
	translationStreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")

	translationCmd.AddCommand(translationStreamCmd)
	translationCmd.AddCommand(translationInteractiveCmd)
}
