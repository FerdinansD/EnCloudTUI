package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/piwi/encloud-tui/internal/config"
	"github.com/piwi/encloud-tui/internal/engram"
)

func TestWizardSavesAndMasksToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(path)
	updated, _ := m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != wizard {
		t.Fatalf("screen = %v, want wizard", m.screen)
	}
	token := "12345678901234567890123456789012"
	m.inputs[0].SetValue("https://engram.example.com")
	m.inputs[1].SetValue(token)
	m.inputs[2].SetValue("alpha, beta")
	updated, _ = m.saveWizard()
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
	updated, _ := m.Update(keyMessage("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyMessage("esc"))
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

func TestHomeRoutesToDashboardAndConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha", "beta"}}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	updated, _ := New(path).Update(keyMessage("enter"))
	m := updated.(Model)
	if m.screen != dashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
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

func TestWizardViewRendersInitialConfigurationLayout(t *testing.T) {
	m := Model{screen: wizard, width: 100, height: 32}
	m.setupInputs(config.Config{})
	if got := m.inputs[0].Styles().Focused.Text.GetForeground(); got != successGreen {
		t.Fatalf("focused input color = %v, want %v", got, successGreen)
	}
	view := stripANSI(m.wizardView())

	for _, text := range []string{
		"EnCloud TUI", "Initial configuration", "Step 1 of 2", "Next: Sync dashboard",
		"Configure your cloud connection to continue.", "After setup, EnCloud TUI will open the full sync dashboard.",
		"Server URL", "Cloud server base URL", "Cloud Token", "Your EnCloud API token", "Projects", "Comma-separated project IDs",
		"[x] Preview config", "Required before continuing", "tab Next field", "enter Save & continue", "esc Cancel",
		"Press Enter to save and start configuration.",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("wizard view missing %q:\n%s", text, view)
		}
	}
	if strings.Count(view, "╔") < 4 {
		t.Fatalf("wizard view = %q, want bordered panel and inputs", view)
	}
}

func TestWizardViewKeepsErrorsAndFitsCompactTerminals(t *testing.T) {
	m := Model{screen: wizard, width: 40, height: 20, message: "Cannot save configuration: token must contain 32 to 512 non-whitespace characters"}
	m.setupInputs(config.Config{})
	view := stripANSI(m.wizardView())
	if !strings.Contains(view, "Cannot save configuration: token must") || !strings.Contains(view, "characters") {
		t.Fatalf("wizard view missing save error: %s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, m.width, line)
		}
	}
}

func TestHomeViewReflectsKnownSetupStatus(t *testing.T) {
	m := Model{screen: home, width: 120, height: 30}
	view := stripANSI(m.View().Content)
	for _, text := range []string{"Main Menu", "Initial configuration", "Setup Status", "Configuration: Invalid", "Server: Not configured", "Token: Missing", "Projects: 0 configured, 0 selected", "Operation: Idle", "╔", "╚"} {
		if !strings.Contains(view, text) {
			t.Fatalf("home view missing %q:\n%s", text, view)
		}
	}

	lines := strings.Split(view, "\n")
	if strings.Index(lines[1], "██") < 0 || strings.LastIndex(lines[1], "╔") <= strings.Index(lines[1], "██") {
		t.Fatalf("home header and status are not side by side:\n%s", view)
	}
	setupLine, menuLine := -1, -1
	for index, line := range lines {
		if strings.Contains(line, "Setup Status") {
			setupLine = index
		}
		if strings.Contains(line, "Main Menu") {
			menuLine = index
		}
	}
	if setupLine < 0 || menuLine <= setupLine {
		t.Fatalf("status must precede the menu in the right column:\n%s", view)
	}
}

func TestHomeViewFitsCompactTerminal(t *testing.T) {
	m := Model{screen: home, width: 40, height: 20}
	for _, line := range strings.Split(stripANSI(m.View().Content), "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, m.width, line)
		}
	}
}

func TestHomeMenuNavigationAndActionRouting(t *testing.T) {
	m := Model{screen: home, cfg: config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}
	if got, want := strings.Join(m.homeMenuItems(), "|"), "Open dashboard|Select sync projects|Edit configuration|Exit"; got != want {
		t.Fatalf("home menu = %q, want %q", got, want)
	}
	updated, _ := m.Update(keyMessage("j"))
	m = updated.(Model)
	if m.homeMenu != 1 {
		t.Fatalf("menu index = %d, want 1", m.homeMenu)
	}
	updated, _ = m.Update(keyMessage("enter"))
	m = updated.(Model)
	if m.screen != dashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
	}
}

func TestSharedHeaderKeepsAnimatedLogoAcrossScreens(t *testing.T) {
	m := Model{screen: wizard}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	if !m.welcomeAnimating || cmd == nil {
		t.Fatal("shared header did not start logo animation")
	}
	updated, _ = m.Update(welcomeTickMsg{})
	m = updated.(Model)
	if !strings.Contains(stripANSI(m.shellHeader()), string([]rune(welcomeLogoLines()[0])[:1])) {
		t.Fatal("shared header did not render logo reveal")
	}
}

func TestWelcomeLogoUsesProvidedShadedWordmark(t *testing.T) {
	logoText := embeddedWelcomeLogo()
	lines := strings.Split(logoText, "\n")
	want := []string{
		"██▀██ ███▄██ ██▀██ ██    ██▀██ ██ ██ ██▀█▄      ▀██▀ ██ ██ ▀██▀",
		"██▄   ██ ▀██ ██    ██    ██ ██ ██ ██ ██ ██       ██  ██ ██  ██ ",
		"█▓░▄▄ █▓░ █▓ █▓░▄▄ █▓░▄▄ █▓░█▓ █▓░█▓ █▓░█▓       █▓░ █▓░█▓  █▓░",
		"▀▀▀▀▀ ▀▀  ▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀▀ ▀▀▀▀        ▀▀  ▀▀▀▀▀ ▀▀▀▀",
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
			if string(gotGlyphs[:column]) != string(wantGlyphs[:column]) || strings.TrimSpace(string(gotGlyphs[column:])) != "" {
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

func TestHomeQuitKeysQuit(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			_, cmd := (Model{screen: home}).Update(keyMessage(key))
			if cmd == nil {
				t.Fatal("quit command is nil")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("command message = %T, want tea.QuitMsg", cmd())
			}
		})
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
