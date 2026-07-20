package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
	if m.screen != dashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
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

func TestWizardViewRendersFocusedFullScreenStepWithoutWelcomeLogo(t *testing.T) {
	m := Model{screen: wizard, width: 100, height: 32}
	m.setupInputs(config.Config{})
	if got := m.inputs[0].Styles().Focused.Text.GetForeground(); got != successGreen {
		t.Fatalf("focused input color = %v, want %v", got, successGreen)
	}
	view := stripANSI(m.wizardView())

	for _, text := range []string{
		"Initial Configuration", "Step 1 of 3", "Server URL", "HTTPS server URL",
		"Enter continues to the next step.", "[ Enter ]", "Continue", "[ Esc ]", "Discard & return home", "[ Ctrl+C ]", "Quit",
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
			want: []string{"☁", "STATUS: SETUP COMPLETE", "Remote: Configured", "Token: Present", "Projects: 1 configured, 1 selected", "Next step: Open dashboard"},
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
	if got, want := strings.Join(m.homeMenuItems(), "|"), "Open workspace|Add project|Sync center|Edit configuration|Exit"; got != want {
		t.Fatalf("home menu = %q, want %q", got, want)
	}

	for _, tt := range []struct {
		name    string
		index   int
		screen  screen
		message string
	}{
		{name: "open workspace", screen: dashboard},
		{name: "add project placeholder", index: 1, screen: home, message: "Add project: Coming soon"},
		{name: "sync center placeholder", index: 2, screen: home, message: "Sync center: Coming soon"},
		{name: "edit configuration", index: 3, screen: wizard},
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
		})
	}
}

func TestSubmenuViewsDoNotRenderWelcomeLogo(t *testing.T) {
	for _, current := range []screen{wizard, dashboard, confirm, running} {
		m := Model{screen: current, width: 100, height: 30}
		if current == wizard {
			m.setupInputs(config.Config{})
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

func TestNonInputScreensQuitWithQOrCtrlC(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			for _, current := range []screen{home, dashboard, confirm, running} {
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

func TestWizardAcceptsQAsInputAndQuitsWithCtrlC(t *testing.T) {
	m := Model{screen: wizard}
	m.setupInputs(config.Config{})
	updated, cmd := m.Update(keyMessage("q"))
	m = updated.(Model)
	if m.inputs[0].Value() != "q" {
		t.Fatalf("q = input value %q, want q", m.inputs[0].Value())
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q returned a quit command")
		}
	}
	_, cmd = m.Update(keyMessage("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c quit command is nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command message = %T, want tea.QuitMsg", cmd())
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
		{"dashboard", Model{screen: dashboard, cfg: valid, selected: map[string]bool{"alpha": true}, statuses: map[string]string{"alpha": "Idle"}}.dashboardView(), []string{"[ Space ] Select", "[ q / Ctrl+C ] Quit"}},
		{"wizard", Model{screen: wizard, focus: 1}.wizardFooter(false), []string{"[ Shift+Tab ] Back", "[ Enter ] Continue", "[ Esc ] Discard & return home", "[ Ctrl+C ] Quit"}},
		{"confirmation", Model{screen: confirm}.confirmView(), []string{"[ Enter / y ] Confirm", "[ Esc / n ] Cancel", "[ q / Ctrl+C ] Quit"}},
		{"running", Model{screen: running}.runningView(), []string{"[ Esc ] Cancel operation", "[ q / Ctrl+C ] Quit"}},
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
