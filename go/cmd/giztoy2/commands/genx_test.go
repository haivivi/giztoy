package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenxAddGenerator(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	path := writeTestConfig(t, "gen.json", `{
		"schema": "openai/chat/v1", "type": "generator",
		"models": [{"name": "test/model", "model": "gpt-4"}]
	}`)
	stdout, _, code := runCmd(t, "genx", "add", path)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Added") {
		t.Fatalf("expected 'Added', got: %s", stdout)
	}
}

func TestGenxAddTTS(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	path := writeTestConfig(t, "tts.json", `{
		"schema": "minimax/speech/v1", "type": "tts",
		"voices": [{"name": "test/voice", "voice_id": "female-test"}]
	}`)
	stdout, _, code := runCmd(t, "genx", "add", path)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Added") {
		t.Fatalf("expected 'Added', got: %s", stdout)
	}
}

func TestGenxAddNotFound(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	_, _, code := runCmd(t, "genx", "add", "/nonexistent/file.json")
	if code == 0 {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestGenxListAll(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	genPath := writeTestConfig(t, "gen.json", `{
		"schema": "openai/chat/v1", "type": "generator",
		"models": [{"name": "test/gen", "model": "m"}]
	}`)
	ttsPath := writeTestConfig(t, "tts.json", `{
		"schema": "minimax/speech/v1", "type": "tts",
		"voices": [{"name": "test/voice", "voice_id": "v"}]
	}`)
	runCmd(t, "genx", "add", genPath)
	runCmd(t, "genx", "add", ttsPath)

	stdout, _, code := runCmd(t, "genx", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "generator") || !strings.Contains(stdout, "tts") {
		t.Fatalf("expected generator and tts, got: %s", stdout)
	}
}

func TestGenxListByType(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	genPath := writeTestConfig(t, "gen.json", `{
		"schema": "openai/chat/v1", "type": "generator",
		"models": [{"name": "test/gen", "model": "m"}]
	}`)
	ttsPath := writeTestConfig(t, "tts.json", `{
		"schema": "minimax/speech/v1", "type": "tts",
		"voices": [{"name": "test/voice", "voice_id": "v"}]
	}`)
	runCmd(t, "genx", "add", genPath)
	runCmd(t, "genx", "add", ttsPath)

	stdout, _, code := runCmd(t, "genx", "list", "--type", "generator")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "test/gen") {
		t.Fatalf("expected test/gen, got: %s", stdout)
	}
	if strings.Contains(stdout, "test/voice") {
		t.Fatalf("should not contain tts entry when filtering by generator: %s", stdout)
	}
}

func TestGenxListEmpty(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "genx", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "No genx configs") {
		t.Fatalf("expected 'No genx configs', got: %s", stdout)
	}
}

func TestGenxRemove(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	path := writeTestConfig(t, "gen.json", `{
		"schema": "openai/chat/v1", "type": "generator",
		"models": [{"name": "test/rm", "model": "m"}]
	}`)
	runCmd(t, "genx", "add", path)

	stdout, _, code := runCmd(t, "genx", "remove", "test/rm")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Removed") {
		t.Fatalf("expected 'Removed', got: %s", stdout)
	}

	stdout, _, _ = runCmd(t, "genx", "list")
	if !strings.Contains(stdout, "No genx configs") {
		t.Fatalf("expected empty after remove, got: %s", stdout)
	}
}

func TestGenxRemoveNotFound(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	_, _, code := runCmd(t, "genx", "remove", "nonexistent/pattern")
	if code == 0 {
		t.Fatal("expected error for nonexistent pattern")
	}
}
