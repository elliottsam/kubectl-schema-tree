# kubectl schema-tree

Browse the API schemas of your cluster in an interactive tree.

`kubectl schema-tree` reads the OpenAPI v3 documents your cluster serves and
shows every resource and field as a navigable, colourised tree, with the field
documentation next to it. Custom resources are included automatically.

<!-- Generate the demo with `vhs demo.tape`, then uncomment:
![demo](docs/demo.gif)
-->

## Install

With [Krew](https://krew.sigs.k8s.io/) (once published to the index):

```sh
kubectl krew install schema-tree
```

From a release: download the archive for your platform from the
[releases page](https://github.com/elliottsam/kubectl-schema-tree/releases) and
put `kubectl-schema_tree` somewhere on your `PATH`.

From source:

```sh
go install github.com/elliottsam/kubectl-schema-tree/cmd/kubectl-schema_tree@latest
```

Use the full path including `/cmd/kubectl-schema_tree`. The repository root has
no `main` package, so `go install github.com/elliottsam/kubectl-schema-tree@latest`
fails with "module ... found, but does not contain package".

kubectl discovers the plugin through the binary name, so it must be called
`kubectl-schema_tree` (the underscore becomes the dash in `schema-tree`).

## Usage

```sh
kubectl schema-tree                              # browse everything
kubectl schema-tree deployment                   # open at a resource
kubectl schema-tree apps/v1/Deployment --expand-depth 2
kubectl schema-tree --context staging crontab    # any kubectl connection flag works
```

Requires a cluster that serves `/openapi/v3` (Kubernetes 1.27 or newer).

| Flag | Meaning |
| --- | --- |
| `--expand-depth N` | Levels to pre-expand below the opened resource. |
| `--no-color` | Disable colours. `NO_COLOR` is also honoured. |
| `--debug` | Write a log to `<user cache dir>/kubectl-schema-tree/debug.log`. |

All the usual kubectl flags (`--context`, `--kubeconfig`, `--as`, ...) are
supported.

## Keys

| Key | Action |
| --- | --- |
| `↑` `k` / `↓` `j` | Move up / down |
| `→` `l` | Expand, or step into the first child |
| `←` `h` | Collapse, or jump to the parent |
| `g` / `G` | Top / bottom |
| `ctrl+d` / `ctrl+u` | Half page down / up |
| `enter` `space` | Toggle expansion |
| `E` / `C` | Expand all (limited depth) / collapse all |
| `tab` | Switch between the tree and the detail pane |
| `/` | Fuzzy search kinds and field paths |
| `n` / `N` | Next / previous match |
| `esc` | Clear the search |
| `?` | Help |
| `q` `ctrl+c` | Quit |

The selected row is highlighted with a full-width background (reverse video
without colour). Required fields carry a `*`, deprecated fields are struck
through, and recursive types such as `JSONSchemaProps` stop with `↻` instead of
expanding forever.

Searching loads every API group first, so the first search on a large cluster
takes a moment.

## Development

```sh
make build              # bin/kubectl-schema_tree
make test               # go test -race -shuffle=on -cover ./...
make cover              # enforce the per-package coverage floor
make lint               # golangci-lint
make fuzz               # fuzz the schema parser
make kind-up            # local cluster for integration tests and fixtures
make test-integration
make fixtures           # refresh trimmed OpenAPI fixtures from a cluster
make golden-update      # rewrite TUI golden files, review the diff
```

Layout:

```text
cmd/kubectl-schema_tree   wiring only
internal/cli              flags, cluster connection, debug logging
internal/schema           OpenAPI v3 loader and lazy schema nodes
internal/tree             pure navigation state
internal/search           fuzzy matching
internal/tui              Bubble Tea model, views, key map, styles
internal/schematest       fakes and documents for tests
internal/integration      tests against a real cluster (build tag)
```

## Releasing

Every merge to `main` creates the next version tag and publishes a release; there
is no manual tagging. The bump is read from a line in the pull request
description:

```text
Release: patch   (default)
Release: minor
Release: major
```

The pull request template includes the line. The most significant bump wins if
several are present, and text inside `<!-- -->` comments is ignored. The
version logic is in `internal/release`, run by `.github/workflows/tag.yml`,
which then calls `release.yml` (GoReleaser).

Contributor and agent conventions live in [CLAUDE.md](CLAUDE.md) and
`.claude/skills/`.
