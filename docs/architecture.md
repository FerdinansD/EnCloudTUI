# Architecture

EnCloud TUI is a local Bubble Tea application that orchestrates the installed `engram` CLI for explicitly configured projects. It is not a visual alias: it owns secure persistence, environment isolation, cancellable orchestration, output handling, and interpretation of local sync snapshots. Engram owns Cloud communication, and Engram Cloud remains authoritative for current remote state.

## System Boundary

```text
keyboard/window event
        |
        v
Bubble Tea model -> asynchronous command -> sequential engram child processes
        ^                                      |
        |                                      v
        +--- local config/state <- redacted event stream <- Engram Cloud
```

| EnCloud TUI owns | Engram owns |
| --- | --- |
| Screen flow, focus, selection, confirmation, responsive rendering, and user feedback. | Authentication and communication with Engram Cloud. |
| Configuration and local state validation, binding, permissions, and atomic persistence. | Cloud endpoint configuration and project enrollment behavior. |
| Sequential command planning, cancellation lifecycle, child environment isolation, and bounded redacted logs. | Status, import, and synchronization semantics exposed by its CLI. |
| Classification and display of time-stamped local observations derived from CLI output. | Authoritative remote project and synchronization state. |

EnCloud TUI never calls the Cloud API directly and does not infer that a saved observation is still current.

## Package Landmarks

| Path | Stable responsibility |
| --- | --- |
| `cmd/encloud-tui` | Selects the configuration path and starts the application. |
| `internal/tui` | Owns the Bubble Tea model, screen flow, operation lifecycle, layout, and snapshot interpretation. |
| `internal/config` | Validates configuration and state, binds snapshots to their source, and performs secure atomic persistence. |
| `internal/engram` | Plans and runs the Engram CLI commands, isolates child environments, and streams redacted events. |

There is one Bubble Tea model rather than one program per screen. Useful user-facing landmarks are Home, configuration, Add Project, Sync Center, confirmation, and running/result views; internal function names are intentionally not part of this architecture contract.

## Asynchronous Operation Flow

1. The UI validates that at least one project is selected and opens confirmation.
2. Confirmation creates a cancellation context and starts an asynchronous event-producing command, so the Bubble Tea update loop does not perform blocking process work.
3. The runner verifies that `engram` is on `PATH`, creates an isolated runtime data directory, and configures the server once.
4. Selected projects are processed one at a time in selection/configuration order.
5. Enrollment is attempted for each project. A non-cancellation enrollment failure is reported as skipped enrollment, but does not prevent the requested sync command.
6. Standard output and standard error are combined, redacted, bounded in memory, and emitted as UI events.
7. A sync command failure terminates the batch. Projects not yet started are shown as skipped.
8. Cancellation signals the running child process. The UI remains in the running lifecycle until the runner emits a terminal cancellation event; quit-during-operation follows the same wait and quits afterward.
9. The terminal event determines the batch outcome and triggers persistence of compatible per-project snapshots.

Operations are asynchronous to the UI but deliberately sequential at the process boundary. There is no parallel project execution.

## Engram CLI Contract

The executable name is `engram` and it must resolve on `PATH`. EnCloud TUI delegates exactly these command forms:

| Step | Arguments after `engram` |
| --- | --- |
| Configure endpoint once per batch | `cloud config --server SERVER` |
| Best-effort enrollment per project | `cloud enroll PROJECT` |
| Status | `sync --cloud --status --project PROJECT` |
| Pull | `sync --cloud --import --project PROJECT` |
| Push | `sync --cloud --project PROJECT` |

The configured public Cloud token is supplied through `ENGRAM_CLOUD_TOKEN`, never as a command argument. Environment and runtime isolation are specified in [Configuration and security](configuration.md).

## Snapshot Classification

Snapshots are persisted interpretations of command output and completion, not direct reads of Cloud state.

| Evidence | Persisted classification |
| --- | --- |
| Recognized Status output | Synced, Pull required, Push required, or Diverged. |
| Successful Pull or Push | Synced. |
| Failed operation with project output | Error. |
| Cancelled operation | Previous snapshot remains unchanged. |
| Unrecognized Status output | Previous snapshot remains unchanged. |
| No compatible bound snapshot | Unknown / Never. |

Each compatible snapshot records the classification, RFC3339 check time, operation, and a short output-derived summary. **Status refreshes current evidence; the saved result is still local derived state and never live Cloud truth.** Logs are transient, bounded diagnostic feedback rather than persistent operation history or an audit trail.

## UI And Accessibility Invariants

- Home is navigation, while Sync Center is the project-selection and operation surface.
- Status, Pull, and Push require a non-empty selection and explicit confirmation.
- Focus and batch selection are independent and have textual markers; color only supplements them.
- Statuses, counts, errors, and action hints remain explicit without depending on color perception.
- The token is masked and must never appear in rendered values, command arguments, or operation output.
- The viewport controls layout. Panels and inputs shrink, long text wraps or ellipsizes before overlap, and hints stack at narrow widths.
- Focus, selection, primary action, and terminal operation outcome take priority over decorative whitespace or full-size branding.
- The interface does not require mouse input, custom fonts, a graphical terminal, or a fixed canvas size.

## Persistence And Recovery

Configuration and snapshots are separate JSON documents. State is accepted only when its normalized configuration-path and server binding match the active configuration. Changing either boundary starts from empty bound state rather than reusing unrelated observations.

Missing state is recoverable as an empty bound state. Malformed, unreadable, insecurely permissioned, or mismatched data is not silently trusted; the UI reports a warning and preserves the distinction between unavailable local evidence and Cloud state. See [Configuration and security](configuration.md) for exact paths and write guarantees.

## Non-Goals

- Direct Cloud API access or replacement of the Engram CLI.
- Remote project discovery, background synchronization, or parallel project execution.
- Live remote-state claims based on local snapshots.
- Persistent operation history, audit logging, telemetry, user accounts, or role management.
- Importing credentials from unrelated tools or inherited Engram token variables.
- Defining Engram's own installation, compatibility, authentication, or Cloud semantics.

For the user workflow and current distribution status, return to the [project README](../README.md).
