package commands

import (
	"strings"
	"testing"
)

func TestRunMissingFlag(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, _, code := runCmd(t, "run")
	if code == 0 {
		t.Fatal("expected error when -f not provided")
	}
}

func TestRunMissingFile(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, _, code := runCmd(t, "run", "-f", "/nonexistent.yaml")
	if code == 0 {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestRunUnknownKind(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `kind: unknown/thing
name: test
`)
	_, stderr, code := runCmd(t, "run", "-f", path)
	if code == 0 {
		t.Fatal("expected error for unknown run kind")
	}
	if !strings.Contains(stderr, "unknown run kind") {
		t.Fatalf("expected 'unknown run kind', got: %s", stderr)
	}
}

func TestRunMissingKind(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `name: test
messages:
  - role: user
    content: hello
`)
	_, stderr, code := runCmd(t, "run", "-f", path)
	if code == 0 {
		t.Fatal("expected error for missing kind")
	}
	if !strings.Contains(stderr, "kind") {
		t.Fatalf("expected 'kind' error, got: %s", stderr)
	}
}
