package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

var saveSyncState = config.SaveState

func (m *Model) persistSyncState() {
	if m.syncStatePath == "" {
		return
	}
	candidate, _ := config.NormalizeState(m.syncState, m.configPath, m.cfg.Server)
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	updated := false
	for _, project := range m.chosenProjects() {
		syncState, ok := classifyProjectSyncState(m.pending, m.statuses[project], m.projectLogs[project])
		if !ok {
			continue
		}
		syncState.LastCheckedAt = checkedAt
		syncState.LastOperation = string(m.pending)
		candidate.Projects[project] = syncState
		updated = true
	}
	if !updated {
		return
	}
	if err := saveSyncState(m.syncStatePath, candidate); err != nil {
		if config.SaveCommitted(err) {
			m.syncState = candidate
			m.reportSyncStateDirectorySyncWarning()
			return
		}
		m.reportSyncStateWarning(err)
		return
	}
	m.syncState = candidate
	m.syncStateWarning = ""
}

func (m *Model) reportSyncStateWarning(err error) {
	m.syncStateWarning = "Cannot save sync state: " + err.Error()
	m.reportSyncStateWarningMessage()
}

func (m *Model) reportSyncStateDirectorySyncWarning() {
	m.syncStateWarning = "Sync state saved, but directory sync could not be confirmed"
	m.reportSyncStateWarningMessage()
}

func (m *Model) reportSyncStateWarningMessage() {
	if m.message == "" {
		m.message = m.syncStateWarning
		return
	}
	m.message += " " + m.syncStateWarning
}

func (m *Model) rebindSyncState(server string, persist bool) {
	state, _ := config.NormalizeState(m.syncState, m.configPath, server)
	m.syncState = state
	if !persist || m.syncStatePath == "" {
		return
	}
	if err := saveSyncState(m.syncStatePath, state); err != nil {
		if config.SaveCommitted(err) {
			m.reportSyncStateDirectorySyncWarning()
			return
		}
		m.reportSyncStateWarning(err)
		return
	}
	m.syncStateWarning = ""
}

func classifyProjectSyncState(mode engram.Mode, sessionStatus string, logs []string) (config.ProjectSyncState, bool) {
	logs = meaningfulProjectLogs(logs, mode)
	switch sessionStatus {
	case "Complete":
		switch mode {
		case engram.Pull:
			return config.ProjectSyncState{LastStatus: config.SyncStatusSynced, Summary: successSummary(mode, logs)}, true
		case engram.Push:
			return config.ProjectSyncState{LastStatus: config.SyncStatusSynced, Summary: successSummary(mode, logs)}, true
		case engram.Status:
			return classifyStatusLogs(logs)
		}
	case "Failed":
		return config.ProjectSyncState{LastStatus: config.SyncStatusError, Summary: failureSummary(mode, logs)}, true
	}
	return config.ProjectSyncState{}, false
}

func classifyStatusLogs(logs []string) (config.ProjectSyncState, bool) {
	if len(logs) == 0 {
		return config.ProjectSyncState{}, false
	}
	if state, ok := classifyCloudStatus(logs); ok {
		return state, true
	}
	matched := make([]string, 0, len(logs))
	for _, line := range logs {
		matched = append(matched, strings.ToLower(strings.TrimSpace(line)))
	}
	hasPull := containsPositiveAny(matched, "pull required", "needs pull", "behind remote", "behind cloud", "remote ahead", "import required")
	hasPush := containsPositiveAny(matched, "push required", "needs push", "ahead of remote", "ahead of cloud", "local ahead", "export required")
	if containsPositiveAny(matched, "diverged") || (hasPull && hasPush) {
		return config.ProjectSyncState{LastStatus: config.SyncStatusDiverged, Summary: firstMatchingSummary(logs, []string{"diverged"}, "Pull and push required; histories diverged")}, true
	}
	if hasPull {
		return config.ProjectSyncState{LastStatus: config.SyncStatusPullRequired, Summary: firstMatchingSummary(logs, []string{"pull required", "needs pull", "behind", "remote ahead", "import required"}, "Pull required before local changes are current")}, true
	}
	if hasPush {
		return config.ProjectSyncState{LastStatus: config.SyncStatusPushRequired, Summary: firstMatchingSummary(logs, []string{"push required", "needs push", "ahead", "export required"}, "Push required to publish local changes")}, true
	}
	if containsPositiveAny(matched, "up to date", "up-to-date", "in sync", "synced", "nothing to sync", "no synchronization needed", "no sync needed") {
		return config.ProjectSyncState{LastStatus: config.SyncStatusSynced, Summary: firstMatchingSummary(logs, []string{"up to date", "up-to-date", "in sync", "synced", "nothing to sync", "no synchronization needed", "no sync needed"}, "No synchronization needed")}, true
	}
	return config.ProjectSyncState{}, false
}

