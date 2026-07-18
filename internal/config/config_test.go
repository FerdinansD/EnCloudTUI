package config

import (
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
	if got, want := LegacyPath(home), "/home/tester/.config/engram-tui/config.json"; got != want {
		t.Fatalf("LegacyPath() = %q, want %q", got, want)
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
