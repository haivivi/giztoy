package commands

import "github.com/spf13/cobra"

var doubaospeechCmd = &cobra.Command{
	Use:   "doubaospeech",
	Short: "Doubao Speech API (TTS, ASR, voice, realtime, podcast, meeting, translation, media)",
	Long: `Doubao Speech (火山引擎豆包语音) API client.
Supports multiple app instances.

Usage: giztoy doubaospeech <app_name> <command...>

Examples:
  giztoy doubaospeech test tts v2 stream -f request.yaml
  giztoy doubaospeech prod asr v2 stream -f request.yaml --audio input.pcm`,
}

func init() {
	addAppCommands(doubaospeechCmd, "doubaospeech", []appField{
		{Name: "app_id", Flag: "app-id", Required: true, Desc: "Application ID"},
		{Name: "token", Flag: "token", Required: true, Desc: "Bearer Token"},
		{Name: "api_key", Flag: "api-key", Required: false, Desc: "V2 API Key"},
		{Name: "app_key", Flag: "app-key", Required: false, Desc: "App Key (defaults to app_id)"},
		{Name: "base_url", Flag: "base-url", Required: false, Desc: "API base URL"},
		{Name: "console_ak", Flag: "console-ak", Required: false, Desc: "Console Access Key (for voice list)"},
		{Name: "console_sk", Flag: "console-sk", Required: false, Desc: "Console Secret Key (for voice list)"},
	})
	rootCmd.AddCommand(doubaospeechCmd)
}
