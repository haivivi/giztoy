// Package main provides the Doubao Speech CLI tool.
//
// Usage:
//
//	doubao [flags] <service> <command> [args]
//
// Services:
//
//	tts         - Text-to-Speech synthesis service
//	asr         - Automatic Speech Recognition service
//	voice       - Voice cloning service
//	realtime    - Real-time voice conversation service
//	meeting     - Meeting transcription service
//	podcast     - Podcast synthesis service
//	translation - Simultaneous translation service
//	media       - Media processing service (subtitle extraction)
//	console     - Console management service
//	config      - Configuration management
//
// Configuration:
//
//	The CLI stores configuration in ~/.giztoy/doubao/
//	Use 'doubao config' commands to manage contexts.
package main

import (
	"os"

	"github.com/haivivi/giztoy/cmd/doubao/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
