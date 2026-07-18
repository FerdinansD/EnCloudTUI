package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/tui"
)

func main() {
	model := tui.New(ConfigPathFromEnvironment())
	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "encloud-tui:", err)
		os.Exit(1)
	}
}

func ConfigPathFromEnvironment() string {
	if path := os.Getenv("ENCLOUD_TUI_CONFIG"); path != "" {
		return filepath.Clean(path)
	}
	if path := os.Getenv("ENGRAM_TUI_CONFIG"); path != "" {
		return filepath.Clean(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := config.DefaultPath(home)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	legacyPath := config.LegacyPath(home)
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return path
}
