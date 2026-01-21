package cli

import (
	"strings"

	"github.com/haivivi/giztoy/pkg/buffer"
)

// LogBuffer is a thread-safe log buffer with max size using ring buffer.
type LogBuffer = buffer.RingBuffer[string]

// NewLogBuffer creates a new buffer with the given max size.
func NewLogBuffer(maxSize int) *LogBuffer {
	return buffer.RingN[string](maxSize)
}

// LogWriter implements io.Writer and captures log output for TUI display.
// It stores lines in a buffer and notifies via a channel.
type LogWriter struct {
	buf *LogBuffer
	ch  chan string
}

// NewLogWriter creates a new log writer with the given max lines.
func NewLogWriter(maxLines int) *LogWriter {
	return &LogWriter{
		buf: NewLogBuffer(maxLines),
		ch:  make(chan string, 100),
	}
}

// Write implements io.Writer.
// Handles multi-line input by splitting on newlines.
func (w *LogWriter) Write(p []byte) (n int, err error) {
	text := strings.TrimRight(string(p), "\n")
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		_ = w.buf.Add(line)

		// Non-blocking send to channel
		select {
		case w.ch <- line:
		default:
		}
	}
	return len(p), nil
}

// Lines returns all buffered lines.
func (w *LogWriter) Lines() []string {
	return w.buf.Bytes()
}

// Channel returns the notification channel for new lines.
func (w *LogWriter) Channel() <-chan string {
	return w.ch
}
