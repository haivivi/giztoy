package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/haivivi/giztoy/go/pkg/cortex"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

func setupTestEnv(t *testing.T) (*cortex.ConfigStore, func()) {
	t.Helper()
	dir := t.TempDir()
	s, err := cortex.OpenConfigStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("GIZTOY_CONFIG_DIR")
	os.Setenv("GIZTOY_CONFIG_DIR", dir)
	return s, func() {
		if old == "" {
			os.Unsetenv("GIZTOY_CONFIG_DIR")
		} else {
			os.Setenv("GIZTOY_CONFIG_DIR", old)
		}
	}
}

// setupTestEnvWithKV sets up a test env with ctx + shared memory KV.
func setupTestEnvWithKV(t *testing.T) func() {
	t.Helper()
	s, cleanup := setupTestEnv(t)
	s.CtxAdd("test")
	s.CtxUse("test")
	s.CtxConfigSet("kv", "memory://")

	memKV := kv.NewMemory(nil)
	testKVOverride = memKV
	return func() {
		testKVOverride = nil
		cleanup()
	}
}

func runCmd(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	verbose = false
	formatOutput = "table"
	outputFile = ""

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	outBuf.ReadFrom(rOut)
	errBuf.ReadFrom(rErr)

	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		exitCode = 1
		if stderr == "" {
			stderr = err.Error()
		}
	}

	resetFlags(rootCmd)
	return
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// writeTestYAML writes a YAML file to a temp dir and returns its path.
func writeTestYAML(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// ctx tests (same as before)
// ---------------------------------------------------------------------------

func TestCtxAddBasic(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "ctx", "add", "dev")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "created") {
		t.Fatalf("expected 'created' in output, got: %s", stdout)
	}
}

func TestCtxAddDuplicate(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "dev")
	_, stderr, code := runCmd(t, "ctx", "add", "dev")
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("expected 'already exists', got: %s", stderr)
	}
}

func TestCtxListEmpty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "ctx", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "No contexts") {
		t.Fatalf("expected 'No contexts', got: %s", stdout)
	}
}

func TestCtxUseAndCurrent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "dev")
	_, _, code := runCmd(t, "ctx", "use", "dev")
	if code != 0 {
		t.Fatalf("ctx use failed, exit %d", code)
	}

	stdout, _, code := runCmd(t, "ctx", "current")
	if code != 0 {
		t.Fatalf("ctx current failed, exit %d", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Fatalf("expected 'dev', got: %s", stdout)
	}
}

func TestCtxCurrentUnset(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, stderr, code := runCmd(t, "ctx", "current")
	if code == 0 {
		t.Fatal("expected non-zero exit when no context set")
	}
	if !strings.Contains(stderr, "no current context") {
		t.Fatalf("expected 'no current context', got: %s", stderr)
	}
}

func TestCtxRemoveBasic(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "staging")
	_, _, code := runCmd(t, "ctx", "remove", "staging")
	if code != 0 {
		t.Fatalf("ctx remove failed, exit %d", code)
	}
}

func TestCtxRemoveCurrentFails(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "dev")
	runCmd(t, "ctx", "use", "dev")
	_, stderr, code := runCmd(t, "ctx", "remove", "dev")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr, "cannot remove current") {
		t.Fatalf("expected 'cannot remove current', got: %s", stderr)
	}
}

func TestCtxConfigSetAndShow(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "dev")
	runCmd(t, "ctx", "use", "dev")
	runCmd(t, "ctx", "config", "set", "kv", "badger:///data")

	stdout, _, code := runCmd(t, "ctx", "show")
	if code != 0 {
		t.Fatalf("ctx show failed, exit %d", code)
	}
	if !strings.Contains(stdout, "badger:///data") {
		t.Fatalf("expected kv value, got: %s", stdout)
	}
}

func TestCtxConfigSetUnknownKey(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	runCmd(t, "ctx", "add", "dev")
	runCmd(t, "ctx", "use", "dev")
	_, stderr, code := runCmd(t, "ctx", "config", "set", "foo", "bar")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown key")
	}
	if !strings.Contains(stderr, "unknown key") {
		t.Fatalf("expected 'unknown key', got: %s", stderr)
	}
}

func TestCtxConfigList(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "ctx", "config", "list")
	if code != 0 {
		t.Fatalf("ctx config list failed, exit %d", code)
	}
	for _, key := range []string{"kv", "storage", "vecstore", "embed"} {
		if !strings.Contains(stdout, key) {
			t.Fatalf("expected %q in output, got: %s", key, stdout)
		}
	}
}

// ---------------------------------------------------------------------------
// Suppressed import for kv — used in setupTestEnvWithKV
// ---------------------------------------------------------------------------
var _ = kv.NewMemory
