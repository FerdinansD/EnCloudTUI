# Manage Engram Cloud Sync Safely

`encloud-tui` is a standalone full-screen terminal interface for synchronizing selected Engram projects with Engram Cloud. It stores its own configuration locally and runs cancellable, sequential CLI commands.

## Quick Path

1. Install the `engram` CLI so `engram` is available in your `PATH`, then build with `make build`.
2. Run `./encloud-tui` and complete the initial configuration wizard.
3. Select projects and confirm `pull`, `push`, or `status`.

## Initial Configuration

On first launch, run `./encloud-tui`. The TUI opens its configuration wizard automatically.

1. Enter the Engram Cloud server URL. It must use HTTPS.
2. Enter the cloud token.
3. Enter one or more project names, separated by commas.
4. Save the configuration and return to the dashboard.

The TUI saves the configuration to `~/.config/encloud-tui/config.json`. Its directory uses `0700` permissions and the configuration file uses `0600` permissions.

The `engram` CLI must be installed and available in your `PATH` before running operations. This application is standalone and has no dependency on `/home/piwi/engram-sync`.

To change the server, token, or projects later, restart the TUI and press `c` from the dashboard to open the configuration screen.

```json
{
  "server": "https://engram.example.com",
  "token": "YOUR_CLOUD_TOKEN",
  "projects": ["project-a", "project-b"]
}
```

## Operations

| Key | Action |
| --- | --- |
| `space` | Toggle the focused project |
| `a` | Select or clear all projects |
| `p` | Pull cloud data with `sync --cloud --import` |
| `u` | Push local data with `sync --cloud` |
| `s` | Inspect status with `sync --cloud --status` |
| `c` | Edit configuration |
| `esc` / `ctrl+c` | Cancel an active operation |

Each selected project is enrolled before its operation. The TUI processes one command at a time, displays progressive output, and preserves independent project statuses.

## Configuration

Configuration is created and edited in the first-run wizard or with `c` from the dashboard. It is saved at `~/.config/encloud-tui/config.json`; its directory is set to `0700` and the file to `0600`.

`ENCLOUD_TUI_CONFIG` selects a different configuration file path. `ENGRAM_TUI_CONFIG` remains supported temporarily for migration. No other environment variables configure the application.

The example in Initial Configuration uses a safe placeholder token. Replace it only in your local configuration; do not share it. `server` must be an HTTPS URL without credentials, a query, or a fragment. Each project name may contain letters, numbers, `.`, `_`, and `-`.

| Environment variable | Purpose |
| --- | --- |
| `ENCLOUD_TUI_CONFIG` | Preferred override for the configuration file path |
| `ENGRAM_TUI_CONFIG` | Legacy override, used only when `ENCLOUD_TUI_CONFIG` is unset |

### Configuration Migration

Path selection is deterministic: `ENCLOUD_TUI_CONFIG` takes precedence, then `ENGRAM_TUI_CONFIG`. With neither variable set, the application uses `~/.config/encloud-tui/config.json` when it exists. If it does not exist but the legacy `~/.config/engram-tui/config.json` exists, the application safely uses that existing file instead. It never copies, exposes, or imports credentials; use the wizard to save a configuration at the new default path when you are ready to migrate it.

The token input is masked. The application does not put the token in command arguments, logs, or screen output; it supplies it only through `ENGRAM_CLOUD_TOKEN` to the child process. A missing `engram` executable is reported in the TUI without a crash.

## Development

```bash
make fmt
make vet
make test
make build
```

Tests use temporary directories and cover configuration validation and permissions, command plans, missing CLI handling, and key TUI state transitions.
