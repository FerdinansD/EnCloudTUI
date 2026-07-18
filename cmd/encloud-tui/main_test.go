package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi/encloud-tui/internal/config"
)

func TestConfigPathFromEnvironmentPrefersNewOverride(t *testing.T) {
	t.Setenv("ENCLOUD_TUI_CONFIG", "./testdata/../config.json")
	t.Setenv("ENGRAM_TUI_CONFIG", "./legacy.json")
	if got, want := ConfigPathFromEnvironment(), filepath.Clean("./testdata/../config.json"); got != want {
		t.Fatalf("ConfigPathFromEnvironment() = %q, want %q", got, want)
	}
}

func TestConfigPathFromEnvironmentUsesLegacyOverride(t *testing.T) {
	t.Setenv("ENCLOUD_TUI_CONFIG", "")
	t.Setenv("ENGRAM_TUI_CONFIG", "./testdata/../legacy.json")
	if got, want := ConfigPathFromEnvironment(), filepath.Clean("./testdata/../legacy.json"); got != want {
		t.Fatalf("ConfigPathFromEnvironment() = %q, want %q", got, want)
	}
}

func TestConfigPathFromEnvironmentUsesLegacyFileWhenNewFileIsAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ENCLOUD_TUI_CONFIG", "")
	t.Setenv("ENGRAM_TUI_CONFIG", "")
	legacyPath := config.LegacyPath(home)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := ConfigPathFromEnvironment(); got != legacyPath {
		t.Fatalf("ConfigPathFromEnvironment() = %q, want legacy path %q", got, legacyPath)
	}
}

func TestConfigPathFromEnvironmentUsesNewDefaultWhenNoConfigExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ENCLOUD_TUI_CONFIG", "")
	t.Setenv("ENGRAM_TUI_CONFIG", "")
	if got, want := ConfigPathFromEnvironment(), config.DefaultPath(home); got != want {
		t.Fatalf("ConfigPathFromEnvironment() = %q, want new default %q", got, want)
	}
}
