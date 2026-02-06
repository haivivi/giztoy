package cortex

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// TransformerMode represents the VAD mode for the transformer.
type TransformerMode string

const (
	// ModeManual is the push-to-talk mode without server VAD.
	// The client explicitly controls when to start/stop recording.
	ModeManual TransformerMode = "manual"

	// ModeServerVAD is the server-side VAD mode.
	// The server automatically detects voice activity.
	ModeServerVAD TransformerMode = "server_vad"
)

// TransformerFactory creates a transformer with the specified mode.
type TransformerFactory func(mode TransformerMode) (genx.Transformer, error)

// Config configures an Atom.
type Config struct {
	// Port is the ServerPort to connect to the device.
	Port *chatgear.ServerPort

	// TransformerFactory creates transformers with different modes.
	// This is called when the device state requires a specific VAD mode.
	TransformerFactory TransformerFactory

	// Logger is optional. If nil, uses slog.Default().
	Logger *slog.Logger
}

// Atom connects a ServerPort with a Transformer.
// It handles the state machine and bridges audio between device and transformer.
type Atom struct {
	port               *chatgear.ServerPort
	transformerFactory TransformerFactory
	logger             *slog.Logger

	mu              sync.Mutex
	currentState    chatgear.State
	currentMode     TransformerMode // Current transformer mode
	currentStreamID string
	ignoredStreams  map[string]bool

	// Audio decoder
	decoder *opus.Decoder

	// Track management for audio output
	track     pcm.Track
	trackCtrl *pcm.TrackCtrl

	// Current transform session
	transformer  genx.Transformer
	inputStream  *inputStream
	outputStream genx.Stream
	outputCancel context.CancelFunc
}

// New creates a new Atom.
func New(cfg Config) *Atom {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Atom{
		port:               cfg.Port,
		transformerFactory: cfg.TransformerFactory,
		logger:             logger,
		ignoredStreams:     make(map[string]bool),
	}
}

// Run starts the Atom's main loop.
// It blocks until the context is cancelled or the port is closed.
func (a *Atom) Run(ctx context.Context) error {
	a.logger.Info("cortex: Atom started")
	defer a.logger.Info("cortex: Atom stopped")

	for {
		data, err := a.port.Poll()
		if err != nil {
			if err == buffer.ErrIteratorDone {
				return nil
			}
			return err
		}

		if data.State != nil {
			a.handleState(ctx, data.State)
		}
		if data.Audio != nil {
			a.handleAudio(data.Audio)
		}
	}
}

// Close releases all resources.
func (a *Atom) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.closeSessionLocked()

	if a.decoder != nil {
		a.decoder.Close()
		a.decoder = nil
	}

	// Note: port is managed by the caller (Listener), don't close here
	return nil
}

// handleState processes state changes from the device.
func (a *Atom) handleState(ctx context.Context, evt *chatgear.StateEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	prevState := a.currentState
	a.currentState = evt.State

	a.logger.Info("cortex: state changed",
		"from", prevState,
		"to", evt.State,
	)

	switch evt.State {
	case chatgear.StateRecording:
		a.onRecordingLocked(ctx)

	case chatgear.StateWaitingForResponse:
		a.onWaitingForResponseLocked(ctx, prevState)

	case chatgear.StateCalling:
		a.onCallingLocked(ctx)

	case chatgear.StateReady, chatgear.StateInterrupted:
		a.onReadyLocked()

	case chatgear.StateShuttingDown, chatgear.StateSleeping:
		a.onShutdownLocked()
	}
}

// onRecordingLocked handles StateRecording (push-to-talk mode).
func (a *Atom) onRecordingLocked(ctx context.Context) {
	// Recording state requires manual mode (no server VAD)
	requiredMode := ModeManual

	// Check if we need to switch modes (e.g., from server_vad to manual)
	if a.currentMode != requiredMode && a.transformer != nil {
		a.logger.Info("cortex: switching mode", "from", a.currentMode, "to", requiredMode)
		a.closeSessionLocked()
	}

	// Close current track to stop AI playback immediately (interruption)
	if a.trackCtrl != nil {
		a.trackCtrl.Close()
		a.track = nil
		a.trackCtrl = nil
		a.logger.Info("cortex: closed track for interruption")
	}

	// Mark current OUTPUT stream as ignored so stale audio is filtered
	// Note: Only ignore if we have an active output stream (AI is responding)
	// Don't ignore the input stream we're about to create
	if a.currentStreamID != "" && a.outputStream != nil {
		a.ignoredStreams[a.currentStreamID] = true
		a.logger.Info("cortex: ignoring stream", "streamID", a.currentStreamID)
		a.currentStreamID = ""
	}

	// For manual mode, we need to close the old session and create a new one
	// because DashScope's manual mode expects one CommitInput+CreateResponse per session
	// If we reuse the session, we get "Conversation has none active response" errors
	if a.transformer != nil {
		a.logger.Info("cortex: closing session for new recording turn")
		a.closeSessionLocked()
	}

	// Create new input stream for this recording
	a.ensureInputStreamLocked()

	// Set required mode for transform
	a.currentMode = requiredMode
}

