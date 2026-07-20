package engram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piwi/encloud-tui/internal/config"
)

func TestPlan(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want []string
	}{
		{"pull imports cloud data", Pull, []string{"sync", "--cloud", "--import", "--project", "alpha"}},
		{"push uploads cloud data", Push, []string{"sync", "--cloud", "--project", "alpha"}},
		{"status does not mutate", Status, []string{"sync", "--cloud", "--status", "--project", "alpha"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := Plan(tt.mode, "alpha")
			if err != nil {
				t.Fatal(err)
			}
			if len(plan) != 2 {
				t.Fatalf("commands = %d, want 2", len(plan))
			}
			for i := range tt.want {
				if plan[1][i] != tt.want[i] {
					t.Fatalf("operation = %#v, want %#v", plan[1], tt.want)
				}
			}
		})
	}
}

func TestRunnerReportsCancelledContextAsTerminalError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := Runner{Binary: "sh"}
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	event := <-runner.Start(ctx, cfg, Status, cfg.Projects)
	if !event.Done || !errors.Is(event.Err, context.Canceled) {
		t.Fatalf("event = %#v, want terminal cancellation error", event)
	}
}

func TestPlanRejectsUnknownMode(t *testing.T) {
	if _, err := Plan("delete", "alpha"); err == nil {
		t.Fatal("expected unsupported mode error")
	}
}

func TestRunnerReportsMissingCLI(t *testing.T) {
	runner := Runner{Binary: "definitely-not-engram"}
	cfg := config.Config{Server: "https://engram.example.com", Token: "12345678901234567890123456789012", Projects: []string{"alpha"}}
	event := <-runner.Start(context.Background(), cfg, Status, cfg.Projects)
	if event.Err == nil || !event.Done {
		t.Fatalf("event = %#v, want completed missing CLI error", event)
	}
}

func TestRunnerExecutesSequentiallyAndRedactsStreamedOutput(t *testing.T) {
	if os.Getenv("ENGRAM_TUI_TEST_HELPER") == "1" {
		return
	}
	if testing.Short() {
		t.Skip("runs a controlled child process")
	}
	path := filepath.Join(t.TempDir(), "commands")
	token := "12345678901234567890123456789012"
	t.Setenv("ENGRAM_TUI_TEST_HELPER", "1")
	t.Setenv("ENGRAM_TUI_TEST_COMMANDS", path)
	t.Setenv("ENGRAM_TOKEN", "inherited-private-token-must-not-be-used")
	t.Setenv("ENGRAM_CLOUD_TOKEN", "inherited-token-must-not-be-used")
	runner := Runner{Binary: os.Args[0]}
	cfg := config.Config{Server: "https://engram.example.com", Token: token, Projects: []string{"alpha", "beta"}}

	var events []Event
	for event := range runner.Start(context.Background(), cfg, Status, cfg.Projects) {
		events = append(events, event)
	}
	if len(events) == 0 || !events[len(events)-1].Done || events[len(events)-1].Err != nil {
		t.Fatalf("events = %#v, want successful terminal event", events)
	}
	for _, event := range events {
		if strings.Contains(event.Text, token) {
			t.Fatalf("event exposed token: %#v", event)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"cloud config --server https://engram.example.com|" + token + "|",
		"cloud enroll alpha|" + token + "|",
		"sync --cloud --status --project alpha|" + token + "|",
		"cloud enroll beta|" + token + "|",
		"sync --cloud --status --project beta|" + token + "|",
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commands = %#v, want %#v", got, want)
		}
	}
}

func TestRunnerHelperProcess(t *testing.T) {
	if os.Getenv("ENGRAM_TUI_TEST_HELPER") != "1" {
		return
	}
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	file, err := os.OpenFile(os.Getenv("ENGRAM_TUI_TEST_COMMANDS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(2)
	}
	defer file.Close()
	fmt.Printf("token=%s\n", os.Getenv("ENGRAM_CLOUD_TOKEN"))
	if _, err := fmt.Fprintln(file, strings.Join(args, " ")+"|"+os.Getenv("ENGRAM_CLOUD_TOKEN")+"|"+os.Getenv("ENGRAM_TOKEN")); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
