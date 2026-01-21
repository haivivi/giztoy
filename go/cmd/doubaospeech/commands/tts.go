package commands

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

var ttsCmd = &cobra.Command{
	Use:   "tts",
	Short: "Text-to-Speech synthesis service",
	Long: `Text-to-Speech (TTS) synthesis service.

Supports two API versions:
  - V1 (Classic): /api/v1/tts - Traditional TTS
  - V2 (BigModel): /api/v3/tts/* - Large model TTS (default)

Synthesis modes:
  - synthesize: Synchronous synthesis (V1) or streaming (V2)
  - stream: Streaming synthesis via HTTP (V2)
  - stream-ws: Streaming synthesis via WebSocket
  - duplex: Bidirectional streaming (input text while receiving audio)
  - async: Asynchronous synthesis for long text

Example request file (tts.yaml):
  text: 你好，这是一段测试语音。
  speaker: zh_female_cancan
  format: mp3
  sample_rate: 24000
  speed_ratio: 1.0
  volume_ratio: 1.0
  pitch_ratio: 1.0`,
}

// useV1 flag for using V1 API instead of V2
var useV1 bool

var ttsSynthesizeCmd = &cobra.Command{
	Use:   "synthesize",
	Short: "Synthesize speech (default: V2 BigModel)",
	Long: `Synthesize speech from text.

By default uses V2 BigModel API (/api/v3/tts/unidirectional) for better quality.
Use --v1 flag to use classic V1 API (/api/v1/tts).

V2 request file (tts.yaml):
  text: 你好，这是一段测试语音。
  speaker: zh_female_cancan
  format: mp3
  sample_rate: 24000

V1 request file (tts-v1.yaml):
  text: 你好，这是一段测试语音。
  voice_type: zh_female_cancan
  encoding: mp3
  sample_rate: 24000
  cluster: volcano_tts

Examples:
  doubao -c myctx tts synthesize -f tts.yaml -o output.mp3
  doubao -c myctx tts synthesize -f tts.yaml -o output.mp3 --v1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		// Create API client
		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		if useV1 {
			// V1 API
			return runTTSV1(reqCtx, client, cliCtx, outputPath)
		}

		// V2 API (default)
		return runTTSV2(reqCtx, client, cliCtx, outputPath)
	},
}

func runTTSV1(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSRequest
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	// Use default voice if not specified
	if req.VoiceType == "" && cliCtx.DefaultVoice != "" {
		req.VoiceType = cliCtx.DefaultVoice
	}

	printVerbose("Using V1 API (classic)")
	printVerbose("Voice type: %s", req.VoiceType)
	printVerbose("Text length: %d characters", len(req.Text))

	resp, err := client.TTS.Synthesize(ctx, &req)
	if err != nil {
		return fmt.Errorf("V1 synthesis failed: %w", err)
	}

	if len(resp.Audio) > 0 {
		if err := outputBytes(resp.Audio, outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(len(resp.Audio))))
	}

	result := map[string]any{
		"api_version": "v1",
		"audio_size":  len(resp.Audio),
		"duration":    resp.Duration,
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

func runTTSV2(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSV2Request
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	// Use default voice if not specified
	if req.Speaker == "" && cliCtx.DefaultVoice != "" {
		req.Speaker = cliCtx.DefaultVoice
	}

	printVerbose("Using V2 API (BigModel)")
	printVerbose("Speaker: %s", req.Speaker)
	printVerbose("Text length: %d characters", len(req.Text))

	var audioBuf bytes.Buffer
	chunkCount := 0

	for chunk, err := range client.TTSV2.Stream(ctx, &req) {
		if err != nil {
			return fmt.Errorf("V2 streaming failed: %w", err)
		}
		if len(chunk.Audio) > 0 {
			audioBuf.Write(chunk.Audio)
			chunkCount++
		}
		if chunk.IsLast {
			break
		}
	}

	if audioBuf.Len() > 0 {
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	result := map[string]any{
		"api_version": "v2",
		"audio_size":  audioBuf.Len(),
		"chunks":      chunkCount,
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

var ttsStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream speech synthesis (HTTP)",
	Long: `Stream speech synthesis via HTTP.

Audio chunks are received as they are generated and saved incrementally.
This provides lower latency than synchronous synthesis.

Examples:
  doubao -c myctx tts stream -f tts.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required for streaming audio, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.TTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default voice if not specified
		if req.VoiceType == "" && cliCtx.DefaultVoice != "" {
			req.VoiceType = cliCtx.DefaultVoice
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Streaming to: %s", outputPath)

		// Create API client
		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		// Call streaming API
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var audioBuf bytes.Buffer
		var lastChunk *ds.TTSChunk

		for chunk, err := range client.TTS.SynthesizeStream(reqCtx, &req) {
			if err != nil {
				return fmt.Errorf("streaming failed: %w", err)
			}
			if chunk.Audio != nil {
				audioBuf.Write(chunk.Audio)
			}
			lastChunk = chunk
		}

		// Write audio to file
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}

		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))

		// Output final info
		result := map[string]any{
			"audio_size":  audioBuf.Len(),
			"output_file": outputPath,
		}
		if lastChunk != nil {
			result["duration"] = lastChunk.Duration
		}

		return outputResult(result, "", isJSONOutput())
	},
}

