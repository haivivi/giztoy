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
  doubaospeech -c myctx realtime connect -f realtime.yaml`,
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

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runRealtimeConnect(reqCtx, client, &config, audioFile)
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

func runRealtimeConnect(ctx context.Context, client *ds.Client, config *ds.RealtimeConfig, audioFile string) error {
	session, err := client.Realtime.Connect(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	printSuccess("Connected to realtime service (session: %s)", session.SessionID())

	// If audio file provided, send it; otherwise read from stdin
	if audioFile != "" {
		audioData, err := os.ReadFile(audioFile)
		if err != nil {
			return fmt.Errorf("read audio file: %w", err)
		}

		printVerbose("Sending audio (%s)...", formatBytes(int64(len(audioData))))

		// Send audio in chunks
		chunkSize := 3200 // 100ms of 16kHz 16-bit mono
		for i := 0; i < len(audioData); i += chunkSize {
			end := i + chunkSize
			if end > len(audioData) {
				end = len(audioData)
			}

			if err := session.SendAudio(ctx, audioData[i:end]); err != nil {
				return fmt.Errorf("send audio: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		// Read audio from stdin
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, os.Stdin); err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		audioData := buf.Bytes()

		if len(audioData) > 0 {
			printVerbose("Sending audio from stdin (%s)...", formatBytes(int64(len(audioData))))
			chunkSize := 3200
			for i := 0; i < len(audioData); i += chunkSize {
				end := i + chunkSize
				if end > len(audioData) {
					end = len(audioData)
				}
				if err := session.SendAudio(ctx, audioData[i:end]); err != nil {
					return fmt.Errorf("send audio: %w", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// Receive events
	outputPath := getOutputFile()
	var audioBuf bytes.Buffer

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
			fmt.Print(event.Text)
		case ds.EventChatEnded:
			fmt.Println()
		case ds.EventAudioReceived:
			if len(event.Audio) > 0 {
				audioBuf.Write(event.Audio)
				printVerbose("[Audio] +%d bytes", len(event.Audio))
			}
		case ds.EventSessionFailed:
			if event.Error != nil {
				return fmt.Errorf("session failed: %s", event.Error.Message)
			}
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

	return nil
}

func init() {
	realtimeConnectCmd.Flags().String("audio", "", "Audio file to send (optional, defaults to stdin)")

	realtimeCmd.AddCommand(realtimeConnectCmd)
	realtimeCmd.AddCommand(realtimeInteractiveCmd)
}
