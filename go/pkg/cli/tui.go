// Package cli provides terminal UI components for CLI applications.
package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme for the TUI.
type Theme struct {
	Primary lipgloss.Color // Main accent color
	Dim     lipgloss.Color // Dimmed/help text color
}

// DefaultTheme is the default bright green theme.
var DefaultTheme = Theme{
	Primary: lipgloss.Color("#00ff9f"),
	Dim:     lipgloss.Color("#6e7681"),
}

// Styles holds all styles derived from a theme.
type Styles struct {
	Title  lipgloss.Style
	Label  lipgloss.Style
	Border lipgloss.Style
	Help   lipgloss.Style
}

// NewStyles creates styles from a theme.
func NewStyles(t Theme) Styles {
	return Styles{
		Title:  lipgloss.NewStyle().Bold(true).Foreground(t.Primary).Padding(0, 1),
		Label:  lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		Border: lipgloss.NewStyle().Foreground(t.Primary),
		Help:   lipgloss.NewStyle().Foreground(t.Dim),
	}
}

// Section represents a labeled section with content.
type Section struct {
	Label   string
	Content func() []string // Dynamic content getter
}

// Frame renders a complete TUI frame with title, sections, and help text.
type Frame struct {
	Styles   Styles
	Title    string
	Status   string
	Sections []Section
	Help     string
}

// Render renders the frame to a string.
func (f Frame) Render(width, height int) string {
	if width == 0 || height == 0 {
		return "Loading..."
	}

	bc := f.Styles.Border
	maxContentWidth := width - 4

	var lines []string

	// Top border
	lines = append(lines, bc.Render("╭"+strings.Repeat("─", width-2)+"╮"))

	// Title line: │ title status    │
	// Width: │(1) + space(1) + title + space(1) + status + padding + space(1) + │(1) = width
	title := f.Styles.Title.Render(f.Title)
	status := f.Styles.Help.Render("[" + f.Status + "]")
	padding := max(0, width-5-lipgloss.Width(title)-lipgloss.Width(status))
	titleLine := bc.Render("│") + " " + title + " " + status +
		strings.Repeat(" ", padding) + " " + bc.Render("│")
	lines = append(lines, titleLine)

	// Empty line after title for spacing
	emptyLine := bc.Render("│") + strings.Repeat(" ", width-2) + bc.Render("│")
	lines = append(lines, emptyLine)

	// Calculate section heights
	numSections := len(f.Sections)
	if numSections == 0 {
		numSections = 1
	}
	// Available: total - top(1) - title(1) - empty(1) - sections*label(1) - bottom(1) - help(1)
	availableHeight := height - 5 - numSections
	sectionHeight := max(availableHeight/numSections, 2)

	// Render each section
	for _, sec := range f.Sections {
		lines = append(lines, f.renderSection(bc, sec.Label, sec.Content(), sectionHeight, width, maxContentWidth)...)
	}

	// Bottom border
	lines = append(lines, bc.Render("╰"+strings.Repeat("─", width-2)+"╯"))

	// Help line
	lines = append(lines, f.Styles.Help.Render(f.Help))

	return strings.Join(lines, "\n")
}

// renderSection renders a single section with embedded label.
func (f Frame) renderSection(bc lipgloss.Style, label string, content []string, height, width, maxContentWidth int) []string {
	var lines []string

	// Separator with embedded label: ├─📤 Label────────┤
	// Width: ├(1) + ─(1) + labelText(?) + ─...(padding) + ┤(1) = width
	labelText := f.Styles.Label.Render(label)
	padding := max(0, width-3-lipgloss.Width(labelText))
	labelSep := bc.Render("├") + bc.Render("─") + labelText +
		bc.Render(strings.Repeat("─", padding)) + bc.Render("┤")
	lines = append(lines, labelSep)

	// Content lines (show last N lines)
	startIdx := 0
	if len(content) > height {
		startIdx = len(content) - height
	}

	for i := 0; i < height; i++ {
		text := ""
		idx := startIdx + i
		if idx < len(content) {
			text = content[idx]
		}
		if maxContentWidth > 1 && lipgloss.Width(text) > maxContentWidth {
			text = truncateString(text, maxContentWidth-1) + "…"
		}
		line := bc.Render("│") + " " + text +
			strings.Repeat(" ", max(0, maxContentWidth-lipgloss.Width(text))) + " " + bc.Render("│")
		lines = append(lines, line)
	}

	return lines
}

// truncateString safely truncates a string to the given width,
// handling multi-byte characters correctly.
func truncateString(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	currentWidth := 0
	for i, r := range runes {
		w := lipgloss.Width(string(r))
		if currentWidth+w > width {
			return string(runes[:i])
		}
		currentWidth += w
	}
	return s
}
