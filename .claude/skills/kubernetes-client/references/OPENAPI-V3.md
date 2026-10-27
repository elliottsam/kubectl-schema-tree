# OpenAPI v3

- Paths: `discovery.OpenAPIV3().Paths()` returns `map[string]openapi.GroupVersion`
  keyed like `apis/apps/v1`, `api/v1`.
- Document: `gv.Schema("application/json")` returns bytes; parse once, cache by
  path plus the server hash.
- Resource roots: schemas whose `x-kubernetes-group-version-kind` extension is
  set. Index GVK -> schema name.
- `$ref` form: `#/components/schemas/<name>`. Resolve inside the same document.
- Useful extensions carried onto `schema.Node`: `x-kubernetes-list-type`,
  `x-kubernetes-list-map-keys`, `x-kubernetes-map-type`,
  `x-kubernetes-int-or-string`, `x-kubernetes-preserve-unknown-fields`,
  `x-kubernetes-embedded-resource`.
- Node kinds: object, array, map (additionalProperties), primitive, ref.
- Recursion: track the chain of schema names for the current path; on repeat
  return a node flagged `Recursive` instead of expanding.

## Gotchas found against real clusters

- Property references arrive as `allOf: [{$ref}]` plus a `description`, not as a
  bare `$ref`. Treat a single-element `allOf` ref as the reference and keep the
  outer description.
- `x-kubernetes-list-type`, `list-map-keys` and defaults live on the outer
  property schema, not on the referenced item type. Read the outer schema first
  and fall back to the resolved one.
- Built-in `IntOrString` is marked by `format: int-or-string` (with `oneOf`), not
  by `x-kubernetes-int-or-string`. Accept both.
- `Paths()` also lists non-schema keys (`apis`, `apis/apps`, `.well-known/...`,
  `version`). Keep only `api/<v>` and `apis/<g>/<v>`.
- Resource roots are schemas with a group-version-kind extension matching the
  document. Skip `*List`, `DeleteOptions`, `WatchEvent`, `Status`, `Patch`.
- Never build a `spec.Ref` with `MustCreateRef` from document-controlled text; it
  panics on bad escapes. Build the root node from the schema directly.
- The kuberc on the dev machine can break `kubectl get --raw`; use the Go
  fixture tool (`make fixtures`) instead.
