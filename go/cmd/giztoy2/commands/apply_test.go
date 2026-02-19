package commands

import (
	"strings"
	"testing"
)

func TestApplyCredsOpenAI(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "creds.yaml", `kind: creds/openai
name: qwen
api_key: sk-test
base_url: https://dashscope.aliyuncs.com/compatible-mode/v1
`)
	stdout, _, code := runCmd(t, "apply", "-f", path)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "created") {
		t.Fatalf("expected 'created', got: %s", stdout)
	}
}

func TestApplyBatch(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "batch.yaml", `---
kind: creds/openai
name: qwen
api_key: sk-q
---
kind: creds/minimax
name: cn
api_key: eyJ
---
kind: genx/generator
name: qwen/turbo
cred: openai:qwen
model: qwen-turbo-latest
`)
	stdout, _, code := runCmd(t, "apply", "-f", path)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Count(stdout, "created") != 3 {
		t.Fatalf("expected 3 'created', got: %s", stdout)
	}
}

func TestApplyUpdate(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "cred.yaml", `kind: creds/openai
name: qwen
api_key: old-key
`)
	runCmd(t, "apply", "-f", path)

	path2 := writeTestYAML(t, "cred2.yaml", `kind: creds/openai
name: qwen
api_key: new-key
`)
	stdout, _, code := runCmd(t, "apply", "-f", path2)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "updated") {
		t.Fatalf("expected 'updated', got: %s", stdout)
	}
}

func TestApplyJSON(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "cred.yaml", `kind: creds/openai
name: qwen
api_key: sk-test
`)
	stdout, _, code := runCmd(t, "apply", "-f", path, "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"status"`) {
		t.Fatalf("expected JSON output, got: %s", stdout)
	}
}

func TestApplyMissingFile(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, _, code := runCmd(t, "apply", "-f", "/nonexistent.yaml")
	if code == 0 {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestApplyMissingFlag(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	_, _, code := runCmd(t, "apply")
	if code == 0 {
		t.Fatal("expected error when -f not provided")
	}
}

func TestApplyUnknownKind(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `kind: foo/bar
name: x
`)
	_, stderr, code := runCmd(t, "apply", "-f", path)
	if code == 0 {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(stderr, "unknown kind") {
		t.Fatalf("expected 'unknown kind', got: %s", stderr)
	}
}

func TestApplyMissingRequiredField(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `kind: creds/openai
name: qwen
`)
	_, stderr, code := runCmd(t, "apply", "-f", path)
	if code == 0 {
		t.Fatal("expected error for missing api_key")
	}
	if !strings.Contains(stderr, "api_key") {
		t.Fatalf("expected api_key error, got: %s", stderr)
	}
}

func TestApplyMissingKind(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `name: qwen
api_key: sk-test
`)
	_, stderr, code := runCmd(t, "apply", "-f", path)
	if code == 0 {
		t.Fatal("expected error for missing kind")
	}
	if !strings.Contains(stderr, "kind") {
		t.Fatalf("expected 'kind' error, got: %s", stderr)
	}
}

func TestApplyBadCredFormat(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `kind: genx/generator
name: test
cred: nocolon
model: m
`)
	_, stderr, code := runCmd(t, "apply", "-f", path)
	if code == 0 {
		t.Fatal("expected error for bad cred format")
	}
	if !strings.Contains(stderr, "service:name") {
		t.Fatalf("expected cred format error, got: %s", stderr)
	}
}

func TestApplyBadTemperature(t *testing.T) {
	cleanup := setupTestEnvWithKV(t)
	defer cleanup()

	path := writeTestYAML(t, "bad.yaml", `kind: genx/generator
name: test
cred: openai:qwen
model: m
temperature: 5.0
`)
	_, _, code := runCmd(t, "apply", "-f", path)
	if code == 0 {
		t.Fatal("expected error for temperature=5")
	}
}
