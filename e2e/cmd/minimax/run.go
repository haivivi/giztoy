// Command run is a cross-platform test executor for the MiniMax CLI.
//
// It replaces the previous run.sh shell script.
//
// Usage:
//
//	bazel run //e2e/cmd/minimax:run -- [runtime] [test_level]
//
//	runtime:
//	  go    - Use Go CLI via giztoy (default)
//	  rust  - Use Rust CLI
//	  both  - Test both Go and Rust
//
//	test_level:
//	  1       - Basic (TTS, Chat)
//	  2       - Image generation
//	  3       - Streaming
//	  4       - Video generation
//	  5       - Voice management
//	  6       - Voice clone
//	  7       - File management
//	  8       - Music generation
//	  all     - All tests (default)
//	  quick   - Quick smoke test (basic + voice list)
//	  help    - Show usage
//
// Environment variables:
//
//	MINIMAX_CONTEXT  - Context name (default: minimax_cn)
//	MINIMAX_API_KEY  - API Key for auto-context setup
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ANSI color codes
const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorCyan   = "\033[0;36m"
	colorReset  = "\033[0m"
)

// testCase describes a single CLI test invocation.
type testCase struct {
	Name  string
	Level int
	Tags  []string
	Args  []string
}

func projectRoot() string {
	if dir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); dir != "" {
		return dir
	}
	dir, _ := os.Getwd()
	return dir
}

// findGiztoy locates the giztoy binary.
func findGiztoy(root string) string {
	bazelBin := filepath.Join(root, "bazel-bin", "go", "cmd", "giztoy", "giztoy_", "giztoy")
	if _, err := os.Stat(bazelBin); err == nil {
		return bazelBin
	}
	if p, err := exec.LookPath("giztoy"); err == nil {
		return p
	}
	return ""
}

// findRustCLI locates the Rust minimax binary.
func findRustCLI(root string) string {
	bazelBin := filepath.Join(root, "bazel-bin", "rust", "cmd", "minimax", "minimax")
	if _, err := os.Stat(bazelBin); err == nil {
		return bazelBin
	}
	cargoRelease := filepath.Join(root, "rust", "target", "release", "minimax")
	if _, err := os.Stat(cargoRelease); err == nil {
		return cargoRelease
	}
	if p, err := exec.LookPath("minimax"); err == nil {
		return p
	}
	return ""
}

func main() {
	runtime := "go"
	testLevel := "all"
	if len(os.Args) > 1 {
		runtime = os.Args[1]
	}
	if len(os.Args) > 2 {
		testLevel = os.Args[2]
	}

	if testLevel == "help" || runtime == "help" || runtime == "-h" || runtime == "--help" {
		showHelp()
		return
	}

	root := projectRoot()
	commandsDir := filepath.Join(root, "e2e", "cmd", "minimax", "commands")
	outputDir := filepath.Join(root, "e2e", "cmd", "minimax", "output")
	contextName := envOr("MINIMAX_CONTEXT", "minimax_cn")

	os.MkdirAll(outputDir, 0755)

	fmt.Println()
	fmt.Println("======================================")
	fmt.Println("   MiniMax API Test Runner")
	fmt.Println("======================================")
	fmt.Println()
	logInfo("Runtime:     %s", runtime)
	logInfo("Test level:  %s", testLevel)
	logInfo("Commands:    %s", commandsDir)
	logInfo("Output:      %s", outputDir)
	logInfo("Context:     %s", contextName)
	fmt.Println()

	switch runtime {
	case "go":
		runWithGo(root, commandsDir, outputDir, contextName, testLevel)
	case "rust":
		runWithRust(root, commandsDir, outputDir, contextName, testLevel)
	case "both":
		fmt.Println("===== Go CLI =====")
		runWithGo(root, commandsDir, outputDir, contextName, testLevel)
		fmt.Println()
		fmt.Println("===== Rust CLI =====")
		runWithRust(root, commandsDir, outputDir, contextName, testLevel)
	default:
		logError("Unknown runtime: %s", runtime)
		showHelp()
		os.Exit(1)
	}
}

func runWithGo(root, commandsDir, outputDir, contextName, testLevel string) {
	cli := findGiztoy(root)
	if cli == "" {
		logError("Cannot find giztoy binary. Run: bazel build //go/cmd/giztoy")
		os.Exit(1)
	}
	logInfo("CLI binary: %s", cli)

	tests := buildTestCases(commandsDir, outputDir, contextName, "go")
	selected := filterTests(tests, testLevel)
	if len(selected) == 0 {
		logError("Unknown test level: %s", testLevel)
		showHelp()
		os.Exit(1)
	}

	setupContextGo(cli, contextName)
	runTests(cli, selected, contextName, "minimax")
}

func runWithRust(root, commandsDir, outputDir, contextName, testLevel string) {
	cli := findRustCLI(root)
	if cli == "" {
		logError("Cannot find Rust minimax binary. Run: bazel build //rust/cmd/minimax")
		os.Exit(1)
	}
	logInfo("CLI binary: %s", cli)

	tests := buildTestCases(commandsDir, outputDir, contextName, "rust")
	selected := filterTests(tests, testLevel)
	if len(selected) == 0 {
		logError("Unknown test level: %s", testLevel)
		showHelp()
		os.Exit(1)
	}

	setupContextRust(cli, contextName)
	runTests(cli, selected, contextName, "")
}

