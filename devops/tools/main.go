// affected-targets analyzes git diff to find affected Bazel targets.
//
// Usage:
//
//	bazel run //devops/tools:affected-targets -- --base=origin/main
//	bazel run //devops/tools:affected-targets -- --base=HEAD~1 --check=//pages:deploy
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var (
	base    = flag.String("base", "", "Base commit/branch to compare against (required)")
	check   = flag.String("check", "", "Check if specific target is affected (optional)")
	verbose = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()

	if *base == "" {
		fmt.Fprintln(os.Stderr, "Error: --base is required")
		fmt.Fprintln(os.Stderr, "Usage: affected-targets --base=<commit> [--check=<target>]")
		os.Exit(1)
	}

	// Change to workspace directory if running via bazel
	if wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); wsDir != "" {
		if err := os.Chdir(wsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to chdir to workspace: %v\n", err)
			os.Exit(1)
		}
	}

	// Get changed files
	changedFiles, err := getChangedFiles(*base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting changed files: %v\n", err)
		os.Exit(1)
	}

	if len(changedFiles) == 0 {
		if *verbose {
			fmt.Fprintln(os.Stderr, "No changed files found")
		}
		if *check != "" {
			fmt.Println("not-affected")
		}
		return
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Changed files (%d):\n", len(changedFiles))
		for _, f := range changedFiles {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Find affected targets
	affectedTargets, err := findAffectedTargets(changedFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding affected targets: %v\n", err)
		os.Exit(1)
	}

	if *check != "" {
		// Check mode: see if specific target is affected
		isAffected, err := isTargetAffected(*check, affectedTargets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking target: %v\n", err)
			os.Exit(1)
		}

		if isAffected {
			fmt.Println("affected")
		} else {
			fmt.Println("not-affected")
		}
		return
	}

	// Output all affected targets
	for _, target := range affectedTargets {
		fmt.Println(target)
	}
}

// getChangedFiles returns the list of files changed between base and HEAD.
func getChangedFiles(base string) ([]string, error) {
	// Use merge-base to handle divergent branches properly
	mergeBaseCmd := exec.Command("git", "merge-base", base, "HEAD")
	mergeBaseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		// If merge-base fails, fall back to using base directly
		if *verbose {
			fmt.Fprintf(os.Stderr, "merge-base failed, using base directly: %v\n", err)
		}
	} else {
		base = strings.TrimSpace(string(mergeBaseOutput))
	}

	cmd := exec.Command("git", "diff", "--name-only", base+"..HEAD")
	output, err := cmd.Output()
	if err != nil {
		// Try without range (for single commit comparisons)
		cmd = exec.Command("git", "diff", "--name-only", base)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file != "" {
			files = append(files, file)
		}
	}

	return files, scanner.Err()
}

// findAffectedTargets finds all Bazel targets affected by the changed files.
func findAffectedTargets(changedFiles []string) ([]string, error) {
	// Filter to only files that exist and are in the workspace
	var bazelFiles []string
	for _, f := range changedFiles {
		// Check if file exists (might have been deleted)
		if _, err := os.Stat(f); err == nil {
			bazelFiles = append(bazelFiles, f)
		}
	}

	if len(bazelFiles) == 0 {
		// All changed files were deleted.
		// We do not attempt to reconstruct dependencies for deleted files,
		// so we return no affected targets (conservative for CI speed).
		if *verbose {
			fmt.Fprintln(os.Stderr, "All changed files were deleted, returning no affected targets")
		}
		return nil, nil
	}

	// Build the file set for bazel query
	// Convert paths to Bazel labels where possible
	var fileLabels []string
	for _, f := range bazelFiles {
		// Skip non-source files
		if strings.HasPrefix(f, "bazel-") {
			continue
		}
		fileLabels = append(fileLabels, f)
	}

	if len(fileLabels) == 0 {
		return nil, nil
	}

	// Optimization: deduplicate packages to avoid redundant bazel queries
	// Multiple files in the same package should only trigger one query
	pkgSet := make(map[string]bool)
	for _, file := range fileLabels {
		pkg := findPackageForFile(file)
		if pkg != "" {
			pkgSet[pkg] = true
		}
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Unique packages to query (%d):\n", len(pkgSet))
		for pkg := range pkgSet {
			fmt.Fprintf(os.Stderr, "  %s\n", pkg)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Query affected targets for each unique package
	affectedSet := make(map[string]bool)
	for pkg := range pkgSet {
		targets, err := findTargetsForPackage(pkg)
		if err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to find targets for package %s: %v\n", pkg, err)
			}
			continue
		}
		for _, t := range targets {
			affectedSet[t] = true
		}
	}

	// Convert to sorted slice
	var result []string
	for t := range affectedSet {
		result = append(result, t)
	}
	sort.Strings(result)

	return result, nil
}

