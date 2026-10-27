# kubectl-schema-tree

A kubectl plugin (`kubectl schema-tree`) written in Go. It connects to a
Kubernetes cluster, reads OpenAPI v3 schemas and lets the user explore them in a
Bubble Tea tree TUI.

- Module: `github.com/elliottsam/kubectl-schema-tree`
- Binary: `kubectl-schema_tree`

## Skills (read before working)

| Skill | Use for |
| --- | --- |
| `.claude/skills/go-style-guide` | All Go code. Vendored from madflojo/go-style-agent-skill v1.0.1, do not edit; see its `NOTICE`. |
| `.claude/skills/schematree-testing` | Tests, golden files, fuzzing, benchmarks, coverage. |
| `.claude/skills/bubbletea-tui` | Bubble Tea models, keys, styles, selected-row highlight. |
| `.claude/skills/kubernetes-client` | client-go, discovery, OpenAPI v3, error mapping. |

Trigger prompts for the project skills are in `.claude/skills/trigger-evals.json`.

## Execution protocol (every task)

1. Inspect the repo before proposing APIs or moving files.
2. Define the contract first: package boundary, `Config`, concrete return type,
   sentinel errors, seams.
3. Write tests early (table-driven; fuzz for parsers; benchmarks for hot paths).
4. Make the smallest maintainable change.
5. Run the finishing checks below.
6. Verify docs, comments, defaults and error behaviour match the code.

## Architecture rules

- `tui` -> `tree`, `schema`, `search`; never the reverse.
- `schema` never imports Bubble Tea; nothing outside `schema` sees kube-openapi types.
- Library packages do not log. Only `internal/cli` owns a `slog` logger (file
  output via `--debug`), because stdout belongs to the TUI.
- `Config` in, concrete struct out. No functional options.
- `main.go` is wiring only.

## Commands

```sh
make build              # bin/kubectl-schema_tree
make test               # go test -race -shuffle=on -cover ./...
make test-integration   # needs a kind cluster, -tags integration
make golden-update      # rewrite golden files, then review the diff
make lint               # golangci-lint run
make fuzz               # short fuzz run on parsers
make bench
```

## Pull requests and releases

`main` is protected: every change goes through a pull request and the
`CI passed` check. Merging creates and publishes the next release tag
automatically, so every PR description must contain one line, `Release: patch`,
`Release: minor`, `Release: major` or `Release: none` (the template has it; patch is
the default). Choose minor for new user-visible features, major for breaking
changes, and none for changes that should not ship (docs, CI, tests only).

## Package map

```text
cmd/kubectl-schema_tree   wiring only
internal/cli              cobra command, kubectl flags, debug log file, Connect/RunProgram seams
internal/schema           Loader, lazy Node tree, sentinel errors
internal/tree             pure navigation state (expand, cursor, filter)
internal/search           fuzzy matching over loaded nodes
internal/release          next version tag from the PR description (used by hack/nextversion)
internal/tui              Bubble Tea v2 model, views, KeyMap, Styles
internal/schematest       fake discovery and small documents for tests
internal/integration      //go:build integration, needs KUBECTL_SCHEMATREE_CONTEXT
hack/fixtures             go run tool that captures trimmed OpenAPI fixtures
```

## Product requirements worth remembering

- The selected tree node has a full-width background highlight (focus-aware,
  reverse video under `NO_COLOR`).
- Navigation: arrow keys and `hjkl`. Fuzzy search with `/`. Detail pane.
- OpenAPI v3 only (Kubernetes >= 1.27), loaded lazily per group/version.
