// Package cortex implements the 'giztoy cortex' subcommand tree.
// It directly calls go/pkg/genx/cortex and go/pkg/chatgear.
package cortex

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/dashscope"
	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/cortex"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
)

// Cmd is the root 'cortex' command.
var Cmd = &cobra.Command{
	Use:   "cortex",
	Short: "Cortex server (bridges devices with AI transformers)",
	Long: `Cortex Atom server for testing.

Starts an embedded MQTT broker and creates Atom instances for each
connected gear device. The Atom bridges device audio with a configurable
transformer (DashScope realtime, Doubao, etc.).

Transformer credentials are read from environment variables:
  dashscope: DASHSCOPE_API_KEY
  doubao:    DOUBAO_APP_ID, DOUBAO_ACCESS_KEY [, DOUBAO_APP_KEY]

Examples:
  giztoy cortex run --port :1883 --transformer dashscope
  giztoy cortex run --transformer doubao --voice zh_female_cancan`,
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
	RunE:  runServer,
}

func init() {
	runCmd.Flags().StringVar(&flagPort, "port", ":1883", "MQTT broker listen address")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "MQTT topic namespace")
	runCmd.Flags().StringVar(&flagTransformer, "transformer", "dashscope", "Transformer type (dashscope, doubao)")
	runCmd.Flags().StringVar(&flagModel, "model", "", "Model name (transformer-specific)")
	runCmd.Flags().StringVar(&flagVoice, "voice", "", "Voice name (transformer-specific)")
	runCmd.Flags().StringVar(&flagInstructions, "instructions", "", "System instructions")
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "Device inactivity timeout")

	Cmd.AddCommand(runCmd)
}

// transformerRegistry holds registered transformer factories.
var transformerRegistry = map[string]cortex.TransformerFactory{
	"dashscope": newDashScopeRealtimeFactory,
	"doubao":    newDoubaoRealtimeFactory,
}

func runServer(cmd *cobra.Command, args []string) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	logger := slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

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

	for {
		accepted, err := ln.Accept()
		if err != nil {
			if err == chatgear.ErrListenerClosed || ctx.Err() != nil {
				break
			}
			logger.Error("Accept error", "error", err)
			continue
		}

		go func(port *chatgear.ServerPort, gearID string) {
			deviceLogger := logger.With("gearID", gearID)
			deviceLogger.Info("Device connected")

			factory, ok := transformerRegistry[flagTransformer]
			if !ok {
				var available []string
				for k := range transformerRegistry {
					available = append(available, k)
				}
				deviceLogger.Error("Unknown transformer", "name", flagTransformer, "available", available)
				return
			}

			atom := cortex.New(cortex.Config{
				Port:               port,
				TransformerFactory: factory,
				Logger:             deviceLogger,
			})

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

// ---------------------------------------------------------------------------
// Transformer Factories
// ---------------------------------------------------------------------------

func newDashScopeRealtimeFactory(mode cortex.TransformerMode) (genx.Transformer, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DASHSCOPE_API_KEY environment variable is required")
	}

	client := dashscope.NewClient(apiKey)

	var opts []transformers.DashScopeRealtimeOption
	if flagModel != "" {
		opts = append(opts, transformers.WithDashScopeRealtimeModel(flagModel))
	}
	if flagVoice != "" {
		opts = append(opts, transformers.WithDashScopeRealtimeVoice(flagVoice))
	}

	instructions := flagInstructions
	if instructions == "" {
		instructions = "你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。"
	}
	opts = append(opts, transformers.WithDashScopeRealtimeInstructions(instructions))

	switch mode {
	case cortex.ModeServerVAD:
		opts = append(opts, transformers.WithDashScopeRealtimeVAD("server_vad"))
	case cortex.ModeManual:
		opts = append(opts, transformers.WithDashScopeRealtimeVAD(""))
	}

	return transformers.NewDashScopeRealtime(client, opts...), nil
}

func newDoubaoRealtimeFactory(mode cortex.TransformerMode) (genx.Transformer, error) {
	_ = mode
	appID := os.Getenv("DOUBAO_APP_ID")
	if appID == "" {
		return nil, fmt.Errorf("DOUBAO_APP_ID environment variable is required")
	}

	accessKey := os.Getenv("DOUBAO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("DOUBAO_TOKEN")
	}
	if accessKey == "" {
		return nil, fmt.Errorf("DOUBAO_ACCESS_KEY or DOUBAO_TOKEN environment variable is required")
	}

	appKey := os.Getenv("DOUBAO_APP_KEY")
	if appKey == "" {
		appKey = appID
	}

	client := doubaospeech.NewClient(appID, doubaospeech.WithV2APIKey(accessKey, appKey))

	var opts []transformers.DoubaoRealtimeOption
	if flagVoice != "" {
		opts = append(opts, transformers.WithDoubaoRealtimeSpeaker(flagVoice))
	}

	instructions := flagInstructions
	if instructions == "" {
		instructions = "你是一个友好的语音助手，善于用简洁清晰的语言回答问题。回复要简短，控制在20个字以内。"
	}
	opts = append(opts, transformers.WithDoubaoRealtimeSystemRole(instructions))

	return transformers.NewDoubaoRealtime(client, opts...), nil
}
