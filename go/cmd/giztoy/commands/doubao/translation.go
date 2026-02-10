package doubao

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var translationCmd = &cobra.Command{
	Use:   "translation",
	Short: "Simultaneous translation service",
	Long: `Real-time speech-to-speech translation.

Example config file (translation.yaml):
  source_language: zh-CN
  target_language: en-US
  enable_tts: true
  tts_voice: en_female_sweet`,
}

var translationStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream simultaneous translation",
	Long: `Send audio in the source language and receive translated audio.

Examples:
  giztoy doubao translation stream -f translation.yaml --audio input.pcm -o output.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, _ := cmd.Flags().GetString("audio")
		if audioFile == "" {
			return fmt.Errorf("--audio flag is required for streaming translation")
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var config ds.TranslationConfig
		if err := loadRequest(inputFile, &config); err != nil {
			return err
		}

		printVerbose("Source language: %s", config.SourceLanguage)
		printVerbose("Target language: %s", config.TargetLanguage)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		session, err := client.Translation.OpenSession(reqCtx, &config)
		if err != nil {
			return fmt.Errorf("failed to open translation session: %w", err)
		}
		defer session.Close()

		printVerbose("Translation session opened")

		if err := sendAudioChunked(reqCtx, session, audioFile); err != nil {
			return err
		}

		var audioBuf bytes.Buffer
		var finalSource, finalTarget string

		for chunk, err := range session.Recv() {
			if err != nil {
				return fmt.Errorf("receive error: %w", err)
			}
			if chunk.SourceText != "" {
				if chunk.IsDefinite {
					printInfo("[Source] %s", chunk.SourceText)
				} else {
					printVerbose("[Source interim] %s", chunk.SourceText)
				}
				finalSource = chunk.SourceText
			}
			if chunk.TargetText != "" {
				if chunk.IsDefinite {
					printInfo("[Target] %s", chunk.TargetText)
				} else {
					printVerbose("[Target interim] %s", chunk.TargetText)
				}
				finalTarget = chunk.TargetText
			}
			if len(chunk.Audio) > 0 {
				audioBuf.Write(chunk.Audio)
			}
			if chunk.IsFinal {
				break
			}
		}

		if audioBuf.Len() > 0 && outputFile != "" {
			if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
				return fmt.Errorf("write audio: %w", err)
			}
			printSuccess("Translated audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))
		}

		result := map[string]any{
			"source_text": finalSource,
			"target_text": finalTarget,
		}
		if audioBuf.Len() > 0 {
			result["audio_size"] = audioBuf.Len()
		}
		return outputResult(result, "", outputJSON)
	},
}

func init() {
	translationStreamCmd.Flags().String("audio", "", "Audio file path (required)")

	translationCmd.AddCommand(translationStreamCmd)
}
