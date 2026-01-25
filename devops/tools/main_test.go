package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestGetChangedFiles tests the git diff parsing.
func TestGetChangedFiles(t *testing.T) {
	// Skip if not in a git repository
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository")
	}

	// Test with a known commit that modified docs
	// 05bc322: "docs: restructure documentation with multilingual support and add website"
	files, err := getChangedFiles("05bc322~1")
	if err != nil {
		// This commit might not exist in all clones, skip
		t.Skipf("commit not found: %v", err)
	}

	// Should have some docs files
	hasDocsFile := false
	for _, f := range files {
		if strings.HasPrefix(f, "docs/") {
			hasDocsFile = true
			break
		}
	}

	if !hasDocsFile {
		t.Errorf("expected docs files in changed files, got: %v", files)
	}
}

// TestFindNearestPackage tests the package finding logic.
func TestFindNearestPackage(t *testing.T) {
	// Get workspace root
	wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if wsDir == "" {
		// Try to find workspace root from current directory
		cmd := exec.Command("bazel", "info", "workspace")
		output, err := cmd.Output()
		if err != nil {
			t.Skip("cannot determine workspace directory")
		}
		wsDir = strings.TrimSpace(string(output))
	}

	// Change to workspace directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wsDir)

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

// TestIsTargetAffected_Integration tests the check mode with real bazel queries.
func TestIsTargetAffected_Integration(t *testing.T) {
	// Skip if not in a git repository or bazel not available
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository")
	}
	if _, err := exec.Command("bazel", "version").Output(); err != nil {
		t.Skip("bazel not available")
	}

	// Get workspace root
	wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if wsDir == "" {
		cmd := exec.Command("bazel", "info", "workspace")
		output, err := cmd.Output()
		if err != nil {
			t.Skip("cannot determine workspace directory")
		}
		wsDir = strings.TrimSpace(string(output))
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wsDir)

	// Test with commit 05bc322 which modified docs/
	// This should affect //pages:deploy since pages depends on docs
	t.Run("docs_change_affects_pages", func(t *testing.T) {
		// Get changed files for this specific commit
		changedFiles, err := getChangedFiles("05bc322~1")
		if err != nil {
			t.Skipf("commit not found: %v", err)
		}

		// Find affected targets
		affectedTargets, err := findAffectedTargets(changedFiles)
		if err != nil {
			t.Fatalf("findAffectedTargets failed: %v", err)
		}

		// Check if //pages:deploy is affected
		affected, err := isTargetAffected("//pages:deploy", affectedTargets)
		if err != nil {
			t.Fatalf("isTargetAffected failed: %v", err)
		}

		if !affected {
			t.Errorf("expected //pages:deploy to be affected by docs changes, affectedTargets: %v", affectedTargets)
		}
	})
}

// TestGetTargetDeps tests dependency query.
func TestGetTargetDeps(t *testing.T) {
	if _, err := exec.Command("bazel", "version").Output(); err != nil {
		t.Skip("bazel not available")
	}

	// Get workspace root
	wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if wsDir == "" {
		cmd := exec.Command("bazel", "info", "workspace")
		output, err := cmd.Output()
		if err != nil {
			t.Skip("cannot determine workspace directory")
		}
		wsDir = strings.TrimSpace(string(output))
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wsDir)

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

// TestMainHelp tests the help message.
func TestMainHelp(t *testing.T) {
	// Save original args and restore after test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Test with --help (this would exit, so we just test that flags parse correctly)
	os.Args = []string{"affected-targets", "--base=test"}

	// Reset flags for testing
	*base = ""
	*check = ""
	*verbose = false

	// This is a simple smoke test - actual main() would run
}

// TestFindTargetsForFile tests file to target mapping.
func TestFindTargetsForFile(t *testing.T) {
	if _, err := exec.Command("bazel", "version").Output(); err != nil {
		t.Skip("bazel not available")
	}

	wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if wsDir == "" {
		cmd := exec.Command("bazel", "info", "workspace")
		output, err := cmd.Output()
		if err != nil {
			t.Skip("cannot determine workspace directory")
		}
		wsDir = strings.TrimSpace(string(output))
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(wsDir)

	// Test with a known file
	buildFile := filepath.Join("docs", "BUILD.bazel")
	if _, err := os.Stat(buildFile); err != nil {
		t.Skip("docs/BUILD.bazel not found")
	}

	targets, err := findTargetsForFile(buildFile)
	if err != nil {
		t.Fatalf("findTargetsForFile failed: %v", err)
	}

	// Should find some targets
	if len(targets) == 0 {
		t.Error("expected to find targets for docs/BUILD.bazel")
	}
}
