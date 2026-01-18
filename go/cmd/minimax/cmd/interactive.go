package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
)

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i", "tui"},
	Short:   "Start interactive TUI mode",
	Long: `Start an interactive TUI mode for exploring MiniMax APIs.

This provides a text-based user interface for:
  - Managing contexts
  - Testing API calls
  - Exploring available options

Examples:
  minimax interactive
  minimax i
  minimax tui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive()
	},
}

func runInteractive() error {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    MiniMax CLI - Interactive Mode            ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Commands:                                                   ║")
	fmt.Println("║    help     - Show this help                                 ║")
	fmt.Println("║    ctx      - Show/switch context                            ║")
	fmt.Println("║    services - List available services                        ║")
	fmt.Println("║    test     - Run a test request                             ║")
	fmt.Println("║    quit     - Exit interactive mode                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Show prompt with current context
		ctx, _ := getContext()
		ctxName := "(none)"
		if ctx != nil {
			ctxName = ctx.Name
		}

		fmt.Printf("\033[36m[%s]\033[0m minimax> ", ctxName)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := parts[0]

		switch command {
		case "help", "h", "?":
			showInteractiveHelp()

		case "quit", "exit", "q":
			fmt.Println("Goodbye!")
			return nil

		case "ctx", "context":
			handleContextCommand(parts[1:])

		case "services", "svc":
			showServices()

		case "test":
			handleTestCommand(parts[1:])

		case "clear", "cls":
			fmt.Print("\033[H\033[2J")

		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
		}

		fmt.Println()
	}

	return scanner.Err()
}

func showInteractiveHelp() {
	fmt.Println(`
Available commands:

  Context Management:
    ctx                  Show current context
    ctx list             List all contexts
    ctx use <name>       Switch to a context
    ctx add <name>       Add a new context (will prompt for details)

  Services:
    services             List all available services
    test <service>       Run a quick test for a service

  General:
    help                 Show this help
    clear                Clear the screen
    quit                 Exit interactive mode

For full CLI usage, run: minimax --help`)
}

func handleContextCommand(args []string) {
	cfg := getConfig()

	if len(args) == 0 {
		// Show current context
		ctx, err := getContext()
		if err != nil {
			fmt.Printf("No context selected: %v\n", err)
			return
		}
		fmt.Printf("Current context: %s\n", ctx.Name)
		fmt.Printf("  API Key: %s\n", cli.MaskAPIKey(ctx.APIKey))
		if ctx.BaseURL != "" {
			fmt.Printf("  Base URL: %s\n", ctx.BaseURL)
		}
		if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
			fmt.Printf("  Default Model: %s\n", defaultModel)
		}
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "list", "ls":
		contexts := cfg.ListContexts()
		if len(contexts) == 0 {
			fmt.Println("No contexts configured. Use 'ctx add <name>' to add one.")
			return
		}
		fmt.Println("Available contexts:")
		for _, name := range contexts {
			marker := "  "
			if name == cfg.CurrentContext {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}

	case "use":
		if len(args) < 2 {
			fmt.Println("Usage: ctx use <name>")
			return
		}
		// Note: This doesn't actually switch the context for the current session
		// because contextName is set via flags. This is just informational.
		fmt.Printf("To use context '%s', run: minimax -c %s <command>\n", args[1], args[1])

	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: ctx add <name>")
			return
		}
		fmt.Printf("To add context '%s', run:\n", args[1])
		fmt.Printf("  minimax config add-context %s --api-key YOUR_API_KEY\n", args[1])

	default:
		fmt.Printf("Unknown context command: %s\n", subCmd)
	}
}

func showServices() {
	fmt.Println(`
Available Services:

  ┌─────────┬──────────────────────────────────────────────────┐
  │ Service │ Description                                      │
  ├─────────┼──────────────────────────────────────────────────┤
  │ text    │ Text generation (chat completions)               │
  │ speech  │ Speech synthesis (TTS)                           │
  │ video   │ Video generation (T2V, I2V)                      │
  │ image   │ Image generation                                 │
  │ music   │ Music generation with vocals                     │
  │ voice   │ Voice management (list, clone, design)           │
  │ file    │ File management (upload, download, list)         │
  └─────────┴──────────────────────────────────────────────────┘

Use 'minimax <service> --help' for service-specific commands.`)
}

func handleTestCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: test <service>")
		fmt.Println("Available services: text, speech, video, image, music, voice, file")
		return
	}

	service := args[0]
	ctx, err := getContext()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please select a context first: ctx use <name>")
		return
	}

	fmt.Printf("Testing %s service with context '%s'...\n", service, ctx.Name)
	fmt.Println("[Note: Actual API calls not implemented yet]")

	switch service {
	case "text":
		fmt.Println("Would test: minimax text chat -f test.yaml")
	case "speech":
		fmt.Println("Would test: minimax speech synthesize -f test.yaml")
	case "video":
		fmt.Println("Would test: minimax video t2v -f test.yaml")
	case "image":
		fmt.Println("Would test: minimax image generate -f test.yaml")
	case "music":
		fmt.Println("Would test: minimax music generate -f test.yaml")
	case "voice":
		fmt.Println("Would test: minimax voice list")
	case "file":
		fmt.Println("Would test: minimax file list")
	default:
		fmt.Printf("Unknown service: %s\n", service)
	}
}
