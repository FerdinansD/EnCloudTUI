package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

func TestWizardAdvancesOneValidatedStepAtATimeAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(path)
	updated, _ := m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != wizard {
		t.Fatalf("screen = %v, want wizard", m.screen)
	}
	token := "12345678901234567890123456789012"
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.focus != 0 || !strings.Contains(m.message, "server must be an HTTPS URL") {
		t.Fatalf("invalid server advanced: step=%d message=%q", m.focus, m.message)
	}
	m.inputs[0].SetValue("https://engram.example.com")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.focus != 1 || m.message != "" {
		t.Fatalf("server did not advance to token: step=%d message=%q", m.focus, m.message)
	}
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.focus != 1 || !strings.Contains(m.message, "token must contain 32 to 512") {
		t.Fatalf("invalid token advanced: step=%d message=%q", m.focus, m.message)
	}
	m.inputs[1].SetValue(token)
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.focus != 2 || m.message != "" {
		t.Fatalf("token did not advance to projects: step=%d message=%q", m.focus, m.message)
	}
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.focus != 2 || !strings.Contains(m.message, "at least one project") {
		t.Fatalf("invalid projects saved: step=%d message=%q", m.focus, m.message)
	}
	m.inputs[2].SetValue("alpha, beta")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != home {
		t.Fatalf("screen = %v, want home", m.screen)
	}
	if view := m.wizardView(); contains(view, token) {
		t.Fatal("wizard rendered token value")
	}
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestWizardBackAndEscapeDiscardDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	if err := config.Save(path, original); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	updated, _ := m.Update(keyMessage("c"))
	m = updated.(Model)
	m.inputs[0].SetValue("https://edited.example.com")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("shift+tab"))
	m = updated.(Model)
	if m.focus != 0 {
		t.Fatalf("back step = %d, want 0", m.focus)
	}
	updated, _ = m.Update(keyMessage("esc"))
	m = updated.(Model)
	if m.screen != home || len(m.inputs) != 0 {
		t.Fatalf("escape did not return home and discard draft: %#v", m)
	}
	if !reflect.DeepEqual(m.cfg, original) || !reflect.DeepEqual(m.storedCfg, original) {
		t.Fatalf("escape changed in-memory configuration: cfg=%#v stored=%#v", m.cfg, m.storedCfg)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, original) {
		t.Fatalf("escape changed saved configuration: %#v", loaded)
	}
}

func TestWizardEditsExistingConfigurationOnlyAfterFinalSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	if err := config.Save(path, original); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	updated, _ := m.Update(keyMessage("c"))
	m = updated.(Model)
	m.inputs[0].SetValue("https://updated.example.com")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	m.inputs[1].SetValue("abcdefghijklmnopqrstuvwxyz123456")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	m.inputs[2].SetValue("beta, gamma")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != home {
		t.Fatalf("screen = %v, want home", m.screen)
	}
	want := config.Config{Server: "https://updated.example.com", Token: "abcdefghijklmnopqrstuvwxyz123456", Projects: []string{"beta", "gamma"}}
	if !reflect.DeepEqual(m.cfg, want) || !reflect.DeepEqual(m.storedCfg, want) {
		t.Fatalf("saved config = %#v, want %#v", m.cfg, want)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("persisted config = %#v, want %#v", loaded, want)
	}
}

func TestCancellationMarksOutstandingProjectsAndLogsCancellation(t *testing.T) {
	m := Model{
		cfg:           config.Config{Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}},
		selected:      map[string]bool{"alpha": true, "beta": true},
		statuses:      map[string]string{"alpha": "Running", "beta": "Queued"},
		activeProject: "alpha",
		screen:        running,
	}
	updated, _ := m.applyEvent(engram.Event{Done: true, Err: context.Canceled})
	m = updated.(Model)
	if m.message != "Operation cancelled" {
		t.Fatalf("message = %q, want cancellation", m.message)
	}
	if m.statuses["alpha"] != "Cancelled" || m.statuses["beta"] != "Cancelled" {
		t.Fatalf("statuses = %#v, want cancelled projects", m.statuses)
	}
	if !reflect.DeepEqual(m.logs, []string{"---- system ----", "Operation cancelled"}) {
		t.Fatalf("logs = %#v, want cancellation log", m.logs)
	}
}

func TestOperationLogsRemainBoundedAndPreserveSyncEvidence(t *testing.T) {
	m := Model{cfg: config.Config{Token: "12345678901234567890123456789012"}, projectLogs: make(map[string][]string)}
	m.appendOperationLog("alpha", "needs pull")
	m.appendOperationLog("alpha", "needs push")
	for index := 0; index < maxProjectOperationLogs+20; index++ {
		m.appendOperationLog("alpha", fmt.Sprintf("detail %d", index))
	}
	for index := 0; index < maxOperationLogs+20; index++ {
		m.appendOperationLog("beta", fmt.Sprintf("output %d", index))
	}
	if len(m.logs) > maxOperationLogs {
		t.Fatalf("global logs = %d, want at most %d", len(m.logs), maxOperationLogs)
	}
	if len(m.projectLogs["alpha"]) > maxProjectOperationLogs {
		t.Fatalf("project logs = %d, want at most %d", len(m.projectLogs["alpha"]), maxProjectOperationLogs)
	}
	state, ok := classifyProjectSyncState(engram.Status, "Complete", m.projectLogs["alpha"])
	if !ok || state.LastStatus != config.SyncStatusDiverged {
		t.Fatalf("sync state = %#v, want diverged", state)
	}
}

func TestClassifyStatusLogsDistinguishesPositiveAndNegatedPhrases(t *testing.T) {
	tests := []struct {
		name   string
		logs   []string
		want   config.SyncStatus
		wantOK bool
	}{
		{name: "pull required", logs: []string{"pull required"}, want: config.SyncStatusPullRequired, wantOK: true},
		{name: "push required", logs: []string{"push required"}, want: config.SyncStatusPushRequired, wantOK: true},
		{name: "pull and push required", logs: []string{"needs pull", "needs push"}, want: config.SyncStatusDiverged, wantOK: true},
		{name: "synced", logs: []string{"up to date"}, want: config.SyncStatusSynced, wantOK: true},
		{name: "in sync", logs: []string{"currently in sync"}, want: config.SyncStatusSynced, wantOK: true},
		{name: "Engram v1.20.0 cloud status is current", logs: []string{"Cloud sync status", "Local chunks: 42", "Remote chunks: 42", "Pending import: 0"}, want: config.SyncStatusSynced, wantOK: true},
		{name: "no pull or push required", logs: []string{"no pull required; no push required"}, wantOK: false},
		{name: "not synced", logs: []string{"not synced"}, wantOK: false},
		{name: "embedded unsynced", logs: []string{"unsynced"}, wantOK: false},
		{name: "not currently synced", logs: []string{"not currently synced"}, wantOK: false},
		{name: "not currently in sync", logs: []string{"not currently in sync"}, wantOK: false},
		{name: "no longer behind remote", logs: []string{"Project is no longer behind remote"}, wantOK: false},
		{name: "behind remote", logs: []string{"Project is behind remote"}, want: config.SyncStatusPullRequired, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, ok := classifyStatusLogs(tt.logs)
			if ok != tt.wantOK {
				t.Fatalf("classifyStatusLogs(%q) ok = %v, want %v; state = %#v", tt.logs, ok, tt.wantOK, state)
			}
			if ok && state.LastStatus != tt.want {
				t.Fatalf("classifyStatusLogs(%q) status = %q, want %q", tt.logs, state.LastStatus, tt.want)
			}
		})
	}
}

