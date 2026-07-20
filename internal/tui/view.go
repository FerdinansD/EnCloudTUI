package tui

import (
	_ "embed"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/piwi/encloud-tui/internal/config"
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
	if m.screen == home {
		return tea.NewView(lipgloss.NewStyle().Padding(1, padding).Render(content))
	}
	return tea.NewView(lipgloss.NewStyle().Padding(1, padding).Render(strings.Join([]string{m.shellHeader(), content}, "\n\n")))
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
	return []string{"Open dashboard", "Select sync projects", "Edit configuration", "Exit"}
}

func (m Model) homeView() string {
	menu := m.homeMenuPanel()
	status := m.setupStatusPanel()
	header := m.homeHeader()
	rightColumn := strings.Join([]string{status, menu}, "\n")
	compact := m.width > 0 && m.width <= lipgloss.Width(header)+2+lipgloss.Width(rightColumn)
	content := lipgloss.JoinHorizontal(lipgloss.Top, header, "  ", rightColumn)
	if compact {
		content = strings.Join([]string{header, status, menu}, "\n")
	}
	footer := "up/down or j/k navigate  enter select  c configuration  esc/q quit"
	if compact {
		footer = "up/down or j/k navigate\nenter select  c configuration\nesc/q quit"
	}
	return strings.Join([]string{content, mutedStyle.Render(footer)}, "\n\n")
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
	return shellPanel("Main Menu", strings.Join(lines, "\n"))
}

func (m Model) setupStatusPanel() string {
	configuration := "Invalid"
	if m.cfg.Validate() == nil {
		configuration = "Complete"
	}
	server := "Not configured"
	if strings.TrimSpace(m.cfg.Server) != "" {
		server = "Configured"
	}
	token := "Missing"
	if strings.TrimSpace(m.cfg.Token) != "" {
		token = "Present"
	}
	operation := "Idle"
	if m.screen == running {
		operation = "Running"
	} else if m.screen == confirm {
		operation = "Awaiting confirmation"
	}
	projects := fmt.Sprintf("Projects: %d configured, %d selected", len(m.cfg.Projects), len(m.chosenProjects()))
	if m.width > 0 && m.width < 76 {
		projects = fmt.Sprintf("Projects: %d, %d selected", len(m.cfg.Projects), len(m.chosenProjects()))
	}
	lines := []string{
		"Configuration: " + configuration,
		"Server: " + server,
		"Token: " + token,
		projects,
		"Operation: " + operation,
	}
	return shellPanel("Setup Status", strings.Join(lines, "\n"))
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
	b.WriteString("\n" + mutedStyle.Render("up/down navigate  space select  a all  p pull  u push  s status  c config  q quit"))
	return b.String()
}

func (m Model) wizardView() string {
	compact := m.width > 0 && m.width < 56
	panel := m.wizardPanel()
	width := lipgloss.Width(panel)

	heading := successStyle.Bold(true).Render("Initial Configuration")
	metadata := mutedStyle.Render("Step 1 of 2") + "\n" + successStyle.Render("Next: Sync dashboard")
	header := heading + "\n" + mutedStyle.Render("Initial configuration")
	if compact {
		header += "\n" + metadata
	} else {
		header = lipgloss.PlaceHorizontal(width, lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, header, strings.Repeat(" ", max(1, width-lipgloss.Width(heading)-lipgloss.Width(metadata))), metadata),
		)
	}

	intro := "Configure your cloud connection to continue.\nAfter setup, EnCloud TUI will open the full sync dashboard."
	if compact {
		intro = "Configure your cloud connection.\nAfter setup, the sync dashboard opens."
	}

	status := lipgloss.NewStyle().Width(width).Render(m.wizardStatus())
	separator := mutedStyle.Render(strings.Repeat("-", width))
	footer := m.wizardFooter(width, compact)
	bottom := lipgloss.NewStyle().Width(width).Render(mutedStyle.Render("Press Enter to save and start configuration."))
	return strings.Join([]string{header, mutedStyle.Render(intro), panel, status, separator, footer, bottom}, "\n\n")
}

func (m Model) wizardPanel() string {
	fields := []struct {
		label string
		help  string
	}{
		{"Server URL", "Cloud server base URL"},
		{"Cloud Token", "Your EnCloud API token"},
		{"Projects", "Comma-separated project IDs"},
	}
	border := lipgloss.Border{Top: "═", Bottom: "═", Left: "║", Right: "║", TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝"}
	rows := make([]string, 0, len(fields)+1)
	for index, field := range fields {
		label := mutedStyle.Render(field.label)
		borderColor := lipgloss.Color("240")
		if index == m.focus {
			label = successStyle.Bold(true).Render(field.label)
			borderColor = successGreen
		}
		input := ""
		if index < len(m.inputs) {
			input = m.inputs[index].View()
		}
		box := lipgloss.NewStyle().Border(border).BorderForeground(borderColor).Padding(0, 1).Render(input)
		rows = append(rows, label+"\n"+mutedStyle.Render(field.help)+"\n"+box)
	}
	preview := successStyle.Render("[x] Preview config") + "  " + mutedStyle.Render("Review values before saving.")
	if m.width > 0 && m.width < 56 {
		preview = successStyle.Render("[x] Preview config") + "\n" + mutedStyle.Render("Review values before saving.")
	}
	rows = append(rows, preview)
	return lipgloss.NewStyle().Border(border).BorderForeground(successGreen).Padding(1, 2).Render(strings.Join(rows, "\n\n"))
}

func (m Model) wizardStatus() string {
	if m.message != "" {
		return errorStyle.Render(m.message)
	}
	if len(m.inputs) != 3 {
		return mutedStyle.Render("Required before continuing")
	}
	cfg := config.Config{Server: strings.TrimSpace(m.inputs[0].Value()), Token: strings.TrimSpace(m.inputs[1].Value()), Projects: splitProjects(m.inputs[2].Value())}
	if cfg.Validate() != nil {
		return mutedStyle.Render("Required before continuing")
	}
	return successStyle.Render("Ready to save configuration")
}

func (m Model) wizardFooter(width int, compact bool) string {
	if compact {
		return mutedStyle.Render("tab Next field\nenter Save & continue\nesc Cancel")
	}
	hints := []string{"tab Next field", "enter Save & continue", "esc Cancel"}
	column := max(1, width/len(hints))
	for index, hint := range hints {
		hints[index] = lipgloss.PlaceHorizontal(column, lipgloss.Left, mutedStyle.Render(hint))
	}
	return strings.Join(hints, "")
}

func (m Model) confirmView() string {
	return titleStyle.Render("Confirm operation") + "\n\n" + fmt.Sprintf("Run %s for %d selected project(s)?\n\n", m.pending, len(m.chosenProjects())) + mutedStyle.Render("enter/y confirm  n/esc cancel")
}

func (m Model) runningView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EnCloud TUI") + " " + m.spinner.View() + "\n")
	b.WriteString(mutedStyle.Render("Operations run sequentially. Press esc or ctrl+c to cancel."))
	b.WriteString("\n\n")
	start := 0
	if len(m.logs) > 12 {
		start = len(m.logs) - 12
	}
	for _, line := range m.logs[start:] {
		b.WriteString(line + "\n")
	}
	return b.String()
}
