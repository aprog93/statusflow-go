# StatusFlow Go

**StatusFlow Go** is a small Go library for consistent JSON HTTP responses,
safe error exposure, and stable application error codes.

- **Author:** Alfred Fonher — `aprog93`
- **Status:** Initial development; the public API is intentionally small and
  should be reviewed before adopting pre-1.0 releases in production.
- **Classification:** Go library/module and infrastructure HTTP SDK.
- **Runtime dependency:** Go standard library only (`net/http`, `encoding/json`).

## What it is—and is not

StatusFlow Go sits at the boundary between application code and `net/http`. It
standardizes response envelopes, translates known errors into safe responses,
and keeps internal causes available to server-side code.

It is **not** an executable application, HTTP server, plugin, router, or web
framework. It does not provide authentication, persistence, request
validation, logging, tracing, metrics, middleware orchestration, or business
rules.

## Installation

From a Go module:

```bash
go get github.com/aprog93/statusflow-go
```

Import the package using its module path. The package name is `statusflow`:

```go
import "github.com/aprog93/status-flow/golang-ver"
```

The module currently declares Go 1.22 compatibility. Pin a released module
version in production and review release notes before upgrading.

## Quick start

```go
package main

import (
	"net/http"

    "github.com/aprog93/statusflow-go"
)

func users(w http.ResponseWriter, r *http.Request) error {
	return statusflow.WriteJSON(w, statusflow.NewSuccessResponse(
		http.StatusOK, "users listed", []string{"Ada", "Linus"},
	))
}

func main() {
	http.Handle("/users", statusflow.Adapt(users))
	_ = http.ListenAndServe(":8080", nil)
}
```

`Adapt` is optional. A conventional `http.Handler` can call `WriteJSON` and
`WriteError` directly, and any external router can register the resulting
handler without importing a router-specific adapter.

## Architecture and limits

The package has three focused layers:

1. **Contract:** `Response` and its constructors define the JSON envelope.
2. **Error boundary:** `HTTPError` and `ApplicationError` separate public
   response data from private causes.
3. **Transport integration:** `WriteJSON`, `WriteError`, and `Adapt` connect
   the contract to `net/http`.

The library owns response construction and serialization only. The application
owns routing, request parsing, validation, authorization, observability,
logging, retries, and domain decisions. See [`docs/architecture.md`](docs/architecture.md)
for the design rationale without repeating the complete API reference.

## API and JSON contract

### Response envelope

```json
{"success":true,"message":"users listed","code":200,"data":["Ada","Linus"]}
```

`Response` exposes:

| JSON field | Meaning |
| --- | --- |
| `success` | `true` only for 2xx status codes |
| `message` | Public human-readable message |
| `code` | The HTTP status code, from 100 through 599 |
| `data` | Optional success payload; omitted for non-2xx responses |
| `details` | Optional caller-supplied public error details |
| `error_code` | Optional validated application/domain code |

Successful responses use a 2xx `code`, `success: true`, and may contain `data`.
Error responses use a 4xx/5xx `code`, `success: false`, never contain `data`,
and may contain `details` or `error_code`:

```json
{"success":false,"message":"invalid user","code":422,"details":{"field":"email"},"error_code":"validation_error"}
```

`error_code` is serialized for `ApplicationError`, not for ordinary
`Response` values or legacy `HTTPError` responses. HTTP status and application
code are deliberately separate: use a valid HTTP status plus a stable domain
code rather than inventing HTTP status values.

### Errors

Use `NewHTTPError` when a deliberate public status/message is needed. Use
`NewHTTPErrorWithDetails` only with details that are intentionally public. Use
`NewApplicationError` for domain failures; built-in codes include
`validation_error`, `resource_not_found`, `conflict`, `invalid_state`, and
`internal_error`. Custom codes must pass `IsValidErrorCode` (lowercase ASCII
letters, digits, and underscores; a letter first; maximum 64 characters).

`HTTPError.Unwrap` retains the internal cause for `errors.Is` and
`errors.As`. `WriteError` never serializes that cause. Unknown errors become a
generic 500 response. Invalid error statuses fall back to 500.

The catalog provides common messages in English (`en`) and Spanish (`es`).
`AdaptWithLanguage` and the `WithLanguage` constructors select a language;
unsupported languages safely fall back to English.

## `net/http` and external routers

StatusFlow Go has no mandatory router dependency:

```go
router.Handle("/users", statusflow.Adapt(users))
```

This registration pattern works with any router that accepts `http.Handler`.
For router-specific handler signatures, call `WriteJSON` or `WriteError` in
the adapter owned by your application. Do not write a second response after
the HTTP headers have been committed.

`WriteJSON` validates the status before writing, encodes the complete JSON body
before committing headers, sets `Content-Type` to
`application/json; charset=utf-8`, and writes no body for 1xx, 204, or 304
responses.

## Security model

- Put only deliberately public, JSON-serializable values in `data` and
  `details`.
- Keep database errors, credentials, stack traces, and other internals in the
  private `cause`; log them through the application's server-side policy.
- Never assume this package redacts arbitrary application data or validates
  business schemas.
- Treat messages and details as part of the public API and review them for
  sensitive information.

## Testing and quality

Tests cover catalog and language fallbacks, constructors, status validation,
JSON serialization, cause redaction, `net/http` integration, and end-to-end
requests through `httptest.NewServer`.

From `golang-ver`, run:

```bash
gofmt -w ./*.go
go vet ./...
go test ./...
```

## Versioning

The module follows Go module versioning and aims for semantic-version-
compatible releases. Before 1.0, API compatibility is not a promise: review
the changelog or diff and test response contracts when upgrading. After 1.0,
public Go symbols and serialized fields should be treated as compatibility
surfaces.

## Roadmap and explicit non-promises

Possible future extensions are intentionally separate from the current
implementation:

- additional catalog languages and a clearer language-resolution policy;
- optional observability hooks for logs, metrics, and tracing;
- adapters for selected router or transport conventions.

These are roadmap ideas only. They are not implemented and are not required
by the core package's standard-library-only design.

## Contributing

Keep changes focused on the response/error boundary, preserve the standard
library-only core, and document public behavior. Add or update behavior tests
with API changes, run `gofmt`, `go vet ./...`, and `go test ./...`, and explain
any change to the JSON contract or security boundary in the change description.

## License

StatusFlow Go is released under the [MIT License](LICENSE).
