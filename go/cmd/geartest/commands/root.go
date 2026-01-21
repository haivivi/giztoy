package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/haivivi/giztoy/pkg/cli"
	"github.com/spf13/cobra"
)

const appName = "geartest"

var (
	cfgFile      string
	contextName  string
	globalConfig *cli.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "geartest",
	Short: "Chatgear device simulator",
	Long: `geartest is a CLI tool to simulate a chatgear device.

It provides a TUI interface and WebRTC-based audio I/O for testing
chatgear server implementations.

Configuration is stored in ~/.giztoy/geartest/ and supports multiple contexts,
allowing you to switch between different environments (dev, staging, prod).`,
	// Run the simulator by default
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSimulator(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", "config file (default is ~/.giztoy/geartest/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context to use (default is current context)")

	// Add subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	var err error
	globalConfig, err = cli.LoadConfigWithPath(appName, cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}

// getContext returns the context to use, resolving from flag or current context.
func getContext() (*cli.Context, error) {
	return globalConfig.ResolveContext(contextName)
}

// GearConfig holds geartest-specific configuration extracted from Context.Extra.
type GearConfig struct {
	GearID     string
	MQTTURL    string
	Namespace  string
	WebPort    int
	SysVersion string
	Headless   bool
}

// DefaultGearConfig returns default configuration.
func DefaultGearConfig() *GearConfig {
	return &GearConfig{
		GearID:     "",
		MQTTURL:    "mqtt://localhost:1883",
		Namespace:  "",
		WebPort:    8088,
		SysVersion: "0_zh",
		Headless:   false,
	}
}

// LoadGearConfig loads geartest configuration from a context.
func LoadGearConfig(ctx *cli.Context) *GearConfig {
	cfg := DefaultGearConfig()
	if ctx == nil {
		return cfg
	}

	if v := ctx.GetExtra("gear_id"); v != "" {
		cfg.GearID = v
	}
	if v := ctx.GetExtra("mqtt_url"); v != "" {
		cfg.MQTTURL = v
	}
	if v := ctx.GetExtra("namespace"); v != "" {
		cfg.Namespace = v
	}
	if v := ctx.GetExtra("web_port"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.WebPort = port
		}
	}
	if v := ctx.GetExtra("sys_version"); v != "" {
		cfg.SysVersion = v
	}
	if v := ctx.GetExtra("headless"); v == "true" {
		cfg.Headless = true
	}

	return cfg
}

// SaveGearConfig saves geartest configuration to a context.
func SaveGearConfig(ctx *cli.Context, cfg *GearConfig) {
	ctx.SetExtra("gear_id", cfg.GearID)
	ctx.SetExtra("mqtt_url", cfg.MQTTURL)
	ctx.SetExtra("namespace", cfg.Namespace)
	ctx.SetExtra("web_port", fmt.Sprintf("%d", cfg.WebPort))
	ctx.SetExtra("sys_version", cfg.SysVersion)
	if cfg.Headless {
		ctx.SetExtra("headless", "true")
	} else {
		ctx.SetExtra("headless", "false")
	}
}