func TestOperationLogViewLimitsLargeGroup(t *testing.T) {
	m := Model{height: 24, logs: []string{"---- alpha ----"}}
	for index := 0; index < 100; index++ {
		m.logs = append(m.logs, fmt.Sprintf("line %d", index))
	}
	if lines := strings.Split(m.operationLogView(), "\n"); len(lines) > 12 {
		t.Fatalf("rendered lines = %d, want at most 12", len(lines))
	}
}

func TestSyncCenterProjectSelectionAndConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	updated, _ := m.Update(keyMessage("down"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != syncCenter || !reflect.DeepEqual(m.selected, map[string]bool{"alpha": false, "beta": false}) || !reflect.DeepEqual(m.statuses, map[string]string{"alpha": "Idle", "beta": "Idle"}) {
		t.Fatalf("open sync center did not preserve project state: %#v", m)
	}
	updated, _ = m.Update(keyMessage("esc"))
	m = updated.(Model)
	if m.screen != home {
		t.Fatalf("screen = %v, want home", m.screen)
	}
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != syncCenter {
		t.Fatalf("screen = %v, want sync center", m.screen)
	}
	updated, _ = m.Update(keyMessage("space"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("down"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("space"))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
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

func TestNewAlwaysStartsAtHome(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{filepath.Join(dir, "missing.json"), path} {
		m := New(path)
		if m.screen != home {
			t.Fatalf("New(%q) screen = %v, want home", path, m.screen)
		}
	}
}

func TestNewRestoresOnlyPersistedConfigurationAndSyncState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	state := config.BoundState(path, cfg.Server)
	state.Projects["alpha"] = config.ProjectSyncState{LastStatus: config.SyncStatusSynced, LastCheckedAt: "2026-08-08T12:00:00Z", LastOperation: "status", Summary: "No synchronization needed"}
	if err := config.SaveState(config.StatePath(path), state); err != nil {
		t.Fatal(err)
	}

	m := New(path)
	if m.screen != home || !reflect.DeepEqual(m.cfg, cfg) || !reflect.DeepEqual(m.syncState, state) {
		t.Fatalf("new model = %#v, want Home with persisted configuration and sync state", m)
	}
	if !reflect.DeepEqual(m.selected, map[string]bool{"alpha": false}) || !reflect.DeepEqual(m.statuses, map[string]string{"alpha": "Idle"}) || m.pending != "" || m.cancel != nil || len(m.logs) != 0 {
		t.Fatalf("new model retained transient TUI state: %#v", m)
	}
}

func TestHomeRoutesToSyncCenterAndConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	updated, _ := New(path).Update(keyMessage("down"))
	m := updated.(Model)
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != syncCenter {
		t.Fatalf("screen = %v, want sync center", m.screen)
	}
	updated, _ = New(path).Update(keyMessage("c"))
	m = updated.(Model)
	if m.screen != wizard {
		t.Fatalf("screen = %v, want wizard", m.screen)
	}
	if m.inputs[0].Value() != cfg.Server || m.inputs[1].Value() != cfg.Token || m.inputs[2].Value() != "alpha, beta" {
		t.Fatalf("inputs were not prefilled from saved config")
	}
}

func TestEscapeReturnsToExplicitTransitionOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	press := func(t *testing.T, m Model, key string) Model {
		t.Helper()
		updated, _ := m.Update(keyMessage(key))
		return updated.(Model)
	}

	t.Run("Home initiated add project returns Home", func(t *testing.T) {
		m := press(t, New(path), "enter")
		m = press(t, m, "esc")
		if m.screen != home {
			t.Fatalf("screen = %v, want home", m.screen)
		}
	})

	t.Run("Sync Center child screens return Sync Center", func(t *testing.T) {
		m := press(t, New(path), "down")
		m = press(t, m, "enter")
		m = press(t, m, "a")
		m = press(t, m, "esc")
		if m.screen != syncCenter {
			t.Fatalf("add project escape screen = %v, want sync center", m.screen)
		}

		m = press(t, m, "c")
		m = press(t, m, "esc")
		if m.screen != syncCenter {
			t.Fatalf("configuration escape screen = %v, want sync center", m.screen)
		}

		m = press(t, m, "space")
		m = press(t, m, "p")
		m = press(t, m, "esc")
		if m.screen != syncCenter {
			t.Fatalf("confirmation escape screen = %v, want sync center", m.screen)
		}

		m = press(t, m, "esc")
		if m.screen != home {
			t.Fatalf("sync center escape screen = %v, want home", m.screen)
		}
	})

	for _, outcome := range []string{"Completed", "Cancelled"} {
		t.Run("operation result "+outcome+" returns operation origin", func(t *testing.T) {
			m := Model{screen: running, operationOutcome: outcome, returnOrigins: []screen{home, syncCenter}}
			m = press(t, m, "esc")
			if m.screen != syncCenter {
				t.Fatalf("screen = %v, want sync center", m.screen)
			}
		})
	}
}

func TestWizardViewRendersFocusedFullScreenStepWithoutWelcomeLogo(t *testing.T) {
	m := Model{screen: wizard, width: 100, height: 32}
	m.setupInputs(config.Config{})
	if got := m.inputs[0].Styles().Focused.Text.GetForeground(); got != successGreen {
		t.Fatalf("focused input color = %v, want %v", got, successGreen)
	}
	view := stripANSI(m.wizardView())

	for _, text := range []string{
		"Initial Configuration", "Step 1 of 3", "Server URL", "HTTPS server URL",
		"Enter continues to the next step.", "[ Enter ]", "Continue", "[ Esc ]", "Discard & return", "[ q / Ctrl+C ]", "Quit",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("wizard view missing %q:\n%s", text, view)
		}
	}
	if strings.Contains(view, "Cloud Token") || strings.Contains(view, "Projects") || strings.Contains(view, "██") {
		t.Fatalf("wizard view exposed another step or the welcome logo:\n%s", view)
	}
	if strings.Count(view, "╔") != 1 || strings.Count(view, "╚") != 1 {
		t.Fatalf("wizard view = %q, want one double-line full-screen box", view)
	}
	if strings.Index(view, "Initial Configuration") < strings.Index(view, "╔") {
		t.Fatalf("wizard title rendered outside the panel:\n%s", view)
	}
	if !strings.Contains(m.wizardView(), successStyle.Bold(true).Render("Initial Configuration")) {
		t.Fatalf("wizard title does not use the green theme:\n%s", m.wizardView())
	}
	for _, line := range strings.Split(stripANSI(m.View().Content), "\n") {
		if strings.Contains(line, "╔") && lipgloss.Width(line) != m.width {
			t.Fatalf("full-screen box width = %d, want %d: %q", lipgloss.Width(line), m.width, line)
		}
	}
}

