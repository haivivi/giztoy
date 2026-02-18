package commands

import (
	"strings"
	"testing"
)

func setupAppTestEnv(t *testing.T) func() {
	t.Helper()
	_, cleanup := setupTestEnv(t)
	runCmd(t, "ctx", "add", "dev")
	runCmd(t, "ctx", "use", "dev")
	return cleanup
}

// ---------------------------------------------------------------------------
// minimax app tests
// ---------------------------------------------------------------------------

func TestMinimaxAppAdd(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "minimax", "app", "add", "cn", "--api-key", "test-key", "--base-url", "https://api.minimaxi.com")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "added") {
		t.Fatalf("expected 'added', got: %s", stdout)
	}
}

func TestMinimaxAppAddNoKey(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	_, stderr, code := runCmd(t, "minimax", "app", "add", "cn")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "--api-key is required") {
		t.Fatalf("expected '--api-key is required', got: %s", stderr)
	}
}

func TestMinimaxAppList(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "minimax", "app", "add", "cn", "--api-key", "k1")
	runCmd(t, "minimax", "app", "add", "global", "--api-key", "k2")

	stdout, _, code := runCmd(t, "minimax", "app", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "cn") || !strings.Contains(stdout, "global") {
		t.Fatalf("expected cn and global, got: %s", stdout)
	}
}

func TestMinimaxAppUse(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "minimax", "app", "add", "cn", "--api-key", "k1")
	stdout, _, code := runCmd(t, "minimax", "app", "use", "cn")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Switched") {
		t.Fatalf("expected 'Switched', got: %s", stdout)
	}
}

func TestMinimaxAppShow(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "minimax", "app", "add", "cn", "--api-key", "test-key-123", "--base-url", "https://api.minimaxi.com")
	stdout, _, code := runCmd(t, "minimax", "app", "show", "cn")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "minimaxi.com") {
		t.Fatalf("expected base_url, got: %s", stdout)
	}
}

func TestMinimaxAppRemove(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "minimax", "app", "add", "cn", "--api-key", "k1")
	runCmd(t, "minimax", "app", "add", "global", "--api-key", "k2")
	stdout, _, code := runCmd(t, "minimax", "app", "remove", "global")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "removed") {
		t.Fatalf("expected 'removed', got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// doubaospeech app tests
// ---------------------------------------------------------------------------

func TestDoubaospeechAppAdd(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "doubaospeech", "app", "add", "test", "--app-id", "111", "--token", "tok")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "added") {
		t.Fatalf("expected 'added', got: %s", stdout)
	}
}

func TestDoubaospeechAppAddNoAppID(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	_, stderr, code := runCmd(t, "doubaospeech", "app", "add", "test", "--token", "tok")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "--app-id is required") {
		t.Fatalf("expected '--app-id is required', got: %s", stderr)
	}
}

func TestDoubaospeechAppList(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "doubaospeech", "app", "add", "test", "--app-id", "111", "--token", "t1")
	runCmd(t, "doubaospeech", "app", "add", "prod", "--app-id", "222", "--token", "t2")

	stdout, _, code := runCmd(t, "doubaospeech", "app", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "test") || !strings.Contains(stdout, "prod") {
		t.Fatalf("expected test and prod, got: %s", stdout)
	}
}

func TestDoubaospeechAppRemove(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "doubaospeech", "app", "add", "test", "--app-id", "111", "--token", "tok")
	stdout, _, code := runCmd(t, "doubaospeech", "app", "remove", "test")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "removed") {
		t.Fatalf("expected 'removed', got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// dashscope app tests
// ---------------------------------------------------------------------------

func TestDashscopeAppAdd(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	stdout, _, code := runCmd(t, "dashscope", "app", "add", "default", "--api-key", "sk-test")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "added") {
		t.Fatalf("expected 'added', got: %s", stdout)
	}
}

func TestDashscopeAppList(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "dashscope", "app", "add", "default", "--api-key", "k1")
	runCmd(t, "dashscope", "app", "add", "intl", "--api-key", "k2")

	stdout, _, code := runCmd(t, "dashscope", "app", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "default") || !strings.Contains(stdout, "intl") {
		t.Fatalf("expected default and intl, got: %s", stdout)
	}
}

func TestDashscopeAppRemove(t *testing.T) {
	cleanup := setupAppTestEnv(t)
	defer cleanup()

	runCmd(t, "dashscope", "app", "add", "default", "--api-key", "k1")
	stdout, _, code := runCmd(t, "dashscope", "app", "remove", "default")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "removed") {
		t.Fatalf("expected 'removed', got: %s", stdout)
	}
}
