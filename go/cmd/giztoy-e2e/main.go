// Command giztoy-e2e runs end-to-end tests for giztoy CLI v3.
//
// It creates a temporary Cortex environment, injects credentials from
// environment variables via Apply, applies genx configs from testdata,
// and runs all testdata tasks through Cortex.Run().
//
// Usage:
//
//	go run ./cmd/giztoy-e2e [filter]
//
// Filters: all, openai, minimax, doubaospeech, dashscope, genai, memory
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/cortex"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorReset  = "\033[0m"
)

type testResult struct {
	File    string
	Kind    string
	Status  string // "pass", "fail", "skip"
	Elapsed time.Duration
	Error   string
}

func main() {
	filter := "all"
	if len(os.Args) > 1 {
		filter = os.Args[1]
	}

	fmt.Println()
	fmt.Println("======================================")
	fmt.Println("   giztoy E2E Test Runner")
	fmt.Println("======================================")
	fmt.Println()

	root := projectRoot()
	testdataDir := filepath.Join(root, "testdata", "cmd")
	logInfo("Project root: %s", root)
	logInfo("Testdata: %s", testdataDir)
	logInfo("Filter: %s", filter)

	tmpDir, err := os.MkdirTemp("", "giztoy-e2e-*")
	if err != nil {
		fatal("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := cortex.OpenConfigStoreAt(filepath.Join(tmpDir, "config"))
	if err != nil {
		fatal("open config store: %v", err)
	}
	if err := store.CtxAdd("e2e"); err != nil {
		fatal("ctx add: %v", err)
	}
	if err := store.CtxUse("e2e"); err != nil {
		fatal("ctx use: %v", err)
	}
	if err := store.CtxConfigSet("kv", "badger://"+filepath.Join(tmpDir, "kv")); err != nil {
		fatal("ctx config set: %v", err)
	}

	ctx := context.Background()
	c, err := cortex.New(ctx, store)
	if err != nil {
		fatal("create cortex: %v", err)
	}
	defer c.Close()

	available := applyCredsFromEnv(ctx, c)
	logInfo("Creds: %s", formatAvailable(available))

	// Verify creds are readable (keys masked)
	for service := range available {
		docs, _ := c.List(ctx, "creds:"+service+":*", cortex.ListOpts{All: true})
		for _, d := range docs {
			logInfo("  creds:%s:%s ✓", service, d.Name())
		}
	}

	applyCount := applyTestdataConfigs(ctx, c, filepath.Join(testdataDir, "apply"))
	logInfo("Applied %d genx configs", applyCount)

	var results []testResult
	results = append(results, runApplyErrors(ctx, c, filepath.Join(testdataDir, "apply"))...)
	results = append(results, runTestdata(ctx, c, filepath.Join(testdataDir, "run"), filter, available, tmpDir, root)...)

	exitCode := printReport(results)

	// Close resources before exit (defer won't run after os.Exit)
	c.Close()
	os.RemoveAll(tmpDir)
	os.Exit(exitCode)
}

func projectRoot() string {
	if dir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); dir != "" {
		return dir
	}
	// Walk up from cwd looking for testdata/cmd/
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "testdata", "cmd")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}

