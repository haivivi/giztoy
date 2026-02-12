// Package doubao implements the 'giztoy doubao' subcommand tree.
// It directly calls the go/pkg/doubaospeech SDK.
package doubao

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
	"github.com/haivivi/giztoy/go/pkg/cli"
	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

// Cmd is the root 'doubao' command.
var Cmd = &cobra.Command{
	Use:   "doubao",
	Short: "Doubao Speech API (tts, asr, voice, realtime, meeting, podcast, ...)",
	Long: `Doubao Speech API client (火山引擎豆包语音).

Supported services:
  tts          Text-to-Speech synthesis (v1 classic, v2 bigmodel)
  asr          Automatic Speech Recognition (v1 classic, v2 bigmodel)
  voice        Voice cloning (train, status, list)
  realtime     End-to-end realtime voice conversation
  meeting      Meeting transcription
  podcast      Multi-speaker podcast synthesis
  translation  Simultaneous translation
  media        Audio/video subtitle extraction

Configuration (doubao.yaml in context dir):
  app_id: YOUR_APP_ID
  api_key: YOUR_ACCESS_TOKEN
  app_key: YOUR_APP_KEY          # optional, defaults to app_id
  console_ak: YOUR_CONSOLE_AK    # for voice list
  console_sk: YOUR_CONSOLE_SK    # for voice list
  default_voice: zh_female_cancan

Examples:
  giztoy config set dev doubao app_id 12345
  giztoy config set dev doubao api_key sk-xxx
  giztoy doubao tts v2 stream -f tts-v2.yaml -o output.mp3`,
}

// Flags shared by all doubao subcommands.
var (
	contextName string
	outputFile  string
	inputFile   string
	outputJSON  bool
)

func init() {
	Cmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")
	Cmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	Cmd.PersistentFlags().StringVarP(&inputFile, "file", "f", "", "input request file (YAML or JSON)")
	Cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output as JSON (for piping)")

	Cmd.AddCommand(ttsCmd)
	Cmd.AddCommand(asrCmd)
	Cmd.AddCommand(voiceCmd)
	Cmd.AddCommand(realtimeCmd)
	Cmd.AddCommand(meetingCmd)
	Cmd.AddCommand(podcastCmd)
	Cmd.AddCommand(translationCmd)
	Cmd.AddCommand(mediaCmd)
}

// ServiceConfig is the per-context doubao.yaml schema.
type ServiceConfig struct {
	AppID        string `yaml:"app_id"`
	APIKey       string `yaml:"api_key"`
	AppKey       string `yaml:"app_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultVoice string `yaml:"default_voice"`
	ConsoleAK    string `yaml:"console_ak"`
	ConsoleSK    string `yaml:"console_sk"`
}

// loadServiceConfig loads doubao.yaml from the resolved context directory.
func loadServiceConfig() (*ServiceConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	contextDir, err := cfg.ResolveContext(contextName)
	if err != nil {
		if contextName == "" {
			return nil, fmt.Errorf("no context set; use -c flag or 'giztoy config use-context <name>'")
		}
		return nil, err
	}
	svc, err := config.LoadService[ServiceConfig](contextDir, "doubao")
	if err != nil {
		return nil, fmt.Errorf("doubao config: %w", err)
	}
	return svc, nil
}

// createClient creates a Doubao Speech SDK client (loads config internally).
func createClient() (*ds.Client, error) {
	svc, err := loadServiceConfig()
	if err != nil {
		return nil, err
	}
	return createClientWith(svc)
}

// createClientWith creates a Doubao Speech SDK client from an already-loaded config.
func createClientWith(svc *ServiceConfig) (*ds.Client, error) {
	if svc.AppID == "" {
		return nil, fmt.Errorf("doubao app_id not configured; run: giztoy config set <context> doubao app_id <id>")
	}

	var opts []ds.Option
	if svc.APIKey != "" {
		opts = append(opts, ds.WithBearerToken(svc.APIKey))
	}
	appKey := svc.AppKey
	if appKey == "" {
		appKey = svc.AppID
	}
	if svc.APIKey != "" && appKey != "" {
		opts = append(opts, ds.WithV2APIKey(svc.APIKey, appKey))
	}
	if svc.BaseURL != "" {
		opts = append(opts, ds.WithBaseURL(svc.BaseURL))
	}

	return ds.NewClient(svc.AppID, opts...), nil
}

// createConsoleWith creates a Doubao Console client from an already-loaded config.
func createConsoleWith(svc *ServiceConfig) (*ds.Console, error) {
	if svc.ConsoleAK == "" || svc.ConsoleSK == "" {
		return nil, fmt.Errorf("console credentials (console_ak, console_sk) not configured")
	}
	return ds.NewConsole(svc.ConsoleAK, svc.ConsoleSK), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isVerbose() bool {
	v, _ := Cmd.Root().PersistentFlags().GetBool("verbose")
	return v
}

func printVerbose(format string, args ...any) {
	cli.PrintVerbose(isVerbose(), format, args...)
}

func requireInputFile() error {
	if inputFile == "" {
		return fmt.Errorf("input file is required, use -f flag")
	}
	return nil
}

func loadRequest(path string, v any) error { return cli.LoadRequest(path, v) }

func outputResult(result any, path string, asJSON bool) error {
	format := cli.FormatYAML
	if asJSON {
		format = cli.FormatJSON
	}
	return cli.Output(result, cli.OutputOptions{Format: format, File: path})
}

func outputBytes(data []byte, path string) error { return cli.OutputBytes(data, path) }

func formatBytes(bytes int64) string { return cli.FormatBytes(bytes) }

func printSuccess(format string, args ...any) { cli.PrintSuccess(format, args...) }
func printInfo(format string, args ...any)    { cli.PrintInfo(format, args...) }

// audioSender is the interface for sending audio chunks with an isLast flag.
type audioSender interface {
	SendAudio(ctx context.Context, audio []byte, isLast bool) error
}

// sendAudioChunked reads audio from a file (or stdin) and sends it in 3200-byte
// chunks with 100ms delay to simulate real-time streaming.
func sendAudioChunked(ctx context.Context, sender audioSender, audioFile string) error {
	return sendAudioChunkedFn(ctx, audioFile, func(chunk []byte, isLast bool) error {
		return sender.SendAudio(ctx, chunk, isLast)
	})
}

// sendAudioChunkedFn is the core chunked audio sender.
func sendAudioChunkedFn(_ context.Context, audioFile string, sendFn func(chunk []byte, isLast bool) error) error {
	audioData, err := readAudioInput(audioFile)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}
	if len(audioData) == 0 {
		return fmt.Errorf("empty audio data: provide a non-empty audio file")
	}

	printVerbose("Sending audio (%s)...", formatBytes(int64(len(audioData))))

	chunkSize := 3200 // 100ms of 16kHz 16-bit mono
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}
		if err := sendFn(audioData[i:end], isLast); err != nil {
			return fmt.Errorf("send audio: %w", err)
		}
		if !isLast {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func readAudioInput(audioFile string) ([]byte, error) {
	if audioFile != "" {
		return os.ReadFile(audioFile)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, os.Stdin); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return buf.Bytes(), nil
}
