package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/genx/cortex"
	"github.com/spf13/cobra"
)

// transformerRegistry holds registered transformer factories.
var transformerRegistry = map[string]cortex.TransformerFactory{}

// RegisterTransformer registers a mode-aware transformer factory with a name.
func RegisterTransformer(name string, factory cortex.TransformerFactory) {
	transformerRegistry[name] = factory
}

var (
	flagPort         string
	flagNamespace    string
	flagTransformer  string
	flagModel        string
	flagVoice        string
	flagInstructions string
	flagTimeout      time.Duration
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run cortex server with embedded MQTT broker",
	Long: `Run the cortex server with an embedded MQTT broker.

The server listens for gear connections and creates Atom instances
for each connected device. Each Atom bridges the device audio with
the configured transformer.

Example:
  cortextest run --port :1883 --transformer dashscope`,
	RunE: runServer,
}

func init() {
	runCmd.Flags().StringVar(&flagPort, "port", ":1883", "MQTT broker listen address")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "MQTT topic namespace")
	runCmd.Flags().StringVar(&flagTransformer, "transformer", "dashscope", "Transformer type (dashscope, doubao)")
	runCmd.Flags().StringVar(&flagModel, "model", "", "Model name (transformer-specific)")
	runCmd.Flags().StringVar(&flagVoice, "voice", "", "Voice name (transformer-specific)")
	runCmd.Flags().StringVar(&flagInstructions, "instructions", "", "System instructions")
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "Device inactivity timeout")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Setup logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	logger := slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	// Start MQTT listener
	logger.Info("Starting MQTT listener", "addr", flagPort)
	ln, err := chatgear.ListenMQTT0(ctx, chatgear.ListenerConfig{
		Addr:    flagPort,
		Scope:   flagNamespace,
		Timeout: flagTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to start MQTT listener: %w", err)
	}
	defer ln.Close()
	logger.Info("MQTT listener started", "addr", ln.Addr())

	logger.Info("Server ready, waiting for devices...")

	// Accept loop - handle new device connections
	for {
		accepted, err := ln.Accept()
		if err != nil {
			if err == chatgear.ErrListenerClosed || ctx.Err() != nil {
				break
			}
			logger.Error("Accept error", "error", err)
			continue
		}

		// Handle each device in a goroutine
		go func(port *chatgear.ServerPort, gearID string) {
			deviceLogger := logger.With("gearID", gearID)
			deviceLogger.Info("Device connected")

			// Get transformer factory for this device
			factory, err := getTransformerFactory(flagTransformer)
			if err != nil {
				deviceLogger.Error("Failed to get transformer factory", "error", err)
				return
			}

			// Create and run Atom with transformer factory
			atom := cortex.New(cortex.Config{
				Port:               port,
				TransformerFactory: factory,
				Logger:             deviceLogger,
			})

			// Run Atom (blocks until port is closed or context cancelled)
			if err := atom.Run(ctx); err != nil {
				deviceLogger.Error("Atom error", "error", err)
			}

			atom.Close()
			deviceLogger.Info("Device disconnected")
		}(accepted.Port, accepted.GearID)
	}

	logger.Info("Server stopped")
	return nil
}

func getTransformerFactory(name string) (cortex.TransformerFactory, error) {
	factory, ok := transformerRegistry[name]
	if !ok {
		// List available transformers
		var available []string
		for k := range transformerRegistry {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown transformer: %s (available: %v)", name, available)
	}
	return factory, nil
}
