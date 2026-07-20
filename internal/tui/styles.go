package tui

import "charm.land/lipgloss/v2"

var (
	successGreen  = lipgloss.Color("42")
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(successGreen)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	shortcutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
)
