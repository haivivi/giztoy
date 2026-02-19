// Command install copies the giztoy binary to ~/go/bin/.
//
// This is a standalone program used by the Bazel :install target.
// It finds the giztoy binary built by Bazel (via runfiles) and copies
// it to the user's Go bin directory.
//
// Usage:
//
//	bazel run //go/cmd/giztoy:install
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	src := findGiztoyBinary()
	if src == "" {
		fmt.Fprintln(os.Stderr, "Error: cannot find giztoy binary")
		os.Exit(1)
	}

	dest := filepath.Join(installDir(), "giztoy")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}

	os.MkdirAll(filepath.Dir(dest), 0755)
	if err := copyFile(src, dest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Chmod(dest, 0755)
	fmt.Printf("Installed: %s\n", dest)

	if path, err := exec.LookPath("giztoy"); err == nil {
		fmt.Printf("Available: %s\n", path)
	} else {
		fmt.Printf("Note: %s is not in PATH\n", filepath.Dir(dest))
	}
}

func findGiztoyBinary() string {
	// Bazel runfiles: the binary is a sibling in the same directory
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(self)

	// Try common Bazel output paths
	candidates := []string{
		filepath.Join(dir, "giztoy"),
		filepath.Join(dir, "giztoy_", "giztoy"),
	}

	// Also check BUILD_WORKSPACE_DIRECTORY (bazel run sets this)
	if ws := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); ws != "" {
		candidates = append(candidates,
			filepath.Join(ws, "bazel-bin", "go", "cmd", "giztoy", "giztoy_", "giztoy"),
		)
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func installDir() string {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return gobin
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go", "bin")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}
