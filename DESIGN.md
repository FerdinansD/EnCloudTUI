# EnCloud TUI Visual Design Specification

This specification translates the supplied visual references into a terminal-safe, implementation-oriented design for EnCloud TUI. It defines presentation only: it does not add cloud capabilities, persistence, or operations beyond the existing configuration and selected-project sync workflow.

## Reference Set

The original visual reference images are intentionally excluded from this repository. This specification preserves their terminal-safe design decisions without requiring local image paths or image assets.

The target feeling is a restrained dark operations console: black or near-black ground, crisp green structure, monospaced content, clear bordered regions, and keyboard-first control. Readability and state clarity take priority over decorative effects.

## Terminal Constraints

The references include graphical glow, blur, gradients, pixel typography, a window chrome, and a large canvas. A terminal cannot dependably reproduce these effects.

| Reference treatment | Terminal-safe approximation |
| --- | --- |
| Neon glow or blur | Bright ANSI green for focal text, plus bold weight and surrounding empty space. Do not simulate blur with repeated characters. |
| Pixel font | Use the terminal's monospace font. Do not require a custom font or render text as pixels. |
| Soft gradients and shadows | Use a uniform dark background with one or two ANSI text intensities. |
| Rounded window and traffic-light controls | Do not render as interactive UI. The terminal emulator owns its window chrome. |
| Fine box-drawing | Prefer Unicode box drawing when supported; provide ASCII borders as a complete fallback. |

Never require true color, a particular font, a graphical terminal, or mouse support. ANSI colors, bold, borders, alignment, and spacing are the dependable visual vocabulary.

## Visual System

### Principles

1. Make the current task and focused control apparent before secondary context.
2. Divide information into named panels; avoid unbounded text blocks.
3. Treat keyboard controls as visible interface elements, not hidden shortcuts.
4. Keep the baseline quiet: bright green identifies focus, success, headings, and actionable affordances rather than filling the screen.
5. Pair every meaningful color with a text label, symbol, or both. Color never conveys critical state alone.

### Semantic Palette

Use named semantic styles rather than color literals in screen renderers. Exact ANSI mappings may adapt to terminal capability while retaining the meanings below.

| Token | Intended ANSI treatment | Meaning and use |
| --- | --- | --- |
| `canvas` | Black or terminal default | Whole-screen background. |
| `surface` | Black or very dark default | Panel interior; do not depend on a separate background color. |
| `text` | White or bright gray | Primary labels, values, and body copy. |
| `muted` | Dim white or gray | Descriptions, metadata, inactive help, and empty-state detail. |
| `accent` | Bright green, bold for headings | Product title, focused border, active control, key label, and action affordance. |
| `border` | Green or dim green | Panel edges and section rules; focused borders use `accent`. |
| `success` | Bright green | Success label with `OK`, `Synced`, or a check icon. |
| `warning` | Yellow | Attention-needed label with explicit text such as `Pending push`. |
| `info` | Cyan when available; otherwise `text` | Informational label with explicit text such as `Local only`. |
| `error` | Red | Error label with explicit text such as `Failed` or `Invalid`. |
| `selection` | Accent text plus focused row border or leading cursor | Current cursor or selected interactive row. |

If a terminal has no color support, preserve hierarchy with bold, dim, border changes, prefixes, and explicit words. For example, render `[OK] Synced`, `[WARN] Pending push`, `[INFO] Local only`, and `[ERROR] Failed`.

### Typography and Spacing

Use the terminal's native monospace face. Hierarchy comes from size only where the terminal supports it poorly, so use weight, case, spacing, and line placement instead.

| Element | Treatment |
| --- | --- |
| Product title | Bold `accent`; the largest available terminal text treatment. Use `EnCloud TUI`. |
| Screen title | Bold `accent`; one level below product title. |
| Panel title | Bold `accent`, optionally preceded by a semantic Unicode icon. |
| Body and values | `text`, regular weight. |
| Helper text | `muted`; one short line per idea. |
| Status text | Semantic color plus an explicit label and, where practical, an icon. |
| Footer guidance | `muted` by default; active key labels use `accent`. |

Use a 1-cell base rhythm: one blank row separates major regions, one cell separates a border from its contents, and two or more cells separate parallel columns. Do not use trailing whitespace for alignment. Align labels and values to stable columns only while width permits.

### Panels and Borders

Panels establish the layout. Each panel has a one-cell inner padding, a title line, a horizontal separator below the title when it contains multiple rows, and a `border` outline. The focused panel or input upgrades its border to `accent`; no other panel should appear equally active.

Preferred Unicode frame:

```text
╭─ Projects ───────────────────╮
│ [x] project-a       [OK] Synced │
╰──────────────────────────────╯
```

ASCII fallback:

```text
+- Projects ------------------+
| [x] project-a     [OK] Synced |
+-----------------------------+
```

Use one border weight only. Avoid nested frames unless the inner element is an input control. A full-width status banner may use a bordered single row without a title.

