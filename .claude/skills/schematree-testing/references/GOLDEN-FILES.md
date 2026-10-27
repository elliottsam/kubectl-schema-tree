# Golden files

- Location: `testdata/<test-name>.golden` next to the test.
- Flag: `var update = flag.Bool("update", false, "rewrite golden files")`.
- Update with `make golden-update` (or `go test ./... -update`), then **read the
  diff** before committing.
- Goldens store `ansi.Strip`ped views so they are readable and stable. Colour is
  asserted separately on exact SGR strings, not through goldens.
- Normalise anything non-deterministic (spinner frames, durations) before
  comparing; do not strip meaningful whitespace.
- Fail with a `go-cmp` diff, not a bare "mismatch".
