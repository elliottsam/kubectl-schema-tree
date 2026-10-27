---
name: schematree-testing
description: >
  Project testing conventions for kubectl-schema-tree. Use whenever writing or
  changing Go tests, golden files, fuzz targets, benchmarks, Bubble Tea model
  tests, view snapshots, Kubernetes OpenAPI fixtures or fakes, or when
  checking coverage and the finishing test commands. Extends go-style-guide.
license: Apache-2.0
metadata:
  version: "1.0.0"
---

# schematree-testing

Extends `go-style-guide` (read its `references/TESTING.md` first). Where the two
differ, `go-style-guide` wins unless this file says "project rule".

## TL;DR

- Table-driven, `t.Run`, `t.Parallel()`, black-box `package foo_test`.
- Stdlib assertions plus `go-cmp` for struct diffs. No testify (project rule).
- Fakes injected through `Config`; use `internal/schematest` (fake discovery plus
  small documents) instead of writing a new fake per package.
- TUI: test `Update` directly first, snapshot `View()` second. No teatest; the
  model is driven with messages and `View().Content` is compared directly.
- Golden files live in `testdata/`, rewritten with `-update`, never by hand.
- Parsers and resolvers get a fuzz target. Hot paths get a `b.Loop()` benchmark.
- Bug fixes start with a failing test.

## Quick Rules Table

| Topic | Rule | Reference |
| --- | --- | --- |
| Bubble Tea models | Drive `Update` with messages, assert state, then snapshot | `references/TUI-TESTING.md` |
| Kubernetes fakes | `schematest.Discovery` over small docs; real trimmed fixtures in `schema/testdata` | `references/K8S-FAKES.md` |
| Golden files | `-update` flag; compare `ansi.Strip`ped views unless testing colour | `references/GOLDEN-FILES.md` |
| Coverage | >=85% outside `cmd/`; a signal, not proof | `references/COVERAGE.md` |
| Finishing | `go test -race -shuffle=on ./...` and `golangci-lint run` | this file |

## Finishing checks

```sh
gofmt -l .            # must print nothing
go vet ./...
golangci-lint run
go test -race -shuffle=on -cover ./...
```

Run `make test` for the same thing. Integration tests need
`-tags integration` and a cluster (`make test-integration`).

## Common Pitfalls

- Asserting on raw ANSI output when the plain text would do (strip with `ansi.Strip`).
- Golden files edited by hand or regenerated without reading the diff.
- Tests that need a live cluster but are not behind `//go:build integration`.
- Sleeping in tests instead of driving messages or using fake clocks.
- Snapshotting a view when a state assertion would be clearer.
- Leaking goroutines from `tea.Cmd` helpers; every test must finish cleanly.
- Sharing one `tree.Node` / `schema.Node` across parallel subtests: nodes expand
  lazily and are single-goroutine. Build a fresh tree per subtest.
- Executing spinner or cursor-blink commands in a test harness; they sleep. Drop
  `spinner.TickMsg` and disable blink.

## Reference Loading

- [references/TUI-TESTING.md](references/TUI-TESTING.md): Bubble Tea model tests and the harness.
- [references/K8S-FAKES.md](references/K8S-FAKES.md): OpenAPI fixtures and discovery fakes.
- [references/GOLDEN-FILES.md](references/GOLDEN-FILES.md): golden file workflow and colour profiles.
- [references/COVERAGE.md](references/COVERAGE.md): coverage gate and what to test.