### Keyboard-Key Treatment

Render each shortcut key as a small outlined token, for example `[enter]`, `[space]`, `[j/k]`, or `[q]`. The token uses `accent` text and border; the adjacent action label uses `text` or `muted`. Separate shortcut groups with a vertical rule or at least two cells.

Key labels must describe the action, not merely repeat the command. For example, `[space] Toggle`, `[p] Pull`, and `[esc] Cancel`. Keep the keyboard rail visible whenever the screen has room. It must not be the only way an action is explained: the focused control and screen copy must still identify the primary action.

## Screen Specifications

### Welcome

**Purpose:** establish identity and provide one unambiguous entry action before configuration.

**Hierarchy**

1. Centered product title: `EnCloud TUI` in bold `accent`.
2. Small attribution line only when product copy requires it; render it in `muted` and do not make it interactive.
3. Bottom-centered primary prompt: `[enter] Start configuration`.

**Layout**

- Use the available viewport with generous vertical whitespace; title cluster is visually centered, and the entry prompt sits in the lower third.
- A single outer frame is optional only at comfortable dimensions. It must not imitate terminal window buttons.
- No operational status, project data, configuration values, or secret-bearing data appears here.

**States and interactions**

| State | Presentation | Input | Result |
| --- | --- | --- |
| Ready | Primary prompt is `accent`; all other copy is quiet. | `enter` | Transition to Initial Configuration. |
| Unsupported/narrow viewport | Keep title and `[enter] Start configuration`; drop outer frame and attribution first. | `enter` | Same transition. |

There is no pointer affordance, animation dependency, or timed auto-transition.

### Initial Configuration

**Purpose:** collect the existing connection settings required to continue, while keeping the token secret.

**Hierarchy**

1. Product title and `Initial configuration` heading at upper left.
2. Step indicator at upper right: `Step 1 of 2` with the next destination stated as `Next: Main dashboard`.
3. One concise explanation that configuration is needed before sync operations.
4. Centered configuration panel containing Server URL, Cloud Token, and Projects in that order.
5. Validation or readiness line below the panel.
6. Keyboard rail and a final action hint at the bottom.

**Components**

| Component | Required rendering | State rules |
| --- | --- | --- |
| Server URL field | Label, one-line helper, separator, bordered text input. | Shows input focus through `accent` border. Validation errors include `[ERROR]` and explanatory text. |
| Cloud Token field | Label, one-line helper, separator, bordered masked input. | Never render the token value, including in preview, validation, logs, or status copy. |
| Projects field | Label, one-line helper identifying comma-separated project IDs, separator, bordered input. | Preserve entered values as appropriate to the existing editor; invalid input has explicit text. |
| Preview action | `[v] Preview config` in the panel footer when this existing interaction is available. | It must show redacted values only. Do not introduce persistence or remote validation solely for preview. |
| Readiness line | Check icon plus `Required before continuing` while incomplete, or a clear validation outcome. | Icon and phrase remain present without color. |
| Actions | `[tab] Next field`, `[enter] Save & continue`, `[esc] Cancel`. | Disable or reject Save & continue until valid, with an explicit reason. |

**Interactions and transitions**

- `tab` advances focus through fields in displayed order; the focused input is the single strongest visual emphasis.
- `enter` requests save only when the existing configuration validation succeeds. On success, transition to Main Dashboard.
- If validation fails, remain on this screen, preserve safe user input, focus the invalid field when practical, and render the error near the field or readiness line.
- `esc` cancels editing. If a valid saved configuration exists, return to Main Dashboard; otherwise exit according to the existing application behavior. Do not add a discard confirmation unless the application already has one.
- Editing configuration from the dashboard uses this same screen and returns to Main Dashboard after save or cancellation.

### Main Dashboard

**Purpose:** let the user select configured projects, understand their state, and invoke the existing pull, push, or status operations.

**Hierarchy**

1. Product title at upper left, followed by `Main dashboard`.
2. Connection summary beneath it: remote/profile/readiness information available from existing state only.
3. Primary Projects panel on the left or top.
4. Workspace status and Selection summary panels alongside it on wide terminals.
5. Full-width outcome banner below the panels.
6. Persistent keyboard rail, then one concise bottom status sentence.

**Projects panel**

- The title uses a folder icon when Unicode is available, otherwise `Projects` alone.
- Each row contains a selection mark, project name, and explicit status label. A focused row adds a cursor or focused border; a selected row uses `[x]`, and an unselected row uses `[ ]`.
- Do not use a green checkmark alone to mean selection. Checkbox state and project sync state are separate concepts.
- Display a footer count such as `6 projects available | 3 selected` using actual state.
- An empty configured-project list must show an explicit empty state and direct the user to configuration; it must not render an empty selectable frame.

**Status panels**

