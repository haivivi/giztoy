package chatgear

import (
	"errors"
	"log/slog"
	"testing"
)

func TestDefaultLogger(t *testing.T) {
	logger := DefaultLogger()
	if logger == nil {
		t.Fatal("DefaultLogger returned nil")
	}

	// These should not panic
	logger.ErrorPrintf("test error %d", 1)
	logger.WarnPrintf("test warn %s", "msg")
	logger.InfoPrintf("test info")
	logger.DebugPrintf("test debug")

	err := logger.Errorf("test error: %w", errors.New("test"))
	if err == nil {
		t.Error("Errorf should return error")
	}
}

func TestSlogLogger(t *testing.T) {
	logger := SlogLogger(slog.Default())
	if logger == nil {
		t.Fatal("SlogLogger returned nil")
	}

	// These should not panic
	logger.ErrorPrintf("test error %d", 1)
	logger.WarnPrintf("test warn %s", "msg")
	logger.InfoPrintf("test info")
	logger.DebugPrintf("test debug")

	err := logger.Errorf("test error: %w", errors.New("test"))
	if err == nil {
		t.Error("Errorf should return error")
	}
}
