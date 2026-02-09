package commands

import (
	doubaocmd "github.com/haivivi/giztoy/go/cmd/doubaospeech/commands"
)

func init() {
	cmd := doubaocmd.Command()
	cmd.Use = "doubao"
	cmd.Short = "Doubao Speech API (tts, asr, voice, realtime, meeting, podcast, ...)"
	rootCmd.AddCommand(cmd)
}
