package commands

import (
	minimaxcmd "github.com/haivivi/giztoy/go/cmd/minimax/commands"
)

func init() {
	cmd := minimaxcmd.Command()
	cmd.Use = "minimax"
	cmd.Short = "MiniMax API (text, speech, video, image, music, voice, file)"
	rootCmd.AddCommand(cmd)
}
