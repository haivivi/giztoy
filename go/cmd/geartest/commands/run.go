package commands

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	// Command-line overrides
	flagGearID     string
	flagMQTTURL    string
	flagNamespace  string
	flagWebPort    int
	flagSysVersion string
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the gear simulator",
	Long: `Run the chatgear device simulator.

The simulator connects to an MQTT broker and simulates a chatgear device,
responding to commands and sending state/stats updates.

Audio is handled via WebRTC - open the web control panel in a browser
to connect microphone and speaker.`,
	RunE: runSimulator,
}

func init() {
	runCmd.Flags().StringVar(&flagGearID, "gear-id", "", "gear ID to simulate (required)")
	runCmd.Flags().StringVar(&flagMQTTURL, "mqtt", "", "MQTT broker URL")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "MQTT topic namespace")
	runCmd.Flags().IntVar(&flagWebPort, "web", 0, "web control panel port")
	runCmd.Flags().StringVar(&flagSysVersion, "version", "", "system version (format: version_lang)")
}

func runSimulator(cmd *cobra.Command, args []string) error {
	// Load context configuration
	var cfg *GearConfig
	ctx, err := getContext()
	if err != nil {
		// No context set, use defaults
		cfg = DefaultGearConfig()
	} else {
		cfg = LoadGearConfig(ctx)
	}

	// Apply command-line overrides
	if flagGearID != "" {
		cfg.GearID = flagGearID
	}
	if flagMQTTURL != "" {
		cfg.MQTTURL = flagMQTTURL
	}
	if flagNamespace != "" {
		cfg.Namespace = flagNamespace
	}
	if flagWebPort != 0 {
		cfg.WebPort = flagWebPort
	}
	if flagSysVersion != "" {
		cfg.SysVersion = flagSysVersion
	}

	// Validate required fields
	if cfg.GearID == "" {
		return fmt.Errorf("gear ID is required. Use --gear-id flag or configure a context:\n" +
			"  geartest config context set dev --gear-id=<id> --mqtt=mqtt://localhost:1883\n" +
			"  geartest config context use dev")
	}

	// Setup logging to stdout (Debug level for audio troubleshooting)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// Create simulator
	sim := NewSimulator(SimulatorConfig{
		MQTTURL:   cfg.MQTTURL,
		GearID:    cfg.GearID,
		Namespace: cfg.Namespace,
	})

	// Load saved state (or keep defaults from NewSimulator)
	sim.LoadStateOrDefaults(cfg.SysVersion)

	// Start web control panel
	webServer := NewWebServer(sim, cfg.WebPort)
	sim.SetWebServer(webServer) // Connect simulator to web server for log forwarding
	webServer.Start()
	fmt.Printf("Web control panel: http://localhost:%d\n", cfg.WebPort)
	fmt.Println("Device is OFF. Use web UI to power on and control.")
	fmt.Println("Press Ctrl+C to exit")

	defer sim.Stop()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	return nil
}
