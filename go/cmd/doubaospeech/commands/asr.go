package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

var asrCmd = &cobra.Command{
	Use:   "asr",
	Short: "Automatic Speech Recognition service",
	Long: `Automatic Speech Recognition (ASR) service.

Supports multiple recognition modes:
  - one-sentence: Short audio recognition (< 60 seconds)
  - stream: Real-time streaming recognition
  - file: Async file recognition for long audio

Example request file (asr.yaml):
  audio_url: https://example.com/audio.mp3
  format: mp3
  sample_rate: 16000
  language: zh-CN
  enable_itn: true
  enable_punc: true
  enable_ddc: false`,
}

var asrOneSentenceCmd = &cobra.Command{
	Use:   "one-sentence",
	Short: "Recognize short audio (< 60s)",
	Long: `Recognize short audio files (less than 60 seconds).

Supports audio URL or local file input.

Example request file (asr-one-sentence.yaml):
  audio_url: https://example.com/audio.mp3
  format: mp3
  language: zh-CN
  enable_itn: true
  enable_punc: true

Examples:
  doubao -c myctx asr one-sentence -f asr.yaml
  doubao -c myctx asr one-sentence -f asr.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.OneSentenceRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Format: %s", req.Format)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var asrStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Real-time streaming recognition",
	Long: `Real-time streaming speech recognition via WebSocket.

Send audio in real-time and receive transcription results.

Example config file (asr-stream.yaml):
  format: pcm
  sample_rate: 16000
  bits: 16
  channel: 1
  language: zh-CN
  enable_itn: true
  enable_punc: true

Examples:
  doubao -c myctx asr stream -f asr-stream.yaml --audio input.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, err := cmd.Flags().GetString("audio")
		if err != nil {
			return fmt.Errorf("failed to read 'audio' flag: %w", err)
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.StreamASRConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Audio file: %s", audioFile)

		// TODO: Implement streaming recognition
		fmt.Println("[Streaming recognition not implemented yet]")
		if audioFile != "" {
			fmt.Printf("Would stream audio from %s\n", audioFile)
		} else {
			fmt.Println("Would stream audio from stdin")
		}

		return nil
	},
}

var asrFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Async file recognition",
	Long: `Submit audio file for asynchronous recognition.

For long audio files. Returns a task ID for tracking.
Use 'doubao asr status <task_id>' to check the task status.

Example request file (asr-file.yaml):
  audio_url: https://example.com/long-audio.mp3
  format: mp3
  language: zh-CN
  callback_url: https://your-callback-url.com/webhook

Examples:
  doubao -c myctx asr file -f asr-file.yaml
  doubao -c myctx asr file -f asr-file.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.FileASRRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement file recognition
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var asrStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Long: `Query the status of an asynchronous recognition task.

Examples:
  doubao -c myctx asr status task_12345
  doubao -c myctx asr status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Querying task: %s", taskID)

		// TODO: Implement task status query
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"task_id":  taskID,
			"status":   "pending",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	asrStreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")

	asrCmd.AddCommand(asrOneSentenceCmd)
	asrCmd.AddCommand(asrStreamCmd)
	asrCmd.AddCommand(asrFileCmd)
	asrCmd.AddCommand(asrStatusCmd)
}
