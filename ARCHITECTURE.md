# EnCloud TUI Architecture

EnCloud TUI is a small, professional terminal manager for selected Engram Cloud projects. It owns local configuration, presents a focused Bubble Tea workflow, and executes safe, cancellable Engram CLI operations one project at a time. This document describes the target structure while preserving the current standalone module as the migration baseline.

## Quick Path

1. Start at the dashboard, select projects, and request pull, push, or status.
2. Confirm the operation; the model starts one cancellable command stream and renders its events.
3. Return to the dashboard with per-project status, redacted logs, and a clear final outcome.

## Decisions

| Topic | Decision |
| --- | --- |
| Product name | **EnCloud TUI** is the canonical display name in the UI and documentation. |
| Identity | `github.com/piwi/encloud-tui` module, `encloud-tui` binary, and `cmd/encloud-tui` command are the canonical identity. |
| Configuration migration | Prefer `ENCLOUD_TUI_CONFIG`, then the temporary `ENGRAM_TUI_CONFIG` fallback. Without an override, use the new default when present, otherwise an existing legacy configuration file. |
| UI architecture | One Bubble Tea `Model`, explicit screen constants, and narrow custom messages. No screen-local programs or global mutable UI state. |
| Work execution | The model owns state transitions; `tea.Cmd` bridges asynchronous runner events back into `Update`. The runner owns child processes only. |
| Scope | Direct orchestration of the existing Engram CLI for a configured list of projects. No cloud API client is introduced. |

## Package Boundaries

The target stays deliberately small. Packages own concrete responsibilities rather than abstract interfaces.

| Path | Owns | Must not own |
| --- | --- | --- |
| `cmd/encloud-tui` | Process entry point, config-path selection, Bubble Tea program startup, process-level error reporting | Domain rules, rendering, command plans |
| `internal/tui` | One model, screen state, input state, selection, messages, commands, rendering | File persistence or `os/exec` calls |
| `internal/config` | Config shape, validation, default path, secure atomic load/save | UI controls or CLI execution |
| `internal/engram` | Concrete command plans, sequential CLI runner, child-process environment, emitted operation events | Bubble Tea types, rendering, persisted config writes |

Within `internal/tui`, split the current single file only when making the refactor slice:

| File | Responsibility |
| --- | --- |
| `model.go` | `Model`, screen constants, constructor, immutable state helpers |
| `update.go` | `Update`, key handling, screen transitions, `tea.Cmd` creation, operation-event application |
| `view.go` | `View` and screen-specific rendering functions |
| `styles.go` | Palette and named semantic Lip Gloss styles |

This is an organizational split, not a new architectural layer. Keep helpers close to their sole caller and introduce an interface only when a real second implementation or test seam requires one.

## Main Data Flow

```text
keyboard / window event
          |
          v
Bubble Tea Program -> Model.Update -> model state + tea.Cmd
                                      |
                                      v
                              Runner.Start(context, config, mode, projects)
                                      |
                                      v
                    sequential `engram` child processes -> Event stream
                                      |
                                      v
                         operationEventMsg -> Model.Update -> Model.View
```

`config.Load` provides the initial configuration to `tui.New`. The wizard validates and persists through `config.Save`, then resets dashboard selection and statuses. `engram.Runner` configures the server, enrolls each selected project, and runs its requested operation in order. It emits small `engram.Event` values; the UI wraps each one in `operationEventMsg`, the only runner-to-UI message type.

## Screen State Model

Use a private `screen` enum with these constants:

| Screen | Purpose | Allowed transitions |
| --- | --- | --- |
| `dashboard` | Project selection and operation choice | `confirm`, `wizard`, quit |
| `wizard` | First-run and edit configuration | `dashboard`, quit when no valid configuration exists |
| `confirm` | Explicit approval of a pending mode and selection | `running`, `dashboard` |
| `running` | Progress, streaming logs, and cancellation | `dashboard` |

The model remains the single source of truth for the current screen, selected projects, pending mode, statuses, logs, terminal message, viewport dimensions, and cancellation function. It receives Bubble Tea messages plus narrow domain messages such as `operationEventMsg`; it does not poll shared state or let the runner mutate it.

## Async Operations, Cancellation, and Logging

1. `startOperation` rejects an empty selection, resets operation logs, marks selected projects queued, and creates `context.WithCancel`.
2. It starts the concrete runner and returns `tea.Batch(spinner.Tick, nextEvent(events))`. `nextEvent` blocks within a `tea.Cmd`, never inside `Update`.
3. Each event updates only the necessary UI state and schedules the next event command until a terminal event arrives.
4. `esc` or `ctrl+c` while running calls the stored cancel function. The runner must emit a terminal event whose error wraps `context.Canceled`; closing the channel alone is not a cancellation result.
5. A terminal event clears cancellation state, returns to the dashboard, and marks outstanding projects complete, failed, or cancelled.