func buildTestCases(commandsDir, outputDir, _ string, runtime string) []testCase {
	cmd := func(name string) string { return filepath.Join(commandsDir, name) }
	out := func(name string) string { return filepath.Join(outputDir, name) }

	return []testCase{
		// Level 1: Basic (TTS, Chat)
		{Name: "TTS Speech Synthesis", Level: 1, Tags: []string{"tts"},
			Args: []string{"speech", "synthesize", "-f", cmd("speech.yaml"), "-o", out("speech_" + runtime + ".mp3")}},
		{Name: "Text Chat", Level: 1, Tags: []string{"chat"},
			Args: []string{"text", "chat", "-f", cmd("chat.yaml")}},

		// Level 2: Image generation
		{Name: "Image Generation", Level: 2, Tags: []string{"image"},
			Args: []string{"image", "generate", "-f", cmd("image.yaml")}},

		// Level 3: Streaming
		{Name: "Streaming TTS", Level: 3, Tags: []string{"tts", "stream"},
			Args: []string{"speech", "stream", "-f", cmd("speech.yaml"), "-o", out("speech_stream_" + runtime + ".mp3")}},
		{Name: "Streaming Text Chat", Level: 3, Tags: []string{"chat", "stream"},
			Args: []string{"text", "stream", "-f", cmd("chat.yaml")}},

		// Level 4: Video generation
		{Name: "Video T2V", Level: 4, Tags: []string{"video"},
			Args: []string{"video", "t2v", "-f", cmd("video-t2v.yaml")}},

		// Level 5: Voice management
		{Name: "Voice List", Level: 5, Tags: []string{"voice"},
			Args: []string{"voice", "list", "--json"}},

		// Level 6: Voice clone
		{Name: "Voice Clone Source", Level: 6, Tags: []string{"voice"},
			Args: []string{"speech", "synthesize", "-f", cmd("clone-source.yaml"), "-o", out("clone_source_" + runtime + ".mp3")}},

		// Level 7: File management (requires level 1 output)
		{Name: "File Upload", Level: 7, Tags: []string{"file"},
			Args: []string{"file", "upload", "--file", out("speech_" + runtime + ".mp3"), "--purpose", "voice_clone"}},
		{Name: "File List", Level: 7, Tags: []string{"file"},
			Args: []string{"file", "list", "--json"}},

		// Level 8: Music generation
		{Name: "Music Generation", Level: 8, Tags: []string{"music"},
			Args: []string{"music", "generate", "-f", cmd("music.yaml"), "-o", out("music_" + runtime + ".mp3")}},
	}
}

func filterTests(tests []testCase, level string) []testCase {
	switch level {
	case "all":
		return tests
	case "quick":
		return filterByLevels(tests, 1, 5)
	case "1", "2", "3", "4", "5", "6", "7", "8":
		n := int(level[0] - '0')
		return filterByLevels(tests, n)
	default:
		return nil
	}
}

func filterByLevels(tests []testCase, levels ...int) []testCase {
	set := make(map[int]bool, len(levels))
	for _, l := range levels {
		set[l] = true
	}
	var out []testCase
	for _, tc := range tests {
		if set[tc.Level] {
			out = append(out, tc)
		}
	}
	return out
}

func setupContextGo(cli, contextName string) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey != "" {
		run(cli, "minimax", "config", "add-context", contextName, "--api-key", apiKey)
	}
	run(cli, "minimax", "config", "use-context", contextName)
	logInfo("Context ready: %s", contextName)
}

func setupContextRust(cli, contextName string) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey != "" {
		run(cli, "config", "add-context", contextName, "--api-key", apiKey)
	}
	run(cli, "config", "use-context", contextName)
	logInfo("Context ready: %s", contextName)
}

// runTests runs the selected tests. subcmd is the giztoy subcommand prefix
// (e.g. "minimax" for Go), or "" for Rust direct invocation.
func runTests(cli string, tests []testCase, contextName, subcmd string) {
	logInfo("Tests: %d", len(tests))
	fmt.Println()

	passed, failed := 0, 0
	for _, tc := range tests {
		var args []string
		if subcmd != "" {
			args = append(args, subcmd, "-c", contextName)
		} else {
			args = append(args, "-c", contextName)
		}
		args = append(args, tc.Args...)
		if runTest(cli, tc.Name, args) {
			passed++
		} else {
			failed++
		}
	}

	fmt.Println()
	fmt.Println("======================================")
	fmt.Printf("   Results: %s%d passed%s", colorGreen, passed, colorReset)
	if failed > 0 {
		fmt.Printf(", %s%d failed%s", colorRed, failed, colorReset)
	}
	fmt.Println()
	fmt.Println("======================================")
	fmt.Println()

	if failed > 0 {
		os.Exit(1)
	}
}

func runTest(cli, name string, args []string) bool {
	logInfo("Testing: %s", name)
	cmd := exec.Command(cli, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logFail("%s: %v", name, err)
		return false
	}
	logPass(name)
	return true
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Run() //nolint:errcheck
}

// Logging helpers
func logInfo(format string, args ...any) {
	fmt.Printf("%s[INFO]%s %s\n", colorBlue, colorReset, fmt.Sprintf(format, args...))
}
func logPass(format string, args ...any) {
	fmt.Printf("%s[PASS]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}
func logFail(format string, args ...any) {
	fmt.Printf("%s[FAIL]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}
func logError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func showHelp() {
	fmt.Println(`MiniMax CLI Test Runner

Usage:
  bazel run //e2e/cmd/minimax:run -- [runtime] [test_level]

Runtime:
  go      Use Go CLI via giztoy (default)
  rust    Use Rust CLI
  both    Test both Go and Rust

Test levels:
  1         Basic (TTS, Chat)
  2         Image generation
  3         Streaming (TTS, Chat)
  4         Video generation
  5         Voice management
  6         Voice clone
  7         File management
  8         Music generation
  all       All tests (default)
  quick     Quick smoke test (basic + voice list)
  help      Show this help

Environment variables:
  MINIMAX_CONTEXT    Context name (default: minimax_cn)
  MINIMAX_API_KEY    API Key for auto-context setup`)
}

