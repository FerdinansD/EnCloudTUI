package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SyncStatus string

const (
	SyncStatusUnknown      SyncStatus = "unknown"
	SyncStatusSynced       SyncStatus = "synced"
	SyncStatusPullRequired SyncStatus = "pull_required"
	SyncStatusPushRequired SyncStatus = "push_required"
	SyncStatusDiverged     SyncStatus = "diverged"
	SyncStatusError        SyncStatus = "error"
)

type ProjectSyncState struct {
	LastStatus    SyncStatus `json:"last_status"`
	LastCheckedAt string     `json:"last_checked_at"`
	LastOperation string     `json:"last_operation"`
	Summary       string     `json:"summary"`
}

type State struct {
	ConfigPath string                      `json:"config_path,omitempty"`
	Server     string                      `json:"server,omitempty"`
	Projects   map[string]ProjectSyncState `json:"projects"`
}

func cloneProjectSyncStates(projects map[string]ProjectSyncState) map[string]ProjectSyncState {
	cloned := make(map[string]ProjectSyncState, len(projects))
	for project, syncState := range projects {
		cloned[project] = syncState
	}
	return cloned
}

func DefaultStatePath(home string) string {
	return filepath.Join(home, ".config", "encloud-tui", "state.json")
}

func StatePath(configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		return ""
	}
	cleanPath := filepath.Clean(configPath)
	if filepath.Base(cleanPath) == "config.json" {
		return LegacyStatePath(cleanPath)
	}
	digest := sha256.Sum256([]byte(cleanPath))
	return filepath.Join(filepath.Dir(cleanPath), fmt.Sprintf(".encloud-state-%x.json", digest))
}

// LegacyStatePath returns the shared state path used before custom configurations were isolated.
func LegacyStatePath(configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		return ""
	}
	cleanPath := filepath.Clean(configPath)
	statePath := filepath.Join(filepath.Dir(cleanPath), "state.json")
	if statePathAliasesConfig(cleanPath, statePath) {
		return filepath.Join(filepath.Dir(cleanPath), ".encloud-state.json")
	}
	return statePath
}

func statePathAliasesConfig(configPath, statePath string) bool {
	if strings.EqualFold(filepath.Clean(configPath), filepath.Clean(statePath)) {
		return true
	}
	resolvedConfigPath, err := os.Stat(configPath)
	if err != nil {
		return false
	}
	resolvedStatePath, err := os.Stat(statePath)
	if err != nil {
		return false
	}
	return os.SameFile(resolvedConfigPath, resolvedStatePath)
}

func BoundState(configPath, server string) State {
	cleanPath := strings.TrimSpace(configPath)
	if cleanPath != "" {
		cleanPath = filepath.Clean(cleanPath)
	}
	return State{
		ConfigPath: cleanPath,
		Server:     strings.TrimSpace(server),
		Projects:   make(map[string]ProjectSyncState),
	}
}

func NormalizeState(state State, configPath, server string) (State, bool) {
	normalized := BoundState(configPath, server)
	if len(state.Projects) == 0 {
		return normalized, false
	}
	if strings.TrimSpace(state.ConfigPath) == "" && strings.TrimSpace(state.Server) == "" {
		return normalized, true
	}
	if state.matchesConfig(configPath, server) {
		normalized.Projects = cloneProjectSyncStates(state.Projects)
		return normalized, false
	}
	return normalized, true
}

func LoadState(path string) (State, error) {
	var state State
	if err := loadJSONFile(path, "sync state", &state); err != nil {
		return State{}, err
	}
	if state.Projects == nil {
		state.Projects = make(map[string]ProjectSyncState)
	}
	if err := state.Validate(); err != nil {
		return State{}, err
	}
	return state, nil
}

func SaveState(path string, state State) error {
	if state.Projects == nil {
		state.Projects = make(map[string]ProjectSyncState)
	}
	if err := state.Validate(); err != nil {
		return err
	}
	return saveJSONFile(path, state, ".state.json-*", "sync state")
}

func (s State) matchesConfig(configPath, server string) bool {
	cleanPath := strings.TrimSpace(configPath)
	if cleanPath != "" {
		cleanPath = filepath.Clean(cleanPath)
	}
	return s.ConfigPath == cleanPath && s.Server == strings.TrimSpace(server)
}

func (s State) Validate() error {
	for project, syncState := range s.Projects {
		if !projectName.MatchString(project) {
			return fmt.Errorf("invalid project name %q", project)
		}
		if err := syncState.Validate(); err != nil {
			return fmt.Errorf("project %q: %w", project, err)
		}
	}
	return nil
}

func (s ProjectSyncState) Validate() error {
	switch s.LastStatus {
	case SyncStatusUnknown, SyncStatusSynced, SyncStatusPullRequired, SyncStatusPushRequired, SyncStatusDiverged, SyncStatusError:
	default:
		return fmt.Errorf("invalid last status %q", s.LastStatus)
	}
	if s.LastCheckedAt == "" {
		return fmt.Errorf("last checked time is required")
	}
	if _, err := time.Parse(time.RFC3339, s.LastCheckedAt); err != nil {
		return fmt.Errorf("last checked time must be RFC3339: %w", err)
	}
	switch s.LastOperation {
	case "status", "pull", "push":
	default:
		return fmt.Errorf("invalid last operation %q", s.LastOperation)
	}
	if strings.TrimSpace(s.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}
