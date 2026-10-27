# Cluster error mapping

| Condition | Detect | Surface |
| --- | --- | --- |
| No OpenAPI v3 (< 1.27) | 404 on `/openapi/v3` | `ErrOpenAPIV3Unsupported`; fatal message explaining minimum version |
| RBAC forbidden on a group/version | `apierrors.IsForbidden` | `ErrForbidden`; greyed row with reason |
| Timeout / cancelled | `errors.Is(err, context.DeadlineExceeded)` / `context.Canceled` | status-bar error, retry on re-expand |
| Malformed document | JSON/spec3 decode error | wrapped error naming the group/version |
| Unreachable cluster | `net.Error` | fatal message with the context name |

Always wrap with the group/version: `fmt.Errorf("load %s: %w", gv, err)`.
