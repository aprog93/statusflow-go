package statusflow

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveMessageSupportsLanguagesAndSafeFallbacks(t *testing.T) {
	tests := []struct {
		name string
		code int
		lang Language
		want string
	}{
		{"Spanish bad request", http.StatusBadRequest, LanguageSpanish, "solicitud incorrecta"},
		{"English not found", http.StatusNotFound, LanguageEnglish, "resource not found"},
		{"Unsupported language falls back to English", http.StatusConflict, Language("fr"), "conflict"},
		{"Unknown status uses status text", http.StatusTeapot, LanguageSpanish, "I'm a teapot"},
		{"Invalid status uses generic message", 99, LanguageSpanish, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveMessage(tt.code, tt.lang); got != tt.want {
				t.Fatalf("ResolveMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPredefinedErrorsUseCatalogAndRetainPrivateCause(t *testing.T) {
	cause := errors.New("database password=secret")
	err := NewNotFoundWithLanguage(LanguageSpanish, cause)
	if err.StatusCode() != http.StatusNotFound || err.Error() != "recurso no encontrado" {
		t.Fatalf("unexpected error: status=%d message=%q", err.StatusCode(), err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("predefined error did not retain its cause")
	}

	for _, tt := range []struct {
		name string
		make func(error) *HTTPError
		code int
	}{
		{"bad request", NewBadRequest, http.StatusBadRequest},
		{"unauthorized", NewUnauthorized, http.StatusUnauthorized},
		{"forbidden", NewForbidden, http.StatusForbidden},
		{"not found", NewNotFound, http.StatusNotFound},
		{"conflict", NewConflict, http.StatusConflict},
		{"unprocessable entity", NewUnprocessableEntity, http.StatusUnprocessableEntity},
		{"too many requests", NewTooManyRequests, http.StatusTooManyRequests},
		{"internal server error", NewInternalServerError, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.make(nil); got.StatusCode() != tt.code || got.Error() == "" {
				t.Fatalf("unexpected predefined error: status=%d message=%q", got.StatusCode(), got.Error())
			}
		})
	}
}

func TestAdaptWithLanguageWritesLocalizedSafeError(t *testing.T) {
	server := httptest.NewServer(AdaptWithLanguage(func(http.ResponseWriter, *http.Request) error {
		return NewHTTPError(http.StatusNotFound, "", errors.New("private cause"))
	}, LanguageSpanish))
	defer server.Close()

	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(string(body), "recurso no encontrado") || strings.Contains(string(body), "private cause") {
		t.Fatalf("unsafe or unexpected body: %s", body)
	}
}

func TestEndToEndRoutesCoverSuccessKnownUnknownAndPublicDetails(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/success", Adapt(func(w http.ResponseWriter, _ *http.Request) error {
		return WriteJSON(w, NewSuccessResponse(http.StatusOK, "ready", nil))
	}))
	mux.Handle("/known", Adapt(func(http.ResponseWriter, *http.Request) error {
		return NewNotFoundWithLanguage(LanguageSpanish, errors.New("private lookup failure"))
	}))
	mux.Handle("/unknown", Adapt(func(http.ResponseWriter, *http.Request) error {
		return errors.New("private database failure")
	}))
	mux.Handle("/details", Adapt(func(http.ResponseWriter, *http.Request) error {
		return NewHTTPErrorWithDetails(http.StatusConflict, "duplicate user", map[string]string{"field": "email"}, errors.New("private constraint"))
	}))
	mux.Handle("/application", Adapt(func(http.ResponseWriter, *http.Request) error {
		return NewApplicationError(http.StatusUnprocessableEntity, ErrorCodeValidation, "invalid user", map[string]string{"field": "email"}, errors.New("private validator details"))
	}))

	server := httptest.NewServer(mux)
	defer server.Close()
	for _, tt := range []struct {
		path       string
		status     int
		contains   string
		notContain string
	}{
		{"/success", http.StatusOK, "ready", ""},
		{"/known", http.StatusNotFound, "recurso no encontrado", "private lookup failure"},
		{"/unknown", http.StatusInternalServerError, "internal server error", "private database failure"},
		{"/details", http.StatusConflict, "email", "private constraint"},
		{"/application", http.StatusUnprocessableEntity, `"error_code":"validation_error"`, "private validator details"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if response.StatusCode != tt.status || !strings.Contains(string(body), tt.contains) || (tt.notContain != "" && strings.Contains(string(body), tt.notContain)) {
				t.Fatalf("unexpected response: status=%d body=%s", response.StatusCode, body)
			}
		})
	}
}
