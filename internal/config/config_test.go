package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validConfig() Config {
	return Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}
}

func TestConfigurationPaths(t *testing.T) {
	home := "/home/tester"
	if got, want := DefaultPath(home), "/home/tester/.config/encloud-tui/config.json"; got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
	if got, want := DefaultStatePath(home), "/home/tester/.config/encloud-tui/state.json"; got != want {
		t.Fatalf("DefaultStatePath() = %q, want %q", got, want)
	}
	if got, want := LegacyPath(home), "/home/tester/.config/engram-tui/config.json"; got != want {
		t.Fatalf("LegacyPath() = %q, want %q", got, want)
	}
}

func TestStatePathUsesConfigurationDirectory(t *testing.T) {
	configPath := "/home/tester/.config/encloud-tui/config.json"
	if got, want := StatePath(configPath), "/home/tester/.config/encloud-tui/state.json"; got != want {
		t.Fatalf("StatePath() = %q, want %q", got, want)
	}
}

func TestStatePathKeysCustomConfigurationFile(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "personal.json")
	second := filepath.Join(dir, "work.json")
	firstStatePath := StatePath(first)
	secondStatePath := StatePath(second)

	if firstStatePath == secondStatePath {
		t.Fatalf("custom configurations shared state path %q", firstStatePath)
	}
	if filepath.Dir(firstStatePath) != dir || filepath.Dir(secondStatePath) != dir {
		t.Fatalf("state paths are not stored beside their configurations: %q, %q", firstStatePath, secondStatePath)
	}
}

func TestStatePathNeverAliasesConfigurationFile(t *testing.T) {
	configPath := "/home/tester/.config/encloud-tui/state.json"
	if got := StatePath(configPath); got == configPath {
		t.Fatalf("StatePath() aliased configuration path %q", got)
	}
}

func TestStatePathNeverAliasesCaseInsensitiveConfigurationFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"STATE.JSON", "State.Json", "sTaTe.JsOn"} {
		t.Run(name, func(t *testing.T) {
			configPath := filepath.Join(dir, name)
			if got := StatePath(configPath); got == configPath {
				t.Fatalf("StatePath() aliased configuration path %q", got)
			}
		})
	}
}

