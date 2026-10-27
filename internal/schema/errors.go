package schema

import "errors"

var (
	// ErrOpenAPIV3Unsupported is returned when the cluster does not serve
	// /openapi/v3, which requires Kubernetes 1.27 or newer.
	ErrOpenAPIV3Unsupported = errors.New("cluster does not serve OpenAPI v3 (requires Kubernetes 1.27+)")

	// ErrForbidden is returned when RBAC prevents reading a schema document.
	ErrForbidden = errors.New("forbidden")

	// ErrNotFound is returned when a requested group/version or resource kind
	// does not exist.
	ErrNotFound = errors.New("not found")

	// ErrInvalidConfig is returned by NewLoader when the Config is unusable.
	ErrInvalidConfig = errors.New("invalid schema loader config")
)