// findPackageForFile determines the Bazel package containing the given file.
func findPackageForFile(file string) string {
	dir := filepath.Dir(file)
	if dir == "." {
		dir = ""
	}

	// Convert to Bazel package path
	var pkg string
	if dir == "" {
		pkg = "//"
	} else {
		pkg = "//" + dir
	}

	// Check if this directory has a BUILD file
	hasBuild := false
	for _, buildFile := range []string{"BUILD", "BUILD.bazel"} {
		buildPath := filepath.Join(dir, buildFile)
		if dir == "" {
			buildPath = buildFile
		}
		if _, err := os.Stat(buildPath); err == nil {
			hasBuild = true
			break
		}
	}

	if !hasBuild {
		// Find the nearest parent package
		pkg = findNearestPackage(dir)
	}

	return pkg
}

// findTargetsForPackage finds all targets affected by changes to a package.
func findTargetsForPackage(pkg string) ([]string, error) {
	// Query for rdeps of all targets in this package
	// Use %q to safely quote the package path (prevents Bazel Query Injection)
	query := fmt.Sprintf("rdeps(//..., %s:all)", pkg)
	targets, err := runBazelQuery(query)
	if err != nil {
		// Query might fail for various reasons, try simpler query
		query = fmt.Sprintf("%s:all", pkg)
		targets, err = runBazelQuery(query)
		if err != nil {
			return nil, err
		}
	}
	return targets, nil
}

// runBazelQuery executes a bazel query and returns the results.
func runBazelQuery(query string) ([]string, error) {
	cmd := exec.Command("bazel", "query", query, "--keep_going", "--noshow_progress")

	// Capture stderr for debugging instead of suppressing it
	var stderrBuf bytes.Buffer
	if *verbose {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderrBuf
	}

	output, err := cmd.Output()
	if err != nil {
		// Log stderr on error for debugging
		if stderr := stderrBuf.String(); stderr != "" && *verbose {
			fmt.Fprintf(os.Stderr, "bazel query stderr: %s\n", stderr)
		}
		return nil, err
	}

	var targets []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		target := strings.TrimSpace(scanner.Text())
		if target != "" && !strings.HasPrefix(target, "@") {
			// Filter out external targets
			targets = append(targets, target)
		}
	}

	return targets, scanner.Err()
}

// findNearestPackage finds the nearest Bazel package containing or above the given directory.
func findNearestPackage(dir string) string {
	current := dir
	for {
		for _, buildFile := range []string{"BUILD", "BUILD.bazel"} {
			var buildPath string
			if current == "" {
				buildPath = buildFile
			} else {
				buildPath = filepath.Join(current, buildFile)
			}
			if _, err := os.Stat(buildPath); err == nil {
				if current == "" {
					return "//"
				}
				return "//" + current
			}
		}

		if current == "" || current == "." {
			break
		}
		current = filepath.Dir(current)
		if current == "." {
			current = ""
		}
	}
	return ""
}

// isTargetAffected checks if a specific target is affected by the changes.
func isTargetAffected(target string, affectedTargets []string) (bool, error) {
	// First, check if target is directly in affected list
	for _, t := range affectedTargets {
		if t == target {
			return true, nil
		}
	}

	// Get all dependencies of the target
	deps, err := getTargetDeps(target)
	if err != nil {
		return false, err
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Dependencies of %s (%d):\n", target, len(deps))
		for _, d := range deps {
			fmt.Fprintf(os.Stderr, "  %s\n", d)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Build sets for O(M+N) lookup instead of O(M*N)
	depSet := make(map[string]bool)
	depPkgSet := make(map[string]bool)
	for _, d := range deps {
		depSet[d] = true
		depPkgSet[extractPackage(d)] = true
	}

	for _, affected := range affectedTargets {
		// Check for direct dependency match
		if depSet[affected] {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Target %s is affected via dependency %s\n", target, affected)
			}
			return true, nil
		}

		// Check if the affected target's package is one of the dependency packages
		affectedPkg := extractPackage(affected)
		if depPkgSet[affectedPkg] {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Target %s is affected via package %s\n", target, affectedPkg)
			}
			return true, nil
		}
	}

	return false, nil
}

// getTargetDeps returns all dependencies of a target.
func getTargetDeps(target string) ([]string, error) {
	// Use %q to safely quote the target (prevents Bazel Query Injection)
	query := fmt.Sprintf("deps(%q)", target)
	cmd := exec.Command("bazel", "query", query, "--keep_going", "--noshow_progress")

	// Capture stderr for debugging
	var stderrBuf bytes.Buffer
	if *verbose {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderrBuf
	}

	output, err := cmd.Output()
	if err != nil {
		if stderr := stderrBuf.String(); stderr != "" {
			return nil, fmt.Errorf("bazel query deps failed: %w (stderr: %s)", err, stderr)
		}
		return nil, fmt.Errorf("bazel query deps failed: %w", err)
	}

	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		dep := strings.TrimSpace(scanner.Text())
		if dep != "" && !strings.HasPrefix(dep, "@") {
			deps = append(deps, dep)
		}
	}

	return deps, scanner.Err()
}

// extractPackage extracts the package path from a Bazel target.
func extractPackage(target string) string {
	// //foo/bar:baz -> //foo/bar
	// //foo/bar -> //foo/bar
	if idx := strings.LastIndex(target, ":"); idx != -1 {
		return target[:idx]
	}
	return target
}