func TestWizardRendersEachFocusedStep(t *testing.T) {
	m := Model{screen: wizard, width: 100, height: 32}
	m.setupInputs(config.Config{})
	for _, tt := range []struct {
		step  int
		label string
	}{
		{0, "Server URL"},
		{1, "Cloud Token"},
		{2, "Projects"},
	} {
		t.Run(tt.label, func(t *testing.T) {
			m.focus = tt.step
			view := stripANSI(m.wizardView())
			if !strings.Contains(view, fmt.Sprintf("Step %d of 3", tt.step+1)) || !strings.Contains(view, tt.label) {
				t.Fatalf("step %d missing label or progress:\n%s", tt.step+1, view)
			}
		})
	}
}

func TestWizardViewKeepsErrorsAndFitsCompactTerminals(t *testing.T) {
	m := Model{screen: wizard, width: 40, height: 20, message: "token must contain 32 to 512 non-whitespace characters"}
	m.setupInputs(config.Config{})
	view := stripANSI(m.wizardView())
	if !strings.Contains(view, "token must contain 32 to 512") || !strings.Contains(view, "characters") {
		t.Fatalf("wizard view missing save error: %s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, m.width, line)
		}
	}
}

func TestHomeViewRendersCombinedHeaderAndTruthfulSetupStatus(t *testing.T) {
	validConfig := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	for _, tt := range []struct {
		name string
		cfg  config.Config
		want []string
	}{
		{
			name: "setup required",
			want: []string{"☁", "STATUS: SETUP REQUIRED", "Remote: Not configured", "Token: Missing", "Projects: 0 configured, 0 selected", "Next step: Run initial configuration"},
		},
		{
			name: "setup complete",
			cfg:  validConfig,
			want: []string{"☁", "STATUS: SETUP COMPLETE", "Remote: Configured", "Token: Present", "Projects: 1 configured, 1 selected", "Next step: Open Sync Center"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{screen: home, width: 120, height: 30, cfg: tt.cfg, selected: map[string]bool{"alpha": true}}
			view := stripANSI(m.View().Content)
			for _, text := range append(tt.want, "> encloud-tui — Cloud synchronization workspace", "Main Menu") {
				if !strings.Contains(view, text) {
					t.Fatalf("home view missing %q:\n%s", text, view)
				}
			}
			if strings.Contains(view, "Setup Status") || strings.Contains(view, "Operation:") {
				t.Fatalf("home view retained a separate status panel:\n%s", view)
			}

			lines := strings.Split(view, "\n")
			logoLine, footerLine, bottomLine, menuLine := -1, -1, -1, -1
			for index, line := range lines {
				if strings.Contains(line, "☁") && !strings.Contains(line, tt.want[1]) {
					t.Fatalf("top line must contain the setup status on the right:\n%s", view)
				}
				if strings.Contains(line, "██") && logoLine < 0 {
					logoLine = index
				}
				if strings.Contains(line, "> encloud-tui") {
					footerLine = index
				}
				if strings.Contains(line, "╚") && bottomLine < 0 {
					bottomLine = index
				}
				if strings.Contains(line, "Main Menu") {
					menuLine = index
				}
			}
			if logoLine < 0 || footerLine <= logoLine || bottomLine <= footerLine || menuLine <= bottomLine {
				t.Fatalf("combined panel must contain logo, footer, then precede Main Menu:\n%s", view)
			}
			if got := strings.Count(view, "╔"); got != 1 {
				t.Fatalf("bordered panels = %d, want combined header only:\n%s", got, view)
			}
			menuIndent := strings.Repeat(" ", 3) // Two columns from the view and one from the menu block.
			if !strings.HasPrefix(lines[menuLine], menuIndent+"Main Menu") || !strings.HasPrefix(lines[menuLine+1], menuIndent+"> "+m.homeMenuItems()[0]) {
				t.Fatalf("Main Menu heading or selected item spacing changed:\n%s", view)
			}
		})
	}
}

func TestHomeSummaryPanelUsesNaturalWidthAndPadding(t *testing.T) {
	m := Model{screen: home, width: 120, height: 30}
	panel := stripANSI(m.homeSummaryPanel())
	lines := strings.Split(panel, "\n")

	naturalTextWidth := max(welcomeLogoWidth(), lipgloss.Width("> encloud-tui — Cloud synchronization workspace"))
	if got, want := lipgloss.Width(panel), naturalTextWidth+6; got != want {
		t.Fatalf("panel width = %d, want natural content width plus border and padding %d", got, want)
	}
	if got := lipgloss.Width(panel); got >= m.width-4 {
		t.Fatalf("panel width = %d, want less than available width %d", got, m.width-4)
	}
	if got, want := lines[1], "║"+strings.Repeat(" ", naturalTextWidth+4)+"║"; got != want {
		t.Fatalf("top padding row = %q, want %q", got, want)
	}
	if !strings.HasPrefix(lines[2], "║  ☁") || !strings.HasSuffix(lines[len(lines)-2], "  ║") {
		t.Fatalf("panel content is not padded on both sides:\n%s", panel)
	}
}

func TestHomeViewFitsCompactTerminal(t *testing.T) {
	m := Model{screen: home, width: 40, height: 20}
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "☁") || !strings.Contains(view, "STATUS: SETUP REQUIRED") || !strings.Contains(view, "Main Menu") {
		t.Fatalf("compact home view is missing combined-panel content:\n%s", view)
	}
	if strings.Contains(view, "██") {
		t.Fatalf("compact home view rendered the full wordmark:\n%s", view)
	}
	if got := lipgloss.Width(stripANSI(m.homeSummaryPanel())); got > m.width {
		t.Fatalf("compact summary panel width = %d, exceeds terminal width %d", got, m.width)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, m.width, line)
		}
	}
}

