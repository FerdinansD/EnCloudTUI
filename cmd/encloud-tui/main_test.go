package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi/encloud-tui/internal/config"
)

func TestPrintVersion(t *testing.T) {
	originalVersion, originalCommit, originalDate := version, commit, date
	t.Cleanup(func() { version, commit, date = originalVersion, originalCommit, originalDate })
	version, commit, date = "v1.2.3", "abcdef0", "2026-08-08T00:00:00Z"

	var output bytes.Buffer
	if !printVersion([]string{"--version"}, &output) {
		t.Fatal("printVersion() = false, want true")
	}
	if got, want := output.String(), "encloud-tui v1.2.3\ncommit: abcdef0\ndate: 2026-08-08T00:00:00Z\n"; got != want {
		t.Fatalf("printVersion() output = %q, want %q", got, want)
	}
}

func TestPrintVersionIgnoresOtherArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"version"}, {"--version", "extra"}} {
		var output bytes.Buffer
		if printVersion(args, &output) {
			t.Fatalf("printVersion(%q) = true, want false", args)
		}
		if output.Len() != 0 {
			t.Fatalf("printVersion(%q) wrote %q", args, output.String())
		}
	}
}

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
