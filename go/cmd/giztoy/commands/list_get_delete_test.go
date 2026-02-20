package commands

import (
	"strings"
	"testing"
)

func applyTestCreds(t *testing.T) {
	t.Helper()
	path := writeTestYAML(t, "setup.yaml", `---
kind: creds/openai
name: qwen
api_key: sk-q
---
kind: creds/openai
name: deepseek
api_key: sk-d
base_url: https://api.deepseek.com/v1
---
kind: creds/minimax
name: cn
api_key: eyJ
base_url: https://api.minimaxi.com
---
kind: genx/generator
name: qwen/turbo
cred: openai:qwen
model: qwen-turbo-latest
---
kind: genx/tts
name: minimax/shaonv
cred: minimax:cn
voice_id: female-shaonv
`)
	_, _, code := runCmd(t, "apply", "-f", path)
	if code != 0 {
		t.Fatal("setup apply failed")
	}
}

// ---------------------------------------------------------------------------
// list tests
// ---------------------------------------------------------------------------

func TestListCredsAll(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "creds:*", "--all")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "qwen") || !strings.Contains(stdout, "deepseek") || !strings.Contains(stdout, "cn") {
		t.Fatalf("expected all creds, got: %s", stdout)
	}
}

func TestListCredsPrefix(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "creds:openai:*", "--all")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "qwen") {
		t.Fatalf("expected qwen, got: %s", stdout)
	}
	if strings.Contains(stdout, "minimax") {
		t.Fatalf("should not contain minimax: %s", stdout)
	}
}

func TestListGenxAll(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "genx:*", "--all")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "qwen/turbo") || !strings.Contains(stdout, "minimax/shaonv") {
		t.Fatalf("expected genx entries, got: %s", stdout)
	}
}

func TestListGenxGeneratorOnly(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "genx:generator:*", "--all")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "qwen/turbo") {
		t.Fatalf("expected qwen/turbo, got: %s", stdout)
	}
	if strings.Contains(stdout, "shaonv") {
		t.Fatalf("should not contain tts: %s", stdout)
	}
}

func TestListLimit(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "creds:*", "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Count(stdout, "\n") > 4 {
		t.Fatalf("expected limited output, got: %s", stdout)
	}
}

func TestListEmpty(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "list", "creds:*")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "No resources") {
		t.Fatalf("expected 'No resources', got: %s", stdout)
	}
}

func TestListJSON(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "creds:openai:*", "--all", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"kind"`) {
		t.Fatalf("expected JSON, got: %s", stdout)
	}
}

func TestListName(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "list", "creds:openai:*", "--all", "--format", "name")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "creds:openai:qwen") {
		t.Fatalf("expected name format, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// get tests
// ---------------------------------------------------------------------------

func TestGetCreds(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "get", "creds:openai:qwen")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "sk-q") {
		t.Fatalf("expected api_key, got: %s", stdout)
	}
}

func TestGetGenx(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "get", "genx:generator:qwen/turbo")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "qwen-turbo-latest") {
		t.Fatalf("expected model, got: %s", stdout)
	}
}

func TestGetNotFound(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, stderr, code := runCmd(t, "get", "creds:openai:nonexistent")
	if code == 0 {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(stderr, "not found") {
		t.Fatalf("expected 'not found', got: %s", stderr)
	}
}

func TestGetJSON(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "get", "creds:openai:qwen", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"api_key"`) {
		t.Fatalf("expected JSON, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// delete tests
// ---------------------------------------------------------------------------

func TestDeleteCreds(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "delete", "creds:openai:deepseek")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Deleted") {
		t.Fatalf("expected 'Deleted', got: %s", stdout)
	}

	_, _, code = runCmd(t, "get", "creds:openai:deepseek")
	if code == 0 {
		t.Fatal("expected not found after delete")
	}
}

func TestDeleteGenx(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "delete", "genx:generator:qwen/turbo")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Deleted") {
		t.Fatalf("expected 'Deleted', got: %s", stdout)
	}
}

func TestDeleteNotFound(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, stderr, code := runCmd(t, "delete", "creds:openai:nonexistent")
	if code == 0 {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(stderr, "not found") {
		t.Fatalf("expected 'not found', got: %s", stderr)
	}
}

func TestDeleteJSON(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()
	applyTestCreds(t)

	stdout, _, code := runCmd(t, "delete", "creds:openai:deepseek", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"deleted"`) {
		t.Fatalf("expected JSON with deleted, got: %s", stdout)
	}
}