func TestStatePathNeverAliasesSymlinkedConfigurationFile(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(statePath, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(statePath, configPath); err != nil {
		t.Fatal(err)
	}
	if got := StatePath(configPath); got == statePath {
		t.Fatalf("StatePath() aliased symlink target %q", got)
	}
}

func TestCustomConfigurationSnapshotsRemainIsolated(t *testing.T) {
	dir := t.TempDir()
	firstConfigPath := filepath.Join(dir, "personal.json")
	secondConfigPath := filepath.Join(dir, "work.json")
	first := BoundState(firstConfigPath, "https://personal.example.com")
	first.Projects["alpha"] = ProjectSyncState{
		LastStatus:    SyncStatusSynced,
		LastCheckedAt: "2026-08-07T15:04:05Z",
		LastOperation: "status",
		Summary:       "Personal snapshot",
	}
	second := BoundState(secondConfigPath, "https://work.example.com")
	second.Projects["beta"] = ProjectSyncState{
		LastStatus:    SyncStatusPushRequired,
		LastCheckedAt: "2026-08-07T15:04:05Z",
		LastOperation: "status",
		Summary:       "Work snapshot",
	}

	if err := SaveState(StatePath(firstConfigPath), first); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(StatePath(secondConfigPath), second); err != nil {
		t.Fatal(err)
	}

	loadedFirst, err := LoadState(StatePath(firstConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	loadedSecond, err := LoadState(StatePath(secondConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	if got := loadedFirst.Projects["alpha"].Summary; got != "Personal snapshot" {
		t.Fatalf("first snapshot = %q, want personal snapshot", got)
	}
	if got := loadedSecond.Projects["beta"].Summary; got != "Work snapshot" {
		t.Fatalf("second snapshot = %q, want work snapshot", got)
	}
	if normalized, reset := NormalizeState(loadedFirst, secondConfigPath, "https://work.example.com"); !reset || len(normalized.Projects) != 0 {
		t.Fatalf("mismatched state binding was accepted: state=%#v reset=%v", normalized, reset)
	}
}

func TestNormalizeStateClearsProjectsWhenBindingChanges(t *testing.T) {
	state, reset := NormalizeState(State{
		ConfigPath: "/tmp/config-a.json",
		Server:     "https://engram-a.example.com",
		Projects: map[string]ProjectSyncState{
			"alpha": {
				LastStatus:    SyncStatusSynced,
				LastCheckedAt: "2026-08-07T15:04:05Z",
				LastOperation: "status",
				Summary:       "No synchronization needed",
			},
		},
	}, "/tmp/config-b.json", "https://engram-b.example.com")
	if !reset {
		t.Fatal("NormalizeState() did not report a binding reset")
	}
	if len(state.Projects) != 0 {
		t.Fatalf("NormalizeState() kept stale projects: %#v", state.Projects)
	}
	if state.ConfigPath != "/tmp/config-b.json" || state.Server != "https://engram-b.example.com" {
		t.Fatalf("NormalizeState() binding = %#v", state)
	}
}

func TestNormalizeStateClonesProjectsWhenBindingMatches(t *testing.T) {
	original := State{
		ConfigPath: "/tmp/config.json",
		Server:     "https://engram.example.com",
		Projects: map[string]ProjectSyncState{
			"alpha": {
				LastStatus:    SyncStatusSynced,
				LastCheckedAt: "2026-08-07T15:04:05Z",
				LastOperation: "status",
				Summary:       "No synchronization needed",
			},
		},
	}

	normalized, reset := NormalizeState(original, original.ConfigPath, original.Server)
	if reset {
		t.Fatal("NormalizeState() unexpectedly reported a binding reset")
	}

	updated := normalized.Projects["alpha"]
	updated.LastOperation = "pull"
	normalized.Projects["alpha"] = updated

	if got := original.Projects["alpha"].LastOperation; got != "status" {
		t.Fatalf("NormalizeState() aliased original projects map, last operation = %q", got)
	}
}

func TestSaveLeavesExistingDirectoryPermissionsUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := Save(path, validConfig()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0755 {
		t.Fatalf("existing directory permissions = %o, want 0755", got)
	}
}

func TestSaveReplacesPermissiveTargetWithRestrictedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old configuration"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, validConfig()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("replacement file permissions = %o, want 0600", got)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRetainsExistingConfigWhenTemporaryWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	previous := validConfig()
	if err := Save(path, previous); err != nil {
		t.Fatal(err)
	}
	previousData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeTempFile = func(*os.File, []byte) (int, error) { return 0, os.ErrInvalid }
	t.Cleanup(func() { writeTempFile = func(file *os.File, data []byte) (int, error) { return file.Write(data) } })

	err = Save(path, Config{Server: previous.Server, Token: "abcdef1234567890abcdef1234567890", Projects: []string{"replacement"}})
	if err == nil {
		t.Fatal("Save() succeeded after temporary file was closed")
	}
	if err == nil {
		t.Fatalf("Save() error = %v, want write failure", err)
	}
	actualData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualData) != string(previousData) {
		t.Fatal("existing configuration was changed after write failure")
	}
}

func TestSaveReportsCommittedReplacementWhenDirectorySyncFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	originalSyncDirectory := syncDirectory
	syncDirectory = func(string) error { return errors.New("directory unavailable") }
	t.Cleanup(func() { syncDirectory = originalSyncDirectory })

	err := Save(path, validConfig())
	if err == nil || !SaveCommitted(err) {
		t.Fatalf("Save() error = %v, want committed replacement error", err)
	}
	loaded, loadErr := Load(path)
	if loadErr != nil || loaded.Server != validConfig().Server {
		t.Fatalf("replacement was not available: config=%#v err=%v", loaded, loadErr)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"valid", validConfig(), true},
		{"http rejected", Config{Server: "http://engram.example.com", Token: validConfig().Token, Projects: []string{"alpha"}}, false},
		{"token whitespace rejected", Config{Server: validConfig().Server, Token: "1234567890123456789012345678901 ", Projects: []string{"alpha"}}, false},
		{"duplicate projects rejected", Config{Server: validConfig().Server, Token: validConfig().Token, Projects: []string{"alpha", "alpha"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Validate() == nil; got != tt.want {
				t.Fatalf("Validate() success = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveSecuresConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Save(path, validConfig()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file permissions = %o, want 0600", got)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dir.Mode().Perm(); got != 0700 {
		t.Fatalf("directory permissions = %o, want 0700", got)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server != validConfig().Server {
		t.Fatalf("loaded server = %q", loaded.Server)
	}
}

func TestSaveStateSecuresSyncState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	state := State{Projects: map[string]ProjectSyncState{
		"alpha": {
			LastStatus:    SyncStatusSynced,
			LastCheckedAt: "2026-08-07T15:04:05Z",
			LastOperation: "status",
			Summary:       "No synchronization needed",
		},
	}}
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("state file permissions = %o, want 0600", got)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Projects["alpha"].Summary; got != "No synchronization needed" {
		t.Fatalf("loaded summary = %q", got)
	}
}

func TestSaveStateReportsCommittedReplacementWhenDirectorySyncFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	originalSyncDirectory := syncDirectory
	syncDirectory = func(string) error { return errors.New("directory unavailable") }
	t.Cleanup(func() { syncDirectory = originalSyncDirectory })

	err := SaveState(path, State{Projects: map[string]ProjectSyncState{
		"alpha": {
			LastStatus:    SyncStatusSynced,
			LastCheckedAt: "2026-08-07T15:04:05Z",
			LastOperation: "status",
			Summary:       "No synchronization needed",
		},
	}})
	if err == nil || !SaveCommitted(err) {
		t.Fatalf("SaveState() error = %v, want committed replacement error", err)
	}
	if _, err := LoadState(path); err != nil {
		t.Fatalf("committed state was not available: %v", err)
	}
}
