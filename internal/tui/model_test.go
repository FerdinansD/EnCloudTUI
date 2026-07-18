package tui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

func TestWizardSavesAndMasksToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(path)
	if m.screen != wizard {
		t.Fatalf("screen = %v, want wizard", m.screen)
	}
	token := "12345678901234567890123456789012"
	m.inputs[0].SetValue("https://engram.example.com")
	m.inputs[1].SetValue(token)
	m.inputs[2].SetValue("alpha, beta")
	updated, _ := m.saveWizard()
	m = updated.(Model)
	if m.screen != dashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
	}
	if view := m.wizardView(); contains(view, token) {
		t.Fatal("wizard rendered token value")
	}
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationMarksOutstandingProjectsAndLogsCancellation(t *testing.T) {
	m := Model{
		cfg:      config.Config{Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}},
		selected: map[string]bool{"alpha": true, "beta": true},
		statuses: map[string]string{"alpha": "Running", "beta": "Queued"},
		screen:   running,
	}
	updated, _ := m.applyEvent(engram.Event{Done: true, Err: context.Canceled})
	m = updated.(Model)
	if m.message != "Operation cancelled" {
		t.Fatalf("message = %q, want cancellation", m.message)
	}
	if m.statuses["alpha"] != "Cancelled" || m.statuses["beta"] != "Cancelled" {
		t.Fatalf("statuses = %#v, want cancelled projects", m.statuses)
	}
	if len(m.logs) != 1 || m.logs[0] != "system: Operation cancelled" {
		t.Fatalf("logs = %#v, want cancellation log", m.logs)
	}
}

func TestDashboardProjectSelectionAndConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	m = updated.(Model)
	if m.screen != confirm || m.pending != "pull" {
		t.Fatal("pull key did not open confirmation")
	}
	// Independent selection is preserved while preparing an operation.
	m.selected["alpha"] = false
	if len(m.chosenProjects()) != 1 || m.chosenProjects()[0] != "beta" {
		t.Fatalf("selection not independent: %#v", m.chosenProjects())
	}
}

func TestUpdateRejectsEmptySelection(t *testing.T) {
	m := Model{
		screen:   confirm,
		pending:  engram.Pull,
		cfg:      config.Config{Projects: []string{"alpha"}},
		selected: map[string]bool{"alpha": false},
		statuses: map[string]string{"alpha": "Idle"},
	}
	updated, _ := m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != dashboard || m.message != "Select at least one project" {
		t.Fatalf("model = %#v, want dashboard empty-selection message", m)
	}
}

func TestUpdateRejectsConfirmation(t *testing.T) {
	m := Model{screen: confirm, pending: engram.Push}
	updated, _ := m.Update(keyMessage("n"))
	m = updated.(Model)
	if m.screen != dashboard || m.message != "Operation cancelled" {
		t.Fatalf("model = %#v, want cancelled confirmation", m)
	}
}

func TestUpdateRequestsRunningCancellation(t *testing.T) {
	cancelled := false
	m := Model{screen: running, cancel: func() { cancelled = true }}
	updated, _ := m.Update(keyMessage("esc"))
	m = updated.(Model)
	if !cancelled || m.message != "Cancelling operation..." || m.screen != running {
		t.Fatalf("model = %#v, cancelled = %v, want running cancellation request", m, cancelled)
	}
}

func TestUpdateHandlesTerminalFailure(t *testing.T) {
	m := Model{
		screen:   running,
		cfg:      config.Config{Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}},
		selected: map[string]bool{"alpha": true, "beta": true},
		statuses: map[string]string{"alpha": "Running", "beta": "Queued"},
		cancel:   func() {},
	}
	updated, _ := m.Update(operationEventMsg{event: engram.Event{Done: true, Err: errors.New("command failed")}})
	m = updated.(Model)
	if m.screen != dashboard || m.cancel != nil || m.message != "Operation failed: command failed" {
		t.Fatalf("model = %#v, want terminal failure", m)
	}
	if m.statuses["alpha"] != "Failed" || m.statuses["beta"] != "Failed" {
		t.Fatalf("statuses = %#v, want failed projects", m.statuses)
	}
}

func TestUpdateRedactsOutputBeforeRendering(t *testing.T) {
	token := "12345678901234567890123456789012"
	m := Model{
		screen:   running,
		cfg:      config.Config{Token: token},
		selected: map[string]bool{},
		statuses: map[string]string{},
		events:   make(chan engram.Event),
	}
	updated, _ := m.Update(operationEventMsg{event: engram.Event{Project: "alpha", Text: "token=" + token}})
	m = updated.(Model)
	if contains(m.logs[0], token) || contains(m.runningView(), token) {
		t.Fatalf("rendered output exposed token: logs = %#v", m.logs)
	}
	if m.logs[0] != "alpha: token=[redacted]" {
		t.Fatalf("logs = %#v, want redacted output", m.logs)
	}
}

func keyMessage(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: key})
}

func contains(value, part string) bool {
	return len(part) > 0 && len(value) >= len(part) && stringContains(value, part)
}

func stringContains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
