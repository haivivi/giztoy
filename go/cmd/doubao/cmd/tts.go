package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	dsi "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

var ttsCmd = &cobra.Command{
	Use:   "tts",
	Short: "Text-to-Speech synthesis service",
	Long: `Text-to-Speech (TTS) synthesis service.

Supports multiple synthesis modes:
  - synthesize: Synchronous synthesis for short text
  - stream: Streaming synthesis via HTTP
  - stream-ws: Streaming synthesis via WebSocket
  - duplex: Bidirectional streaming (input text while receiving audio)
  - async: Asynchronous synthesis for long text

Example request file (tts.yaml):
  text: 你好，这是一段测试语音。
  voice_type: zh_female_cancan
  encoding: mp3
  sample_rate: 24000
  speed_ratio: 1.0
  volume_ratio: 1.0
  pitch_ratio: 1.0
  enable_timestamp: false`,
}

var ttsSynthesizeCmd = &cobra.Command{
	Use:   "synthesize",
	Short: "Synthesize speech synchronously",
	Long: `Synthesize speech from text (synchronous mode).

Best for short text (< 1000 characters). The audio output
will be saved to the specified output file.

Examples:
  doubao -c myctx tts synthesize -f tts.yaml -o output.mp3
  doubao -c myctx tts synthesize -f tts.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.TTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default voice if not specified
		if req.VoiceType == "" {
			if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
				req.VoiceType = defaultVoice
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Voice type: %s", req.VoiceType)
		printVerbose("Text length: %d characters", len(req.Text))

		// TODO: Implement actual API call using doubaospeech client
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
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

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.TTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use default voice if not specified
		if req.VoiceType == "" {
			if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
				req.VoiceType = defaultVoice
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Streaming to: %s", outputPath)

		// TODO: Implement actual streaming API call
		fmt.Println("[Streaming not implemented yet]")
		fmt.Printf("Would stream speech to %s\n", outputPath)

		return nil
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

		var req dsi.TTSRequest
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

		var req dsi.TTSRequest
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

		var req dsi.AsyncTTSRequest
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
	ttsCmd.AddCommand(ttsSynthesizeCmd)
	ttsCmd.AddCommand(ttsStreamCmd)
	ttsCmd.AddCommand(ttsStreamWSCmd)
	ttsCmd.AddCommand(ttsDuplexCmd)
	ttsCmd.AddCommand(ttsAsyncCmd)
	ttsCmd.AddCommand(ttsStatusCmd)
}
