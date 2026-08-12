package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if printVersion(os.Args[1:], os.Stdout) {
		return
	}
	model := tui.New(ConfigPathFromEnvironment())
	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "encloud-tui:", err)
		os.Exit(1)
	}
}

func printVersion(args []string, output io.Writer) bool {
	if len(args) != 1 || args[0] != "--version" {
		return false
	}
	fmt.Fprintf(output, "encloud-tui %s\ncommit: %s\ndate: %s\n", version, commit, date)
	return true
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
