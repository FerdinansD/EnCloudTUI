package tui

import (
	"context"
	"errors"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case operationEventMsg:
		return m.applyEvent(msg.event)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	if m.screen == wizard {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.screen == running {
		if key == "ctrl+c" || key == "esc" {
			if m.cancel != nil {
				m.cancel()
				m.message = "Cancelling operation..."
			}
		}
		return m, nil
	}
	if m.screen == confirm {
		switch key {
		case "y", "enter":
			return m.startOperation()
		case "n", "esc":
			m.screen = dashboard
			m.message = "Operation cancelled"
		}
		return m, nil
	}
	if m.screen == wizard {
		switch key {
		case "ctrl+c", "esc":
			if m.cfg.Validate() == nil {
				m.screen = dashboard
				m.resetProjects()
			} else {
				return m, tea.Quit
			}
			return m, nil
		case "tab", "down":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus + 1) % len(m.inputs)
			return m.focusInput()
		case "shift+tab", "up":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus + len(m.inputs) - 1) % len(m.inputs)
			return m.focusInput()
		case "enter":
			if m.focus < len(m.inputs)-1 {
				m.inputs[m.focus].Blur()
				m.focus++
				return m.focusInput()
			}
			return m.saveWizard()
		}
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.project > 0 {
			m.project--
		}
	case "down", "j":
		if m.project < len(m.cfg.Projects)-1 {
			m.project++
		}
	case " ":
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
		m.pending, m.screen = engram.Pull, confirm
	case "u":
		m.pending, m.screen = engram.Push, confirm
	case "s":
		m.pending, m.screen = engram.Status, confirm
	case "c":
		m.screen = wizard
		m.setupInputs(m.storedCfg)
	}
	return m, nil
}

func (m Model) focusInput() (tea.Model, tea.Cmd) {
	return m, m.inputs[m.focus].Focus()
}

func (m Model) saveWizard() (tea.Model, tea.Cmd) {
	cfg := config.Config{Server: strings.TrimSpace(m.inputs[0].Value()), Token: strings.TrimSpace(m.inputs[1].Value()), Projects: splitProjects(m.inputs[2].Value())}
	if err := config.Save(m.configPath, cfg); err != nil {
		m.message = "Cannot save configuration: " + err.Error()
		return m, nil
	}
	m.storedCfg = cfg
	m.cfg = cfg
	m.resetProjects()
	m.screen = dashboard
	m.message = "Configuration saved with restricted permissions"
	return m, nil
}

func (m Model) startOperation() (tea.Model, tea.Cmd) {
	projects := m.chosenProjects()
	if len(projects) == 0 {
		m.screen = dashboard
		m.message = "Select at least one project"
		return m, nil
	}
	m.logs = nil
	for _, project := range projects {
		m.statuses[project] = "Queued"
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.events = (engram.Runner{}).Start(ctx, m.cfg, m.pending, projects)
	m.screen = running
	return m, tea.Batch(m.spinner.Tick, nextEvent(m.events))
}

func (m Model) applyEvent(event engram.Event) (tea.Model, tea.Cmd) {
	if event.Text != "" {
		prefix := event.Project
		if prefix == "" {
			prefix = "system"
		}
		m.logs = append(m.logs, prefix+": "+redact(event.Text, m.cfg.Token))
	}
	if event.Project != "" && event.Done == false {
		m.statuses[event.Project] = "Running"
	}
	if event.Done {
		m.cancel = nil
		m.screen = dashboard
		if errors.Is(event.Err, context.Canceled) {
			m.message = "Operation cancelled"
			m.logs = append(m.logs, "system: Operation cancelled")
			for _, project := range m.chosenProjects() {
				if m.statuses[project] == "Running" || m.statuses[project] == "Queued" {
					m.statuses[project] = "Cancelled"
				}
			}
		} else if event.Err != nil {
			m.message = "Operation failed: " + redact(event.Err.Error(), m.cfg.Token)
			for _, project := range m.chosenProjects() {
				if m.statuses[project] == "Running" || m.statuses[project] == "Queued" {
					m.statuses[project] = "Failed"
				}
			}
		} else {
			m.message = event.Text
			for _, project := range m.chosenProjects() {
				m.statuses[project] = "Complete"
			}
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
