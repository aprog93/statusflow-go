# StatusFlow Go examples

This directory shows how to integrate StatusFlow at the `net/http` boundary.
The core package has no router dependency: use the standard library directly,
or register any `http.Handler` with the router your application already uses.

## Run the demo

From the repository root:

```bash
go run ./cmd/statusflow-demo
```

The server listens on `:8080` by default. Set `PORT` to use another port:

```bash
PORT=9090 go run ./cmd/statusflow-demo
```

Try the routes:

```bash
curl http://localhost:8080/success
curl http://localhost:8080/not-found
curl http://localhost:8080/conflict
curl http://localhost:8080/unknown
curl http://localhost:8080/localized/en
curl http://localhost:8080/localized/es
```

The demo covers a success payload, a catalog-backed 404, an application 409
with `error_code` and public `details`, safe handling of an unknown error, and
explicit English/Spanish localization. Internal causes are never serialized.

## Direct `net/http` integration

Use `WriteJSON` when the handler already has a response, and return an error
to `Adapt` when the library should serialize the error:

```go
func users(w http.ResponseWriter, _ *http.Request) error {
	return statusflow.WriteJSON(w, statusflow.NewSuccessResponse(
		http.StatusOK, "users listed", []string{"Ada", "Linus"},
	))
}

func createUser(_ http.ResponseWriter, _ *http.Request) error {
	return statusflow.NewApplicationError(
		http.StatusConflict, statusflow.ErrorCodeConflict,
		"user already exists", map[string]string{"field": "email"}, nil,
	)
}

http.Handle("/users", statusflow.Adapt(users))
http.Handle("/users/create", statusflow.Adapt(createUser))
```

For a conventional `http.Handler`, call `WriteError` yourself:

```go
func handler(w http.ResponseWriter, r *http.Request) {
	if err := doWork(r); err != nil {
		_ = statusflow.WriteError(w, err)
		return
	}
	_ = statusflow.WriteJSON(w, statusflow.NewSuccessResponse(
		http.StatusNoContent, "", nil,
	))
}
```

Pass only deliberately public values as `data` or `details`. Keep database
errors, credentials, and stack traces in the private cause; `WriteError`
reduces unknown errors to a generic 500 response.

## External routers

`Adapt` returns the standard `http.Handler` interface, so a router that accepts
`http.Handler` can register it directly:

```go
router.Handle("/users", statusflow.Adapt(users))
```

If a router has its own handler signature, keep the adapter in your application
and call `WriteJSON` or `WriteError` from that adapter. StatusFlow does not need
to know which router is in use.
