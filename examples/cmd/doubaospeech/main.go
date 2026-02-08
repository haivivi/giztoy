// Command run is a cross-platform test executor for the doubaospeech CLI.
//
// It replaces the previous run.sh shell script to comply with the
// workspace rule: "用 Go 写跨平台的 executor（go_binary），避免平台依赖".
//
// Usage:
//
//	bazel run //examples/cmd/doubaospeech:run -- [test_level]
//
//	test_level:
//	  1       - TTS V2 HTTP streaming (BigModel) - recommended
//	  2       - TTS V2 bidirectional WebSocket
//	  3       - TTS V1 (classic, requires volcano_tts grant)
//	  4       - ASR V2 streaming (uses testdata audio)
//	  5       - ASR V2 file recognition (requires audio URL)
//	  6       - Podcast SAMI (WebSocket)
//	  7       - Meeting transcription
//	  8       - Subtitle extraction
//	  9       - Realtime
//	  all     - All tests (default)
//	  quick   - Quick smoke test (TTS V2 HTTP + ASR V2 stream)
//	  tts     - All TTS tests
//	  asr     - All ASR tests
//	  help    - Show usage
//
// Environment variables:
//
//	DOUBAO_CONTEXT  - Context name (default: test)
//	DOUBAO_APP_ID   - App ID for auto-context setup
//	DOUBAO_API_KEY  - API Key for auto-context setup
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	Name  string   // Human-readable test name
	Level int      // Test level (1-9)
	Tags  []string // Tags for filtering: "tts", "asr", etc.
	Args  []string // CLI arguments (after the binary name)
}

// projectRoot returns the workspace root directory.
// Under `bazel run`, BUILD_WORKSPACE_DIRECTORY is set.
// Otherwise, derive from the executable or working directory.
func projectRoot() string {
	if dir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); dir != "" {
		return dir
	}
	// Fallback: assume cwd is the project root
	dir, _ := os.Getwd()
	return dir
}

// findCLI locates the doubaospeech CLI binary.
func findCLI(root string) string {
	// Bazel-built binary location
	bazelBin := filepath.Join(root, "bazel-bin", "go", "cmd", "doubaospeech", "doubaospeech_", "doubaospeech")
	if _, err := os.Stat(bazelBin); err == nil {
		return bazelBin
	}
	// Fallback: PATH lookup
	if p, err := exec.LookPath("doubaospeech"); err == nil {
		return p
	}
	return ""
}