func applyCredsFromEnv(ctx context.Context, c *cortex.Cortex) map[string]bool {
	available := map[string]bool{}

	type credDef struct {
		envKey  string
		service string
		doc     cortex.Document
	}

	defs := []credDef{
		{envKey: "QWEN_API_KEY", service: "openai", doc: cortex.Document{Kind: "creds/openai", Fields: map[string]any{
			"name": "qwen", "api_key": os.Getenv("QWEN_API_KEY"),
			"base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
		}}},
		{envKey: "DEEPSEEK_API_KEY", service: "openai", doc: cortex.Document{Kind: "creds/openai", Fields: map[string]any{
			"name": "deepseek", "api_key": os.Getenv("DEEPSEEK_API_KEY"),
			"base_url": "https://api.deepseek.com/v1",
		}}},
		{envKey: "MINIMAX_API_KEY", service: "minimax", doc: cortex.Document{Kind: "creds/minimax", Fields: map[string]any{
			"name": "cn", "api_key": os.Getenv("MINIMAX_API_KEY"),
		}}},
		{envKey: "DOUBAO_APP_ID", service: "doubaospeech", doc: cortex.Document{Kind: "creds/doubaospeech", Fields: map[string]any{
			"name": "test", "app_id": os.Getenv("DOUBAO_APP_ID"),
			"token": os.Getenv("DOUBAO_TOKEN"), "api_key": os.Getenv("DOUBAO_API_KEY"),
		}}},
		{envKey: "QWEN_API_KEY", service: "dashscope", doc: cortex.Document{Kind: "creds/dashscope", Fields: map[string]any{
			"name": "default", "api_key": os.Getenv("QWEN_API_KEY"),
		}}},
		{envKey: "GEMINI_API_KEY", service: "genai", doc: cortex.Document{Kind: "creds/genai", Fields: map[string]any{
			"name": "default", "api_key": os.Getenv("GEMINI_API_KEY"),
		}}},
	}

	for _, d := range defs {
		if os.Getenv(d.envKey) == "" {
			continue
		}
		if _, err := c.Apply(ctx, []cortex.Document{d.doc}); err != nil {
			logWarn("apply cred %s: %v", d.service, err)
			continue
		}
		available[d.service] = true
	}
	return available
}

func formatAvailable(available map[string]bool) string {
	services := []string{"openai", "minimax", "doubaospeech", "dashscope", "genai"}
	var parts []string
	for _, s := range services {
		if available[s] {
			parts = append(parts, colorGreen+s+"✓"+colorReset)
		} else {
			parts = append(parts, colorYellow+s+"✗"+colorReset)
		}
	}
	return strings.Join(parts, " ")
}

func applyTestdataConfigs(ctx context.Context, c *cortex.Cortex, dir string) int {
	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || strings.HasPrefix(e.Name(), "error-") {
			continue
		}
		// Skip batch-all.yaml (contains cred placeholders like ${MINIMAX_API_KEY})
		if e.Name() == "batch-all.yaml" {
			continue
		}
		docs, err := cortex.ParseDocumentsFromFile(filepath.Join(dir, e.Name()))
		if err != nil {
			logWarn("parse %s: %v", e.Name(), err)
			continue
		}
		// Skip docs with creds kind — creds are injected from env vars, not testdata
		var filtered []cortex.Document
		for _, d := range docs {
			if strings.HasPrefix(d.Kind, "creds/") {
				continue
			}
			filtered = append(filtered, d)
		}
		if len(filtered) == 0 {
			continue
		}
		results, err := c.Apply(ctx, filtered)
		if err != nil {
			logWarn("apply %s: %v", e.Name(), err)
			continue
		}
		count += len(results)
	}
	return count
}

func runApplyErrors(ctx context.Context, c *cortex.Cortex, dir string) []testResult {
	var results []testResult
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "error-") || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		start := time.Now()
		docs, parseErr := cortex.ParseDocumentsFromFile(filepath.Join(dir, e.Name()))
		if parseErr != nil {
			r := testResult{File: "apply/" + e.Name(), Kind: "apply-error", Status: "pass", Elapsed: time.Since(start)}
			results = append(results, r)
			printResult(r)
			continue
		}
		_, applyErr := c.Apply(ctx, docs)
		elapsed := time.Since(start)
		var r testResult
		if applyErr != nil {
			r = testResult{File: "apply/" + e.Name(), Kind: "apply-error", Status: "pass", Elapsed: elapsed}
		} else {
			r = testResult{File: "apply/" + e.Name(), Kind: "apply-error", Status: "fail", Elapsed: elapsed, Error: "expected error"}
		}
		results = append(results, r)
		printResult(r)
	}
	return results
}

