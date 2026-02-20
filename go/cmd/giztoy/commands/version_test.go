package commands

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "version")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "giztoy") {
		t.Fatalf("expected 'giztoy', got: %s", stdout)
	}
}

func TestVersionJSON(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "version", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"version"`) {
		t.Fatalf("expected JSON, got: %s", stdout)
	}
}
