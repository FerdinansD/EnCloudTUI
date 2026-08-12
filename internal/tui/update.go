package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if (m.screen == wizard || m.screen == addProject) && len(m.inputs) > 0 {
			m.sizeWizardInputs()
		}
		if !m.welcomeRevealStarted && m.welcomeCanAnimate() {
			m.welcomeRevealStarted = true
			m.welcomeAnimating = true
			return m, welcomeTick()
		}
		if m.welcomeAnimating && !m.welcomeCanAnimate() {
			m.welcomeAnimating = false
			m.welcomeReveal = welcomeLogoWidth()
		}
		return m, nil
	case welcomeTickMsg:
		if !m.welcomeAnimating {
			return m, nil
		}
		m.welcomeReveal++
		if m.welcomeReveal >= welcomeLogoWidth() {
			m.welcomeReveal = welcomeLogoWidth()
			m.welcomeAnimating = false
			return m, nil
		}
		return m, welcomeTick()
	case operationEventMsg:
		return m.applyEvent(msg.event)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	if m.screen == wizard || m.screen == addProject {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

type welcomeTickMsg struct{}

func welcomeTick() tea.Cmd {
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
		return welcomeTickMsg{}
	})
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.screen == running && m.cancel != nil {
		switch key {
		case "q", "ctrl+c":
			m.quitAfterOperation = true
			m.cancel()
			m.message = "Cancelling operation; EnCloud TUI will quit when it finishes."
			return m, nil
		case "esc":
			m.cancel()
			m.message = "Cancelling operation..."
			return m, nil
		}
		return m, nil
	}
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.screen == running {
		if key == "esc" {
			m.returnToOrigin()
			return m, nil
		}
		return m, nil
	}
	if m.screen == syncCenter {
		switch key {
		case "esc":
			m.returnToOrigin()
		case "up", "k":
			if m.project > 0 {
				m.project--
			}
		case "down", "j":
			if m.project < len(m.cfg.Projects)-1 {
				m.project++
			}
		case " ", "space":
			if len(m.cfg.Projects) > 0 {
				name := m.cfg.Projects[m.project]
				m.selected[name] = !m.selected[name]
				m.message = ""
			}
		case "p", "u", "s":
			var operation engram.Mode
			switch key {
			case "p":
				operation = engram.Pull
			case "u":
				operation = engram.Push
			case "s":
				operation = engram.Status
			}
			if len(m.chosenProjects()) == 0 {
				m.message = "Select at least one project before running " + operationLabel(string(operation))
				return m, nil
			}
			m.pending = operation
			m.openScreen(confirm)
		case "a":
			m.openScreen(addProject)
			m.setupProjectInput()
		case "c":
			m.openScreen(wizard)
			m.setupInputs(m.storedCfg)
		}
		return m, nil
	}
	if m.screen == confirm {
		switch key {
		case "y", "enter":
			return m.startOperation()
		case "n", "esc":
			m.returnToOrigin()
			m.message = "Operation cancelled"
		}
		return m, nil
	}
	if m.screen == wizard {
		switch key {
		case "esc":
			m.returnToOrigin()
			m.inputs = nil
			m.focus = 0
			m.message = ""
			return m, nil
		case "shift+tab":
			if m.focus == 0 {
				return m, nil
			}
			m.inputs[m.focus].Blur()
			m.focus--
			m.message = ""
			return m.focusInput()
		case "enter":
			if err := m.validateWizardStep(); err != nil {
				m.message = err.Error()
				return m, nil
			}
			if m.focus < len(m.inputs)-1 {
				m.inputs[m.focus].Blur()
				m.focus++
				m.message = ""
				return m.focusInput()
			}
			return m.saveWizard()
		}
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	if m.screen == addProject {
		switch key {
		case "esc":
			m.returnToOrigin()
			m.inputs = nil
			m.message = ""
			return m, nil
		case "enter":
			return m.saveProject()
		}
		var cmd tea.Cmd
		m.inputs[0], cmd = m.inputs[0].Update(msg)
		return m, cmd
	}
	if m.screen == home {
		if key == "esc" {
			return m, tea.Quit
		}
		if key == "c" {
			m.openScreen(wizard)
			m.setupInputs(m.storedCfg)
			return m, nil
		}
		items := m.homeMenuItems()
		switch key {
		case "up", "k":
			if m.homeMenu > 0 {
				m.homeMenu--
			}
		case "down", "j":
			if m.homeMenu < len(items)-1 {
				m.homeMenu++
			}
		case "enter":
			switch items[m.homeMenu] {
			case "Initial configuration", "Edit configuration":
				m.openScreen(wizard)
				m.setupInputs(m.storedCfg)
			case "Add project":
				m.openScreen(addProject)
				m.setupProjectInput()
			case "Sync center":
				m.openScreen(syncCenter)
			case "Exit":
				return m, tea.Quit
			}
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.returnToOrigin()
		return m, nil
	case "up", "k":
		if m.project > 0 {
			m.project--
		}
	case "down", "j":
		if m.project < len(m.cfg.Projects)-1 {
			m.project++
		}
	case " ", "space":
		if len(m.cfg.Projects) > 0 {
			name := m.cfg.Projects[m.project]
			m.selected[name] = !m.selected[name]
		}
	case "a":
		all := true
		for _, project := range m.cfg.Projects {
			all = all && m.selected[project]
		}
		for _, project := range m.cfg.Projects {
			m.selected[project] = !all
		}
	case "p":
		m.pending = engram.Pull
		m.openScreen(confirm)
	case "u":
		m.pending = engram.Push
		m.openScreen(confirm)
	case "s":
		m.pending = engram.Status
		m.openScreen(confirm)
	case "c":
		m.openScreen(wizard)
		m.setupInputs(m.storedCfg)
	}
	return m, nil
}

func (m Model) focusInput() (tea.Model, tea.Cmd) {
	return m, m.inputs[m.focus].Focus()
}

func (m Model) saveWizard() (tea.Model, tea.Cmd) {
	cfg := config.Config{Server: strings.TrimSpace(m.inputs[0].Value()), Token: m.inputs[1].Value(), Projects: splitProjects(m.inputs[2].Value())}
	err := config.Save(m.configPath, cfg)
	if err != nil && !config.SaveCommitted(err) {
		m.message = "Cannot save configuration: " + err.Error()
		return m, nil
	}
	m.storedCfg = cfg
	m.cfg = cfg
	m.resetProjects()
	m.returnToOrigin()
	m.message = "Configuration saved with restricted permissions"
	if err != nil {
		m.message += "; directory sync could not be confirmed"
	}
	m.rebindSyncState(cfg.Server, true)
	return m, nil
}

func (m Model) saveProject() (tea.Model, tea.Cmd) {
	if len(m.inputs) != 1 {
		m.message = "Project field is unavailable"
		return m, nil
	}
	cfg := m.storedCfg
	cfg.Projects = append(cfg.Projects, strings.TrimSpace(m.inputs[0].Value()))
	if err := cfg.Validate(); err != nil {
		m.message = err.Error()
		return m, nil
	}
	err := config.Save(m.configPath, cfg)
	if err != nil && !config.SaveCommitted(err) {
		m.message = "Cannot save project: " + err.Error()
		return m, nil
	}
	m.storedCfg = cfg
	m.cfg = cfg
	m.resetProjects()
	m.returnToOrigin()
	m.message = "Project added"
	if err != nil {
		m.message += "; directory sync could not be confirmed"
	}
	m.rebindSyncState(cfg.Server, true)
	return m, nil
}

func (m Model) validateWizardStep() error {
	if len(m.inputs) != 3 {
		return errors.New("configuration fields are unavailable")
	}
	cfg := config.Config{
		Server:   strings.TrimSpace(m.inputs[0].Value()),
		Token:    m.inputs[1].Value(),
		Projects: splitProjects(m.inputs[2].Value()),
	}
	switch m.focus {
	case 0:
		return config.Config{Server: cfg.Server, Token: strings.Repeat("x", 32), Projects: []string{"placeholder"}}.Validate()
	case 1:
		return config.Config{Server: "https://example.com", Token: cfg.Token, Projects: []string{"placeholder"}}.Validate()
	default:
		return cfg.Validate()
	}
}

func (m Model) startOperation() (tea.Model, tea.Cmd) {
	projects := m.chosenProjects()
	if len(projects) == 0 {
		m.returnToOrigin()
		m.message = "Select at least one project"
		return m, nil
	}
	m.resetOperationView()
	for _, project := range projects {
		m.statuses[project] = "Queued"
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.events = (engram.Runner{ConfigPath: m.configPath}).Start(ctx, m.cfg, m.pending, projects)
	m.screen = running
	return m, tea.Batch(m.spinner.Tick, nextEvent(m.events))
}

func (m Model) applyEvent(event engram.Event) (tea.Model, tea.Cmd) {
	if event.Project != "" && !event.Done && event.Text == event.Project+": "+string(m.pending) {
		if m.activeProject != "" && m.activeProject != event.Project && m.statuses[m.activeProject] == "Running" {
			m.statuses[m.activeProject] = "Complete"
		}
		m.activeProject = event.Project
	}
	if event.Text != "" && !event.Done {
		m.appendOperationLog(event.Project, event.Text)
	}
	if event.Project != "" && event.Done == false {
		m.statuses[event.Project] = "Running"
	}
	if event.Done {
		m.cancel = nil
		m.events = nil
		m.screen = running
		if errors.Is(event.Err, context.Canceled) {
			m.operationOutcome = "Cancelled"
			m.message = "Operation cancelled"
			m.appendOperationLog("", m.message)
			if m.activeProject != "" && m.statuses[m.activeProject] == "Running" {
				m.statuses[m.activeProject] = "Cancelled"
			}
			for _, project := range m.chosenProjects() {
				if m.statuses[project] == "Queued" {
					m.statuses[project] = "Cancelled"
				}
			}
		} else if event.Err != nil {
			m.operationOutcome = "Failed"
			m.message = "Operation failed: " + redact(event.Err.Error(), m.cfg.Token)
			m.appendOperationLog("", m.message)
			if m.activeProject != "" && len(m.projectLogs[m.activeProject]) > 0 {
				m.appendOperationLog(m.activeProject, m.message)
			}
			if m.activeProject != "" && m.statuses[m.activeProject] == "Running" {
				m.statuses[m.activeProject] = "Failed"
			}
			for _, project := range m.chosenProjects() {
				if m.statuses[project] == "Queued" {
					m.statuses[project] = "Skipped"
				}
			}
		} else {
			m.operationOutcome = "Completed"
			m.message = event.Text
			m.appendOperationLog("", m.message)
			if m.activeProject != "" && m.statuses[m.activeProject] == "Running" {
				m.statuses[m.activeProject] = "Complete"
			}
		}
		m.persistSyncState()
		if m.quitAfterOperation {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nextEvent(m.events)
}

func nextEvent(events <-chan engram.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return operationEventMsg{event: engram.Event{Done: true}}
		}
		return operationEventMsg{event: event}
	}
}
