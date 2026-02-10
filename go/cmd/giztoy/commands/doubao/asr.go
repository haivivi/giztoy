package doubao

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var asrCmd = &cobra.Command{
	Use:   "asr",
	Short: "Automatic Speech Recognition service",
	Long: `Automatic Speech Recognition (ASR) service.

Supports two API versions:
  v1  Classic API (/api/v1/asr)
  v2  BigModel API (/api/v3/sauc/*) - Recommended

V2 Resource IDs:
  volc.bigasr.sauc.duration  Streaming ASR
  volc.bigasr.auc.duration   File ASR`,
}

// ============================================================================
// V1 Commands
// ============================================================================

var asrV1Cmd = &cobra.Command{
	Use:   "v1",
	Short: "ASR V1 API (Classic)",
}

var asrV1RecognizeCmd = &cobra.Command{
	Use:   "recognize",
	Short: "V1 one-sentence recognition (< 60s)",
	Long: `Recognize short audio files (less than 60 seconds) using V1 API.

Example:
  giztoy doubao asr v1 recognize -f asr-v1.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.OneSentenceRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Using V1 API (classic)")
		printVerbose("Format: %s", req.Format)

		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := client.ASR.RecognizeOneSentence(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("V1 recognition failed: %w", err)
		}

		printSuccess("Recognition complete")
		if result.Text != "" {
			fmt.Println(result.Text)
		}

		output := map[string]any{
			"api_version": "v1",
			"text":        result.Text,
			"duration":    result.Duration,
		}
		return outputResult(output, outputFile, outputJSON)
	},
}

var asrV1StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V1 streaming recognition",
	Long: `Real-time streaming speech recognition using V1 API.

Example:
  giztoy doubao asr v1 stream -f asr-v1-stream.yaml --audio input.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, _ := cmd.Flags().GetString("audio")

		client, err := createClient()
		if err != nil {
			return err
		}

		var config ds.StreamASRConfig
		if err := loadRequest(inputFile, &config); err != nil {
			return err
		}

		printVerbose("Using V1 API (classic streaming)")
		printVerbose("Audio file: %s", audioFile)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		session, err := client.ASR.OpenStreamSession(reqCtx, &config)
		if err != nil {
			return fmt.Errorf("failed to open stream session: %w", err)
		}
		defer session.Close()

		printVerbose("Stream session opened")

		if err := sendAudioChunked(reqCtx, session, audioFile); err != nil {
			return err
		}

		var finalText string
		for chunk, err := range session.Recv() {
			if err != nil {
				return fmt.Errorf("receive error: %w", err)
			}
			if chunk.Text != "" {
				if chunk.IsDefinite {
					fmt.Println(chunk.Text)
				} else {
					printVerbose("[interim] %s", chunk.Text)
				}
				finalText = chunk.Text
			}
			if chunk.IsFinal {
				break
			}
		}

		return outputResult(map[string]any{"api_version": "v1", "text": finalText}, outputFile, outputJSON)
	},
}

// ============================================================================
// V2 Commands
// ============================================================================

var asrV2Cmd = &cobra.Command{
	Use:   "v2",
	Short: "ASR V2 API (BigModel) - Recommended",
}

var asrV2StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V2 BigModel streaming recognition (recommended)",
	Long: `Real-time streaming speech recognition using V2 BigModel API.

Example:
  giztoy doubao asr v2 stream -f asr-v2.yaml --audio input.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		audioFile, _ := cmd.Flags().GetString("audio")

		client, err := createClient()
		if err != nil {
			return err
		}

		var config ds.ASRV2Config
		if err := loadRequest(inputFile, &config); err != nil {
			return err
		}

		printVerbose("Using V2 API (BigModel streaming)")
		printVerbose("Audio file: %s", audioFile)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		session, err := client.ASRV2.OpenStreamSession(reqCtx, &config)
		if err != nil {
			return fmt.Errorf("failed to open stream session: %w", err)
		}
		defer session.Close()

		printVerbose("V2 stream session opened")

		if err := sendAudioChunked(reqCtx, session, audioFile); err != nil {
			return err
		}

		var finalText string
		var allUtterances []ds.ASRV2Utterance
		for result, err := range session.Recv() {
			if err != nil {
				return fmt.Errorf("receive error: %w", err)
			}
			if result.Text != "" {
				if result.IsFinal {
					fmt.Println(result.Text)
				} else {
					printVerbose("[interim] %s", result.Text)
				}
				finalText = result.Text
			}
			allUtterances = append(allUtterances, result.Utterances...)
		}

		output := map[string]any{"api_version": "v2", "text": finalText}
		if len(allUtterances) > 0 {
			output["utterances"] = allUtterances
		}
		return outputResult(output, outputFile, outputJSON)
	},
}

var asrV2FileCmd = &cobra.Command{
	Use:   "file",
	Short: "V2 BigModel file recognition",
	Long: `Submit audio file for asynchronous recognition.

Returns a task ID. Use 'giztoy doubao asr v2 status <task_id>' to check.

Example:
  giztoy doubao asr v2 file -f asr-v2-file.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.ASRV2AsyncRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Audio URL: %s", req.AudioURL)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := client.ASRV2.SubmitAsync(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("V2 file ASR submit failed: %w", err)
		}

		printSuccess("File ASR task submitted!")
		printInfo("Task ID: %s", result.TaskID)

		return outputResult(map[string]any{
			"api_version": "v2",
			"task_id":     result.TaskID,
			"status":      result.Status,
		}, outputFile, outputJSON)
	},
}

var asrV2StatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		client, err := createClient()
		if err != nil {
			return err
		}

		printVerbose("Querying task: %s", taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := client.ASRV2.QueryAsync(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query task status failed: %w", err)
		}

		printInfo("Task ID: %s", result.TaskID)
		printInfo("Status: %s", result.Status)
		if result.Text != "" {
			printSuccess("Text: %s", result.Text)
		}

		output := map[string]any{
			"api_version": "v2",
			"task_id":     result.TaskID,
			"status":      result.Status,
		}
		if result.Text != "" {
			output["text"] = result.Text
		}
		return outputResult(output, outputFile, outputJSON)
	},
}

func init() {
	asrV1StreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")
	asrV1Cmd.AddCommand(asrV1RecognizeCmd)
	asrV1Cmd.AddCommand(asrV1StreamCmd)

	asrV2StreamCmd.Flags().String("audio", "", "Audio file path (optional, defaults to stdin)")
	asrV2Cmd.AddCommand(asrV2StreamCmd)
	asrV2Cmd.AddCommand(asrV2FileCmd)
	asrV2Cmd.AddCommand(asrV2StatusCmd)

	asrCmd.AddCommand(asrV1Cmd)
	asrCmd.AddCommand(asrV2Cmd)
}
