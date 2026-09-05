package statusflow

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewResponseMarksHTTP2xxAsSuccessful(t *testing.T) {
	for _, tt := range []struct {
		name string
		code int
		want bool
	}{
		{"success", http.StatusCreated, true},
		{"redirect", http.StatusFound, false},
		{"client error", http.StatusBadRequest, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewResponse(tt.code, "message", nil).Success; got != tt.want {
				t.Fatalf("Success = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewResponseDoesNotKeepDataForNon2xxStatus(t *testing.T) {
	response := NewResponse(http.StatusBadRequest, "invalid", map[string]string{"secret": "no"})
	if response.Success {
		t.Fatal("4xx response marked successful")
	}
	if response.Data != nil {
		t.Fatalf("non-2xx response retained data: %#v", response.Data)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "data") {
		t.Fatalf("non-2xx response serialized data: %s", body)
	}
}

func TestNewSuccessResponseDoesNotKeepDataForNon2xxStatus(t *testing.T) {
	response := NewSuccessResponse(http.StatusInternalServerError, "failure", []string{"must not leak"})
	if response.Success {
		t.Fatal("5xx response marked successful")
	}
	if response.Data != nil {
		t.Fatalf("non-2xx success constructor retained data: %#v", response.Data)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "must not leak") || strings.Contains(string(body), "data") {
		t.Fatalf("non-2xx success constructor serialized data: %s", body)
	}
}

func TestResponseContractOmitsApplicationCodeOutsideApplicationErrors(t *testing.T) {
	response := NewSuccessResponse(http.StatusOK, "ready", nil)
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "error_code") {
		t.Fatalf("success response unexpectedly contains error_code: %s", body)
	}

	legacy := NewErrorResponse(http.StatusBadRequest, "bad request")
	body, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "error_code") {
		t.Fatalf("legacy error unexpectedly contains error_code: %s", body)
	}
}

func TestApplicationErrorUsesValidatedApplicationCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"catalog code", ErrorCodeValidation, ErrorCodeValidation},
		{"custom code", "payment_required", "payment_required"},
		{"invalid code", "contains spaces", ""},
		{"empty code", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewApplicationError(http.StatusUnprocessableEntity, tt.code, "invalid input", nil, errors.New("private cause"))
			if got := err.ErrorCode(); got != tt.want {
				t.Fatalf("ErrorCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValidErrorCodeRejectsUnsafeValues(t *testing.T) {
	for _, code := range []string{"", "Validation", "validation-code", "validation code", "_validation", strings.Repeat("a", 65)} {
		if IsValidErrorCode(code) {
			t.Errorf("IsValidErrorCode(%q) = true", code)
		}
	}
	for _, code := range []string{"validation_error", "resource404", "x"} {
		if !IsValidErrorCode(code) {
			t.Errorf("IsValidErrorCode(%q) = false", code)
		}
	}
}

func TestApplicationErrorWritesCodeDetailsAndRedactsCause(t *testing.T) {
	recorder := httptest.NewRecorder()
	cause := errors.New("secret database constraint")
	err := NewApplicationError(http.StatusConflict, ErrorCodeConflict, "user already exists", map[string]string{"field": "email"}, cause)
	if err.ErrorCode() != ErrorCodeConflict || !errors.Is(err, cause) {
		t.Fatal("application error did not retain its code and cause")
	}
	if writeErr := WriteError(recorder, err); writeErr != nil {
		t.Fatal(writeErr)
	}
	var response Response
	if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if recorder.Code != http.StatusConflict || response.Success || response.ErrorCode != ErrorCodeConflict || response.Details == nil {
		t.Fatalf("unexpected application response: status=%d response=%+v", recorder.Code, response)
	}
	if strings.Contains(recorder.Body.String(), cause.Error()) {
		t.Fatalf("private cause leaked: %s", recorder.Body)
	}
}

func TestWriteJSONWritesUniformResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	response := NewSuccessResponse(http.StatusCreated, "created", map[string]string{"id": "42"})
	if err := WriteJSON(recorder, response); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	var got Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusCreated || !got.Success || got.Message != "created" {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body)
	}
	if recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", recorder.Header().Get("Content-Type"))
	}
}