| Panel | Content |
| --- | --- |
| Workspace status | Existing remote, profile, connection outcome, last sync if known, configured-project count, and active operation. Unknown values render `Unknown` or `None`, not invented data. |
| Selection summary | Count selected projects and each known category. Every category has a text label, number, and optional semantic color. |

Use explicit status semantics in all panels and rows:

| Semantic state | Required non-color representation |
| --- | --- |
| Synced/success | `[OK] Synced` or a check icon plus `Synced` |
| Pending push/warning | `[WARN] Pending push` |
| Local-only/informational | `[INFO] Local only` |
| Not synced/neutral | `[ ] Not synced` |
| Failed/error | `[ERROR] Failed` plus a concise cause when safely available |
| Running | `[RUNNING] <operation>`; never imply completion |
| Cancelled | `[CANCELLED] <operation>` |

**Outcome banner**

Place one bordered, full-width status banner under the content panels. It reports the latest safe outcome, for example `[OK] Workspace loaded successfully`, `[ERROR] Operation failed`, or `[CANCELLED] Pull cancelled`. It is a summary, not an audit log, and must not expose tokens, command arguments, or environment values.

**Keyboard rail and interactions**

Render the existing operations in a stable order:

```text
[j/k] Navigate  [space] Toggle  [a] Select all  [p] Pull  [u] Push
[s] Status  [c] Config  [r] Refresh  [q] Quit
```

- `j/k` changes only the focused project row.
- `space` toggles only the focused project.
- `a` selects or clears all configured projects according to current behavior.
- `p`, `u`, and `s` request the existing pull, push, and status flows for the current selection. Empty selection must be rejected with an explicit message.
- `c` opens Initial Configuration in edit mode.
- `r` refreshes only if the existing application supports refresh; otherwise omit this key rather than introducing behavior.
- `q` exits only when no operation is active, following existing safe cancellation behavior for active work.

**Transitions**

| From | Event | To | Visual outcome |
| --- | --- | --- | --- |
| Welcome | `enter` | Initial Configuration | Replace centered splash with the form; focus Server URL. |
| Initial Configuration | valid save | Main Dashboard | Render saved configuration summary and a success banner. |
| Main Dashboard | `c` | Initial Configuration | Preserve dashboard context; focus the first editable field. |
| Main Dashboard | operation request | existing confirmation/running flow | Keep selection visible where available; banner and status text identify pending/running work. |
| Existing running flow | completion, failure, or cancellation | Main Dashboard | Update known project outcomes and banner with explicit status text. |

No transition relies on fade, glow, animation, or color change alone.

## Responsive and Fallback Rules

| Condition | Required behavior |
| --- | --- |
| Wide terminal | Dashboard uses a primary Projects panel plus side-by-side Workspace status and Selection summary panels. |
| Medium terminal | Stack side panels below Projects. Keep project selection, status text, and primary action visible. |
| Narrow terminal | Use one column. Truncate project names before status labels, shorten helper copy, and wrap the keyboard rail into rows. Never hide the active input, focused row, primary action, or explicit status label. |
| Very short terminal | Prioritize current title, focused content, outcome message, and primary key hint. Omit decorative outer frames, long descriptions, and secondary summaries first. |
| No Unicode | Use ASCII frames, `[x]`/`[ ]`, `[OK]`, `[WARN]`, `[INFO]`, `[ERROR]`, and `[RUNNING]`. |
| No color | Preserve status with the explicit labels above plus bold/dim hierarchy and borders. |
| Limited color | Use `text`, `muted`, `accent`, and `error` first; do not rely on cyan or yellow availability. |

Do not hard-code a reference-image width. Measure the terminal viewport and allocate columns from available cells. Ellipsize long text safely; do not allow it to overwrite adjacent status or key labels.

## Implementation Acceptance Criteria

- [ ] `encloud-tui` renders all visual styling through centralized semantic style tokens rather than scattered color literals.
- [ ] Welcome, Initial Configuration, and Main Dashboard each render the hierarchy and primary action defined here.
- [ ] The terminal UI does not require graphical blur, glow, gradients, pixel fonts, mouse input, or terminal window chrome.
- [ ] Unicode frames and icons have an ASCII-equivalent rendering path.
- [ ] Every critical sync, validation, connection, and operation state has an explicit text label or symbol in addition to any color.
- [ ] The Cloud Token remains masked and is absent from previews, banners, logs, validation output, and dashboard values.
- [ ] Focus is visible through a unique border, cursor, or selection treatment on every interactive screen.
- [ ] Configuration keeps its existing three fields, validation, save path, and safe cancellation behavior; the design adds no remote validation or persistence feature.
- [ ] Dashboard selection and status remain distinct, and project rows retain both at all supported widths.
- [ ] Width-constrained layouts preserve the active screen's primary action, focused control, and status label before decorative content.
- [ ] Existing pull, push, status, configuration, refresh when supported, cancellation, and quit commands retain their documented behavior; the design does not invent runtime operations.
