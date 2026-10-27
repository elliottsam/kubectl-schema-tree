# Keymap

| Key | Action |
| --- | --- |
| up / k, down / j | Move up / down |
| right / l | Expand, or step into first child |
| left / h | Collapse, or jump to parent |
| g / G | Top / bottom |
| ctrl+d / ctrl+u | Half page down / up |
| enter / space | Toggle expand |
| E / C | Expand all (depth-limited) / collapse all |
| tab | Switch pane focus |
| / | Search; n / N next / previous; esc clears |
| ? | Toggle full help |
| q / ctrl+c | Quit |

Define with `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` in a single
`KeyMap` struct implementing `help.KeyMap`. Test that every binding in the
table above is reachable.
