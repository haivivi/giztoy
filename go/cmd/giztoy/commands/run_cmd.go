package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cortex"
)

var runFile string

var runTaskCmd = &cobra.Command{
	Use:   "run -f <file>",
	Short: "Execute a task from YAML",
	Long: `Execute a task defined in a YAML file. Use '-' to read from stdin.

The task YAML must have a 'kind' field that determines the operation.

Run kinds (genx):
  genx/generator    LLM text generation
  genx/tts          Text-to-speech synthesis
  genx/asr          Automatic speech recognition

Run kinds (direct SDK):
  minimax/text/chat, minimax/speech/synthesize, ...
  doubaospeech/tts/v2/stream, doubaospeech/asr/v2/stream, ...
  dashscope/omni/chat
  openai/text/chat
  genai/text/generate

Run kinds (memory):
  memory/create, memory/recall, memory/search, memory/add, ...

Examples:
  giztoy run -f testdata/run/genx/generator-chat.yaml
  giztoy run -f testdata/run/minimax/text-chat.yaml --format json
  giztoy run -f testdata/run/minimax/speech-synthesize.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runFile == "" {
			return fmt.Errorf("flag -f is required")
		}

		docs, err := cortex.ParseDocumentsFromFile(runFile)
		if err != nil {
			return err
		}
		if len(docs) != 1 {
			return fmt.Errorf("run expects exactly 1 document, got %d", len(docs))
		}

		task := docs[0]

		// Override output file if -o provided
		if outputFile != "" {
			task.Fields["output"] = outputFile
		}

		c, err := openCortex(cmd.Context())
		if err != nil {
			return err
		}
		defer c.Close()

		result, err := c.Run(cmd.Context(), task)
		if err != nil {
			return err
		}

		if formatOutput == "json" {
			return printJSON(result)
		}

		// Default: human-readable
		if result.Text != "" {
			fmt.Println(result.Text)
		}
		if result.AudioFile != "" {
			fmt.Printf("Audio saved to: %s (%d bytes)\n", result.AudioFile, result.AudioSize)
		}
		if result.TaskID != "" {
			fmt.Printf("Task ID: %s\n", result.TaskID)
		}
		if result.Status != "" && result.Text == "" && result.AudioFile == "" && result.TaskID == "" {
			fmt.Printf("Status: %s\n", result.Status)
		}
		return nil
	},
}

func init() {
	runTaskCmd.Flags().StringVarP(&runFile, "file", "f", "", "task YAML file (use '-' for stdin)")
	rootCmd.AddCommand(runTaskCmd)
}