func runTestdata(ctx context.Context, c *cortex.Cortex, dir, filter string, available map[string]bool, tmpDir string, root string) []testResult {
	var results []testResult

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		rel, _ := filepath.Rel(dir, path)
		parts := strings.SplitN(rel, string(os.PathSeparator), 2)
		service := parts[0]

		if filter != "all" && service != filter {
			return nil
		}

		isErrorCase := strings.HasPrefix(d.Name(), "error-")

		docs, err := cortex.ParseDocumentsFromFile(path)
		if err != nil {
			if isErrorCase {
				results = append(results, testResult{File: "run/" + rel, Kind: "parse-error", Status: "pass", Elapsed: 0})
			} else {
				results = append(results, testResult{File: "run/" + rel, Status: "fail", Error: "parse: " + err.Error()})
			}
			return nil
		}
		if len(docs) != 1 {
			results = append(results, testResult{File: "run/" + rel, Status: "fail", Error: "expected 1 document"})
			return nil
		}

		task := docs[0]

		credRef := task.GetString("cred")
		if credRef != "" {
			credService := strings.SplitN(credRef, ":", 2)[0]
			if !available[credService] {
				results = append(results, testResult{File: "run/" + rel, Kind: task.Kind, Status: "skip"})
				return nil
			}
		}

		if output := task.GetString("output"); output != "" {
			tmpOutput := filepath.Join(tmpDir, "output", rel+filepath.Ext(output))
			os.MkdirAll(filepath.Dir(tmpOutput), 0755)
			task.Fields["output"] = tmpOutput
		}

		// Resolve relative audio/file paths to project root
		for _, pathField := range []string{"audio", "file_path"} {
			if p := task.GetString(pathField); p != "" && !filepath.IsAbs(p) {
				task.Fields[pathField] = filepath.Join(root, p)
			}
		}

		start := time.Now()
		_, runErr := c.Run(ctx, task)
		elapsed := time.Since(start)

		var r testResult
		if isErrorCase {
			if runErr != nil {
				r = testResult{File: "run/" + rel, Kind: task.Kind, Status: "pass", Elapsed: elapsed}
			} else {
				r = testResult{File: "run/" + rel, Kind: task.Kind, Status: "fail", Elapsed: elapsed, Error: "expected error"}
			}
		} else {
			if runErr != nil {
				r = testResult{File: "run/" + rel, Kind: task.Kind, Status: "fail", Elapsed: elapsed, Error: runErr.Error()}
			} else {
				r = testResult{File: "run/" + rel, Kind: task.Kind, Status: "pass", Elapsed: elapsed}
			}
		}
		results = append(results, r)
		printResult(r)
		return nil
	})
	return results
}

func printResult(r testResult) {
	switch r.Status {
	case "pass":
		fmt.Printf("%s[PASS]%s %-50s %-30s %s\n", colorGreen, colorReset, r.File, r.Kind, r.Elapsed)
	case "fail":
		fmt.Printf("%s[FAIL]%s %-50s %-30s %s\n", colorRed, colorReset, r.File, r.Kind, r.Error)
	case "skip":
		fmt.Printf("%s[SKIP]%s %-50s %s\n", colorYellow, colorReset, r.File, "(no creds)")
	}
	flush()
}

func printReport(results []testResult) int {
	fmt.Println()
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skip":
			skipped++
		}
	}

	fmt.Println()
	fmt.Println("======================================")
	fmt.Printf("   Results: %s%d passed%s", colorGreen, passed, colorReset)
	if failed > 0 {
		fmt.Printf(", %s%d failed%s", colorRed, failed, colorReset)
	}
	if skipped > 0 {
		fmt.Printf(", %s%d skipped%s", colorYellow, skipped, colorReset)
	}
	fmt.Println()
	fmt.Println("======================================")
	fmt.Println()

	if passed+failed+skipped == 0 {
		fmt.Fprintf(os.Stderr, "%s[ERROR]%s No tests were executed. Check testdata path and filter.\n", colorRed, colorReset)
		return 1
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func flush() { os.Stdout.Sync() }
func logInfo(format string, args ...any) {
	fmt.Printf("%s[INFO]%s %s\n", colorBlue, colorReset, fmt.Sprintf(format, args...))
	flush()
}
func logWarn(format string, args ...any) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, fmt.Sprintf(format, args...))
	flush()
}
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s[FATAL]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
	os.Exit(1)
}
