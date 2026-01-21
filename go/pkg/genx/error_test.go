package genx

import (
	"errors"
	"testing"
)

func TestDone(t *testing.T) {
	usage := Usage{
		PromptTokenCount:    100,
		GeneratedTokenCount: 50,
	}

	state := Done(usage)

	if state.Status() != StatusDone {
		t.Errorf("Status() = %v, want %v", state.Status(), StatusDone)
	}

	if state.Usage().PromptTokenCount != 100 {
		t.Errorf("Usage().PromptTokenCount = %d, want 100", state.Usage().PromptTokenCount)
	}

	if state.Usage().GeneratedTokenCount != 50 {
		t.Errorf("Usage().GeneratedTokenCount = %d, want 50", state.Usage().GeneratedTokenCount)
	}

	if !errors.Is(state.Unwrap(), ErrDone) {
		t.Errorf("Unwrap() should return ErrDone")
	}

	if state.Error() != "genx: generate done" {
		t.Errorf("Error() = %q, want %q", state.Error(), "genx: generate done")
	}
}

func TestBlocked(t *testing.T) {
	usage := Usage{
		PromptTokenCount: 100,
	}

	state := Blocked(usage, "content policy violation")

	if state.Status() != StatusBlocked {
		t.Errorf("Status() = %v, want %v", state.Status(), StatusBlocked)
	}

	if state.Usage().PromptTokenCount != 100 {
		t.Errorf("Usage().PromptTokenCount = %d, want 100", state.Usage().PromptTokenCount)
	}

	errMsg := state.Error()
	if errMsg != "genx: generate blocked: content policy violation" {
		t.Errorf("Error() = %q", errMsg)
	}
}

func TestTruncated(t *testing.T) {
	usage := Usage{
		PromptTokenCount:    100,
		GeneratedTokenCount: 4096,
	}

	state := Truncated(usage)

	if state.Status() != StatusTruncated {
		t.Errorf("Status() = %v, want %v", state.Status(), StatusTruncated)
	}

	if state.Usage().GeneratedTokenCount != 4096 {
		t.Errorf("Usage().GeneratedTokenCount = %d, want 4096", state.Usage().GeneratedTokenCount)
	}

	errMsg := state.Error()
	if errMsg != "genx: generate truncated" {
		t.Errorf("Error() = %q, want %q", errMsg, "genx: generate truncated")
	}
}

func TestError(t *testing.T) {
	usage := Usage{
		PromptTokenCount: 50,
	}

	originalErr := errors.New("network timeout")
	state := Error(usage, originalErr)

	if state.Status() != StatusError {
		t.Errorf("Status() = %v, want %v", state.Status(), StatusError)
	}

	if state.Usage().PromptTokenCount != 50 {
		t.Errorf("Usage().PromptTokenCount = %d, want 50", state.Usage().PromptTokenCount)
	}

	// Should wrap the original error
	if !errors.Is(state.Unwrap(), originalErr) {
		t.Errorf("Unwrap() should contain original error")
	}
}

func TestState_UnexpectedStatus(t *testing.T) {
	// Test unexpected status value
	state := &State{
		status: Status(999), // Invalid status
	}

	errMsg := state.Error()
	if errMsg == "" {
		t.Error("Error() should return non-empty string for unexpected status")
	}
}

func TestErrDone(t *testing.T) {
	if ErrDone.Error() != "genx: done" {
		t.Errorf("ErrDone.Error() = %q, want %q", ErrDone.Error(), "genx: done")
	}
}

func TestUsage_ZeroValues(t *testing.T) {
	usage := Usage{}

	state := Done(usage)

	if state.Usage().PromptTokenCount != 0 {
		t.Errorf("Usage().PromptTokenCount = %d, want 0", state.Usage().PromptTokenCount)
	}

	if state.Usage().CachedContentTokenCount != 0 {
		t.Errorf("Usage().CachedContentTokenCount = %d, want 0", state.Usage().CachedContentTokenCount)
	}

	if state.Usage().GeneratedTokenCount != 0 {
		t.Errorf("Usage().GeneratedTokenCount = %d, want 0", state.Usage().GeneratedTokenCount)
	}
}

func TestUsage_WithCachedContent(t *testing.T) {
	usage := Usage{
		PromptTokenCount:        1000,
		CachedContentTokenCount: 800,
		GeneratedTokenCount:     200,
	}

	state := Done(usage)

	if state.Usage().CachedContentTokenCount != 800 {
		t.Errorf("Usage().CachedContentTokenCount = %d, want 800", state.Usage().CachedContentTokenCount)
	}
}
