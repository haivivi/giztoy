package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// =============================================================================
// Unit Tests (no external dependencies, always run)
// =============================================================================

// TestExtractPackage tests the package extraction from target labels.
func TestExtractPackage(t *testing.T) {
	tests := []struct {
		target   string
		expected string
	}{
		{"//go/pkg/audio:audio", "//go/pkg/audio"},
		{"//pages:deploy", "//pages"},
		{"//docs:srcs", "//docs"},
		{"//:gazelle", "//"},
		{"//foo/bar", "//foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			result := extractPackage(tt.target)
			if result != tt.expected {
				t.Errorf("extractPackage(%q) = %q, want %q", tt.target, result, tt.expected)
			}
		})
	}
}

// TestFlagParsing tests that command-line flags are properly defined.
func TestFlagParsing(t *testing.T) {
	// Verify flags are defined with correct defaults
	if *base != "" {
		t.Errorf("base flag should default to empty string, got %q", *base)
	}
	if *check != "" {
		t.Errorf("check flag should default to empty string, got %q", *check)
	}
	if *output != "" {
		t.Errorf("output flag should default to empty string, got %q", *output)
	}
	if *oneline != false {
		t.Error("oneline flag should default to false")
	}
	if *verbose != false {
		t.Error("verbose flag should default to false")
	}
}

// =============================================================================
// Integration Tests (require git, may be skipped in some environments)
// =============================================================================

// TestGetChangedFiles_TempRepo tests git diff parsing using a temporary repository.
func TestGetChangedFiles_TempRepo(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v, output: %s", args, err, string(out))
		}
	}

	// Initialize repository and configure user
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")

	// Create initial commit with a single file
	if err := os.WriteFile("file.txt", []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write file.txt: %v", err)
	}
	runGit("add", "file.txt")
	runGit("commit", "-m", "initial commit")

	// Record the base commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get base commit hash: %v", err)
	}
	baseRef := strings.TrimSpace(string(out))

	// Modify existing file and add a new file, then commit the changes
	if err := os.WriteFile("file.txt", []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("failed to modify file.txt: %v", err)
	}
	if err := os.WriteFile("other.txt", []byte("second file\n"), 0o644); err != nil {
		t.Fatalf("failed to write other.txt: %v", err)
	}
	runGit("add", "file.txt", "other.txt")
	runGit("commit", "-m", "second commit with changes")

	// Test getChangedFiles
	files, err := getChangedFiles(baseRef)
	if err != nil {
		t.Fatalf("getChangedFiles(%q) returned error: %v", baseRef, err)
	}

	// We expect both the modified and the newly added file to be reported
	if !slices.Contains(files, "file.txt") {
		t.Errorf("expected file.txt in changed files, got: %v", files)
	}
	if !slices.Contains(files, "other.txt") {
		t.Errorf("expected other.txt in changed files, got: %v", files)
	}
}

// TestFindNearestPackage tests the package finding logic.
func TestFindNearestPackage(t *testing.T) {
	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return // Test was skipped
	}
	defer cleanup()

	tests := []struct {
		dir      string
		expected string
	}{
		{"go/pkg/audio", "//go/pkg/audio"},
		{"docs/en", "//docs"},
		{"pages/home", "//pages"},
		{"", "//"},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			result := findNearestPackage(tt.dir)
			if result != tt.expected {
				t.Errorf("findNearestPackage(%q) = %q, want %q", tt.dir, result, tt.expected)
			}
		})
	}
}

// TestFindPackageForFile tests file to package mapping.
func TestFindPackageForFile(t *testing.T) {
	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return
	}
	defer cleanup()

	tests := []struct {
		file     string
		expected string
	}{
		{"go/pkg/audio/audio.go", "//go/pkg/audio"},
		{"docs/en/SUMMARY.md", "//docs"},
		{"pages/home/index.html", "//pages"},
		{"BUILD.bazel", "//"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := findPackageForFile(tt.file)
			if result != tt.expected {
				t.Errorf("findPackageForFile(%q) = %q, want %q", tt.file, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration Tests (require bazel, may be slow)
// =============================================================================

// TestGetTargetDeps_Integration tests dependency query with real bazel.
func TestGetTargetDeps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return
	}
	defer cleanup()

	if !hasBazel() {
		t.Skip("bazel not available")
	}

	deps, err := getTargetDeps("//pages:deploy")
	if err != nil {
		t.Fatalf("getTargetDeps failed: %v", err)
	}

	// //pages:deploy should depend on //pages:www and //docs:srcs
	expectedDeps := []string{"//pages:www", "//docs:srcs"}
	for _, expected := range expectedDeps {
		if !slices.Contains(deps, expected) {
			t.Errorf("expected %s in deps of //pages:deploy, got: %v", expected, deps)
		}
	}
}

// TestFindTargetsForPackage_Integration tests package query with real bazel.
func TestFindTargetsForPackage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return
	}
	defer cleanup()

	if !hasBazel() {
		t.Skip("bazel not available")
	}

	targets, err := findTargetsForPackage("//docs")
	if err != nil {
		t.Fatalf("findTargetsForPackage failed: %v", err)
	}

	// Should find some targets
	if len(targets) == 0 {
		t.Error("expected to find targets for //docs package")
	}
}

