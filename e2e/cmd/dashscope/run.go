// Command run is a cross-platform test executor for the DashScope CLI.
//
// It replaces the previous run.sh shell script.
//
// Usage:
//
//	bazel run //e2e/cmd/dashscope:run -- [runtime] [test_level]
//
//	runtime: go | rust | both (default: go)
//	test_level: 1, all, quick (default: quick)
//
// Environment variables:
//
//	DASHSCOPE_CONTEXT  - Context name (default: dashscope_cn)
//	DASHSCOPE_API_KEY  - API Key for auto-context setup
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
	colorBlue   = "\033[0;34m"
	colorReset  = "\033[0m"
)

type testCase struct {
	Name  string
	Level int
	Args  []string
}

func projectRoot() string {
	if dir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); dir != "" {
		return dir
	}
	dir, _ := os.Getwd()
	return dir
}

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

func findRustCLI(root string) string {
	bazelBin := filepath.Join(root, "bazel-bin", "rust", "cmd", "dashscope", "dashscope")
	if _, err := os.Stat(bazelBin); err == nil {
		return bazelBin
	}
	if p, err := exec.LookPath("dashscope"); err == nil {
		return p
	}
	return ""
}

func main() {
	runtime := "go"
	testLevel := "quick"
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
	commandsDir := filepath.Join(root, "e2e", "cmd", "dashscope", "commands")
	outputDir := filepath.Join(root, "e2e", "cmd", "dashscope", "output")
	contextName := envOr("DASHSCOPE_CONTEXT", "dashscope_cn")

	os.MkdirAll(outputDir, 0755)

	fmt.Println()
	fmt.Println("======================================")
	fmt.Println("   DashScope API Test Runner")
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

	tests := buildTestCases(commandsDir, outputDir, "go")
	selected := filterTests(tests, testLevel)
	if len(selected) == 0 {
		logError("Unknown test level: %s", testLevel)
		showHelp()
		os.Exit(1)
	}

	setupContext(cli, contextName, "dashscope")
	execTests(cli, selected, contextName, "dashscope")
}

func runWithRust(root, commandsDir, outputDir, contextName, testLevel string) {
	cli := findRustCLI(root)
	if cli == "" {
		logError("Cannot find Rust dashscope binary. Run: bazel build //rust/cmd/dashscope")
		os.Exit(1)
	}
	logInfo("CLI binary: %s", cli)

	tests := buildTestCases(commandsDir, outputDir, "rust")
	selected := filterTests(tests, testLevel)
	if len(selected) == 0 {
		logError("Unknown test level: %s", testLevel)
		showHelp()
		os.Exit(1)
	}

	setupContext(cli, contextName, "")
	execTests(cli, selected, contextName, "")
}

func buildTestCases(commandsDir, outputDir, runtime string) []testCase {
	cmd := func(name string) string { return filepath.Join(commandsDir, name) }
	out := func(name string) string { return filepath.Join(outputDir, name) }

	return []testCase{
		// Level 1: Omni Chat (realtime audio)
		{Name: "Omni Chat", Level: 1,
			Args: []string{"omni", "chat", "-f", cmd("omni-chat.yaml"), "-o", out("omni_output_" + runtime + ".pcm")}},
	}
}

func filterTests(tests []testCase, level string) []testCase {
	switch level {
	case "all":
		return tests
	case "quick":
		// Quick: just check CLI help works
		return []testCase{{Name: "CLI Help Check", Level: 0, Args: []string{"--help"}}}
	case "1":
		return filterByLevels(tests, 1)
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

func setupContext(cli, contextName, subcmd string) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey != "" {
		if subcmd != "" {
			run(cli, subcmd, "config", "add-context", contextName, "--api-key", apiKey)
		} else {
			run(cli, "config", "add-context", contextName, "--api-key", apiKey)
		}
	}

	// Always select the context (may be pre-configured without API key env var)
	if subcmd != "" {
		run(cli, subcmd, "config", "use-context", contextName)
	} else {
		run(cli, "config", "use-context", contextName)
	}
	logInfo("Context ready: %s", contextName)
}

func execTests(cli string, tests []testCase, contextName, subcmd string) {
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
	fmt.Println(`DashScope CLI Test Runner

Usage:
  bazel run //e2e/cmd/dashscope:run -- [runtime] [test_level]

Runtime:
  go      Use Go CLI via giztoy (default)
  rust    Use Rust CLI
  both    Test both Go and Rust

Test levels:
  1         Omni Chat (realtime audio)
  all       All tests
  quick     Quick smoke test (CLI help check, default)
  help      Show this help

Environment variables:
  DASHSCOPE_CONTEXT    Context name (default: dashscope_cn)
  DASHSCOPE_API_KEY    API Key for auto-context setup`)
}