Logs are operational output, not an audit system. Stream trimmed command output with its project name, redact the exact configured token before it enters model state, and never render command arguments or environment values. Keep a bounded recent-log window in the view; a future persistent log feature requires an explicit retention and secret-redaction design.

## Configuration and Secrets

`config.Config` is the boundary for server URL, token, and project list. It validates HTTPS-only server URLs without credentials, queries, or fragments; token shape; non-empty unique project names; and file permissions on load.

The default configuration path is `~/.config/encloud-tui/config.json`. `ENCLOUD_TUI_CONFIG` overrides it; `ENGRAM_TUI_CONFIG` is a temporary fallback when the preferred variable is unset. With no override, an existing new path wins, otherwise an existing `~/.config/engram-tui/config.json` is used without copying its credentials. Saving creates a new config directory with `0700` permissions, writes a same-directory temporary file with `0600`, syncs it, renames it atomically, and syncs the directory. Existing directory permissions are not changed.

The token is masked in the wizard and is never a command argument, UI string, or log value. The runner passes it only as `ENGRAM_CLOUD_TOKEN` in the child process environment. It is not an application-level environment override and must never be persisted from process environment.

## Layout and Style System

Render from `tea.WindowSizeMsg` dimensions rather than fixed terminal assumptions. A normal-width dashboard shows selection, project name, and status in aligned columns. At narrow widths, preserve the cursor, selection marker, project name, and status by truncating project names and stacking or shortening help text; never hide the active screen's primary action. Running and wizard screens prioritize the active input or latest logs and keep controls discoverable.

`styles.go` defines one centralized semantic dark palette and derives named styles from it. Use semantic names such as `title`, `muted`, `accent`, `success`, `warning`, `error`, `selection`, and `border`, rather than scattering color codes across views. Views choose meaning, not literal colors. The palette must preserve readable contrast in common dark terminals and must not convey status by color alone.

## Testing Strategy

| Layer | Tests |
| --- | --- |
| `internal/config` | Table-driven validation, permission enforcement, atomic-save failures, and no replacement on a failed write. Use temporary directories. |
| `internal/engram` | Exact command plans, unavailable CLI, cancellation terminal event, sequential execution, and token redaction in streamed output. Use a controlled test executable only when process behavior must be proven. |
| `internal/tui` | Model transitions for every screen, selection independence, confirmation, empty selection, cancellation outcome, redaction before rendering, and width-dependent view invariants. Drive `Update` directly with messages. |
| Acceptance | `make fmt`, `make vet`, `make test`, and `make build`; manually exercise wizard, narrow terminal, a missing CLI, a failed operation, and cancellation. |

Do not add snapshot or end-to-end terminal automation until rendering behavior becomes sufficiently complex to justify its maintenance cost.

## Incremental Refactor Plan

### Safe First Slice

Move only the existing TUI code into `internal/tui/model.go`, `update.go`, `view.go`, and `styles.go` without changing exported behavior, package APIs, screens, messages, commands, colors, or text. Extract the existing style declarations into `styles.go` as the initial centralized palette. Keep tests passing unchanged, then add a small transition test only if the move exposes a gap.

### Follow-on Slices

1. Introduce semantic palette tokens and replace direct color literals without changing layout.
2. Make dashboard and help rendering width-aware with focused view tests.
3. Improve per-project terminal outcomes only when runner events can represent them accurately; do not infer detail the CLI has not emitted.

Each slice remains independently buildable and must retain secure persistence, cancellation terminal events, and token redaction.

## Deliberate Non-Goals

- A direct Engram Cloud HTTP client, background synchronization, or parallel project operations.
- Multiple Bubble Tea models, a router framework, an event bus, dependency injection, or repository/service interfaces without concrete need.
- Credential import from external scripts, hidden path dependencies, or application configuration through `ENGRAM_CLOUD_TOKEN`.
- Persistent operation history, telemetry, remote project discovery, user accounts, or role management.
- Renaming the module, binary, or command independently.

## Review Checklist

- [ ] One Bubble Tea model remains the UI state authority.
- [ ] Every state transition is triggered in `Update` and asynchronous work re-enters through `tea.Cmd` messages.
- [ ] The runner remains sequential, cancellable, and unable to expose tokens in output.
- [ ] Configuration remains standalone, validated, atomic, and permission-restricted.
- [ ] `model.go` / `update.go` / `view.go` / `styles.go` improves navigation without adding speculative abstractions.
- [ ] Module, binary, and command use the canonical EnCloud TUI identity together.