func main() {
	testLevel := "all"
	if len(os.Args) > 1 {
		testLevel = os.Args[1]
	}

	if testLevel == "help" || testLevel == "-h" || testLevel == "--help" {
		showHelp()
		return
	}

	root := projectRoot()
	commandsDir := filepath.Join(root, "examples", "cmd", "doubaospeech", "commands")
	outputDir := filepath.Join(root, "examples", "cmd", "doubaospeech", "output")
	contextName := envOr("DOUBAO_CONTEXT", "test")

	// Ensure output directory exists
	os.MkdirAll(outputDir, 0755)

	// Find CLI binary
	cli := findCLI(root)
	if cli == "" {
		logError("Cannot find doubaospeech binary. Run: bazel build //go/cmd/doubaospeech")
		os.Exit(1)
	}
	logInfo("CLI binary: %s", cli)

	// Build all test cases
	tests := buildTestCases(commandsDir, outputDir, contextName)

	// Filter by level
	selected := filterTests(tests, testLevel)
	if len(selected) == 0 {
		logError("Unknown test level: %s", testLevel)
		showHelp()
		os.Exit(1)
	}

	// Setup context
	setupContext(cli, contextName)

	// Print header
	fmt.Println()
	fmt.Println("======================================")
	fmt.Println("   Doubao Speech API Test Runner")
	fmt.Println("======================================")
	fmt.Println()
	logInfo("Test level:  %s", testLevel)
	logInfo("Commands:    %s", commandsDir)
	logInfo("Output:      %s", outputDir)
	logInfo("Context:     %s", contextName)
	logInfo("Tests:       %d", len(selected))
	fmt.Println()

	// Run tests
	passed, failed := 0, 0
	for _, tc := range selected {
		args := append([]string{"-c", contextName}, tc.Args...)
		if runTest(cli, tc.Name, args) {
			passed++
		} else {
			failed++
		}
	}

	// Summary
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

// buildTestCases returns all test case definitions.
// commandsDir is the absolute path to the YAML request files.
// outputDir is the absolute path for output audio files.
func buildTestCases(commandsDir, outputDir, _ string) []testCase {
	cmd := func(name string) string { return filepath.Join(commandsDir, name) }
	out := func(name string) string { return filepath.Join(outputDir, name) }
	testdata := func(name string) string {
		return filepath.Join(filepath.Dir(commandsDir), "testdata", name)
	}

	return []testCase{
		// Level 1: TTS V2 HTTP Streaming
		{
			Name:  "TTS V2 HTTP Streaming",
			Level: 1, Tags: []string{"tts", "v2"},
			Args: []string{"tts", "v2", "stream", "-f", cmd("tts-v2.yaml"), "-o", out("tts_v2_stream.mp3")},
		},

		// Level 2: TTS V2 WebSocket bidirectional
		{
			Name:  "TTS V2 Bidirectional WebSocket",
			Level: 2, Tags: []string{"tts", "v2"},
			Args: []string{"tts", "v2", "bidirectional", "-f", cmd("tts-v2.yaml"), "-o", out("tts_v2_bidi.mp3")},
		},

		// Level 3: TTS V1 (Classic, requires volcano_tts grant)
		{
			Name:  "TTS V1 Synchronous",
			Level: 3, Tags: []string{"tts", "v1"},
			Args: []string{"tts", "v1", "synthesize", "-f", cmd("tts-v1.yaml"), "-o", out("tts_v1.mp3")},
		},

		// Level 4: ASR V2 Streaming (uses testdata audio)
		{
			Name:  "ASR V2 Streaming",
			Level: 4, Tags: []string{"asr", "v2"},
			Args: []string{"asr", "v2", "stream", "-f", cmd("asr-v2-stream.yaml"), "--audio", testdata("test_speech.mp3"), "--json"},
		},

		// Level 5: ASR V2 File (requires audio URL -- placeholder)
		{
			Name:  "ASR V2 File Recognition",
			Level: 5, Tags: []string{"asr", "v2"},
			Args: []string{"asr", "v2", "file", "-f", cmd("asr-v2-file.yaml"), "--json"},
		},

		// Level 6: Podcast SAMI
		{
			Name:  "Podcast SAMI WebSocket",
			Level: 6, Tags: []string{"podcast"},
			Args: []string{"podcast", "sami", "-f", cmd("podcast-sami.yaml"), "-o", out("podcast_sami.mp3")},
		},

		// Level 7: Meeting
		{
			Name:  "Meeting Transcription Create",
			Level: 7, Tags: []string{"meeting"},
			Args: []string{"meeting", "create", "-f", cmd("meeting.yaml"), "--json"},
		},

		// Level 8: Subtitle
		{
			Name:  "Subtitle Extraction",
			Level: 8, Tags: []string{"media"},
			Args: []string{"media", "subtitle", "-f", cmd("subtitle.yaml"), "--json"},
		},

		// Level 9: Realtime (text greeting mode)
		{
			Name:  "Realtime Connect",
			Level: 9, Tags: []string{"realtime"},
			Args: []string{"realtime", "connect", "-f", cmd("realtime.yaml"), "-g", "你好，今天天气怎么样？", "--json"},
		},
	}
}

// filterTests selects tests matching the given level/tag.
func filterTests(tests []testCase, level string) []testCase {
	switch level {
	case "all":
		return tests
	case "quick":
		return filterByLevels(tests, 1, 4) // TTS V2 HTTP + ASR V2 stream
	case "tts":
		return filterByTag(tests, "tts")
	case "asr":
		return filterByTag(tests, "asr")
	case "realtime":
		return filterByTag(tests, "realtime")
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
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

func filterByTag(tests []testCase, tag string) []testCase {
	var out []testCase
	for _, tc := range tests {
		for _, t := range tc.Tags {
			if t == tag {
				out = append(out, tc)
				break
			}
		}
	}
	return out
}

// setupContext configures the CLI context from environment variables.
func setupContext(cli, contextName string) {
	appID := os.Getenv("DOUBAO_APP_ID")
	apiKey := os.Getenv("DOUBAO_API_KEY")

	if appID != "" && apiKey != "" {
		// Auto-create context
		run(cli, "config", "add-context", contextName,
			"--app-id", appID, "--api-key", apiKey)
	}

	// Try to use the context
	run(cli, "config", "use-context", contextName)

	// Verify context exists
	out, err := exec.Command(cli, "config", "list-contexts").CombinedOutput()
	if err != nil || !strings.Contains(string(out), contextName) {
		logError("Context '%s' not found. Set DOUBAO_APP_ID and DOUBAO_API_KEY, or run:", contextName)
		logError("  doubaospeech config add-context %s --app-id YOUR_APP_ID --api-key YOUR_API_KEY", contextName)
		os.Exit(1)
	}

	logInfo("Context ready: %s", contextName)
}

// runTest executes a single test case and reports pass/fail.
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

// run executes a command silently, ignoring errors.
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
	fmt.Println(`Doubao Speech CLI Test Runner

Usage:
  bazel run //examples/cmd/doubaospeech:run -- [test_level]

Test levels:
  1         TTS V2 HTTP streaming (BigModel) - recommended
  2         TTS V2 bidirectional WebSocket
  3         TTS V1 (classic, requires volcano_tts grant)
  4         ASR V2 streaming (uses testdata audio)
  5         ASR V2 file recognition (requires audio URL)
  6         Podcast SAMI (WebSocket)
  7         Meeting transcription
  8         Subtitle extraction
  9         Realtime
  all       All tests (default)
  quick     Quick smoke test (TTS V2 HTTP + ASR V2 stream)
  tts       All TTS tests
  asr       All ASR tests
  realtime  Realtime tests
  help      Show this help

Environment variables:
  DOUBAO_CONTEXT    Context name (default: test)
  DOUBAO_APP_ID     App ID for auto-context setup
  DOUBAO_API_KEY    API Key (Bearer Token from Volcengine console)`)
}
