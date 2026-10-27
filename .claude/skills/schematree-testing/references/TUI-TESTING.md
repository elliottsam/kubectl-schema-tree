# Testing Bubble Tea models

This project uses Bubble Tea v2 (`charm.land/bubbletea/v2`). Tests drive the model
directly; there is no teatest dependency.

## Order of preference

1. **State tests.** Send `tea.KeyPressMsg` / `tea.WindowSizeMsg` to `Update` and
   assert on the exported accessors (`Selected`, `Focus`, `Filtering`, `Err`).
2. **Command tests.** `Update` returns a `tea.Cmd`; run it and feed the message
   back. Never perform I/O in `Update` itself.
3. **Snapshot tests.** `View().Content` at a fixed size, `ansi.Strip`ped, compared
   with a golden file. Use for layout, not logic.

## The harness (`internal/tui/helpers_test.go`)

- `newHarness(t, opts...)` builds the model over `schematest` documents at 100x30
  and runs `Init` to completion.
- `collect(cmd)` executes a command, fans `tea.BatchMsg` out concurrently, and
  drops `spinner.TickMsg` so loading cannot loop or sleep.
- `h.key("j", "ctrl+d", "enter", "esc", "tab", "space", "G")` sends keys by the
  same names the bindings use; `h.typeText("abc")` types into the search box.
- `h.plain()` is the stripped view; `h.raw()` keeps escapes for colour assertions.

## Rules

- Always send `tea.WindowSizeMsg` before asserting on a view.
- Colour: lipgloss v2 always emits full-colour escapes; the terminal downsamples
  at output. Assert on exact truecolor SGR strings (for example the focused
  selection background `48;2;31;74;122`) rather than pinning a profile.
- Light theme: send `tea.BackgroundColorMsg{Color: ...}`; dark is the default.
- The selected-row tests must prove the bar is full width: every styled run in
  the tree pane section of the row carries the background.
- Find a row by looking only at the tree-pane section of a line. The detail pane
  repeats names, and matching the whole line selects the wrong row.
- Cover: resize and every size from 120x40 down to 30x6 (`TestLayoutFitsTerminal`
  checks exact width and height), loading, forbidden group, fatal error, empty
  tree, search with no matches, narrow mode.
- Drive `Update` from one goroutine. Nodes expand lazily and are not safe for
  concurrent use.

## Pattern

```go
func TestNavigation(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name string
        keys []string
        want string
    }{
        {"down", []string{"j"}, "apps/v1"},
        {"clamped-at-top", []string{"k"}, "v1"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            h := newHarness(t)
            h.key(tt.keys...)
            if got := h.selected(); got != tt.want {
                t.Fatalf("selected = %q, want %q", got, tt.want)
            }
        })
    }
}
```
