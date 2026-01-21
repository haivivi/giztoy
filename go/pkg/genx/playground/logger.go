package playground

import (
	"fmt"
	"log/slog"
	"strings"
)

// ConsoleLogger is a logger that writes to the standard log output.
type ConsoleLogger struct {
	level LogLevel
}

// LogLevel represents the logging level.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelError
)

// NewConsoleLogger creates a new ConsoleLogger.
// Output is written via slog.
func NewConsoleLogger(level LogLevel) *ConsoleLogger {
	return &ConsoleLogger{level: level}
}

func formatKV(keysAndValues []any) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(keysAndValues); i += 2 {
		b.WriteByte(' ')
		fmt.Fprint(&b, keysAndValues[i])
		b.WriteByte('=')
		if i+1 < len(keysAndValues) {
			fmt.Fprint(&b, keysAndValues[i+1])
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func (l *ConsoleLogger) Debug(msg string, keysAndValues ...any) {
	if l.level <= LogLevelDebug {
		slog.Debug("playground: "+msg, keysAndValues...)
	}
}

func (l *ConsoleLogger) Info(msg string, keysAndValues ...any) {
	if l.level <= LogLevelInfo {
		slog.Info("playground: "+msg, keysAndValues...)
	}
}

func (l *ConsoleLogger) Error(msg string, keysAndValues ...any) {
	if l.level <= LogLevelError {
		slog.Error("playground: "+msg, keysAndValues...)
	}
}
