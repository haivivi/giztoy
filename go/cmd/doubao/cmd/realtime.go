package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	dsi "github.com/haivivi/giztoy/pkg/doubao_speech_interface"
)

var realtimeCmd = &cobra.Command{
	Use:   "realtime",
	Short: "Real-time voice conversation service",
	Long: `Real-time end-to-end voice conversation service.

Enables bidirectional voice communication with AI.

Example config file (realtime.yaml):
  asr:
    extra:
      end_smooth_window_ms: 200
  tts:
    speaker: zh_female_cancan
    audio_config:
      channel: 1
      format: pcm
      sample_rate: 24000
  dialog:
    bot_name: 小助手
    system_role: 你是一个友好的语音助手
    speaking_style: 温柔亲切`,
}

var realtimeConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to realtime service",
	Long: `Connect to the real-time conversation service.

Establishes a WebSocket connection for bidirectional communication.

Examples:
  doubao -c myctx realtime connect -f realtime.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.RealtimeConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement realtime connection
		fmt.Println("[Realtime connection not implemented yet]")
		fmt.Println("Would connect to realtime service...")

		return nil
	},
}

var realtimeInteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive voice conversation",
	Long: `Start an interactive voice conversation session.

This mode captures audio from your microphone and plays
responses through your speakers.

Examples:
  doubao -c myctx realtime interactive -f realtime.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		var req dsi.RealtimeConfig
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement interactive mode
		fmt.Println("[Interactive mode not implemented yet]")
		fmt.Println("Would start interactive voice conversation...")
		fmt.Println("Press Ctrl+C to exit")

		return nil
	},
}

func init() {
	realtimeCmd.AddCommand(realtimeConnectCmd)
	realtimeCmd.AddCommand(realtimeInteractiveCmd)
}
