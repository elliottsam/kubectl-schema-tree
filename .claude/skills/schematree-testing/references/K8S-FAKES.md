# Kubernetes fakes and fixtures

## Small documents: `internal/schematest`

- `schematest.NewDiscovery()` is a `schema.Discovery` serving two tiny groups
  (`apis/apps/v1`: Widget, Gadget; `api/v1`: Pod with a self-referencing spec).
- Inject failures: `PathsErr` (whole index), `DocErr` (every document),
  `DocErrFor[path]` (one group). `Fetches()` counts document fetches.
- `schematest.Loader(tb, d)` and `schematest.Resources(tb, l, path)` fail the test
  on error so call sites stay short.
- Add a document there when a test needs a new shape; keep it small.

## Real documents: `internal/schema/testdata`

- Trimmed captures of `api/v1` (ConfigMap, Service), `apis/apps/v1` (Deployment)
  and `apis/apiextensions.k8s.io/v1` (CustomResourceDefinition, which contains
  the recursive `JSONSchemaProps`).
- Refresh with `make fixtures` against a kind cluster (`make kind-up`), then
  commit the diff and say which Kubernetes version produced it.
- `schema_test.go` has its own fake that serves these files by path.

## Errors

- Assert with `errors.Is` against sentinels (`ErrOpenAPIV3Unsupported`,
  `ErrForbidden`, `ErrNotFound`, `ErrInvalidConfig`); never match error text.
- Build API errors with `apierrors.NewNotFound` / `NewForbidden`.

## Fuzzing

`FuzzLoadDocument` feeds arbitrary bytes through the loader and walks the result.
Findings are saved under `testdata/fuzz/` and become permanent regression seeds;
commit them. (It found a `MustCreateRef` panic on a schema name containing `%`.)
