package tui

import (
	_ "embed"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	welcomeTitle = "EnCloud TUI"
)

var (
	welcomeLogoBaseGreen      = successGreen
	welcomeLogoShadowGreen    = lipgloss.Color("#00B873")
	welcomeLogoHighlightGreen = lipgloss.Color("#00A869")
)

//go:embed assets/welcome.ansi
var welcomeLogoANSI string

func embeddedWelcomeLogo() string {
	logo := welcomeLogoANSI

	// Normalize Windows line endings.
	logo = strings.ReplaceAll(logo, "\r\n", "\n")

	// Support files that contain escaped ANSI sequences instead
	// of literal ESC characters.
	logo = strings.ReplaceAll(logo, `\x1b`, "\x1b")
	logo = strings.ReplaceAll(logo, `\033`, "\x1b")
	logo = strings.ReplaceAll(logo, `\e`, "\x1b")

	// Only remove final line breaks.
	// Do not use TrimSpace because ANSI art may rely on spaces.
	return strings.TrimRight(logo, "\r\n")
}

func welcomeLogoLines() []string {
	return strings.Split(embeddedWelcomeLogo(), "\n")
}

func welcomeLogoWidth() int {
	return lipgloss.Width(embeddedWelcomeLogo())
}

func (m Model) welcomeCanAnimate() bool {
	logo := embeddedWelcomeLogo()
	return logo != "" && welcomeLogoWidth() <= m.width-4 && lipgloss.Height(logo) <= m.height-10
}

func (m Model) welcomeLogo() string {
	lines := welcomeLogoLines()
	revealed := welcomeLogoWidth()
	if m.welcomeAnimating {
		revealed = m.welcomeReveal
	}
	for row, line := range lines {
		lines[row] = renderWelcomeLogoLine(line, revealed)
	}
	return strings.Join(lines, "\n")
}

func renderWelcomeLogoLine(line string, revealed int) string {
	var rendered strings.Builder
	column := 0
	for _, glyph := range line {
		width := lipgloss.Width(string(glyph))
		if column+width > revealed {
			rendered.WriteString(strings.Repeat(" ", width))
			column += width
			continue
		}
		if glyph == ' ' {
			rendered.WriteRune(glyph)
		} else {
			rendered.WriteString(welcomeLogoGlyphStyle(glyph).Render(string(glyph)))
		}
		column += width
	}
	return rendered.String()
}

func welcomeLogoGlyphStyle(glyph rune) lipgloss.Style {
	switch glyph {
	case ' ':
		return lipgloss.NewStyle()
	case '▓':
		return lipgloss.NewStyle().Foreground(welcomeLogoShadowGreen)
	case '░':
		return lipgloss.NewStyle().Foreground(welcomeLogoHighlightGreen)
	default:
		return lipgloss.NewStyle().Foreground(welcomeLogoBaseGreen)
	}
}

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case home:
		content = m.homeView()
	case wizard:
		content = m.wizardView()
	case confirm:
		content = m.confirmView()
	case running:
		content = m.runningView()
	default:
		content = m.dashboardView()
	}
	padding := 2
	if m.width > 0 && m.width < 80 {
		padding = 0
	}
	return tea.NewView(lipgloss.NewStyle().Padding(1, padding).Render(content))
}

func (m Model) shellHeader() string {
	if m.welcomeCanAnimate() {
		return lipgloss.PlaceHorizontal(max(1, m.width-4), lipgloss.Center, m.welcomeLogo())
	}
	return successStyle.Bold(true).Render(welcomeTitle)
}

func (m Model) homeHeader() string {
	if m.welcomeCanAnimate() {
		return m.welcomeLogo()
	}
	return successStyle.Bold(true).Render(welcomeTitle)
}

func (m Model) homeMenuItems() []string {
	if m.cfg.Validate() != nil {
		return []string{"Initial configuration", "Exit"}
	}
	return []string{"Open workspace", "Add project", "Sync center", "Edit configuration", "Exit"}
}

