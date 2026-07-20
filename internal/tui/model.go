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
	wizard
	confirm
	running
)

type operationEventMsg struct{ event engram.Event }

type Model struct {
	screen               screen
	configPath           string
	storedCfg            config.Config
	cfg                  config.Config
	inputs               []textinput.Model
	focus                int
	selected             map[string]bool
	project              int
	homeMenu             int
	pending              engram.Mode
	statuses             map[string]string
	logs                 []string
	message              string
	spinner              spinner.Model
	cancel               context.CancelFunc
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
	storedCfg, err := config.Load(path)
	cfg := storedCfg
	m := Model{
		configPath: path,
		storedCfg:  storedCfg,
		cfg:        cfg,
		selected:   make(map[string]bool),
		statuses:   make(map[string]string),
		spinner:    spinner.New(),
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		m.message = "Configuration needs attention: " + err.Error()
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
		m.selected[project] = true
		m.statuses[project] = "Idle"
	}
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