func classifyCloudStatus(logs []string) (config.ProjectSyncState, bool) {
	local, hasLocal := cloudStatusCount(logs, "local chunks")
	remote, hasRemote := cloudStatusCount(logs, "remote chunks")
	pending, hasPending := cloudStatusCount(logs, "pending import")
	if !containsPositiveAny([]string{strings.ToLower(strings.Join(logs, "\n"))}, "cloud sync status") || !hasLocal || !hasRemote || !hasPending {
		return config.ProjectSyncState{}, false
	}
	if pending > 0 || local < remote {
		return config.ProjectSyncState{LastStatus: config.SyncStatusPullRequired, Summary: "Pending import from cloud"}, true
	}
	if local > remote {
		return config.ProjectSyncState{LastStatus: config.SyncStatusPushRequired, Summary: "Local chunks pending export"}, true
	}
	return config.ProjectSyncState{LastStatus: config.SyncStatusSynced, Summary: "No synchronization needed"}, true
}

func cloudStatusCount(logs []string, label string) (int, bool) {
	for _, line := range logs {
		key, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), label) {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(value))
		return count, err == nil && count >= 0
	}
	return 0, false
}

func isSyncStateEvidence(line string) bool {
	return syncStateEvidenceCategory(line) != ""
}

func syncStateEvidenceCategory(line string) string {
	line = strings.ToLower(strings.TrimSpace(line))
	if containsPositiveAny([]string{line}, "diverged") {
		return "diverged"
	}
	if containsPositiveAny([]string{line}, "pull required", "needs pull", "behind remote", "behind cloud", "remote ahead", "import required") {
		return "pull"
	}
	if containsPositiveAny([]string{line}, "push required", "needs push", "ahead of remote", "ahead of cloud", "local ahead", "export required") {
		return "push"
	}
	if containsPositiveAny([]string{line}, "up to date", "up-to-date", "in sync", "synced", "nothing to sync", "no synchronization needed", "no sync needed") {
		return "synced"
	}
	return ""
}

func meaningfulProjectLogs(logs []string, mode engram.Mode) []string {
	filtered := make([]string, 0, len(logs))
	suffix := ": " + string(mode)
	for _, line := range logs {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || strings.HasSuffix(lower, suffix) || lower == "enrollment skipped" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func successSummary(mode engram.Mode, logs []string) string {
	if len(logs) > 0 {
		return logs[len(logs)-1]
	}
	return fmt.Sprintf("%s completed successfully", operationLabel(string(mode)))
}

func failureSummary(mode engram.Mode, logs []string) string {
	if len(logs) > 0 {
		return logs[len(logs)-1]
	}
	return fmt.Sprintf("%s failed", operationLabel(string(mode)))
}

func firstMatchingSummary(lines, keywords []string, fallback string) string {
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		for _, keyword := range keywords {
			if containsPositiveAny([]string{lower}, keyword) {
				return line
			}
		}
	}
	return fallback
}

func containsPositiveAny(lines []string, keywords ...string) bool {
	for _, line := range lines {
		words := syncStatusWords(line)
		for _, keyword := range keywords {
			phrase := syncStatusWords(keyword)
			if containsPositivePhrase(words, phrase) {
				return true
			}
		}
	}
	return false
}

func syncStatusWords(input string) []string {
	return strings.FieldsFunc(input, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}

func containsPositivePhrase(words, phrase []string) bool {
	for start := 0; start+len(phrase) <= len(words); start++ {
		if !matchesPhrase(words[start:], phrase) || negatedPhrase(words, start) {
			continue
		}
		return true
	}
	return false
}

func matchesPhrase(words, phrase []string) bool {
	for index, word := range phrase {
		if words[index] != word {
			return false
		}
	}
	return true
}

func negatedPhrase(words []string, start int) bool {
	if start > 0 && (words[start-1] == "no" || words[start-1] == "not") {
		return true
	}
	if start > 1 && ((words[start-2] == "not" && words[start-1] == "currently") || (words[start-2] == "no" && words[start-1] == "longer")) {
		return true
	}
	return false
}

func syncStatusLabel(status config.SyncStatus) string {
	switch status {
	case config.SyncStatusSynced:
		return "Synced"
	case config.SyncStatusPullRequired:
		return "Pull required"
	case config.SyncStatusPushRequired:
		return "Push required"
	case config.SyncStatusDiverged:
		return "Diverged"
	case config.SyncStatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

func syncStatusDisplay(project string, state config.State) (config.ProjectSyncState, bool) {
	projectState, ok := state.Projects[project]
	return projectState, ok
}
