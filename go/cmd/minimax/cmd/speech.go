package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
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

		var req mm.SpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		// Use defaults if not specified
		if req.Model == "" {
			req.Model = mm.ModelSpeech26HD
		}
		if req.VoiceSetting != nil && req.VoiceSetting.VoiceID == "" {
			if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
				req.VoiceSetting.VoiceID = defaultVoice
			}
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Model: %s", req.Model)
		printVerbose("Text length: %d characters", len(req.Text))

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
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

		var req mm.SpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Streaming to: %s", outputPath)

		// TODO: Implement actual streaming API call
		fmt.Println("[Streaming not implemented yet]")
		fmt.Printf("Would stream speech to %s\n", outputPath)

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

		var req mm.AsyncSpeechRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Text length: %d characters", len(req.Text))

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	speechCmd.AddCommand(speechSynthesizeCmd)
	speechCmd.AddCommand(speechStreamCmd)
	speechCmd.AddCommand(speechAsyncCmd)
}
