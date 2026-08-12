package tui

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

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
	case syncCenter:
		content = m.syncCenterView()
	case wizard:
		content = m.wizardView()
	case addProject:
		content = m.addProjectView()
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
	return []string{"Add project", "Sync center", "Edit configuration", "Exit"}
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
		nextStep = "Open Sync Center"
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

func (m Model) formPanel(content []string) string {
	width := 76
	if m.width > 0 {
		width = max(20, m.width-4)
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.Border{Top: "═", Bottom: "═", Left: "║", Right: "║", TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝"}).
		BorderForeground(successGreen).
		Padding(1, 2).
		Render(strings.Join(content, "\n"))
}

func (m Model) dashboardView() string {
	compact := m.width > 0 && m.width < 56
	content := []string{
		successStyle.Bold(true).Render("Project Operations"),
		"",
		successStyle.Bold(true).Render("Step 1 of 1"),
		mutedStyle.Render("Select projects, then run a safe sequential operation."),
		"",
		successStyle.Bold(true).Render(fmt.Sprintf("Projects (%d configured, %d selected)", len(m.cfg.Projects), len(m.chosenProjects()))),
		m.dashboardProjectRows(),
	}
	if m.message != "" {
		content = append(content, "", m.message)
	}
	content = append(content, "", shortcutFooter(compact,
		shortcutHint{"Esc", "Back"},
		shortcutHint{"Space", "Toggle current"},
		shortcutHint{"a", "Toggle all"},
		shortcutHint{"p", "Pull"},
		shortcutHint{"u", "Push"},
		shortcutHint{"s", "Status"},
		shortcutHint{"c", "Configure"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	))
	return m.formPanel(content)
}

func (m Model) dashboardProjectRows() string {
	if len(m.cfg.Projects) == 0 {
		return mutedStyle.Render("No projects configured. Return home to add one.")
	}
	lines := make([]string, 0, len(m.cfg.Projects)*3)
	const statusIndent = "      "
	for i, project := range m.cfg.Projects {
		marker := "[ ]"
		if m.selected[project] {
			marker = "[x]"
		}
		cursor := " "
		if i == m.project {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %s %s", cursor, marker, project)
		if i == m.project {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
		status := "Unknown"
		if projectState, ok := syncStatusDisplay(project, m.syncState); ok {
			status = syncStatusLabel(projectState.LastStatus)
		}
		statusLine := statusIndent + mutedStyle.Render("status: "+status)
		if i == m.project {
			statusLine = statusIndent + selectedStyle.Render("status: "+status)
		}
		lines = append(lines, statusLine)
		if i < len(m.cfg.Projects)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) syncCenterView() string {
	hints := []shortcutHint{
		{"Space", "Select"},
		{"s", "Status"},
		{"p", "Pull"},
		{"u", "Push"},
		{"a", "Add"},
		{"c", "Configure"},
		{"Esc", "Back"},
		{"q / Ctrl+C", "Quit"},
	}
	footerWidth := 66
	if m.width > 0 {
		footerWidth = m.width - 10
	}
	compact := footerWidth < shortcutFooterWidth(hints...)
	content := []string{
		successStyle.Bold(true).Render("Sync Center"),
		"",
		m.syncCenterMetadata(),
		"",
		m.syncCenterSections(),
	}
	if hint := m.syncCenterActionHint(); hint != "" {
		content = append(content, "", hint)
	}
	if m.message != "" {
		content = append(content, "", m.message)
	}
	content = append(content, "", shortcutFooter(compact, hints...))
	return m.formPanel(content)
}

func (m Model) syncCenterSections() string {
	availableWidth := 76
	if m.width > 0 {
		availableWidth = max(20, m.width-10)
	}
	return lipgloss.NewStyle().Width(availableWidth).Render(shellPanel("Projects", m.syncCenterConfiguredProjects(availableWidth)))
}

func (m Model) syncCenterConfiguredProjects(width int) string {
	if len(m.cfg.Projects) == 0 {
		return strings.Join([]string{
			"No projects configured.",
			"",
			"Press a to add your first project.",
		}, "\n")
	}

	contentWidth := max(8, width-4)
	gap, selectionWidth, projectWidth, statusWidth, lastSyncWidth := syncCenterTableLayout(contentWidth)
	pad := func(text string, width int) string {
		return fmt.Sprintf("%-*s", width, syncCenterEllipsize(text, width))
	}
	join := func(parts ...string) string {
		return strings.Join(parts, gap)
	}
	lines := []string{join(
		pad("Sel", selectionWidth),
		pad("Project", projectWidth),
		pad("Status", statusWidth),
		pad("Last sync", lastSyncWidth),
	)}
	lines = append(lines, mutedStyle.Render(join(
		strings.Repeat("-", selectionWidth),
		strings.Repeat("-", projectWidth),
		strings.Repeat("-", statusWidth),
		strings.Repeat("-", lastSyncWidth),
	)))
	for i, project := range m.cfg.Projects {
		marker := "[ ]"
		if m.selected[project] {
			marker = "[x]"
		}
		status := "Unknown"
		checked := "Never"
		if projectState, ok := syncStatusDisplay(project, m.syncState); ok {
			status = syncStatusLabel(projectState.LastStatus)
			checked = syncCenterCheckedAt(projectState.LastCheckedAt)
		}
		row := join(
			pad(marker, selectionWidth),
			pad(project, projectWidth),
			pad(status, statusWidth),
			pad(checked, lastSyncWidth),
		)
		if i == m.project {
			row = selectedStyle.Render(row)
		}
		lines = append(lines, row)
	}
	return lipgloss.NewStyle().Width(contentWidth).Render(strings.Join(lines, "\n"))
}

func syncCenterTableLayout(contentWidth int) (string, int, int, int, int) {
	const selectionWidth = 3
	if contentWidth >= 45 {
		statusWidth := 13
		lastSyncWidth := 16
		gap := "  "
		projectWidth := contentWidth - selectionWidth - statusWidth - lastSyncWidth - (len(gap) * 3)
		return gap, selectionWidth, projectWidth, statusWidth, lastSyncWidth
	}

	gap := " "
	gapWidth := len(gap) * 3
	if contentWidth < selectionWidth+3+gapWidth {
		gap = ""
		gapWidth = 0
	}

	remaining := max(3, contentWidth-selectionWidth-gapWidth)
	projectWidth, statusWidth, lastSyncWidth := 1, 1, 1

	grow := func(width *int, target int) {
		if remaining == 0 {
			return
		}
		need := target - *width
		if need <= 0 {
			return
		}
		addition := min(need, remaining)
		*width += addition
		remaining -= addition
	}

	remaining -= projectWidth + statusWidth + lastSyncWidth
	grow(&projectWidth, len("Project"))
	grow(&statusWidth, len("Status"))
	grow(&lastSyncWidth, len("Last"))
	projectWidth += remaining

	return gap, selectionWidth, projectWidth, statusWidth, lastSyncWidth
}

func (m Model) syncCenterMetadata() string {
	metadata := []string{fmt.Sprintf("%d project(s)", len(m.cfg.Projects))}
	if len(m.cfg.Projects) > 0 {
		metadata = append(metadata, fmt.Sprintf("%d selected", len(m.chosenProjects())))
		persistedCount := 0
		for _, project := range m.cfg.Projects {
			if _, ok := syncStatusDisplay(project, m.syncState); ok {
				persistedCount++
			}
		}
		if persistedCount == 0 {
			metadata = append(metadata, "No saved results")
		} else {
			metadata = append(metadata, fmt.Sprintf("%d saved result(s)", persistedCount))
		}
	}
	return mutedStyle.Render(strings.Join(metadata, "  |  "))
}

func (m Model) syncCenterActionHint() string {
	lines := []string{}
	if len(m.cfg.Projects) == 0 {
		lines = append(lines, mutedStyle.Render("Add a project to start tracking saved sync results."))
	} else if len(m.chosenProjects()) == 0 {
		lines = append(lines, mutedStyle.Render("Navigate rows and press Space to select projects."))
	} else {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d selected project(s) ready for Status, Pull, or Push.", len(m.chosenProjects()))))
	}
	if m.syncStateWarning != "" {
		lines = append(lines, mutedStyle.Render("Save warning: "+m.syncStateWarning))
	} else {
		lines = append(lines, mutedStyle.Render("Last sync values are local persisted snapshots only."))
	}
	return strings.Join(lines, "\n")
}

func syncCenterCheckedAt(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Never"
	}
	if timestamp, err := time.Parse(time.RFC3339, value); err == nil {
		return timestamp.Local().Format("2006-01-02 15:04")
	}
	return value
}

func syncCenterEllipsize(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	var builder strings.Builder
	currentWidth := 0
	for _, r := range text {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width-3 {
			break
		}
		builder.WriteRune(r)
		currentWidth += runeWidth
	}
	return builder.String() + "..."
}

func (m Model) wizardView() string {
	compact := m.width > 0 && m.width < 56
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
	return m.formPanel(content)
}

func (m Model) addProjectView() string {
	compact := m.width > 0 && m.width < 56
	content := []string{
		successStyle.Bold(true).Render("Add Project"),
		"",
		successStyle.Bold(true).Render("Step 1 of 1"),
		mutedStyle.Render("Project ID using letters, numbers, dots, underscores, or hyphens"),
		"",
		m.addProjectSections(),
	}
	if m.message != "" {
		content = append(content, "", errorStyle.Render(m.message))
	} else {
		content = append(content, "", mutedStyle.Render("Enter validates and saves the project."))
	}
	content = append(content, "", shortcutFooter(compact,
		shortcutHint{"Enter", "Add project"},
		shortcutHint{"Esc", "Discard & return"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	))
	return m.formPanel(content)
}

func (m Model) addProjectSections() string {
	availableWidth := 76
	if m.width > 0 {
		availableWidth = max(20, m.width-10)
	}
	if availableWidth >= 72 {
		leftWidth := max(24, (availableWidth-2)/2)
		rightWidth := max(24, availableWidth-2-leftWidth)
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.addProjectExistingPanel(leftWidth),
			m.addProjectInputPanel(rightWidth),
		)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.addProjectExistingPanel(availableWidth),
		"",
		m.addProjectInputPanel(availableWidth),
	)
}

func (m Model) addProjectExistingPanel(width int) string {
	return lipgloss.NewStyle().Width(width).Render(shellPanel("Configured Projects", m.addProjectExistingRows(width)))
}

func (m Model) addProjectExistingRows(width int) string {
	if len(m.cfg.Projects) == 0 {
		return mutedStyle.Render("No configured projects yet.")
	}
	lines := make([]string, 0, len(m.cfg.Projects)+1)
	lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d configured", len(m.cfg.Projects))))
	for _, project := range m.cfg.Projects {
		lines = append(lines, successStyle.Render("- ")+lipgloss.NewStyle().MaxWidth(max(8, width-8)).Render(project))
	}
	return strings.Join(lines, "\n")
}

func (m Model) addProjectInputPanel(width int) string {
	input := m.inputs[0]
	input.SetWidth(max(4, width-8))
	return lipgloss.NewStyle().Width(width).Render(shellPanel("New Project", successStyle.Bold(true).Render("Project")+"\n"+input.View()))
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
		shortcutHint{"Esc", "Discard & return"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	)
	return shortcutFooter(compact, hints...)
}

func operationLabel(mode string) string {
	if mode == "" {
		return "Pending"
	}
	return strings.ToUpper(mode[:1]) + mode[1:]
}

func operationProjectStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "":
		return "No status reported"
	case "Queued":
		return "Waiting to start"
	case "Running":
		return "In progress"
	case "Complete":
		return "Done"
	case "Failed":
		return "Needs attention"
	case "Cancelled":
		return "Cancelled"
	case "Skipped":
		return "Not run"
	case "Idle":
		return "Ready"
	default:
		return status
	}
}

func (m Model) operationLogView() string {
	if len(m.logs) == 0 {
		return mutedStyle.Render("Waiting for command output...")
	}
	limit := 12
	if m.height > 24 {
		limit = min(18, m.height-12)
	}
	start := max(0, len(m.logs)-limit)
	visible := m.logs[start:]
	if start > 0 && !strings.HasPrefix(m.logs[start], "---- ") {
		for index := start - 1; index >= 0; index-- {
			if strings.HasPrefix(m.logs[index], "---- ") {
				visible = append([]string{m.logs[index]}, m.logs[start+1:]...)
				break
			}
		}
	}
	return strings.Join(visible, "\n")
}

func (m Model) operationSummaryView() string {
	projects := m.chosenProjects()
	lines := []string{
		fmt.Sprintf("Operation: %s", operationLabel(string(m.pending))),
		fmt.Sprintf("Final status: %s", m.operationOutcome),
		fmt.Sprintf("Projects selected: %d", len(projects)),
	}
	if m.message != "" {
		lines = append(lines, "Details: "+m.message)
	}
	if len(projects) > 0 {
		lines = append(lines, "", successStyle.Bold(true).Render("Project results"))
		for _, project := range projects {
			lines = append(lines, fmt.Sprintf("- %s: %s", project, operationProjectStatusLabel(m.statuses[project])))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) confirmView() string {
	projects := m.chosenProjects()
	content := []string{
		successStyle.Bold(true).Render("Project Operations"),
		"",
		successStyle.Bold(true).Render("Confirm operation"),
		mutedStyle.Render("Review the queued operation before continuing."),
		"",
		successStyle.Bold(true).Render("Operation"),
		operationLabel(string(m.pending)),
		"",
		successStyle.Bold(true).Render("Selection"),
		fmt.Sprintf("%d project(s) selected", len(projects)),
	}
	if len(projects) > 0 {
		content = append(content,
			"",
			successStyle.Bold(true).Render("Projects"),
			strings.Join(projects, "\n"),
		)
	}
	content = append(content, "", shortcutFooter(m.width > 0 && m.width < 64,
		shortcutHint{"Enter / y", "Confirm"},
		shortcutHint{"Esc / n", "Cancel"},
		shortcutHint{"q / Ctrl+C", "Quit"},
	))
	return m.formPanel(content)
}

func (m Model) runningView() string {
	compact := m.width > 0 && m.width < 64
	if m.cancel == nil && m.operationOutcome != "" {
		content := []string{
			successStyle.Bold(true).Render("Project Operations"),
			"",
			successStyle.Bold(true).Render(operationLabel(string(m.pending)) + " complete"),
			mutedStyle.Render("Review the final status and recent activity before returning to the project list."),
			"",
			successStyle.Bold(true).Render("Overview"),
			m.operationSummaryView(),
			"",
			successStyle.Bold(true).Render("Recent activity"),
			mutedStyle.Render("Grouped output from the last part of the operation."),
			m.operationLogView(),
			"",
			shortcutFooter(compact,
				shortcutHint{"Esc", "Back"},
				shortcutHint{"q / Ctrl+C", "Quit"},
			),
		}
		return m.formPanel(content)
	}
	content := []string{
		successStyle.Bold(true).Render("Project Operations"),
		"",
		successStyle.Bold(true).Render("Running " + operationLabel(string(m.pending)) + " " + m.spinner.View()),
		mutedStyle.Render("Operations run sequentially. Esc cancels; q or Ctrl+C cancels and quits after completion."),
		"",
		successStyle.Bold(true).Render("Progress"),
		m.operationLogView(),
		"",
		shortcutFooter(compact,
			shortcutHint{"Esc", "Cancel operation"},
			shortcutHint{"q / Ctrl+C", "Cancel and quit"},
		),
	}
	return m.formPanel(content)
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

func shortcutFooterWidth(hints ...shortcutHint) int {
	width := 0
	for index, hint := range hints {
		if index > 0 {
			width += 5 // Width of the inline footer separator.
		}
		width += lipgloss.Width("[ " + hint.key + " ] " + hint.label)
	}
	return width
}
