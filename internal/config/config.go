package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var projectName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var createTempFile = os.CreateTemp
var writeTempFile = func(file *os.File, data []byte) (int, error) { return file.Write(data) }

// Config holds the public Engram Cloud credential and projects managed by the application.
type Config struct {
	Server   string   `json:"server"`
	Token    string   `json:"token"` // Public Engram Cloud token.
	Projects []string `json:"projects"`
}

func DefaultPath(home string) string {
	return filepath.Join(home, ".config", "encloud-tui", "config.json")
}

func LegacyPath(home string) string {
	return filepath.Join(home, ".config", "engram-tui", "config.json")
}

func Load(path string) (Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Config{}, fmt.Errorf("inspect configuration: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		return Config{}, errors.New("configuration file permissions must be 0600")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create configuration directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect configuration directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	temp, err := createTempFile(dir, ".config.json-*")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary configuration: %w", err)
	}
	if _, err := writeTempFile(temp, append(data, '\n')); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary configuration: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary configuration: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace configuration: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open configuration directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync configuration directory: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	u, err := url.ParseRequestURI(c.Server)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("server must be an HTTPS URL without credentials, query, or fragment")
	}
	if len(c.Token) < 32 || len(c.Token) > 512 || strings.ContainsAny(c.Token, " \t\r\n") {
		return errors.New("token must contain 32 to 512 non-whitespace characters")
	}
	if len(c.Projects) == 0 {
		return errors.New("at least one project is required")
	}
	seen := make(map[string]struct{}, len(c.Projects))
	for _, project := range c.Projects {
		if !projectName.MatchString(project) {
			return fmt.Errorf("invalid project name %q", project)
		}
		if _, ok := seen[project]; ok {
			return fmt.Errorf("duplicate project %q", project)
		}
		seen[project] = struct{}{}
	}
	return nil
}