func TestHomeMenuNavigationAndActionRouting(t *testing.T) {
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	m := Model{screen: home, cfg: cfg, storedCfg: cfg, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}
	if got, want := strings.Join(m.homeMenuItems(), "|"), "Add project|Sync center|Edit configuration|Exit"; got != want {
		t.Fatalf("home menu = %q, want %q", got, want)
	}

	for _, tt := range []struct {
		name    string
		index   int
		screen  screen
		message string
	}{
		{name: "add project", screen: addProject},
		{name: "sync center", index: 1, screen: syncCenter},
		{name: "edit configuration", index: 2, screen: wizard},
	} {
		t.Run(tt.name, func(t *testing.T) {
			menu := m
			menu.homeMenu = tt.index
			updated, _ := menu.Update(keyMessage("enter"))
			got := updated.(Model)
			if got.screen != tt.screen {
				t.Fatalf("screen = %v, want %v", got.screen, tt.screen)
			}
			if got.message != tt.message {
				t.Fatalf("message = %q, want %q", got.message, tt.message)
			}
			if tt.message != "" && !strings.Contains(stripANSI(got.View().Content), tt.message) {
				t.Fatalf("home view is missing %q", tt.message)
			}
			if tt.screen == wizard && (got.inputs[0].Value() != menu.cfg.Server || got.inputs[1].Value() != menu.cfg.Token || got.inputs[2].Value() != "alpha") {
				t.Fatalf("configuration wizard was not prefilled")
			}
			if tt.screen == addProject && (len(got.inputs) != 1 || got.inputs[0].Value() != "") {
				t.Fatalf("add project wizard was not initialized")
			}
		})
	}
}

func TestSyncCenterStatusShortcutUsesCurrentSelection(t *testing.T) {
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	m := Model{screen: syncCenter, cfg: cfg, storedCfg: cfg, selected: map[string]bool{"alpha": true}}

	updated, _ := m.Update(keyMessage("s"))
	got := updated.(Model)

	if got.screen != confirm {
		t.Fatalf("screen = %v, want confirm", got.screen)
	}
	if got.pending != engram.Status {
		t.Fatalf("pending = %q, want %q", got.pending, engram.Status)
	}
}

func TestSyncCenterStatusShortcutNeedsSelection(t *testing.T) {
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	m := Model{screen: syncCenter, cfg: cfg, storedCfg: cfg, selected: map[string]bool{"alpha": false}}

	updated, _ := m.Update(keyMessage("s"))
	got := updated.(Model)

	if got.screen != syncCenter {
		t.Fatalf("screen = %v, want sync center", got.screen)
	}
	if got.message != "Select at least one project before running Status" {
		t.Fatalf("message = %q", got.message)
	}
	if !strings.Contains(stripANSI(got.syncCenterView()), got.message) {
		t.Fatalf("sync center did not show selection feedback: %s", stripANSI(got.syncCenterView()))
	}
}

func TestSyncCenterSelectsFocusedProjectAndStartsOperations(t *testing.T) {
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}
	m := Model{screen: syncCenter, cfg: cfg, storedCfg: cfg, selected: map[string]bool{}}

	updated, _ := m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != syncCenter {
		t.Fatalf("Enter changed screen to %v, want sync center", m.screen)
	}
	updated, _ = m.Update(keyMessage("down"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("space"))
	m = updated.(Model)
	if m.project != 1 || !m.selected["beta"] || m.selected["alpha"] {
		t.Fatalf("selection = project:%d selected:%#v, want beta selected", m.project, m.selected)
	}

	for _, tt := range []struct {
		key  string
		want engram.Mode
	}{
		{key: "s", want: engram.Status},
		{key: "p", want: engram.Pull},
		{key: "u", want: engram.Push},
	} {
		t.Run(tt.key, func(t *testing.T) {
			updated, _ := m.Update(keyMessage(tt.key))
			got := updated.(Model)
			if got.screen != confirm || got.pending != tt.want {
				t.Fatalf("operation = screen:%v pending:%q, want confirm with %q", got.screen, got.pending, tt.want)
			}
		})
	}
}

func TestSyncCenterViewShowsSelectionControls(t *testing.T) {
	m := Model{screen: syncCenter, width: 90, cfg: config.Config{Projects: []string{"alpha", "beta"}}, selected: map[string]bool{}, project: 1}
	view := stripANSI(m.syncCenterView())

	for _, snippet := range []string{
		"Navigate rows and press Space to select projects.",
		"[ Space ]", "Select", "[ s ]", "Status", "[ p ]", "Pull", "[ u ]", "Push", "[ a ]", "Add", "[ c ]", "Configure", "[ Esc ]", "Back", "[ q / Ctrl+C ]", "Quit",
	} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("sync center view missing %q:\n%s", snippet, view)
		}
	}
	for _, snippet := range []string{"[ Up/Down ]", "Toggle current", "for selected"} {
		if strings.Contains(view, snippet) {
			t.Fatalf("sync center view retained removed footer hint %q:\n%s", snippet, view)
		}
	}
	if strings.Contains(view, "[ Space ] Select  ·  [ s ] Status") {
		t.Fatalf("sync center footer stayed inline when it did not fit:\n%s", view)
	}

	wide := Model{screen: syncCenter, width: 180, cfg: m.cfg, selected: map[string]bool{}, project: 1}
	if !strings.Contains(stripANSI(wide.syncCenterView()), "[ Space ] Select  ·  [ s ] Status  ·  [ p ] Pull  ·  [ u ] Push  ·  [ a ] Add  ·  [ c ] Configure  ·  [ Esc ] Back  ·  [ q / Ctrl+C ] Quit") {
		t.Fatalf("sync center footer did not stay on one row when it fits:\n%s", stripANSI(wide.syncCenterView()))
	}
	if strings.Contains(view, "Open project operations") || strings.Contains(view, "Selection is managed in Project Operations") {
		t.Fatalf("sync center retained the redundant selection flow:\n%s", view)
	}
}

func TestSyncCenterViewShowsFocusedSectionsAndEmptyStateGuidance(t *testing.T) {
	m := Model{screen: syncCenter, width: 90, cfg: config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012"}}

	view := stripANSI(m.syncCenterView())

	for _, snippet := range []string{
		"Projects",
		"0 project(s)",
		"No projects configured.",
		"Press a to add your first project.",
		"Add a project to start tracking saved sync results.",
		"Last sync values are local persisted snapshots only.",
	} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("sync center view missing %q:\n%s", snippet, view)
		}
	}
}

