# Styles

- One file, `internal/tui/styles.go`, builds a `Styles` struct with
  `NewStyles(dark, noColor)`. Dark/light is `lipgloss.LightDark(isDark)`, where
  `isDark` comes from `tea.BackgroundColorMsg` (default dark); on a new value the
  model calls `restyle()`. Views receive `Styles`; they do not construct styles.
- Palette: adaptive light/dark pairs. Type colours: object, array, map, string,
  number, bool, plus muted (descriptions, deprecated) and error.
- Required fields: `*` suffix and bold. Deprecated: faint.

## Selected node highlight (requirement)

- The cursor row is rendered, then padded to the full tree-pane width, then
  styled with a background. Highlighting only the text is a bug.
- Tree focused: accent background. Detail focused: muted background, so the
  position stays visible.
- Foreground colours stay legible on the bar (checked on light and dark).
- `NO_COLOR` or `--no-color`: use reverse video so the bar still shows. Also reset
  third-party component styles (`help.Styles{}`, textinput styles) or they keep
  emitting colour.
- Search match characters keep underline/bold on the selected row.
- Width maths uses `lipgloss.Width`, never `len`.
