package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

// asrCmd is the root command for ASR services
var asrCmd = &cobra.Command{
	Use:   "asr",
	Short: "Automatic Speech Recognition service",
	Long: `Automatic Speech Recognition (ASR) service.

Supports two API versions:
  - v1: Classic API (/api/v1/asr)
  - v2: BigModel API (/api/v3/sauc/*) - Recommended

Commands:
  asr v1 recognize   - V1 one-sentence recognition
  asr v1 stream      - V1 streaming recognition
  asr v2 stream      - V2 BigModel streaming recognition (recommended)
  asr v2 file        - V2 BigModel file recognition

V2 Resource IDs:
  - volc.bigasr.sauc.duration: Streaming ASR (duration-based billing)
  - volc.bigasr.auc.duration: File ASR (duration-based billing)`,
}

// ============================================================================
// V1 Commands
// ============================================================================

var asrV1Cmd = &cobra.Command{
	Use:   "v1",
	Short: "ASR V1 API (Classic)",
	Long: `ASR V1 Classic API (/api/v1/asr).

Uses Bearer Token authentication and cluster-based configuration.

Example request file (asr-v1.yaml):
  audio_url: https://example.com/audio.mp3
  format: mp3
  sample_rate: 16000
  language: zh-CN
  cluster: volcengine_streaming_common`,
}

var asrV1RecognizeCmd = &cobra.Command{
	Use:   "recognize",
	Short: "V1 one-sentence recognition (< 60s)",
	Long: `Recognize short audio files (less than 60 seconds) using V1 API.

Example:
  doubaospeech asr v1 recognize -f asr-v1.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.OneSentenceRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using V1 API (classic)")
		printVerbose("Format: %s", req.Format)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":       "API call not implemented yet",
			"api_version": "v1",
			"_context":    cliCtx.Name,
			"request":     req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var asrV1StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V1 streaming recognition",
	Long: `Real-time streaming speech recognition using V1 API.

Example:
  doubaospeech asr v1 stream -f asr-v1-stream.yaml --audio input.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, err := cmd.Flags().GetString("audio")
		if err != nil {
			return fmt.Errorf("failed to read 'audio' flag: %w", err)
		}

		if _, err := getContext(); err != nil {
			return err
		}

		var req ds.StreamASRConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using V1 API (classic streaming)")
		printVerbose("Audio file: %s", audioFile)

		// TODO: Implement streaming recognition
		fmt.Println("[V1 streaming recognition not implemented yet]")
		if audioFile != "" {
			fmt.Printf("Would stream audio from %s\n", audioFile)
		} else {
			fmt.Println("Would stream audio from stdin")
		}

		return nil
	},
}

// ============================================================================
// V2 Commands
// ============================================================================

var asrV2Cmd = &cobra.Command{
	Use:   "v2",
	Short: "ASR V2 API (BigModel) - Recommended",
	Long: `ASR V2 BigModel API (/api/v3/sauc/*).

Uses X-Api-* headers authentication.

Resource IDs:
  - volc.bigasr.sauc.duration: Streaming ASR
  - volc.bigasr.auc.duration: File ASR

Example request file (asr-v2.yaml):
  resource_id: volc.bigasr.sauc.duration
  format: pcm
  sample_rate: 16000
  language: zh-CN`,
}

var asrV2StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V2 BigModel streaming recognition (recommended)",
	Long: `Real-time streaming speech recognition using V2 BigModel API.

Endpoint: WSS /api/v3/sauc/bigmodel
Resource ID: volc.bigasr.sauc.duration

Example:
  doubaospeech asr v2 stream -f asr-v2.yaml --audio input.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, err := cmd.Flags().GetString("audio")
		if err != nil {
			return fmt.Errorf("failed to read 'audio' flag: %w", err)
		}

		if _, err := getContext(); err != nil {
			return err
		}

		var req ds.StreamASRConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using V2 API (BigModel streaming)")
		printVerbose("Format: %s", req.Format)
		printVerbose("Audio file: %s", audioFile)

		// TODO: Implement V2 streaming recognition
		fmt.Println("[V2 streaming recognition not implemented yet]")
		if audioFile != "" {
			fmt.Printf("Would stream audio from %s\n", audioFile)
		} else {
			fmt.Println("Would stream audio from stdin")
		}

		return nil
	},
}

var asrV2FileCmd = &cobra.Command{
	Use:   "file",
	Short: "V2 BigModel file recognition",
	Long: `Submit audio file for asynchronous recognition using V2 BigModel API.

Endpoint: POST /api/v3/auc/submit
Resource ID: volc.bigasr.auc.duration

For long audio files. Returns a task ID for tracking.
Use 'doubaospeech asr v2 status <task_id>' to check the task status.

Example:
  doubaospeech asr v2 file -f asr-v2-file.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.FileASRRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using V2 API (BigModel file)")
		printVerbose("Audio URL: %s", req.AudioURL)

		// TODO: Implement file recognition
		result := map[string]any{
			"_note":       "API call not implemented yet",
			"api_version": "v2",
			"_context":    cliCtx.Name,
			"request":     req,
			"task_id":     "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var asrV2StatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Long: `Query the status of an asynchronous recognition task.

Example:
  doubaospeech asr v2 status task_12345
  doubaospeech asr v2 status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Querying task: %s", taskID)

		// TODO: Implement task status query
		result := map[string]any{
			"_note":       "API call not implemented yet",
			"api_version": "v2",
			"_context":    cliCtx.Name,
			"task_id":     taskID,
			"status":      "pending",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// ============================================================================
// Init
// ============================================================================

func init() {
	// V1 subcommands
	asrV1StreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")
	asrV1Cmd.AddCommand(asrV1RecognizeCmd)
	asrV1Cmd.AddCommand(asrV1StreamCmd)

	// V2 subcommands
	asrV2StreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")
	asrV2Cmd.AddCommand(asrV2StreamCmd)
	asrV2Cmd.AddCommand(asrV2FileCmd)
	asrV2Cmd.AddCommand(asrV2StatusCmd)

	// Add v1 and v2 to asr
	asrCmd.AddCommand(asrV1Cmd)
	asrCmd.AddCommand(asrV2Cmd)
}