func TestAddProjectWizardValidatesPersistsAndCanDiscard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	if err := config.Save(path, original); err != nil {
		t.Fatal(err)
	}

	updated, _ := New(path).Update(keyMessage("enter"))
	m := updated.(Model)
	if m.screen != addProject {
		t.Fatalf("screen = %v, want add project", m.screen)
	}

	m.inputs[0].SetValue("alpha")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != addProject || !strings.Contains(m.message, "duplicate project") {
		t.Fatalf("duplicate project was accepted: screen=%v message=%q", m.screen, m.message)
	}

	m.inputs[0].SetValue("beta")
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	want := config.Config{Server: original.Server, Token: original.Token, Projects: []string{"alpha", "beta"}}
	if m.screen != home || !reflect.DeepEqual(m.cfg, want) || m.message != "Project added" {
		t.Fatalf("added project state = %#v, want home with %#v", m, want)
	}
	if !reflect.DeepEqual(m.selected, map[string]bool{"alpha": false, "beta": false}) || !reflect.DeepEqual(m.statuses, map[string]string{"alpha": "Idle", "beta": "Idle"}) {
		t.Fatalf("added project did not reset project state: selected=%#v statuses=%#v", m.selected, m.statuses)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("persisted config = %#v, want %#v", loaded, want)
	}

	updated, _ = New(path).Update(keyMessage("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	m.inputs[0].SetValue("gamma")
	updated, _ = m.Update(keyMessage("esc"))
	m = updated.(Model)
	if m.screen != home || len(m.inputs) != 0 || !reflect.DeepEqual(m.cfg, want) {
		t.Fatalf("discard changed state: %#v", m)
	}
	loaded, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("discard changed persisted config: %#v", loaded)
	}
}

func TestAddProjectViewRendersFocusedSingleStep(t *testing.T) {
	m := Model{screen: addProject, width: 100, height: 32}
	m.setupProjectInput()
	view := stripANSI(m.addProjectView())
	for _, text := range []string{"Add Project", "Step 1 of 1", "Project", "Enter validates and saves the project.", "[ Enter ]", "Add project", "[ Esc ]", "Discard & return", "[ q / Ctrl+C ]", "Quit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("add project view missing %q:\n%s", text, view)
		}
	}
	if strings.Contains(view, "Cloud Token") || strings.Contains(view, "Initial Configuration") {
		t.Fatalf("add project view exposed configuration fields:\n%s", view)
	}
}

func TestSubmenuViewsDoNotRenderWelcomeLogo(t *testing.T) {
	for _, current := range []screen{wizard, addProject, dashboard, syncCenter, confirm, running} {
		m := Model{screen: current, width: 100, height: 30}
		if current == wizard {
			m.setupInputs(config.Config{})
		}
		if current == addProject {
			m.setupProjectInput()
		}
		if view := stripANSI(m.View().Content); strings.Contains(view, "██") {
			t.Fatalf("screen %v rendered welcome logo:\n%s", current, view)
		}
	}
}

func TestWelcomeLogoUsesProvidedShadedWordmark(t *testing.T) {
	logoText := embeddedWelcomeLogo()
	lines := strings.Split(logoText, "\n")
	want := []string{
		"██▀██ ███▄██ ██▀██ ██    ██▀██ ██ ██ ██▀█▄",
		"██▄   ██ ▀██ ██    ██    ██ ██ ██ ██ ██ ██",
		"█▓░▄▄ █▓░ █▓ █▓░▄▄ █▓░▄▄ █▓░█▓ █▓░█▓ █▓░█▓",
		"▀▀▀▀▀ ▀▀  ▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀",
	}
	if len(lines) != len(want) {
		t.Fatalf("wordmark height = %d rows, want %d", len(lines), len(want))
	}
	for row := range want {
		if lines[row] != want[row] {
			t.Fatalf("wordmark row %d = %q, want %q", row, lines[row], want[row])
		}
	}
	if strings.HasPrefix(logoText, "\n") {
		t.Fatalf("wordmark = %q, want no leading blank line", logoText)
	}
	for _, tt := range []struct {
		glyph rune
		color any
	}{
		{glyph: '▓', color: lipgloss.Color("#00B873")},
		{glyph: '░', color: lipgloss.Color("#00A869")},
		{glyph: '█', color: successGreen},
		{glyph: '▒', color: successGreen},
		{glyph: '▌', color: successGreen},
		{glyph: '▐', color: successGreen},
	} {
		if got := welcomeLogoGlyphStyle(tt.glyph).GetForeground(); got != tt.color {
			t.Fatalf("glyph %q color = %v, want %v", tt.glyph, got, tt.color)
		}
	}
	if got := welcomeLogoGlyphStyle(' ').Render(" "); got != " " {
		t.Fatalf("space rendering = %q, want unstyled space", got)
	}
	if got := stripANSI((Model{}).welcomeLogo()); got != logoText {
		t.Fatalf("styled wordmark geometry = %q, want %q", got, logoText)
	}
}

func TestWelcomeRevealProgressesLeftToRight(t *testing.T) {
	m := Model{screen: home}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if !m.welcomeRevealStarted || !m.welcomeAnimating || m.welcomeReveal != 0 || cmd == nil {
		t.Fatalf("animation = started:%t active:%t reveal:%d cmd:%t, want active animation", m.welcomeRevealStarted, m.welcomeAnimating, m.welcomeReveal, cmd != nil)
	}
	if contains(m.shellHeader(), welcomeLogoLines()[0]) {
		t.Fatal("initial animation frame rendered logo content")
	}

	for column := 1; column <= welcomeLogoWidth(); column++ {
		updated, cmd = m.Update(welcomeTickMsg{})
		m = updated.(Model)
		if m.welcomeReveal != column {
			t.Fatalf("reveal after tick %d = %d, want %d", column, m.welcomeReveal, column)
		}
		if column < welcomeLogoWidth() && cmd == nil {
			t.Fatalf("tick %d did not schedule the next frame", column)
		}
		for row, line := range strings.Split(stripANSI(m.welcomeLogo()), "\n") {
			want := welcomeLogoLines()[row]
			gotGlyphs := []rune(line)
			wantGlyphs := []rune(want)
			revealed := min(column, len(wantGlyphs))
			if string(gotGlyphs[:revealed]) != string(wantGlyphs[:revealed]) || strings.TrimSpace(string(gotGlyphs[revealed:])) != "" {
				t.Fatalf("tick %d row %d = %q, want only the left %d columns revealed", column, row, line, column)
			}
		}
	}
	if m.welcomeAnimating || cmd != nil {
		t.Fatalf("completed animation = active:%t cmd:%t, want stopped", m.welcomeAnimating, cmd != nil)
	}
	if got := stripANSI(m.welcomeLogo()); got != embeddedWelcomeLogo() {
		t.Fatalf("completed wordmark geometry = %q, want %q", got, embeddedWelcomeLogo())
	}
}