// onWaitingForResponseLocked handles StateWaitingForResponse.
func (a *Atom) onWaitingForResponseLocked(ctx context.Context, prevState chatgear.State) {
	if prevState != chatgear.StateRecording {
		return
	}

	// Commit input stream (signal end of user speech)
	if a.inputStream != nil {
		a.inputStream.Commit()
		a.logger.Info("cortex: committed input stream")
	}

	// Start transform if not already running
	a.ensureTransformLocked(ctx)
}

// onCallingLocked handles StateCalling (server VAD mode).
func (a *Atom) onCallingLocked(ctx context.Context) {
	// Calling state requires server VAD mode
	requiredMode := ModeServerVAD

	// Check if we need to switch modes
	if a.currentMode != requiredMode && a.outputStream != nil {
		a.logger.Info("cortex: switching mode", "from", a.currentMode, "to", requiredMode)
		a.closeSessionLocked()
	}

	// Set required mode for transform
	a.currentMode = requiredMode

	// Ensure input stream and transform are running
	a.ensureInputStreamLocked()
	a.ensureTransformLocked(ctx)
}

// onReadyLocked handles StateReady and StateInterrupted.
func (a *Atom) onReadyLocked() {
	// Do NOT close session on ready - keep DashScope session alive
	// for continuous conversation with server-side VAD.
	// Session will be closed on shutdown/sleeping only.

	// However, we should stop current audio playback (e.g., user pressed ESC)
	// Close current track to stop AI playback immediately
	if a.trackCtrl != nil {
		a.trackCtrl.Close()
		a.track = nil
		a.trackCtrl = nil
		a.logger.Info("cortex: closed track on ready")
	}

	// Mark current stream as ignored so stale audio is filtered
	if a.currentStreamID != "" {
		a.ignoredStreams[a.currentStreamID] = true
		a.logger.Info("cortex: ignoring stream on ready", "streamID", a.currentStreamID)
		a.currentStreamID = ""
	}
}

// onShutdownLocked handles StateShuttingDown and StateSleeping.
func (a *Atom) onShutdownLocked() {
	// Close everything
	a.closeSessionLocked()
}

// handleAudio processes audio frames from the device.
func (a *Atom) handleAudio(frame *chatgear.StampedOpusFrame) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Only process audio in active states
	if a.currentState != chatgear.StateRecording && a.currentState != chatgear.StateCalling {
		a.logger.Debug("cortex: ignoring audio in state", "state", a.currentState)
		return
	}

	// Ensure decoder exists
	if a.decoder == nil {
		dec, err := opus.NewDecoder(16000, 1)
		if err != nil {
			a.logger.Error("cortex: failed to create opus decoder", "error", err)
			return
		}
		a.decoder = dec
	}

	// Decode Opus to PCM
	pcmData, err := a.decoder.Decode(frame.Frame)
	if err != nil {
		a.logger.Error("cortex: failed to decode opus", "error", err)
		return
	}

	// Write to input stream
	if a.inputStream != nil {
		if err := a.inputStream.Write(pcmData); err != nil {
			a.logger.Error("cortex: failed to write to input stream", "error", err)
		} else {
			a.logger.Debug("cortex: wrote audio to input stream", "len", len(pcmData))
		}
	} else {
		a.logger.Warn("cortex: no input stream to write audio")
	}
}

// ensureInputStreamLocked creates input stream if not exists.
func (a *Atom) ensureInputStreamLocked() {
	if a.inputStream != nil {
		return
	}

	streamID := genx.NewStreamID()
	a.inputStream = newInputStream(streamID)
	a.currentStreamID = streamID
	a.logger.Info("cortex: created input stream", "streamID", streamID)
}

