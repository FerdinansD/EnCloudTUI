package tui

import (
	"context"
	"errors"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

type screen uint8

const (
	home screen = iota
	dashboard
	syncCenter
	wizard
	addProject
	confirm
	running
)

type operationEventMsg struct{ event engram.Event }

const (
	maxOperationLogs        = 300
	maxProjectOperationLogs = 100
)

type Model struct {
	screen               screen
	returnOrigins        []screen
	configPath           string
	syncStatePath        string
	storedCfg            config.Config
	cfg                  config.Config
	syncState            config.State
	syncStateWarning     string
	inputs               []textinput.Model
	focus                int
	selected             map[string]bool
	project              int
	homeMenu             int
	pending              engram.Mode
	activeProject        string
	statuses             map[string]string
	projectLogs          map[string][]string
	logs                 []string
	operationOutcome     string
	lastLogGroup         string
	message              string
	spinner              spinner.Model
	cancel               context.CancelFunc
	quitAfterOperation   bool
	events               <-chan engram.Event
	width                int
	height               int
	welcomeRevealStarted bool
	welcomeAnimating     bool
	welcomeReveal        int
}

func New(path string) Model {
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = config.DefaultPath(home)
		}
	}
	statePath := config.StatePath(path)
	storedCfg, err := config.Load(path)
	cfg := storedCfg
	storedState, stateErr := config.LoadState(statePath)
	if errors.Is(stateErr, os.ErrNotExist) && statePath != config.LegacyStatePath(path) {
		legacyState, legacyErr := config.LoadState(config.LegacyStatePath(path))
		if legacyErr == nil {
			storedState, _ = config.NormalizeState(legacyState, path, cfg.Server)
			stateErr = nil
		} else if !errors.Is(legacyErr, os.ErrNotExist) {
			stateErr = legacyErr
		}
	}
	if errors.Is(stateErr, os.ErrNotExist) {
		storedState = config.BoundState(path, cfg.Server)
		stateErr = nil
	} else if stateErr == nil {
		storedState, _ = config.NormalizeState(storedState, path, cfg.Server)
	}
	m := Model{
		configPath:    path,
		syncStatePath: statePath,
		storedCfg:     storedCfg,
		cfg:           cfg,
		syncState:     storedState,
		selected:      make(map[string]bool),
		statuses:      make(map[string]string),
		projectLogs:   make(map[string][]string),
		spinner:       spinner.New(),
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		m.message = "Configuration needs attention: " + err.Error()
	}
	if stateErr != nil {
		m.syncStateWarning = "Saved sync state needs attention: " + stateErr.Error()
	}
	if cfg.Validate() == nil {
		m.resetProjects()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) setupInputs(cfg config.Config) {
	m.inputs = []textinput.Model{textinput.New(), textinput.New(), textinput.New()}
	m.inputs[0].Placeholder = "https://engram.example.com"
	m.inputs[0].SetValue(cfg.Server)
	m.inputs[1].Placeholder = "Cloud access token"
	m.inputs[1].EchoMode = textinput.EchoPassword
	m.inputs[1].SetValue(cfg.Token)
	m.inputs[2].Placeholder = "project-a, project-b"
	m.inputs[2].SetValue(strings.Join(cfg.Projects, ", "))
	m.sizeWizardInputs()
	m.focus = 0
	m.inputs[0].Focus()
}

func (m *Model) setupProjectInput() {
	m.inputs = []textinput.Model{textinput.New()}
	m.inputs[0].Placeholder = "project-a"
	m.sizeWizardInputs()
	m.focus = 0
	m.inputs[0].Focus()
}

func (m *Model) sizeWizardInputs() {
	width := 64
	if m.width > 0 {
		width = min(64, max(4, m.width-13))
	}
	for index := range m.inputs {
		styles := textinput.DefaultStyles(true)
		styles.Focused.Text = successStyle
		styles.Focused.Placeholder = mutedStyle
		styles.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		styles.Blurred.Placeholder = mutedStyle
		styles.Cursor.Color = successGreen
		m.inputs[index].SetStyles(styles)
		m.inputs[index].SetWidth(width)
	}
}

func (m *Model) resetProjects() {
	m.selected = make(map[string]bool, len(m.cfg.Projects))
	m.statuses = make(map[string]string, len(m.cfg.Projects))
	for _, project := range m.cfg.Projects {
		m.selected[project] = false
		m.statuses[project] = "Idle"
	}
}

// openScreen records a concrete origin so Esc always follows the transition taken.
func (m *Model) openScreen(next screen) {
	m.returnOrigins = append(m.returnOrigins, m.screen)
	m.screen = next
}

func (m *Model) returnToOrigin() {
	if len(m.returnOrigins) == 0 {
		m.screen = home
		return
	}
	last := len(m.returnOrigins) - 1
	m.screen = m.returnOrigins[last]
	m.returnOrigins = m.returnOrigins[:last]
}

func (m Model) chosenProjects() []string {
	projects := make([]string, 0, len(m.cfg.Projects))
	for _, project := range m.cfg.Projects {
		if m.selected[project] {
			projects = append(projects, project)
		}
	}
	return projects
}

func splitProjects(value string) []string {
	parts := strings.Split(value, ",")
	projects := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			projects = append(projects, part)
		}
	}
	return projects
}

func redact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[redacted]")
}

func (m *Model) resetOperationView() {
	m.logs = nil
	m.message = ""
	m.operationOutcome = ""
	m.lastLogGroup = ""
	m.activeProject = ""
	m.projectLogs = make(map[string][]string)
}

func (m *Model) appendOperationLog(project, text string) {
	if text == "" {
		return
	}
	group := project
	if group == "" {
		group = "system"
	}
	if m.lastLogGroup != group {
		if len(m.logs) > 0 {
			m.logs = append(m.logs, "")
		}
		m.logs = append(m.logs, "---- "+group+" ----")
		m.lastLogGroup = group
	}
	text = redact(text, m.cfg.Token)
	m.logs = append(m.logs, text)
	if len(m.logs) > maxOperationLogs {
		m.logs = m.logs[len(m.logs)-maxOperationLogs:]
	}
	if project != "" {
		if m.projectLogs == nil {
			m.projectLogs = make(map[string][]string)
		}
		m.projectLogs[project] = appendBoundedProjectLog(m.projectLogs[project], text)
	}
}

func appendBoundedProjectLog(logs []string, text string) []string {
	logs = append(logs, text)
	if len(logs) <= maxProjectOperationLogs {
		return logs
	}
	for index, line := range logs {
		if !isSyncStateEvidence(line) {
			return append(logs[:index], logs[index+1:]...)
		}
	}
	for index, line := range logs {
		category := syncStateEvidenceCategory(line)
		for _, later := range logs[index+1:] {
			if syncStateEvidenceCategory(later) == category {
				return append(logs[:index], logs[index+1:]...)
			}
		}
	}
	return logs[len(logs)-maxProjectOperationLogs:]
}