func TestWriteErrorDoesNotExposeUnknownError(t *testing.T) {
	recorder := httptest.NewRecorder()
	secret := errors.New("database password=secret")
	if err := WriteError(recorder, secret); err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), secret.Error()) {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestWriteErrorPreservesSafeHTTPErrorButNotCause(t *testing.T) {
	recorder := httptest.NewRecorder()
	cause := errors.New("private cause")
	httpErr := NewHTTPErrorWithDetails(http.StatusNotFound, "resource not found", map[string]string{"field": "id"}, cause)
	if err := WriteError(recorder, httpErr); err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}
	body := recorder.Body.String()
	if recorder.Code != http.StatusNotFound || !strings.Contains(body, "resource not found") || !strings.Contains(body, "field") || strings.Contains(body, cause.Error()) {
		t.Fatalf("unexpected error response: %s", body)
	}
	if !errors.Is(httpErr, cause) {
		t.Fatal("HTTPError did not retain its cause for server-side inspection")
	}
}

func TestNewHTTPErrorFallsBackForInvalidStatus(t *testing.T) {
	if got := NewHTTPError(200, "", nil); got.StatusCode() != http.StatusInternalServerError || got.Error() != "Internal Server Error" {
		t.Fatalf("unexpected fallback: status=%d message=%q", got.StatusCode(), got.Error())
	}
}

func TestWriteJSONRejectsInvalidStatusBeforeWriting(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := WriteJSON(recorder, NewResponse(99, "bad", nil)); err == nil {
		t.Fatal("WriteJSON accepted invalid status")
	}
	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("invalid response committed output: status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestWriteJSONEncodesBeforeCommitting(t *testing.T) {
	recorder := httptest.NewRecorder()
	response := NewSuccessResponse(http.StatusOK, "not encodable", func() {})
	if err := WriteJSON(recorder, response); err == nil {
		t.Fatal("WriteJSON accepted a value that cannot be encoded")
	}
	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("encoding failure committed output: status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestWriteJSONOmitsBodyForBodylessStatuses(t *testing.T) {
	for _, code := range []int{http.StatusNoContent, http.StatusNotModified} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if err := WriteJSON(recorder, NewResponse(code, "ignored", map[string]string{"secret": "no"})); err != nil {
				t.Fatalf("WriteJSON returned error: %v", err)
			}
			if recorder.Code != code || recorder.Body.Len() != 0 || recorder.Header().Get("Content-Type") != "" {
				t.Fatalf("bodyless response wrote output: status=%d body=%s content-type=%q", recorder.Code, recorder.Body, recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func TestAdaptIntegratesWithNetHTTP(t *testing.T) {
	handler := Adapt(func(w http.ResponseWriter, _ *http.Request) error {
		return NewHTTPError(http.StatusTeapot, "short and stout", nil)
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusTeapot || !strings.Contains(recorder.Body.String(), "short and stout") {
		t.Fatalf("unexpected adapted response: status=%d body=%s", recorder.Code, recorder.Body)
	}
}

func TestAdaptWritesApplicationErrorWithHeadersAndPayload(t *testing.T) {
	handler := Adapt(func(http.ResponseWriter, *http.Request) error {
		return NewApplicationError(http.StatusConflict, ErrorCodeConflict, "duplicate user", map[string]string{"field": "email"}, errors.New("private constraint"))
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusConflict || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected adapted headers: status=%d headers=%v", recorder.Code, recorder.Header())
	}
	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Success || response.Code != http.StatusConflict || response.ErrorCode != ErrorCodeConflict || response.Data != nil {
		t.Fatalf("unexpected adapted response: %+v", response)
	}
}

func TestAdaptWithHTTPServer(t *testing.T) {
	server := httptest.NewServer(Adapt(func(w http.ResponseWriter, _ *http.Request) error {
		return errors.New("private database failure")
	}))
	defer server.Close()

	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusInternalServerError)
	}
	var body Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Message != "internal server error" || strings.Contains(body.Message, "private database failure") {
		t.Fatalf("unexpected server response: %+v", body)
	}
}
