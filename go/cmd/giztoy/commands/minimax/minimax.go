// Package minimax implements the 'giztoy minimax' subcommand tree.
// It directly calls the go/pkg/minimax SDK — no intermediate wrapper layer.
package minimax

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
	"github.com/haivivi/giztoy/go/pkg/cli"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

// Cmd is the root 'minimax' command.
var Cmd = &cobra.Command{
	Use:   "minimax",
	Short: "MiniMax API (text, speech, video, image, music, voice, file)",
	Long: `MiniMax API client.

Supported services:
  text     Text generation (chat completions)
  speech   Speech synthesis (TTS)
  video    Video generation (T2V, I2V, frame)
  image    Image generation
  music    Music generation with vocals
  voice    Voice management (list, clone, design)
  file     File management (upload, download, list)

Configuration is loaded from the active context's minimax.yaml:
  giztoy config set <context> minimax api_key YOUR_KEY

Examples:
  giztoy minimax text chat -f chat.yaml
  giztoy minimax speech synthesize -f speech.yaml -o output.mp3
  giztoy minimax -c dev video t2v -f t2v.yaml --wait`,
}

// Flags shared by all minimax subcommands.
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

	Cmd.AddCommand(textCmd)
	Cmd.AddCommand(speechCmd)
	Cmd.AddCommand(videoCmd)
	Cmd.AddCommand(imageCmd)
	Cmd.AddCommand(musicCmd)
	Cmd.AddCommand(voiceCmd)
	Cmd.AddCommand(fileCmd)
}

// ServiceConfig is the per-context minimax.yaml schema.
type ServiceConfig struct {
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultModel string `yaml:"default_model"`
	DefaultVoice string `yaml:"default_voice"`
	MaxRetries   int    `yaml:"max_retries"`
}

// loadServiceConfig loads minimax.yaml from the resolved context directory.
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
	svc, err := config.LoadService[ServiceConfig](contextDir, "minimax")
	if err != nil {
		return nil, fmt.Errorf("minimax config: %w", err)
	}
	return svc, nil
}

// createClient creates a MiniMax SDK client from the service config.
func createClient() (*minimax.Client, error) {
	svc, err := loadServiceConfig()
	if err != nil {
		return nil, err
	}
	if svc.APIKey == "" {
		return nil, fmt.Errorf("minimax api_key not configured; run: giztoy config set <context> minimax api_key <key>")
	}

	var opts []minimax.Option
	if svc.BaseURL != "" {
		opts = append(opts, minimax.WithBaseURL(svc.BaseURL))
	}
	if svc.MaxRetries > 0 {
		opts = append(opts, minimax.WithRetry(svc.MaxRetries))
	}
	return minimax.NewClient(svc.APIKey, opts...), nil
}

// ---------------------------------------------------------------------------
// Helpers (shared by all minimax subcommands)
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

func loadRequest(path string, v any) error {
	return cli.LoadRequest(path, v)
}

func outputResult(result any, path string, asJSON bool) error {
	format := cli.FormatYAML
	if asJSON {
		format = cli.FormatJSON
	}
	return cli.Output(result, cli.OutputOptions{Format: format, File: path})
}

func outputBytes(data []byte, path string) error {
	return cli.OutputBytes(data, path)
}

func formatBytes(n int) string { return cli.FormatBytesInt(n) }

func printSuccess(format string, args ...any) { cli.PrintSuccess(format, args...) }
func printInfo(format string, args ...any)    { cli.PrintInfo(format, args...) }

// downloadFile downloads a URL to a local file.
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	written, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(dest)
		return fmt.Errorf("write file: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(dest)
		return fmt.Errorf("close file: %w", closeErr)
	}
	printSuccess("File saved to %s (%s)", dest, formatBytes(int(written)))
	return nil
}
