// Package gear implements the 'giztoy gear' subcommand tree.
// It directly calls go/pkg/chatgear for device simulation.
package gear

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
)

// Cmd is the root 'gear' command.
var Cmd = &cobra.Command{
	Use:   "gear",
	Short: "Chatgear device simulator",
	Long: `Chatgear device simulator.

Provides a web interface and WebRTC-based audio I/O for testing
chatgear server implementations.

Configuration (gear.yaml in context dir):
  gear_id: abc123
  mqtt_url: mqtt://localhost:1883
  namespace: ""
  web_port: 8088
  sys_version: 0_zh

Examples:
  giztoy gear run --gear-id abc123
  giztoy gear run --gear-id abc123 --mqtt mqtt://localhost:1883`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSimulatorCmd(cmd, args)
	},
}

var (
	contextName    string
	flagGearID     string
	flagMQTTURL    string
	flagNamespace  string
	flagWebPort    int
	flagSysVersion string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the gear simulator",
	Long: `Run the chatgear device simulator.

The simulator connects to an MQTT broker and simulates a chatgear device.
Audio is handled via WebRTC — open the web control panel in a browser.`,
	RunE: runSimulatorCmd,
}

func init() {
	Cmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")

	runCmd.Flags().StringVar(&flagGearID, "gear-id", "", "gear ID to simulate (required)")
	runCmd.Flags().StringVar(&flagMQTTURL, "mqtt", "", "MQTT broker URL")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "MQTT topic namespace")
	runCmd.Flags().IntVar(&flagWebPort, "web", 0, "web control panel port")
	runCmd.Flags().StringVar(&flagSysVersion, "version", "", "system version (format: version_lang)")

	Cmd.AddCommand(runCmd)
}

// GearServiceConfig is the per-context gear.yaml schema.
type GearServiceConfig struct {
	GearID     string `yaml:"gear_id"`
	MQTTURL    string `yaml:"mqtt_url"`
	Namespace  string `yaml:"namespace"`
	WebPort    int    `yaml:"web_port"`
	SysVersion string `yaml:"sys_version"`
}

func defaultGearConfig() *GearServiceConfig {
	return &GearServiceConfig{
		MQTTURL:    "mqtt://localhost:1883",
		WebPort:    8088,
		SysVersion: "0_zh",
	}
}

func loadGearConfig() *GearServiceConfig {
	cfg := defaultGearConfig()

	rootCfg, err := config.Load()
	if err != nil {
		return cfg
	}
	contextDir, err := rootCfg.ResolveContext(contextName)
	if err != nil {
		return cfg
	}

	svc, err := config.LoadService[GearServiceConfig](contextDir, "gear")
	if err != nil {
		return cfg
	}

	// Merge loaded values into defaults
	if svc.GearID != "" {
		cfg.GearID = svc.GearID
	}
	if svc.MQTTURL != "" {
		cfg.MQTTURL = svc.MQTTURL
	}
	if svc.Namespace != "" {
		cfg.Namespace = svc.Namespace
	}
	if svc.WebPort != 0 {
		cfg.WebPort = svc.WebPort
	}
	if svc.SysVersion != "" {
		cfg.SysVersion = svc.SysVersion
	}
	return cfg
}

func runSimulatorCmd(cmd *cobra.Command, args []string) error {
	cfg := loadGearConfig()

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

	if cfg.GearID == "" {
		return fmt.Errorf("gear ID is required. Use --gear-id flag or configure a context:\n" +
			"  giztoy config set dev gear gear_id <id>\n" +
			"  giztoy config set dev gear mqtt_url mqtt://localhost:1883")
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	sim := NewSimulator(SimulatorConfig{
		MQTTURL:   cfg.MQTTURL,
		GearID:    cfg.GearID,
		Namespace: cfg.Namespace,
	})

	sim.LoadStateOrDefaults(cfg.SysVersion)

	webServer := NewWebServer(sim, cfg.WebPort)
	sim.SetWebServer(webServer)
	webServer.Start()
	fmt.Printf("Web control panel: http://localhost:%d\n", cfg.WebPort)
	fmt.Println("Device is OFF. Use web UI to power on and control.")
	fmt.Println("Press Ctrl+C to exit")

	defer sim.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	return nil
}