func TestWelcomeRevealIsDisabledForCompactFallback(t *testing.T) {
	m := Model{screen: home}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	m = updated.(Model)
	if m.welcomeRevealStarted || m.welcomeAnimating || m.welcomeReveal != 0 || cmd != nil {
		t.Fatalf("compact animation = started:%t active:%t reveal:%d cmd:%t, want disabled", m.welcomeRevealStarted, m.welcomeAnimating, m.welcomeReveal, cmd != nil)
	}
	view := m.shellHeader()
	if !contains(view, "EnCloud TUI") || contains(view, "█") {
		t.Fatalf("compact welcome view = %q, want usable non-animated fallback", view)
	}
	updated, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if !m.welcomeRevealStarted || !m.welcomeAnimating || cmd == nil {
		t.Fatalf("suitable resize = started:%t active:%t cmd:%t, want animation", m.welcomeRevealStarted, m.welcomeAnimating, cmd != nil)
	}
}

func TestShellHeaderUsesCompactFallbackWhenConstrained(t *testing.T) {
	for _, tt := range []struct {
		name          string
		width, height int
	}{
		{name: "narrow", width: lipgloss.Width(embeddedWelcomeLogo()) - 1, height: 30},
		{name: "short", width: 120, height: 12},
		{name: "very narrow", width: 20, height: 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			view := (Model{screen: home, width: tt.width, height: tt.height}).shellHeader()
			if !contains(view, "EnCloud TUI") {
				t.Fatalf("compact header = %q, want title", view)
			}
			if contains(view, "█") {
				t.Fatalf("compact header = %q, want no decorative content", view)
			}
			for _, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > tt.width {
					t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, tt.width, line)
				}
			}
		})
	}
}

func TestAllNonRunningScreensQuitWithQOrCtrlC(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			for _, current := range []screen{home, dashboard, syncCenter, wizard, addProject, confirm, running} {
				m := Model{screen: current}
				_, cmd := m.Update(keyMessage(key))
				if cmd == nil {
					t.Fatalf("screen %v quit command is nil", current)
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatalf("screen %v command message = %T, want tea.QuitMsg", current, cmd())
				}
			}
		})
	}
}

func TestShortcutFootersUseContextualBadges(t *testing.T) {
	valid := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	for _, tt := range []struct {
		name string
		view string
		want []string
	}{
		{"home", Model{screen: home}.homeView(), []string{"[ Enter ] Select", "[ q / Ctrl+C ] Quit"}},
		{"dashboard", Model{screen: dashboard, cfg: valid, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}.dashboardView(), []string{"[ Esc ] Back", "[ Space ] Toggle current", "Ctrl+C ] Quit"}},
		{"sync center", Model{screen: syncCenter, cfg: valid, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}.syncCenterView(), []string{"[ Space ] Select", "[ s ] Status", "[ p ] Pull", "[ u ] Push", "[ a ] Add", "[ Esc ] Back", "[ q / Ctrl+C ] Quit"}},
		{"wizard", Model{screen: wizard, focus: 1}.wizardFooter(false), []string{"[ Shift+Tab ] Back", "[ Enter ] Continue", "[ Esc ] Discard & return", "[ q / Ctrl+C ] Quit"}},
		{"confirmation", Model{screen: confirm}.confirmView(), []string{"[ Enter / y ] Confirm", "[ Esc / n ] Cancel", "[ q / Ctrl+C ] Quit"}},
		{"running", Model{screen: running}.runningView(), []string{"[ Esc ] Cancel operation", "[ q / Ctrl+C ] Cancel and quit"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			view := stripANSI(tt.view)
			for _, text := range tt.want {
				if !strings.Contains(view, text) {
					t.Fatalf("footer missing %q:\n%s", text, view)
				}
			}
		})
	}
	if strings.Contains(stripANSI(Model{screen: dashboard, cfg: valid, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}.dashboardView()), "[ Up/Down ] Navigate") {
		t.Fatal("dashboard footer retained the navigation shortcut hint")
	}
	if got := shortcutStyle.GetForeground(); got == successGreen {
		t.Fatalf("shortcut color = %v, want a complementary accent", got)
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
	if m.screen != home || m.message != "Select at least one project" {
		t.Fatalf("model = %#v, want home empty-selection message", m)
	}
}

func TestUpdateRejectsConfirmation(t *testing.T) {
	m := Model{screen: confirm, pending: engram.Push}
	updated, _ := m.Update(keyMessage("n"))
	m = updated.(Model)
	if m.screen != home || m.message != "Operation cancelled" {
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

func TestUpdateDefersRunningQuitUntilTerminalEvent(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "q", key: keyMessage("q")},
		{name: "Ctrl+C", key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cancelled := false
			m := Model{screen: running, cancel: func() { cancelled = true }}

			updated, cmd := m.Update(tt.key)
			m = updated.(Model)
			if !cancelled || cmd != nil || !m.quitAfterOperation || m.screen != running {
				t.Fatalf("model = %#v, cancelled = %v, cmd = %v; want deferred quit request", m, cancelled, cmd)
			}
			if m.message != "Cancelling operation; EnCloud TUI will quit when it finishes." {
				t.Fatalf("message = %q, want deferred quit message", m.message)
			}

			updated, cmd = m.Update(operationEventMsg{event: engram.Event{Done: true, Err: context.Canceled}})
			m = updated.(Model)
			if cmd == nil || m.cancel != nil || m.operationOutcome != "Cancelled" {
				t.Fatalf("model = %#v, cmd = %v; want terminal cancellation and quit", m, cmd)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("terminal command = %T, want tea.QuitMsg", cmd())
			}
		})
	}
}

func TestUpdateHandlesTerminalFailure(t *testing.T) {
	m := Model{
		screen:        running,
		cfg:           config.Config{Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}},
		selected:      map[string]bool{"alpha": true, "beta": true},
		statuses:      map[string]string{"alpha": "Running", "beta": "Queued"},
		activeProject: "alpha",
		projectLogs:   map[string][]string{"alpha": {"sync failed"}},
		cancel:        func() {},
	}
	updated, _ := m.Update(operationEventMsg{event: engram.Event{Done: true, Err: errors.New("command failed")}})
	m = updated.(Model)
	if m.screen != running || m.cancel != nil || m.message != "Operation failed: command failed" {
		t.Fatalf("model = %#v, want terminal failure", m)
	}
	if m.statuses["alpha"] != "Failed" || m.statuses["beta"] != "Skipped" {
		t.Fatalf("statuses = %#v, want failed active project and skipped queued project", m.statuses)
	}
}