// =============================================================================
// Helper functions
// =============================================================================

// setupWorkspaceDir changes to the Bazel workspace directory and returns a cleanup function.
func setupWorkspaceDir(t *testing.T) (string, func()) {
	t.Helper()

	wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if wsDir == "" {
		// Try to find workspace root from bazel
		cmd := exec.Command("bazel", "info", "workspace")
		output, err := cmd.Output()
		if err != nil {
			t.Skip("cannot determine workspace directory")
			return "", func() {}
		}
		wsDir = strings.TrimSpace(string(output))
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(wsDir); err != nil {
		t.Fatalf("failed to change to workspace: %v", err)
	}

	return wsDir, func() {
		_ = os.Chdir(origDir)
	}
}

// hasBazel checks if bazel is available.
func hasBazel() bool {
	_, err := exec.Command("bazel", "version").Output()
	return err == nil
}

// TestFindTargetsForFile_Integration tests file to target mapping with real bazel.
func TestFindTargetsForFile_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return
	}
	defer cleanup()

	if !hasBazel() {
		t.Skip("bazel not available")
	}

	// Test with a known file
	buildFile := filepath.Join("docs", "BUILD.bazel")
	if _, err := os.Stat(buildFile); err != nil {
		t.Skip("docs/BUILD.bazel not found")
	}

	pkg := findPackageForFile(buildFile)
	if pkg == "" {
		t.Fatal("findPackageForFile returned empty package")
	}

	targets, err := findTargetsForPackage(pkg)
	if err != nil {
		t.Fatalf("findTargetsForPackage failed: %v", err)
	}

	// Should find some targets
	if len(targets) == 0 {
		t.Error("expected to find targets for docs/BUILD.bazel")
	}
}

// =============================================================================
// Golden Tests (compare output against known-good baseline)
// =============================================================================

// TestAffectedTargets_Golden compares affected-targets output against a golden file.
// This ensures the tool produces consistent results for historical commits.
func TestAffectedTargets_Golden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping golden test in short mode")
	}

	wsDir, cleanup := setupWorkspaceDir(t)
	if wsDir == "" {
		return
	}
	defer cleanup()

	if !hasBazel() {
		t.Skip("bazel not available")
	}

	// Test case: commit b320379 (rust/minimax: remove examples)
	// Base: b320379~1, Head: b320379
	const baseCommit = "b320379~1"
	const headCommit = "b320379"
	goldenFile := filepath.Join(wsDir, "devops/tools/testdata/b320379.golden")

	// Verify the commit exists
	cmd := exec.Command("git", "rev-parse", headCommit)
	if err := cmd.Run(); err != nil {
		t.Skipf("commit %s not found in history", headCommit)
	}

	// Read expected output from golden file
	expected, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenFile, err)
	}

	// Get changed files between commits
	changedFiles, err := getChangedFiles(baseCommit, headCommit)
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}

	if len(changedFiles) == 0 {
		t.Fatal("expected changed files between commits")
	}

	// Find affected targets
	affectedTargets, err := findAffectedTargets(changedFiles)
	if err != nil {
		t.Fatalf("findAffectedTargets failed: %v", err)
	}

	// Sort for consistent comparison
	slices.Sort(affectedTargets)
	actual := strings.Join(affectedTargets, "\n") + "\n"

	// Compare
	if actual != string(expected) {
		t.Errorf("affected targets mismatch:\n--- expected (golden file) ---\n%s\n--- actual ---\n%s",
			string(expected), actual)
	}
}
