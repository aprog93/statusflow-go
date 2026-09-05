# StatusFlow Go architecture

StatusFlow Go is a transport boundary, not an application framework. Its
responsibility ends when a safe, valid HTTP response has been written.

## Boundary model

```text
application / domain code
          |
          | Response, HTTPError, ApplicationError
          v
 statusflow contract
          |
          | WriteJSON, WriteError, Adapt
          v
       net/http
          |
          v
      HTTP client
```

The application remains responsible for routing, authentication,
authorization, request validation, business rules, logging, and
observability. This keeps the package useful with `http.ServeMux` and external
routers without making any router a required dependency.

## Key decisions

### One response envelope

`Response` gives clients predictable `success`, `message`, and HTTP `code`
fields. `data`, `details`, and `error_code` are optional extensions. Data is
removed from non-2xx responses so a failed response cannot accidentally carry a
success payload.

### HTTP status and domain code are different

`code` is an HTTP status. `error_code` is a validated application code for
machine clients. Keeping them separate avoids overloading HTTP semantics with
domain state.

### Causes are private by default

Typed errors retain their cause for trusted server-side inspection through
`Unwrap`, while `WriteError` serializes only deliberately public fields.
Unknown errors receive a generic 500 message.

### Validate before committing output

`WriteJSON` validates the status and marshals the entire response before
writing headers. This avoids partial responses when the payload cannot be
encoded. Bodyless HTTP statuses are handled without JSON output.

### Localization stays at the boundary

Catalog-backed constructors and `AdaptWithLanguage` provide the current
English and Spanish messages. Unsupported languages fall back safely; adding
more languages should not change the response or transport abstractions.

## Deliberate non-goals

The core does not own a server lifecycle, router abstraction, middleware
chain, schema validator, logger, tracer, metrics exporter, authentication
system, or persistence layer. Future adapters and observability hooks may be
added independently and must not become mandatory core dependencies.