func TestSyncStateSaveFailureLeavesInMemoryStateUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	blockingPath := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockingPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	checkedAt := "2026-08-07T15:04:05Z"
	m := New(path)
	m.syncState = config.State{
		ConfigPath: path,
		Server:     m.cfg.Server,
		Projects: map[string]config.ProjectSyncState{
			"alpha": {
				LastStatus:    config.SyncStatusSynced,
				LastCheckedAt: checkedAt,
				LastOperation: "status",
				Summary:       "No synchronization needed",
			},
		},
	}
	m.syncStatePath = filepath.Join(blockingPath, "state.json")
	m.pending = engram.Pull
	m.selected = map[string]bool{"alpha": true}
	m.statuses = map[string]string{"alpha": "Complete"}
	m.projectLogs = map[string][]string{"alpha": {"Applied remote updates"}}

	m.persistSyncState()

	persisted := m.syncState.Projects["alpha"]
	if persisted.LastCheckedAt != checkedAt || persisted.LastOperation != "status" || persisted.Summary != "No synchronization needed" {
		t.Fatalf("in-memory sync state changed after save failure: %#v", persisted)
	}
	if !strings.Contains(m.syncStateWarning, "Cannot save sync state") {
		t.Fatalf("syncStateWarning = %q", m.syncStateWarning)
	}
}

func TestCommittedSyncStateSaveUpdatesInMemoryStateAndReportsDirectoryWarning(t *testing.T) {
	originalSaveSyncState := saveSyncState
	saveSyncState = func(string, config.State) error {
		return &config.CommittedSaveError{Err: errors.New("directory unavailable")}
	}
	t.Cleanup(func() { saveSyncState = originalSaveSyncState })

	m := Model{
		configPath:    "config.json",
		syncStatePath: "state.json",
		cfg:           config.Config{Server: "https://engram.example.com", Projects: []string{"alpha"}},
		syncState:     config.BoundState("config.json", "https://engram.example.com"),
		pending:       engram.Pull,
		selected:      map[string]bool{"alpha": true},
		statuses:      map[string]string{"alpha": "Complete"},
		projectLogs:   map[string][]string{"alpha": {"Applied remote updates"}},
	}

	m.persistSyncState()

	persisted := m.syncState.Projects["alpha"]
	if persisted.LastStatus != config.SyncStatusSynced || persisted.LastOperation != "pull" || persisted.Summary != "Applied remote updates" {
		t.Fatalf("in-memory sync state = %#v", persisted)
	}
	if m.syncStateWarning != "Sync state saved, but directory sync could not be confirmed" {
		t.Fatalf("syncStateWarning = %q", m.syncStateWarning)
	}
	if strings.Contains(m.message, "Cannot save sync state") {
		t.Fatalf("message treated committed save as a failure: %q", m.message)
	}
}

func TestFailedOperationWithoutProjectLogsPersistsErrorState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	m.pending = engram.Status
	m.selected = map[string]bool{"alpha": true}
	m.statuses = map[string]string{"alpha": "Running"}
	m.activeProject = "alpha"
	m.cancel = func() {}

	updated, _ := m.applyEvent(engram.Event{Done: true, Err: errors.New("command failed")})
	m = updated.(Model)

	state, err := config.LoadState(config.StatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	persisted := state.Projects["alpha"]
	if persisted.LastStatus != config.SyncStatusError || persisted.LastOperation != "status" || persisted.Summary != "Status failed" {
		t.Fatalf("persisted state = %#v", persisted)
	}
}

func TestFailedOperationPersistsTerminalErrorInsteadOfProjectOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	m.pending = engram.Status
	m.selected = map[string]bool{"alpha": true}
	m.cancel = func() {}

	updated, _ := m.applyEvent(engram.Event{Project: "alpha", Text: "alpha: status"})
	m = updated.(Model)
	updated, _ = m.applyEvent(engram.Event{Project: "alpha", Text: "Project is up to date"})
	m = updated.(Model)
	updated, _ = m.applyEvent(engram.Event{Done: true, Err: errors.New("command failed")})
	m = updated.(Model)

	state, err := config.LoadState(config.StatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	persisted := state.Projects["alpha"]
	if persisted.LastStatus != config.SyncStatusError || persisted.Summary != "Operation failed: command failed" {
		t.Fatalf("persisted state = %#v", persisted)
	}
}

func TestSavingConfigurationForNewServerClearsStaleSyncState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram-a.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	statePath := config.StatePath(path)
	state := config.BoundState(path, "https://engram-a.example.com")
	state.Projects["alpha"] = config.ProjectSyncState{
		LastStatus:    config.SyncStatusSynced,
		LastCheckedAt: "2026-08-07T15:04:05Z",
		LastOperation: "status",
		Summary:       "No synchronization needed",
	}
	if err := config.SaveState(statePath, state); err != nil {
		t.Fatal(err)
	}

	m := New(path)
	m.setupInputs(m.cfg)
	m.inputs[0].SetValue("https://engram-b.example.com")
	m.inputs[1].SetValue("12345678901234567890123456789012")
	m.inputs[2].SetValue("alpha")

	updated, _ := m.saveWizard()
	m = updated.(Model)

	if len(m.syncState.Projects) != 0 {
		t.Fatalf("in-memory sync state kept stale projects: %#v", m.syncState.Projects)
	}
	loaded, err := config.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Projects) != 0 || loaded.Server != "https://engram-b.example.com" || loaded.ConfigPath != path {
		t.Fatalf("persisted sync state = %#v", loaded)
	}
}

func TestSyncCenterShowsUnknownStateUntilAnySyncIsPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	view := stripANSI(New(path).syncCenterView())
	for _, snippet := range []string{"Sel", "Project", "Status", "Last sync", "alpha", "Unknown", "Never", "[ c ]", "Configure"} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("sync center missing %q:\n%s", snippet, view)
		}
	}
}

