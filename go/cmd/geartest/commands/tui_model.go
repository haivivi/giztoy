package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/haivivi/giztoy/pkg/cli"
)

// TUIModel is the TUI model.
type TUIModel struct {
	sim *Simulator

	// Viewports for scrollable content
	leftViewport  viewport.Model
	rightViewport viewport.Model

	// Content buffers
	leftContent  []string // Sent data
	rightContent []string // Received data
	logContent   []string // System logs

	// Log writer for capturing log output
	logWriter *cli.LogWriter

	// UI
	styles cli.Styles
	width  int
	height int

	// Quit flag
	quitting bool
}

// NewTUIModel creates a new TUI model.
func NewTUIModel(sim *Simulator, logWriter *cli.LogWriter) TUIModel {
	return TUIModel{
		sim:          sim,
		leftContent:  []string{},
		rightContent: []string{},
		logContent:   []string{},
		logWriter:    logWriter,
		styles:       cli.NewStyles(cli.DefaultTheme),
	}
}

// SimulatorEventMsg wraps simulator events for bubbletea.
type SimulatorEventMsg SimulatorEvent

// LogMsg wraps log messages for bubbletea.
type LogMsg string

// TickMsg is sent periodically to update the UI.
type TickMsg time.Time

// Init initializes the model.
func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.listenSimulator(),
		m.listenLogs(),
		m.tick(),
	)
}

func (m TUIModel) listenLogs() tea.Cmd {
	if m.logWriter == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-m.logWriter.Channel()
		if !ok {
			return nil
		}
		return LogMsg(line)
	}
}

func (m TUIModel) listenSimulator() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.sim.Events()
		if !ok {
			return nil
		}
		return SimulatorEventMsg(event)
	}
}

func (m TUIModel) tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages.
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
				m.quitting = true
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewports()

	case SimulatorEventMsg:
		m.handleSimulatorEvent(SimulatorEvent(msg))
		cmds = append(cmds, m.listenSimulator())

	case LogMsg:
		m.logContent = append(m.logContent, string(msg))
		if len(m.logContent) > 50 {
			m.logContent = m.logContent[len(m.logContent)-50:]
		}
		cmds = append(cmds, m.listenLogs())

	case TickMsg:
		cmds = append(cmds, m.tick())
	}

	return m, tea.Batch(cmds...)
}

func (m *TUIModel) updateViewports() {
	colWidth := (m.width - 6) / 2
	panelHeight := (m.height - 10) / 2

	m.leftViewport = viewport.New(colWidth, panelHeight)
	m.rightViewport = viewport.New(colWidth, panelHeight)

	m.leftViewport.SetContent(strings.Join(m.leftContent, "\n"))
	m.rightViewport.SetContent(strings.Join(m.rightContent, "\n"))
}

func (m *TUIModel) addLeft(s string) {
	ts := time.Now().Format("15:04:05")
	m.leftContent = append(m.leftContent, fmt.Sprintf("[%s] %s", ts, s))
	if len(m.leftContent) > 50 {
		m.leftContent = m.leftContent[len(m.leftContent)-50:]
	}
	m.leftViewport.SetContent(strings.Join(m.leftContent, "\n"))
	m.leftViewport.GotoBottom()
}

func (m *TUIModel) addRight(s string) {
	ts := time.Now().Format("15:04:05")
	m.rightContent = append(m.rightContent, fmt.Sprintf("[%s] %s", ts, s))
	if len(m.rightContent) > 50 {
		m.rightContent = m.rightContent[len(m.rightContent)-50:]
	}
	m.rightViewport.SetContent(strings.Join(m.rightContent, "\n"))
	m.rightViewport.GotoBottom()
}

func (m *TUIModel) handleSimulatorEvent(e SimulatorEvent) {
	switch e.Type {
	case "state_sent":
		m.addLeft(fmt.Sprintf("state: %s", e.Data))
	case "stats_sent":
		m.addLeft(fmt.Sprintf("stats: %s", e.Data))
	case "command_received":
		m.addRight(fmt.Sprintf("cmd: %s", e.Data))
	case "audio_out":
		m.addRight(fmt.Sprintf("audio: %s bytes", e.Data))
	case "error":
		m.addRight(fmt.Sprintf("ERR: %s", e.Data))
	default:
		m.addRight(fmt.Sprintf("%s: %s", e.Type, e.Data))
	}
}

// View renders the UI.
func (m TUIModel) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Get current state for status
	var stateStr string
	powerState := m.sim.PowerState()
	if powerState == PowerOn {
		stateStr = m.sim.State().String()
	} else {
		stateStr = powerState.String()
	}

	frame := cli.Frame{
		Styles: m.styles,
		Title:  "GEARTEST // SIMULATOR",
		Status: stateStr,
		Sections: []cli.Section{
			{Label: "📤 Sent", Content: func() []string { return m.leftContent }},
			{Label: "📥 Received", Content: func() []string { return m.rightContent }},
			{Label: "📋 System Log", Content: func() []string { return m.logContent }},
		},
		Help: "q/Ctrl+C=quit  |  All controls: http://localhost:8088",
	}

	return frame.Render(m.width, m.height)
}
