package doubao

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

Example config file (realtime.yaml):
  tts:
    speaker: zh_female_cancan
    audio_config:
      channel: 1
      format: pcm
      sample_rate: 24000
  dialog:
    bot_name: 小助手
    system_role: 你是一个友好的语音助手`,
}

var realtimeConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to realtime service",
	Long: `Connect to the real-time conversation service.

Supports both audio input (--audio) and text input (--greeting/-g).

Examples:
  giztoy doubao realtime connect -f realtime.yaml -g "你好" -o reply.pcm
  giztoy doubao realtime connect -f realtime.yaml --audio input.pcm -o reply.pcm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var config ds.RealtimeConfig
		if err := loadRequest(inputFile, &config); err != nil {
			return err
		}

		audioFile, _ := cmd.Flags().GetString("audio")
		greeting, _ := cmd.Flags().GetString("greeting")

		if audioFile == "" && greeting == "" {
			return fmt.Errorf("either --audio or --greeting/-g is required")
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		session, err := client.Realtime.Connect(reqCtx, &config)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer session.Close()

		printSuccess("Connected to realtime service (session: %s)", session.SessionID())

		// Send input
		if greeting != "" {
			printVerbose("Sending greeting: %s", greeting)
			if err := session.SayHello(reqCtx, greeting); err != nil {
				return fmt.Errorf("send greeting: %w", err)
			}
		} else if audioFile != "" {
			if err := sendAudioChunkedFn(reqCtx, audioFile, func(chunk []byte, _ bool) error {
				return session.SendAudio(reqCtx, chunk)
			}); err != nil {
				return err
			}
		}

		// Receive events
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
		if audioBuf.Len() > 0 && outputFile != "" {
			if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
				return fmt.Errorf("write audio: %w", err)
			}
			printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))
		}

		if outputJSON {
			return outputResult(map[string]any{
				"session_id":    session.SessionID(),
				"response_text": responseText,
				"audio_size":    audioBuf.Len(),
			}, "", true)
		}
		return nil
	},
}

func init() {
	realtimeConnectCmd.Flags().String("audio", "", "Audio file to send (PCM format)")
	realtimeConnectCmd.Flags().StringP("greeting", "g", "", "Text greeting to send (uses SayHello API)")

	realtimeCmd.AddCommand(realtimeConnectCmd)
}