var ttsStreamWSCmd = &cobra.Command{
	Use:   "stream-ws",
	Short: "Stream speech synthesis (WebSocket)",
	Long: `Stream speech synthesis via WebSocket.

Similar to HTTP streaming but uses WebSocket protocol for
bidirectional communication capability.

Examples:
  doubao -c myctx tts stream-ws -f tts.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required for streaming audio, use -o flag")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.TTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("WebSocket streaming to: %s", outputPath)

		// TODO: Implement WebSocket streaming
		fmt.Println("[WebSocket streaming not implemented yet]")
		fmt.Printf("Would stream speech via WebSocket to %s\n", outputPath)

		return nil
	},
}

var ttsDuplexCmd = &cobra.Command{
	Use:   "duplex",
	Short: "Bidirectional streaming synthesis",
	Long: `Bidirectional streaming synthesis via WebSocket.

Allows sending text incrementally while receiving audio output.
This is useful for synthesizing text as it is being generated
(e.g., from an LLM).

Examples:
  doubao -c myctx tts duplex -f tts.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required for duplex streaming, use -o flag")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.TTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Duplex streaming to: %s", outputPath)

		// TODO: Implement duplex streaming
		fmt.Println("[Duplex streaming not implemented yet]")
		fmt.Printf("Would duplex stream speech to %s\n", outputPath)

		return nil
	},
}

var ttsAsyncCmd = &cobra.Command{
	Use:   "async",
	Short: "Create async synthesis task",
	Long: `Create an asynchronous synthesis task for long text.

Supports text up to 10,000 characters. Returns a task ID for tracking.
Use 'doubao tts status <task_id>' to check the task status.

Example request file (tts-async.yaml):
  text: Very long text here...
  voice_type: zh_female_cancan
  encoding: mp3
  callback_url: https://your-callback-url.com/webhook

Examples:
  doubao -c myctx tts async -f tts-async.yaml
  doubao -c myctx tts async -f tts-async.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.AsyncTTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Text length: %d characters", len(req.Text))

		// TODO: Implement async API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var ttsStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Long: `Query the status of an asynchronous synthesis task.

Examples:
  doubao -c myctx tts status task_12345
  doubao -c myctx tts status task_12345 --json`,
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
	// Add --v1 flag to synthesize command
	ttsSynthesizeCmd.Flags().BoolVar(&useV1, "v1", false, "Use V1 classic API instead of V2 BigModel")

	ttsCmd.AddCommand(ttsSynthesizeCmd)
	ttsCmd.AddCommand(ttsStreamCmd)
	ttsCmd.AddCommand(ttsStreamWSCmd)
	ttsCmd.AddCommand(ttsDuplexCmd)
	ttsCmd.AddCommand(ttsAsyncCmd)
	ttsCmd.AddCommand(ttsStatusCmd)
}