func TestSyncCenterConfiguredProjectsKeepsPopulatedRowsWithinTableWidth(t *testing.T) {
	m := Model{
		cfg:      config.Config{Projects: []string{"project-with-a-very-long-name-that-must-be-trimmed"}},
		selected: map[string]bool{"project-with-a-very-long-name-that-must-be-trimmed": true},
		syncState: config.State{Projects: map[string]config.ProjectSyncState{
			"project-with-a-very-long-name-that-must-be-trimmed": {
				LastStatus:    config.SyncStatusPullRequired,
				LastCheckedAt: "2026-08-07T15:04:05Z",
			},
		}},
	}

	const tableWidth = 76
	view := stripANSI(m.syncCenterConfiguredProjects(tableWidth))
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > tableWidth-4 {
			t.Fatalf("table row width = %d, exceeds content width %d: %q", got, tableWidth-4, line)
		}
	}
	if !strings.Contains(view, "Pull required") || !strings.Contains(view, syncCenterCheckedAt("2026-08-07T15:04:05Z")) {
		t.Fatalf("populated sync center row missing persisted values:\n%s", view)
	}
	if !strings.Contains(view, "...") {
		t.Fatalf("long project name was not truncated to fit table width:\n%s", view)
	}
}

func TestSyncCenterConfiguredProjectsKeepsNarrowTableWithinWidth(t *testing.T) {
	m := Model{
		cfg:      config.Config{Projects: []string{"project-with-a-very-long-name-that-must-be-trimmed"}},
		selected: map[string]bool{"project-with-a-very-long-name-that-must-be-trimmed": true},
		syncState: config.State{Projects: map[string]config.ProjectSyncState{
			"project-with-a-very-long-name-that-must-be-trimmed": {
				LastStatus:    config.SyncStatusPullRequired,
				LastCheckedAt: "2026-08-07T15:04:05Z",
			},
		}},
	}

	const tableWidth = 20
	view := stripANSI(m.syncCenterConfiguredProjects(tableWidth))
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > tableWidth-4 {
			t.Fatalf("narrow table row width = %d, exceeds content width %d: %q", got, tableWidth-4, line)
		}
	}
	if !strings.Contains(view, "[x]") {
		t.Fatalf("narrow table lost selection marker:\n%s", view)
	}
	if !strings.Contains(view, "...") {
		t.Fatalf("narrow table did not ellipsize overflowing cells:\n%s", view)
	}
}

func TestDashboardShowsPersistedShortSyncStatusPerProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	m.syncState = config.State{
		ConfigPath: path,
		Server:     m.cfg.Server,
		Projects: map[string]config.ProjectSyncState{
			"alpha": {
				LastStatus:    config.SyncStatusPullRequired,
				LastCheckedAt: "2026-08-07T15:04:05Z",
				LastOperation: "status",
				Summary:       "Pull required before local changes are current",
			},
		},
	}

	view := stripANSI(m.dashboardProjectRows())
	if !strings.Contains(view, "status: Pull required") {
		t.Fatalf("dashboard missing persisted short sync status:\n%s", view)
	}
	if !strings.Contains(view, "status: Unknown") {
		t.Fatalf("dashboard missing unknown fallback for unsynced project:\n%s", view)
	}
	if strings.Contains(view, "Pull required before local changes are current") {
		t.Fatalf("dashboard rendered long sync summary:\n%s", view)
	}
}

func TestCustomConfigurationsKeepSeparatePersistedSnapshots(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "personal.json")
	secondPath := filepath.Join(dir, "work.json")
	for _, path := range []string{firstPath, secondPath} {
		if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
			t.Fatal(err)
		}
	}
	firstState := config.BoundState(firstPath, "https://engram.example.com")
	firstState.Projects["alpha"] = config.ProjectSyncState{LastStatus: config.SyncStatusSynced, LastCheckedAt: "2026-08-07T15:04:05Z", LastOperation: "status", Summary: "Personal snapshot"}
	secondState := config.BoundState(secondPath, "https://engram.example.com")
	secondState.Projects["alpha"] = config.ProjectSyncState{LastStatus: config.SyncStatusPushRequired, LastCheckedAt: "2026-08-07T15:04:05Z", LastOperation: "status", Summary: "Work snapshot"}
	if err := config.SaveState(config.StatePath(firstPath), firstState); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveState(config.StatePath(secondPath), secondState); err != nil {
		t.Fatal(err)
	}

	if got := New(firstPath).syncState.Projects["alpha"].Summary; got != "Personal snapshot" {
		t.Fatalf("first configuration snapshot = %q", got)
	}
	if got := New(secondPath).syncState.Projects["alpha"].Summary; got != "Work snapshot" {
		t.Fatalf("second configuration snapshot = %q", got)
	}
}

func TestCustomConfigurationLoadsMatchingLegacySnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "personal.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	legacyState := config.BoundState(path, "https://engram.example.com")
	legacyState.Projects["alpha"] = config.ProjectSyncState{LastStatus: config.SyncStatusSynced, LastCheckedAt: "2026-08-07T15:04:05Z", LastOperation: "status", Summary: "Legacy snapshot"}
	if err := config.SaveState(config.LegacyStatePath(path), legacyState); err != nil {
		t.Fatal(err)
	}

	m := New(path)
	if m.syncStatePath == config.LegacyStatePath(path) {
		t.Fatal("custom configuration still writes to the shared legacy state path")
	}
	if got := m.syncState.Projects["alpha"].Summary; got != "Legacy snapshot" {
		t.Fatalf("legacy snapshot = %q", got)
	}
}

func TestCompletedOperationPersistsPerProjectSyncState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	m := New(path)
	m.pending = engram.Pull
	m.selected = map[string]bool{"alpha": true}
	m.statuses = map[string]string{"alpha": "Running"}
	m.activeProject = "alpha"
	m.projectLogs = map[string][]string{"alpha": {"Applied remote updates"}}
	m.cancel = func() {}

	updated, _ := m.applyEvent(engram.Event{Done: true, Text: "pull completed"})
	m = updated.(Model)
	state, err := config.LoadState(config.StatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	persisted := state.Projects["alpha"]
	if persisted.LastStatus != config.SyncStatusSynced || persisted.LastOperation != "pull" || persisted.Summary != "Applied remote updates" {
		t.Fatalf("persisted state = %#v", persisted)
	}
	if _, err := time.Parse(time.RFC3339, persisted.LastCheckedAt); err != nil {
		t.Fatalf("LastCheckedAt = %q, want RFC3339 timestamp", persisted.LastCheckedAt)
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
	if contains(strings.Join(m.logs, "\n"), token) || contains(m.runningView(), token) {
		t.Fatalf("rendered output exposed token: logs = %#v", m.logs)
	}
	if !reflect.DeepEqual(m.logs, []string{"---- alpha ----", "token=[redacted]"}) {
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

func stripANSI(value string) string {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\x1b' || index+1 >= len(value) || value[index+1] != '[' {
			result.WriteByte(value[index])
			continue
		}
		index += 2
		for index < len(value) && (value[index] < 0x40 || value[index] > 0x7e) {
			index++
		}
	}
	return result.String()
}