// ensureTransformLocked starts the transform if not running.
func (a *Atom) ensureTransformLocked(ctx context.Context) {
	if a.outputStream != nil {
		return
	}

	if a.inputStream == nil {
		return
	}

	// Create transformer with current mode if not exists
	if a.transformer == nil {
		transformer, err := a.transformerFactory(a.currentMode)
		if err != nil {
			a.logger.Error("cortex: failed to create transformer", "error", err, "mode", a.currentMode)
			return
		}
		a.transformer = transformer
		a.logger.Info("cortex: created transformer", "mode", a.currentMode)
	}

	// Start transform
	transformCtx, cancel := context.WithCancel(ctx)
	a.outputCancel = cancel

	output, err := a.transformer.Transform(transformCtx, string(a.currentMode), a.inputStream)
	if err != nil {
		a.logger.Error("cortex: transform failed", "error", err)
		cancel()
		return
	}

	a.outputStream = output
	a.logger.Info("cortex: transform started", "mode", a.currentMode)

	// Start output processing goroutine
	go a.processOutput(transformCtx)
}

// processOutput reads from the transformer output and writes to the device.
func (a *Atom) processOutput(ctx context.Context) {
	a.logger.Info("cortex: output processor started")

	// Keep a local reference to outputStream to avoid nil dereference
	// when closeSessionLocked sets a.outputStream = nil
	a.mu.Lock()
	stream := a.outputStream
	a.mu.Unlock()

	if stream == nil {
		a.logger.Warn("cortex: output processor started with nil stream")
		return
	}

	defer func() {
		a.logger.Info("cortex: output processor stopped")
		// Clean up output state so next transform can start
		a.mu.Lock()
		a.outputStream = nil
		if a.outputCancel != nil {
			a.outputCancel()
			a.outputCancel = nil
		}
		a.mu.Unlock()
	}()

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				a.logger.Info("cortex: output stream EOF")
			} else {
				a.logger.Error("cortex: output stream error", "error", err)
			}
			return
		}

		if chunk == nil {
			continue
		}

		// Get stream ID from chunk
		streamID := ""
		if chunk.Ctrl != nil {
			streamID = chunk.Ctrl.StreamID
		}

		a.mu.Lock()

		// Check if this stream should be ignored
		if streamID != "" && a.ignoredStreams[streamID] {
			a.mu.Unlock()
			continue
		}

		// Handle BOS - create new track
		if chunk.IsBeginOfStream() {
			a.currentStreamID = streamID
			if err := a.newTrackLocked(); err != nil {
				a.logger.Error("cortex: failed to create track", "error", err)
			}
			a.mu.Unlock()
			continue
		}

		// Handle audio data
		if blob, ok := chunk.Part.(*genx.Blob); ok && len(blob.Data) > 0 {
			if a.track != nil {
				// Determine format from MIME type
				format := pcm.L16Mono24K // Default
				if blob.MIMEType == "audio/pcm;rate=16000" {
					format = pcm.L16Mono16K
				}

				pcmChunk := format.DataChunk(blob.Data)
				if err := a.track.Write(pcmChunk); err != nil {
					a.logger.Error("cortex: track write error", "error", err)
				}
			}
		}

		// Handle EOS - close track write
		if chunk.IsEndOfStream() {
			if a.trackCtrl != nil {
				a.trackCtrl.CloseWrite()
			}
		}

		a.mu.Unlock()
	}
}

// newTrackLocked creates a new foreground track.
// Must be called with lock held.
func (a *Atom) newTrackLocked() error {
	track, ctrl, err := a.port.NewForegroundTrack()
	if err != nil {
		return err
	}
	a.track = track
	a.trackCtrl = ctrl
	a.logger.Info("cortex: new track created")
	return nil
}

// closeSessionLocked closes the current transform session.
// Must be called with lock held.
func (a *Atom) closeSessionLocked() {
	if a.outputCancel != nil {
		a.outputCancel()
		a.outputCancel = nil
	}

	if a.outputStream != nil {
		a.outputStream.Close()
		a.outputStream = nil
	}

	if a.inputStream != nil {
		a.inputStream.Close()
		a.inputStream = nil
	}

	// Close transformer (will be recreated with new mode if needed)
	if a.transformer != nil {
		if closer, ok := a.transformer.(interface{ Close() error }); ok {
			closer.Close()
		}
		a.transformer = nil
	}

	if a.trackCtrl != nil {
		a.trackCtrl.Close()
		a.track = nil
		a.trackCtrl = nil
	}

	a.currentStreamID = ""
}
