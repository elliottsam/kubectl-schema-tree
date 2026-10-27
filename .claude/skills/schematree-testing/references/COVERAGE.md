# Coverage

- Gate: >=85% statement coverage per package outside `cmd/`, enforced in CI.
- Coverage is directional (see go-style-guide TESTING.md). A covered line with
  no assertion is not tested.
- Prioritise: error branches, boundary conditions (empty tree, cursor at ends),
  recursive schemas, misuse of constructors (`Config.Validate`).
- Report with `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out`.
