// Command statusflow-demo runs a small net/http server that demonstrates the
// main StatusFlow integration patterns without an external router.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	statusflow "github.com/aprog93/statusflow-go"
)

const defaultPort = "8080"

// main starts the demo server on PORT, or on :8080 when PORT is not set.
func main() {
	addr := ":" + os.Getenv("PORT")
	if os.Getenv("PORT") == "" {
		addr = ":" + defaultPort
	}

	log.Printf("StatusFlow demo listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, demoHandler()); err != nil {
		log.Fatal(err)
	}
}

// demoHandler returns the standard-library router used by the demo.
func demoHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/success", statusflow.Adapt(successHandler))
	mux.Handle("/not-found", statusflow.Adapt(notFoundHandler))
	mux.Handle("/conflict", statusflow.Adapt(conflictHandler))
	mux.Handle("/unknown", statusflow.Adapt(unknownHandler))
	mux.Handle("/localized/en", statusflow.AdaptWithLanguage(localizedHandler, statusflow.LanguageEnglish))
	mux.Handle("/localized/es", statusflow.AdaptWithLanguage(localizedHandler, statusflow.LanguageSpanish))
	return mux
}

func successHandler(w http.ResponseWriter, _ *http.Request) error {
	return statusflow.WriteJSON(w, statusflow.NewSuccessResponse(
		http.StatusOK,
		"users listed",
		[]map[string]any{{"id": 1, "name": "Ada"}, {"id": 2, "name": "Linus"}},
	))
}

func notFoundHandler(_ http.ResponseWriter, _ *http.Request) error {
	return statusflow.NewNotFound(errors.New("user lookup failed: database record 42 is absent"))
}

func conflictHandler(_ http.ResponseWriter, _ *http.Request) error {
	return statusflow.NewApplicationError(
		http.StatusConflict,
		statusflow.ErrorCodeConflict,
		"resource cannot be modified in its current state",
		map[string]any{"resource": "order-42", "state": "already_processed"},
		errors.New("internal transaction conflict: lock held by worker-7"),
	)
}

func unknownHandler(_ http.ResponseWriter, _ *http.Request) error {
	return fmt.Errorf("database connection failed: password=demo-secret: %w", errors.New("connection refused"))
}

func localizedHandler(_ http.ResponseWriter, _ *http.Request) error {
	// Leaving the catalog language unset lets AdaptWithLanguage choose the
	// language configured by each route.
	return statusflow.NewHTTPError(
		http.StatusNotFound,
		"",
		errors.New("private lookup cause: tenant token=demo-secret"),
	)
}
