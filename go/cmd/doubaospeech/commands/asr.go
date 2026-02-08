package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

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

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var req ds.OneSentenceRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
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

		return outputResult(output, getOutputFile(), isJSONOutput())
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

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var config ds.StreamASRConfig
		if err := loadRequest(getInputFile(), &config); err != nil {
			return err
		}

		printVerbose("Using V1 API (classic streaming)")
		printVerbose("Format: %s", config.Format)
		printVerbose("Audio file: %s", audioFile)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runASRV1Stream(reqCtx, client, &config, audioFile)
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

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var config ds.ASRV2Config
		if err := loadRequest(getInputFile(), &config); err != nil {
			return err
		}

		printVerbose("Using V2 API (BigModel streaming)")
		printVerbose("Format: %s", config.Format)
		printVerbose("Audio file: %s", audioFile)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runASRV2Stream(reqCtx, client, &config, audioFile)
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

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var req ds.ASRV2AsyncRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using V2 API (BigModel file)")
		printVerbose("Audio URL: %s", req.AudioURL)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := client.ASRV2.SubmitAsync(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("V2 file ASR submit failed: %w", err)
		}

		printSuccess("File ASR task submitted!")
		printInfo("Task ID: %s", result.TaskID)
		printInfo("Use 'doubaospeech asr v2 status %s' to check status", result.TaskID)

		output := map[string]any{
			"api_version": "v2",
			"task_id":     result.TaskID,
			"status":      result.Status,
		}

		return outputResult(output, getOutputFile(), isJSONOutput())
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

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
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

		return outputResult(output, getOutputFile(), isJSONOutput())
	},
}

// ============================================================================
// Implementation Functions
// ============================================================================

func runASRV1Stream(ctx context.Context, client *ds.Client, config *ds.StreamASRConfig, audioFile string) error {
	session, err := client.ASR.OpenStreamSession(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to open stream session: %w", err)
	}
	defer session.Close()

	printVerbose("Stream session opened")

	// Read audio data from file or stdin
	audioData, err := readAudioInput(audioFile)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}

	// Send audio in chunks
	chunkSize := 3200 // 100ms of 16kHz 16-bit mono
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}

		if err := session.SendAudio(ctx, audioData[i:end], isLast); err != nil {
			return fmt.Errorf("failed to send audio: %w", err)
		}

		if !isLast {
			time.Sleep(100 * time.Millisecond) // Simulate real-time streaming
		}
	}

	// Receive results
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

	result := map[string]any{
		"api_version": "v1",
		"text":        finalText,
	}

	return outputResult(result, getOutputFile(), isJSONOutput())
}

func runASRV2Stream(ctx context.Context, client *ds.Client, config *ds.ASRV2Config, audioFile string) error {
	session, err := client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to open stream session: %w", err)
	}
	defer session.Close()

	printVerbose("V2 stream session opened")

	// Read audio data from file or stdin
	audioData, err := readAudioInput(audioFile)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}

	// Send audio in chunks
	chunkSize := 3200 // 100ms of 16kHz 16-bit mono
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}

		if err := session.SendAudio(ctx, audioData[i:end], isLast); err != nil {
			return fmt.Errorf("failed to send audio: %w", err)
		}

		if !isLast {
			time.Sleep(100 * time.Millisecond) // Simulate real-time streaming
		}
	}

	// Receive results
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

	output := map[string]any{
		"api_version": "v2",
		"text":        finalText,
	}
	if len(allUtterances) > 0 {
		output["utterances"] = allUtterances
	}

	return outputResult(output, getOutputFile(), isJSONOutput())
}

// readAudioInput reads audio data from a file or stdin
func readAudioInput(audioFile string) ([]byte, error) {
	if audioFile != "" {
		data, err := os.ReadFile(audioFile)
		if err != nil {
			return nil, fmt.Errorf("read audio file %s: %w", audioFile, err)
		}
		return data, nil
	}

	// Read from stdin
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, os.Stdin); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return buf.Bytes(), nil
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
