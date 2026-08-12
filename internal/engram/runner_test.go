package engram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestStreamOutputHandlesBoundedLongLines(t *testing.T) {
	var events []Event
	emit := func(event Event) bool {
		events = append(events, event)
		return true
	}
	if err := streamOutput(strings.NewReader(strings.Repeat("x", 128*1024)+"\n"), "alpha", "secret", emit); err != nil {
		t.Fatalf("streamOutput() error = %v", err)
	}
	if len(events) != 1 || len(events[0].Text) != 128*1024 {
		t.Fatalf("events = %#v, want one complete long line", events)
	}
	if err := streamOutput(strings.NewReader(strings.Repeat("x", maxScannerTokenSize+1)), "alpha", "secret", emit); err == nil {
		t.Fatal("streamOutput() succeeded for a line above the scanner limit")
	}
}

func TestRunnerReturnsScannerErrorsWithoutHanging(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a controlled child process")
	}
	t.Setenv("ENGRAM_TUI_TEST_HELPER", "1")
	t.Setenv("ENGRAM_TUI_TEST_COMMANDS", filepath.Join(t.TempDir(), "commands"))
	t.Setenv("ENGRAM_TUI_TEST_LONG_OUTPUT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner := Runner{Binary: os.Args[0]}
	cfg := config.Config{Token: "12345678901234567890123456789012"}
	err := runner.run(ctx, cfg, t.TempDir(), "alpha", []string{"-test.run=TestRunnerHelperProcess", "--"}, func(Event) bool { return true })
	if err == nil || ctx.Err() != nil {
		t.Fatalf("run() error = %v, context = %v; want scanner error before timeout", err, ctx.Err())
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
	t.Setenv("ENGRAM_DATA_DIR", "inherited-data-dir-must-not-be-used")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := Runner{Binary: os.Args[0], ConfigPath: "/tmp/encloud-a.json"}
	cfg := config.Config{Server: "https://engram.example.com", Token: token, Projects: []string{"alpha", "beta"}}
	dataDir, err := runner.runtimeDataDir(cfg.Server)
	if err != nil {
		t.Fatal(err)
	}

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
		"cloud config --server https://engram.example.com|" + token + "||" + dataDir,
		"cloud enroll alpha|" + token + "||" + dataDir,
		"sync --cloud --status --project alpha|" + token + "||" + dataDir,
		"cloud enroll beta|" + token + "||" + dataDir,
		"sync --cloud --status --project beta|" + token + "||" + dataDir,
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

func TestRunnerRuntimeDataDirIsolatesServerAndConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	base := Runner{ConfigPath: "/tmp/encloud-a.json"}
	dataDir, err := base.runtimeDataDir("https://engram-a.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dataDir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("isolated data directory = %q, info = %#v, err = %v; want mode 0700", dataDir, info, err)
	}
	for _, runner := range []Runner{
		{ConfigPath: "/tmp/encloud-b.json"},
		{ConfigPath: "/tmp/encloud-a.json"},
	} {
		server := "https://engram-a.example.com"
		if runner.ConfigPath == "/tmp/encloud-a.json" {
			server = "https://engram-b.example.com"
		}
		otherDataDir, err := runner.runtimeDataDir(server)
		if err != nil {
			t.Fatal(err)
		}
		if otherDataDir == dataDir {
			t.Fatalf("data directories = %q, want distinct identities", otherDataDir)
		}
	}
	repeatedDataDir, err := base.runtimeDataDir("https://engram-a.example.com")
	if err != nil || repeatedDataDir != dataDir {
		t.Fatalf("repeated data directory = %q, err = %v; want %q", repeatedDataDir, err, dataDir)
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
	if os.Getenv("ENGRAM_TUI_TEST_LONG_OUTPUT") == "1" {
		fmt.Print(strings.Repeat("x", maxScannerTokenSize+1))
		return
	}
	fmt.Printf("token=%s\n", os.Getenv("ENGRAM_CLOUD_TOKEN"))
	if _, err := fmt.Fprintln(file, strings.Join(args, " ")+"|"+os.Getenv("ENGRAM_CLOUD_TOKEN")+"|"+os.Getenv("ENGRAM_TOKEN")+"|"+os.Getenv("ENGRAM_DATA_DIR")); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
