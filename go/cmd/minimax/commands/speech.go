package commands

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/minimax"
)

var speechCmd = &cobra.Command{
	Use:   "speech",
	Short: "Speech synthesis service",
	Long: `Speech synthesis (TTS) service.

Supports synchronous and asynchronous speech synthesis.

Example request file (speech.yaml):
  model: speech-2.6-hd
  text: Hello, this is a test message.
  voice_setting:
    voice_id: female-shaonv
    speed: 1.0
    vol: 1.0
    emotion: happy
  audio_setting:
    format: mp3
    sample_rate: 32000
  language_boost: Chinese`,
}

var speechSynthesizeCmd = &cobra.Command{
	Use:   "synthesize",
	Short: "Synthesize speech from text",
	Long: `Synthesize speech from text (synchronous).

The audio output will be saved to the specified output file.

Examples:
  minimax -c myctx speech synthesize -f speech.yaml -o output.mp3
  minimax -c myctx speech synthesize -f speech.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.SpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use defaults if not specified
		if req.Model == "" {
			req.Model = minimax.ModelSpeech26HD
		}
		if req.VoiceSetting != nil && req.VoiceSetting.VoiceID == "" {
			if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
				req.VoiceSetting.VoiceID = defaultVoice
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Text length: %d characters", len(req.Text))

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		resp, err := client.Speech.Synthesize(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("speech synthesis failed: %w", err)
		}

		// Output audio to file if specified
		outputPath := getOutputFile()
		if outputPath != "" && len(resp.Audio) > 0 {
			if err := outputBytes(resp.Audio, outputPath); err != nil {
				return fmt.Errorf("failed to write audio file: %w", err)
			}
			printVerbose("Audio saved to: %s", outputPath)
		}

		// Output result
		result := map[string]any{
			"audio_size":   len(resp.Audio),
			"audio_url":    resp.AudioURL,
			"trace_id":     resp.TraceID,
			"extra_info":   resp.ExtraInfo,
			"output_file":  outputPath,
		}

		return outputResult(result, "", isJSONOutput())
	},
}

var speechStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream speech synthesis",
	Long: `Stream speech synthesis.

Audio will be streamed and saved incrementally.

Examples:
  minimax -c myctx speech stream -f speech.yaml -o output.mp3`,
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

		var req minimax.SpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use defaults if not specified
		if req.Model == "" {
			req.Model = minimax.ModelSpeech26HD
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Streaming to: %s", outputPath)

		// Create API client
		client := createClient(ctx)

		// Call streaming API
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var audioBuf bytes.Buffer
		var lastChunk *minimax.SpeechChunk

		for chunk, err := range client.Speech.SynthesizeStream(reqCtx, &req) {
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

		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(audioBuf.Len()))

		// Output final info
		if lastChunk != nil && lastChunk.ExtraInfo != nil {
			result := map[string]any{
				"audio_size":  audioBuf.Len(),
				"extra_info":  lastChunk.ExtraInfo,
				"trace_id":    lastChunk.TraceID,
				"output_file": outputPath,
			}
			return outputResult(result, "", isJSONOutput())
		}

		return nil
	},
}

var speechAsyncCmd = &cobra.Command{
	Use:   "async",
	Short: "Create async speech synthesis task",
	Long: `Create an asynchronous speech synthesis task for long text.

Supports up to 1,000,000 characters. Returns a task ID for tracking.

Example request file (async-speech.yaml):
  model: speech-2.6-hd
  text: Very long text here...
  voice_setting:
    voice_id: female-shaonv

Examples:
  minimax -c myctx speech async -f long-text.yaml
  minimax -c myctx speech async -f long-text.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req minimax.AsyncSpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use defaults if not specified
		if req.Model == "" {
			req.Model = minimax.ModelSpeech26HD
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Text length: %d characters", len(req.Text))

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		task, err := client.Speech.CreateAsyncTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("create async task failed: %w", err)
		}

		printSuccess("Async task created: %s", task.ID)

		result := map[string]any{
			"task_id": task.ID,
			"status":  "created",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	speechCmd.AddCommand(speechSynthesizeCmd)
	speechCmd.AddCommand(speechStreamCmd)
	speechCmd.AddCommand(speechAsyncCmd)
}
