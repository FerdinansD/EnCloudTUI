package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case wizard:
		content = m.wizardView()
	case confirm:
		content = m.confirmView()
	case running:
		content = m.runningView()
	default:
		content = m.dashboardView()
	}
	return tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(content))
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
	var b strings.Builder
	b.WriteString(titleStyle.Render("EnCloud TUI Configuration"))
	b.WriteString("\n" + mutedStyle.Render("Credentials are stored locally with 0600 permissions. Token input is masked."))
	b.WriteString("\n\n")
	for _, input := range m.inputs {
		b.WriteString(input.View() + "\n\n")
	}
	if m.message != "" {
		b.WriteString(errorStyle.Render(m.message) + "\n\n")
	}
	b.WriteString(mutedStyle.Render("tab next field  enter save  esc cancel"))
	return b.String()
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
