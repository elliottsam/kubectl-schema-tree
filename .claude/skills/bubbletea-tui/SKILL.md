---
name: bubbletea-tui
description: >
  Conventions for building the kubectl-schematree terminal UI with Bubble Tea,
  Bubbles and Lip Gloss. Use whenever writing or changing a tea.Model, Update,
  View, tea.Cmd, key bindings, styles, the tree or detail panes, search input,
  focus handling, window resizing, or the selected-node highlight.
license: Apache-2.0
metadata:
  version: "1.0.0"
---

# bubbletea-tui

Extends `go-style-guide`. Tests: see `schematree-testing`.

Uses Bubble Tea **v2** (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`,
`charm.land/lipgloss/v2`): `View()` returns `tea.View`, key presses are
`tea.KeyPressMsg`, and light/dark comes from `tea.BackgroundColorMsg`.

## TL;DR

- `Update` is pure and fast. All I/O (cluster calls) happens in a `tea.Cmd` that
  returns a message.
- Compose small child models; the root routes messages by focus.
- Key bindings are `bubbles/key` bindings in one `KeyMap`; help is generated.
- All styles live in one `styles.go`. No inline `lipgloss.NewStyle()` in views.
- The selected node has a full-width background highlight. This is a product
  requirement, not decoration.
- Never write to stdout/stderr while the program runs; log to a file.

## Quick Rules Table

| Topic | Rule | Reference |
| --- | --- | --- |
| Components | Child models expose `Update`/`View`/`SetSize`/`Focus`/`Blur` | `references/COMPONENTS.md` |
| Keys | One `KeyMap`, `ShortHelp`/`FullHelp`, vim + arrow keys | `references/KEYMAPS.md` |
| Styles | Adaptive colours, `NO_COLOR` fallback, selected-row bar | `references/STYLES.md` |
| Concurrency | No goroutines in models; use `tea.Cmd` | this file |

## Common Pitfalls

- Blocking in `Update` (network, disk, sleep).
- Mutating shared state from a `tea.Cmd` closure.
- Highlighting only the label text instead of padding to full pane width.
- Forgetting `tea.WindowSizeMsg` propagation to children.
- Measuring strings with `len` instead of `lipgloss.Width` / `ansi.StringWidth`.
- Hard-coded colours without an adaptive pair or `NO_COLOR` fallback.
- Printing with `fmt.Println` inside the TUI.

## Reference Loading

- [references/COMPONENTS.md](references/COMPONENTS.md): model composition, focus, sizing, loading.
- [references/KEYMAPS.md](references/KEYMAPS.md): the key table and help.
- [references/STYLES.md](references/STYLES.md): palette, type colours, selection highlight.
