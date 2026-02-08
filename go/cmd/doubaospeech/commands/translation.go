package commands

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
  doubaospeech -c myctx translation stream -f translation.yaml --audio input.pcm -o output.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, err := cmd.Flags().GetString("audio")
		if err != nil {
			return fmt.Errorf("failed to read 'audio' flag: %w", err)
		}
		if audioFile == "" {
			return fmt.Errorf("--audio flag is required for streaming translation")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var config ds.TranslationConfig
		if err := loadRequest(getInputFile(), &config); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Source language: %s", config.SourceLanguage)
		printVerbose("Target language: %s", config.TargetLanguage)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runTranslationStream(reqCtx, client, &config, audioFile)
	},
}

var translationInteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive translation mode",
	Long: `Start an interactive translation session.

This mode captures audio from your microphone and plays
translated audio through your speakers.

Examples:
  doubaospeech -c myctx translation interactive -f translation.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented: interactive translation mode requires microphone/speaker hardware integration; use 'doubaospeech translation stream' for file-based translation")
	},
}

// ============================================================================
// Implementation Functions
// ============================================================================

func runTranslationStream(ctx context.Context, client *ds.Client, config *ds.TranslationConfig, audioFile string) error {
	session, err := client.Translation.OpenSession(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to open translation session: %w", err)
	}
	defer session.Close()

	printVerbose("Translation session opened")

	// Read audio data
	audioData, err := readAudioInput(audioFile)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}

	printVerbose("Sending audio (%s)...", formatBytes(int64(len(audioData))))

	// Send audio in chunks
	chunkSize := 3200 // 100ms of 16kHz 16-bit mono
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}

		if err := session.SendAudio(ctx, audioData[i:end], isLast); err != nil {
			return fmt.Errorf("send audio: %w", err)
		}

		if !isLast {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Receive results
	outputPath := getOutputFile()
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
			printVerbose("[Audio] +%d bytes", len(chunk.Audio))
		}
		if chunk.IsFinal {
			break
		}
	}

	if audioBuf.Len() > 0 && outputPath != "" {
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("write audio: %w", err)
		}
		printSuccess("Translated audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	result := map[string]any{
		"source_text": finalSource,
		"target_text": finalTarget,
	}
	if audioBuf.Len() > 0 {
		result["audio_size"] = audioBuf.Len()
	}

	return outputResult(result, "", isJSONOutput())
}

func init() {
	translationStreamCmd.Flags().String("audio", "", "Audio file path (required)")

	translationCmd.AddCommand(translationStreamCmd)
	translationCmd.AddCommand(translationInteractiveCmd)
}
