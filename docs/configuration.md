# Configuration And Security

EnCloud TUI stores an HTTPS endpoint, one public Engram Cloud token, and an explicit list of projects. It validates these values locally and forwards the token only to isolated `engram` child processes. Setup does not test the remote connection, discover projects, or establish that saved snapshots are current.

## Quick Path

1. Launch EnCloud TUI and complete the configuration wizard.
2. Enter an HTTPS server URL, a Cloud token, and one or more project IDs.
3. Open Sync Center and run **Status** to obtain current CLI-derived evidence.

The token field is masked. Edit the same values from Home or with `c` in Sync Center.

## Validation

| Setting | Rule |
| --- | --- |
| Server | An HTTPS URL with a host; embedded credentials, query, and fragment are rejected. |
| Token | 32-512 characters with no spaces, tabs, or line breaks. |
| Projects | At least one unique project is required. |
| Project ID | 1-64 characters; starts with an ASCII alphanumeric character and then uses only ASCII alphanumerics, `.`, `_`, or `-`. |

Validation is local structure validation. A valid configuration does not prove that the server is reachable, the token is accepted, or a project exists remotely.

## Configuration Path Selection

The first applicable source wins:

| Priority | Source |
| --- | --- |
| 1 | `ENCLOUD_TUI_CONFIG`, when non-empty. |
| 2 | Legacy `ENGRAM_TUI_CONFIG`, only when the preferred variable is empty. |
| 3 | Existing `~/.config/encloud-tui/config.json`. |
| 4 | Existing legacy `~/.config/engram-tui/config.json`. |
| 5 | `~/.config/encloud-tui/config.json` as the new default. |

Environment-selected paths are cleaned before use. An existing default file takes precedence over the legacy location. Legacy credentials are used in place; EnCloud TUI does not copy or migrate them automatically.

## Local State Paths And Binding

State is separate from configuration and stores derived sync snapshots.

| Active configuration | State path |
| --- | --- |
| Any file named `config.json`, including the default and legacy locations | `state.json` beside that configuration. |
| A custom filename | `.encloud-state-<path-digest>.json` beside that configuration. |

The path digest is deterministic for the cleaned configuration path, keeping multiple custom configurations in one directory isolated. If `state.json` would alias the configuration itself, the fallback state filename is `.encloud-state.json`.

State records its normalized configuration path and server. Snapshots are retained only when both match the active configuration. A changed path or server creates empty bound state; mismatched snapshots are ignored instead of being presented under the wrong connection. Legacy unbound state is reset into the active binding rather than reused; future writes include the binding.

Each project snapshot contains:

- `last_status`: `unknown`, `synced`, `pull_required`, `push_required`, `diverged`, or `error`.
- `last_checked_at`: an RFC3339 timestamp.
- `last_operation`: `status`, `pull`, or `push`.
- `summary`: a non-empty, short output-derived description.

> **Snapshots are local state derived from command output, not live Cloud truth. Status refreshes the evidence when current remote state matters.**

## Secure And Atomic Persistence

- Configuration and state files must have mode `0600` when loaded.
- Newly created parent directories use mode `0700`.
- Existing parent-directory permissions are not changed automatically.
- Saves validate before writing.
- A same-directory temporary file is set to `0600`, written, file-synced, closed, and atomically renamed over the target.
- The containing directory is synced after rename to make the replacement durable where the platform supports it.
- If the rename succeeded but directory sync failed, the UI reports that the save completed but durability could not be confirmed; it does not pretend the target was untouched.

These controls reduce accidental disclosure and partial replacement. They do not encrypt the token at rest or protect it from the current OS account, privileged processes, backups, filesystem snapshots, or a compromised `engram` executable.

## Child Environment And Runtime Isolation

Before every delegated command, EnCloud TUI constructs a child environment that:

1. Removes inherited `ENGRAM_TOKEN`.
2. Removes inherited `ENGRAM_CLOUD_TOKEN`.
3. Removes inherited `ENGRAM_DATA_DIR`.
4. Sets only the configured public token as `ENGRAM_CLOUD_TOKEN`.
5. Sets `ENGRAM_DATA_DIR` to an EnCloud TUI-managed runtime directory.

The runtime directory is under the user's configuration directory at `encloud-tui/engram/<identity-digest>`, uses mode `0700`, and is keyed by the cleaned active configuration path plus server. This prevents ambient Engram state from crossing configuration/server boundaries and prevents EnCloud TUI commands from mutating the user's unrelated default Engram runtime state.

The configured token is never placed in command arguments or rendered by the UI. Child output lines containing the environment variable name are dropped, and exact token values in streamed or UI error text are replaced with `[redacted]`. Recent global and per-project logs are bounded in memory.

Redaction is defense in depth, not a general secret scanner: terminal output, summaries, project names, server URLs, process metadata, crash data, and local files can still contain sensitive operational information. Do not run EnCloud TUI with an untrusted `engram` executable earlier on `PATH`.

## Trust Boundaries

| Boundary | Trusted for | Not trusted for |
| --- | --- | --- |
| Configuration file | User-selected endpoint, token, and project list after local validation. | Remote reachability or authorization. |
| State file | Compatible, time-stamped local observations after validation and binding. | Current Cloud truth or audit history. |
| `engram` on `PATH` | Executing the documented CLI contract with the supplied child environment. | EnCloud TUI code ownership or automatic installation. |
| Engram Cloud | Authoritative remote synchronization state. | Local UI persistence and presentation. |

## Recovery Behavior

| Condition | Behavior |
| --- | --- |
| No configuration exists | The application opens the initial configuration path. |
| Configuration is missing, malformed, unreadable, invalid, or not `0600` | It is not silently accepted; the UI presents setup or warning context so the user can correct it. |
| State file is missing | The application uses empty state bound to the active configuration and server. |
| State is malformed, unreadable, invalid, or not `0600` | The state is not trusted; the UI reports the problem rather than converting it into a Cloud claim. |
| State binding does not match | Existing snapshots are discarded from the active view and empty bound state is used. |
| `engram` is absent from `PATH` | The batch ends before server configuration and the UI reports that the CLI is unavailable. |
| Enrollment fails | The failure is reported, then the requested sync command still runs unless cancellation is active. |
| A sync command fails | The batch terminates and remaining projects are skipped. |
| Cancellation is requested | The child process is cancelled and the UI waits for the runner's terminal event. |

For command order, snapshot classification, and UI invariants, read [Architecture](architecture.md). For installation and operation, return to the [project README](../README.md).
