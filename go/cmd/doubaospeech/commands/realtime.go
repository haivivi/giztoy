package commands

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
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
Supports both audio input (--audio) and text input (--greeting/-g).

Examples:
  # Send a text greeting and get audio response
  doubaospeech -c myctx realtime connect -f realtime.yaml -g "你好" -o reply.pcm

  # Send audio file
  doubaospeech -c myctx realtime connect -f realtime.yaml --audio input.pcm -o reply.pcm`,
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

		var config ds.RealtimeConfig
		if err := loadRequest(getInputFile(), &config); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)

		audioFile, _ := cmd.Flags().GetString("audio")
		greeting, _ := cmd.Flags().GetString("greeting")

		if audioFile == "" && greeting == "" {
			return fmt.Errorf("either --audio or --greeting/-g is required")
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		return runRealtimeConnect(reqCtx, client, &config, audioFile, greeting)
	},
}

var realtimeInteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive voice conversation",
	Long: `Start an interactive voice conversation session.

This mode captures audio from your microphone and plays
responses through your speakers.

Examples:
  doubaospeech -c myctx realtime interactive -f realtime.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented: interactive realtime mode requires microphone/speaker hardware integration; use 'doubaospeech realtime connect' for programmatic access")
	},
}

// ============================================================================
// Implementation Functions
// ============================================================================

func runRealtimeConnect(ctx context.Context, client *ds.Client, config *ds.RealtimeConfig, audioFile, greeting string) error {
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	printSuccess("Connected to realtime service (session: %s)", session.SessionID())

	// Send input
	if greeting != "" {
		// Text input mode: send greeting via SayHello
		printVerbose("Sending greeting: %s", greeting)
		if err := session.SayHello(ctx, greeting); err != nil {
			return fmt.Errorf("send greeting: %w", err)
		}
	} else if audioFile != "" {
		// Audio input mode (realtime SendAudio has no isLast param)
		if err := sendAudioChunkedFn(ctx, audioFile, func(chunk []byte, _ bool) error {
			return session.SendAudio(ctx, chunk)
		}); err != nil {
			return err
		}
	}

	// Receive events with timeout
	outputPath := getOutputFile()
	var audioBuf bytes.Buffer
	var responseText string

	for event, err := range session.Recv() {
		if err != nil {
			return fmt.Errorf("receive error: %w", err)
		}

		switch event.Type {
		case ds.EventASRResponse:
			if event.ASRInfo != nil {
				if event.ASRInfo.IsFinal {
					printInfo("[ASR] %s", event.ASRInfo.Text)
				} else {
					printVerbose("[ASR interim] %s", event.ASRInfo.Text)
				}
			}
		case ds.EventChatResponse:
			responseText += event.Text
			fmt.Print(event.Text)
		case ds.EventChatEnded:
			fmt.Println()
			// Chat ended = response complete, we can exit after collecting remaining audio
		case ds.EventAudioReceived:
			if len(event.Audio) > 0 {
				audioBuf.Write(event.Audio)
				printVerbose("[Audio] +%d bytes (total: %s)", len(event.Audio), formatBytes(int64(audioBuf.Len())))
			}
		case ds.EventTTSFinished:
			printVerbose("TTS finished")
			goto done
		case ds.EventSessionFailed:
			if event.Error != nil {
				return fmt.Errorf("session failed: %s", event.Error.Message)
			}
			return fmt.Errorf("session failed")
		case ds.EventSessionFinished:
			printVerbose("Session finished")
			goto done
		}
	}

done:
	if audioBuf.Len() > 0 && outputPath != "" {
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("write audio: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	if isJSONOutput() {
		result := map[string]any{
			"session_id":    session.SessionID(),
			"response_text": responseText,
			"audio_size":    audioBuf.Len(),
		}
		return outputResult(result, "", true)
	}

	return nil
}

func init() {
	realtimeConnectCmd.Flags().String("audio", "", "Audio file to send (PCM format)")
	realtimeConnectCmd.Flags().StringP("greeting", "g", "", "Text greeting to send (uses SayHello API)")

	realtimeCmd.AddCommand(realtimeConnectCmd)
	realtimeCmd.AddCommand(realtimeInteractiveCmd)
}
