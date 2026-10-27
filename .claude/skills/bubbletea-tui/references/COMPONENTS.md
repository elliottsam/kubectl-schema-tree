# Components

```
tui.Model (root)
├── treeview.Model     rows from tree.State, cursor, scroll offset
├── detailpane.Model   viewport for the selected node
├── searchbar.Model    textinput, active only in search mode
└── statusbar.Model    help (from KeyMap) and errors
```

- Root owns focus (`FocusTree | FocusDetail | FocusSearch`) and routes key
  messages to the focused child only; window-size messages go to all.
- Children take their size from the root via `SetSize(w, h)`; they never query
  the terminal.
- Loading: expanding an unloaded group/version returns a `tea.Cmd` that calls
  the `SchemaSource` and yields `schemaLoadedMsg{gv, nodes, err}`. The row shows
  a spinner (`bubbles/spinner`) until the message arrives. Cancel via context
  when the program quits.
- The `SchemaSource` interface is declared in `tui`, minimal, consumer-defined.
- Errors become a status-bar message or a greyed row, never a panic.
- Search is synchronous in `Update`: about 5 ms for 50k nodes. Do not move it
  to a goroutine; tree nodes expand lazily and are single-goroutine.
- Starting a search loads every unloaded group so results cover the whole API.
- `Config.Open` selects a resource once candidate groups settle, preferring the
  core group; plurals match (`pods`).
