---
name: kubernetes-client
description: >
  How kubectl-schematree talks to Kubernetes. Use whenever touching
  client-go, cli-runtime ConfigFlags, discovery, the OpenAPI v3 endpoint,
  kube-openapi spec3, $ref resolution, CRD schemas, x-kubernetes extensions,
  or mapping cluster errors such as RBAC 403 and pre-1.27 clusters.
license: Apache-2.0
metadata:
  version: "1.0.0"
---

# kubernetes-client

Extends `go-style-guide` (config, errors, interfaces).

## TL;DR

- Connect via `genericclioptions.ConfigFlags` so every kubectl flag works.
- Discovery -> `OpenAPIV3().Paths()` lists group/versions cheaply; fetch each
  document only when needed.
- Convert to the internal `schema.Node` immediately; nothing outside `schema`
  sees kube-openapi types.
- Resolve `$ref` lazily with per-path cycle detection.
- Map failures to sentinel errors: `ErrOpenAPIV3Unsupported`, `ErrForbidden`,
  `ErrNotFound`, `ErrInvalidConfig`. Wrap others with `%w`. Recursion is not an
  error; it is `Node.Recursive`.
- `schema` does not log and does not import Bubble Tea.

## Quick Rules Table

| Topic | Rule | Reference |
| --- | --- | --- |
| OpenAPI v3 | Lazy per-group/version documents, GVK extension index | `references/OPENAPI-V3.md` |
| Errors | RBAC, version, timeout mapping | `references/RBAC-ERRORS.md` |
| Constructors | `schema.NewLoader(schema.Config{...}) (*Loader, error)` | go-style-guide CONFIG.md |

## Common Pitfalls

- Fetching the whole v2 swagger document at startup.
- Passing `context.Background()` instead of the program context.
- Leaking `spec3` types through exported APIs.
- Infinite recursion on `JSONSchemaProps`-style schemas.
- String-matching error text instead of `apierrors.IsForbidden` / `errors.Is`.

## Reference Loading

- [references/OPENAPI-V3.md](references/OPENAPI-V3.md)
- [references/RBAC-ERRORS.md](references/RBAC-ERRORS.md)
