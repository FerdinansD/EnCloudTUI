package engram

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/piwi/encloud-tui/internal/config"
)

type Mode string

const (
	Pull   Mode = "pull"
	Push   Mode = "push"
	Status Mode = "status"
)

type Event struct {
	Project string
	Text    string
	Done    bool
	Err     error
}

type Runner struct {
	Binary string
}

func (r Runner) binary() string {
	if r.Binary != "" {
		return r.Binary
	}
	return "engram"
}

func Plan(mode Mode, project string) ([][]string, error) {
	if project == "" {
		return nil, fmt.Errorf("plan operation: project is required")
	}
	var operation []string
	switch mode {
	case Pull:
		operation = []string{"sync", "--cloud", "--import", "--project", project}
	case Push:
		operation = []string{"sync", "--cloud", "--project", project}
	case Status:
		operation = []string{"sync", "--cloud", "--status", "--project", project}
	default:
		return nil, fmt.Errorf("plan operation: unsupported mode %q", mode)
	}
	return [][]string{{"cloud", "enroll", project}, operation}, nil
}

// Start runs config and each project in order. Events stream command output without exposing secrets.
func (r Runner) Start(ctx context.Context, cfg config.Config, mode Mode, projects []string) <-chan Event {
	events := make(chan Event)
	go func() {
		defer close(events)
		finish := func(event Event) {
			events <- event
		}
		cancelled := func() error {
			return fmt.Errorf("Engram operation cancelled: %w", ctx.Err())
		}
		if ctx.Err() != nil {
			finish(Event{Done: true, Err: cancelled()})
			return
		}
		if _, err := exec.LookPath(r.binary()); err != nil {
			finish(Event{Done: true, Err: fmt.Errorf("Engram CLI is unavailable: %w", err)})
			return
		}
		emit := func(event Event) bool {
			select {
			case events <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if err := r.run(ctx, cfg, "", []string{"cloud", "config", "--server", cfg.Server}, emit); err != nil {
			finish(Event{Done: true, Err: err})
			return
		}
		for _, project := range projects {
			if !emit(Event{Project: project, Text: fmt.Sprintf("%s: %s", project, mode)}) {
				finish(Event{Done: true, Err: cancelled()})
				return
			}
			commands, err := Plan(mode, project)
			if err != nil {
				finish(Event{Done: true, Err: err})
				return
			}
			if err := r.run(ctx, cfg, project, commands[0], emit); err != nil && ctx.Err() == nil {
				emit(Event{Project: project, Text: "enrollment skipped"})
			}
			if err := r.run(ctx, cfg, project, commands[1], emit); err != nil {
				finish(Event{Done: true, Err: err})
				return
			}
		}
		if ctx.Err() != nil {
			finish(Event{Done: true, Err: cancelled()})
			return
		}
		finish(Event{Done: true, Text: fmt.Sprintf("%s completed", mode)})
	}()
	return events
}

func (r Runner) run(ctx context.Context, cfg config.Config, project string, args []string, emit func(Event) bool) error {
	cmd := exec.CommandContext(ctx, r.binary(), args...)
	cmd.Env = append(os.Environ(), "ENGRAM_CLOUD_TOKEN="+cfg.Token)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("capture command output: %w", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Engram command: %w", err)
	}
	streamOutput(pipe, project, cfg.Token, emit)
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("Engram operation cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("Engram command failed: %w", err)
	}
	return nil
}

func streamOutput(reader io.Reader, project, secret string, emit func(Event) bool) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.Contains(line, "ENGRAM_CLOUD_TOKEN") {
			line = strings.ReplaceAll(line, secret, "[redacted]")
			if !emit(Event{Project: project, Text: line}) {
				return
			}
		}
	}
}
