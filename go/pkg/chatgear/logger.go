package chatgear

import (
	"fmt"
	"log/slog"
)

// Logger is the interface for logging in chatgear.
type Logger interface {
	ErrorPrintf(format string, args ...any)
	WarnPrintf(format string, args ...any)
	InfoPrintf(format string, args ...any)
	DebugPrintf(format string, args ...any)
	Errorf(format string, args ...any) error
}

type defaultLogger struct{}

// DefaultLogger returns the default logger instance using slog.
func DefaultLogger() Logger {
	return defaultLogger{}
}

func (f defaultLogger) ErrorPrintf(format string, args ...any) {
	slog.Error("chatgear: " + fmt.Sprintf(format, args...))
}

func (f defaultLogger) WarnPrintf(format string, args ...any) {
	slog.Warn("chatgear: " + fmt.Sprintf(format, args...))
}

func (f defaultLogger) InfoPrintf(format string, args ...any) {
	slog.Info("chatgear: " + fmt.Sprintf(format, args...))
}

func (f defaultLogger) DebugPrintf(format string, args ...any) {
	slog.Debug("chatgear: " + fmt.Sprintf(format, args...))
}

func (f defaultLogger) Errorf(format string, args ...any) error {
	return fmt.Errorf("chatgear: "+format, args...)
}

// SlogLogger creates a Logger from a slog.Logger.
func SlogLogger(l *slog.Logger) Logger {
	return &slogLogger{l}
}

type slogLogger struct {
	*slog.Logger
}

func (s *slogLogger) ErrorPrintf(format string, args ...any) {
	s.Logger.Error("chatgear: " + fmt.Sprintf(format, args...))
}

func (s *slogLogger) WarnPrintf(format string, args ...any) {
	s.Logger.Warn("chatgear: " + fmt.Sprintf(format, args...))
}

func (s *slogLogger) InfoPrintf(format string, args ...any) {
	s.Logger.Info("chatgear: " + fmt.Sprintf(format, args...))
}

func (s *slogLogger) DebugPrintf(format string, args ...any) {
	s.Logger.Debug("chatgear: " + fmt.Sprintf(format, args...))
}

func (s *slogLogger) Errorf(format string, args ...any) error {
	return fmt.Errorf("chatgear: "+format, args...)
}
