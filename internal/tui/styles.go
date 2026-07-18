package tui

import "charm.land/lipgloss/v2"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)