func (m Model) homeView() string {
	content := strings.Join([]string{m.homeSummaryPanel(), m.homeMenuPanel()}, "\n")
	if m.message != "" {
		content += "\n\n" + m.message
	}
	compact := m.width > 0 && m.width < 80
	footer := shortcutFooter(compact,
		shortcutHint{"Up/Down", "Navigate"},
		shortcutHint{"Enter", "Select"},
		shortcutHint{"c", "Configure"},
		shortcutHint{"Esc", "Quit"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	)
	return strings.Join([]string{content, footer}, "\n\n")
}

func (m Model) homeSummaryPanel() string {
	availableWidth := 76
	if m.width > 0 {
		availableWidth = m.width
		if m.width >= 80 {
			availableWidth -= 4
		}
	}
	const horizontalPadding = 2
	contentLimit := max(1, availableWidth-2)
	textWidthLimit := max(1, contentLimit-horizontalPadding*2)
	status := "STATUS: SETUP REQUIRED"
	nextStep := "Run initial configuration"
	if m.cfg.Validate() == nil {
		status = "STATUS: SETUP COMPLETE"
		nextStep = "Open dashboard"
	}

	title := "☁"
	lines := strings.Split(homeSummaryTopLine(title, status, textWidthLimit), "\n")
	if m.welcomeCanAnimate() && welcomeLogoWidth() <= textWidthLimit {
		lines = append(lines, strings.Split(m.welcomeLogo(), "\n")...)
	}
	remote := "Not configured"
	if strings.TrimSpace(m.cfg.Server) != "" {
		remote = "Configured"
	}
	token := "Missing"
	if strings.TrimSpace(m.cfg.Token) != "" {
		token = "Present"
	}
	lines = append(lines,
		"Remote: "+remote,
		"Token: "+token,
		fmt.Sprintf("Projects: %d configured, %d selected", len(m.cfg.Projects), len(m.chosenProjects())),
		"Next step: "+nextStep,
	)
	lines = append(lines, "> encloud-tui — Cloud synchronization workspace")

	textWidth := 1
	for _, line := range lines {
		textWidth = max(textWidth, lipgloss.Width(line))
	}
	textWidth = min(textWidth, textWidthLimit)
	contentWidth := textWidth + horizontalPadding*2

	borderStyle := lipgloss.NewStyle().Foreground(successGreen)
	var panel strings.Builder
	panel.WriteString(borderStyle.Render("╔" + strings.Repeat("═", contentWidth) + "╗"))
	panel.WriteString("\n")
	panel.WriteString(borderStyle.Render("║"))
	panel.WriteString(strings.Repeat(" ", contentWidth))
	panel.WriteString(borderStyle.Render("║"))
	for _, line := range lines {
		for _, wrapped := range wrapHomeText(line, textWidth) {
			panel.WriteString("\n")
			panel.WriteString(borderStyle.Render("║"))
			panel.WriteString(strings.Repeat(" ", horizontalPadding))
			if strings.HasPrefix(wrapped, "> encloud-tui") {
				panel.WriteString(mutedStyle.Render(wrapped))
			} else {
				panel.WriteString(wrapped)
			}
			panel.WriteString(strings.Repeat(" ", max(0, textWidth-lipgloss.Width(wrapped))+horizontalPadding))
			panel.WriteString(borderStyle.Render("║"))
		}
	}
	panel.WriteString("\n")
	panel.WriteString(borderStyle.Render("║"))
	panel.WriteString(strings.Repeat(" ", contentWidth))
	panel.WriteString(borderStyle.Render("║"))
	panel.WriteString("\n")
	panel.WriteString(borderStyle.Render("╚" + strings.Repeat("═", contentWidth) + "╝"))
	return panel.String()
}

func homeSummaryTopLine(title, status string, width int) string {
	if lipgloss.Width(title)+lipgloss.Width(status)+1 > width {
		return title + "\n" + status
	}
	return title + " " + status
}

func wrapHomeText(text string, width int) []string {
	if lipgloss.Width(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	lines := make([]string, 0, len(words))
	line := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			for len(word) > 0 {
				part, rest := homeTextChunk(word, width)
				lines = append(lines, part)
				word = rest
			}
			continue
		}
		if line == "" {
			line = word
			continue
		}
		if lipgloss.Width(line)+1+lipgloss.Width(word) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func homeTextChunk(text string, width int) (string, string) {
	column := 0
	for index, glyph := range text {
		glyphWidth := lipgloss.Width(string(glyph))
		if column+glyphWidth > width {
			return text[:index], text[index:]
		}
		column += glyphWidth
	}
	return text, ""
}

func (m Model) homeMenuPanel() string {
	lines := make([]string, 0, len(m.homeMenuItems()))
	for index, item := range m.homeMenuItems() {
		line := "  " + item
		if index == m.homeMenu {
			line = successStyle.Bold(true).Render("> " + item)
		}
		lines = append(lines, line)
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(successStyle.Bold(true).Render("Main Menu") + "\n" + strings.Join(lines, "\n"))
}

func shellPanel(title, content string) string {
	border := lipgloss.Border{Top: "═", Bottom: "═", Left: "║", Right: "║", TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝"}
	return lipgloss.NewStyle().Border(border).BorderForeground(successGreen).Padding(0, 1).Render(successStyle.Bold(true).Render(title) + "\n" + content)
}

func (m Model) dashboardView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EnCloud TUI"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Select projects, then run a safe sequential operation."))
	b.WriteString("\n\n")
	for i, project := range m.cfg.Projects {
		marker := "[ ]"
		if m.selected[project] {
			marker = "[x]"
		}
		cursor := " "
		if i == m.project {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %s %-28s %s", cursor, marker, project, m.statuses[project])
		if i == m.project {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	if m.message != "" {
		b.WriteString("\n" + m.message + "\n")
	}
	b.WriteString("\n" + shortcutFooter(m.width > 0 && m.width < 80,
		shortcutHint{"Space", "Select"},
		shortcutHint{"a", "All"},
		shortcutHint{"p", "Pull"},
		shortcutHint{"u", "Push"},
		shortcutHint{"s", "Status"},
		shortcutHint{"c", "Configure"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	))
	return b.String()
}

func (m Model) wizardView() string {
	compact := m.width > 0 && m.width < 56
	width := 76
	if m.width > 0 {
		width = max(20, m.width-4)
	}
	field := m.wizardField()
	step := fmt.Sprintf("Step %d of 3", m.focus+1)
	content := []string{
		successStyle.Bold(true).Render("Initial Configuration"),
		"",
		successStyle.Bold(true).Render(step),
		mutedStyle.Render(field.help),
		"",
		successStyle.Bold(true).Render(field.label),
		m.inputs[m.focus].View(),
	}
	if status := m.wizardStatus(); status != "" {
		content = append(content, "", status)
	}
	content = append(content, "", m.wizardFooter(compact))
	panel := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.Border{Top: "═", Bottom: "═", Left: "║", Right: "║", TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝"}).
		BorderForeground(successGreen).
		Padding(1, 2).
		Render(strings.Join(content, "\n"))
	return panel
}

func (m Model) wizardField() struct{ label, help string } {
	fields := []struct{ label, help string }{
		{"Server URL", "HTTPS server URL, for example https://engram.example.com"},
		{"Cloud Token", "32 to 512 non-whitespace characters. Input is masked."},
		{"Projects", "Comma-separated project IDs, for example alpha, beta"},
	}
	if m.focus < 0 || m.focus >= len(fields) {
		return fields[0]
	}
	return fields[m.focus]
}

func (m Model) wizardStatus() string {
	if m.message != "" {
		return errorStyle.Render(m.message)
	}
	if m.focus == 2 {
		return mutedStyle.Render("Enter saves after validating the complete configuration.")
	}
	return mutedStyle.Render("Enter continues to the next step.")
}

func (m Model) wizardFooter(compact bool) string {
	hints := make([]shortcutHint, 0, 4)
	if m.focus > 0 {
		hints = append(hints, shortcutHint{"Shift+Tab", "Back"})
	}
	if m.focus == 0 {
		hints = append(hints, shortcutHint{"Enter", "Continue"})
	} else if m.focus == 2 {
		hints = append(hints, shortcutHint{"Enter", "Save configuration"})
	} else {
		hints = append(hints, shortcutHint{"Enter", "Continue"})
	}
	hints = append(hints,
		shortcutHint{"Esc", "Discard & return home"},
		shortcutHint{"Ctrl+C", "Quit"},
	)
	return shortcutFooter(compact, hints...)
}

func (m Model) confirmView() string {
	footer := shortcutFooter(m.width > 0 && m.width < 64,
		shortcutHint{"Enter / y", "Confirm"},
		shortcutHint{"Esc / n", "Cancel"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	)
	return titleStyle.Render("Confirm operation") + "\n\n" + fmt.Sprintf("Run %s for %d selected project(s)?\n\n", m.pending, len(m.chosenProjects())) + footer
}

func (m Model) runningView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EnCloud TUI") + " " + m.spinner.View() + "\n")
	b.WriteString(mutedStyle.Render("Operations run sequentially."))
	b.WriteString("\n\n")
	start := 0
	if len(m.logs) > 12 {
		start = len(m.logs) - 12
	}
	for _, line := range m.logs[start:] {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + shortcutFooter(m.width > 0 && m.width < 64,
		shortcutHint{"Esc", "Cancel operation"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	))
	return b.String()
}

type shortcutHint struct {
	key   string
	label string
}

func shortcutFooter(compact bool, hints ...shortcutHint) string {
	rendered := make([]string, len(hints))
	for index, hint := range hints {
		rendered[index] = shortcutStyle.Render("[ "+hint.key+" ]") + " " + mutedStyle.Render(hint.label)
	}
	separator := mutedStyle.Render("  ·  ")
	if compact {
		separator = "\n"
	}
	return strings.Join(rendered, separator)
}
