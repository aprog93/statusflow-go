package statusflow

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Response is the JSON envelope returned by the package helpers. Data and
// Details are extension points for values that are safe to expose publicly.
type Response struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

// NewResponse creates a response with an explicit HTTP status code.
// WriteJSON validates the code before sending anything to the client.
func NewResponse(code int, message string, data any) Response {
	success := code >= http.StatusOK && code < http.StatusMultipleChoices
	if !success {
		data = nil
	}
	return Response{
		Success: success,
		Message: message,
		Code:    code,
		Data:    data,
	}
}

// NewSuccessResponse creates a successful response.
func NewSuccessResponse(code int, message string, data any) Response {
	return NewResponse(code, message, data)
}

// NewErrorResponse creates an error response without including an underlying
// error or other implementation detail in the JSON payload.
func NewErrorResponse(code int, message string) Response {
	return NewResponse(code, message, nil)
}

// NewErrorResponseWithDetails creates an error response with caller-supplied,
// intentionally public details. It never includes an error value implicitly.
func NewErrorResponseWithDetails(code int, message string, details any) Response {
	response := NewErrorResponse(code, message)
	response.Details = details
	return response
}

// HTTPError is an HTTP-safe error. Its cause is retained for server-side
// inspection through Unwrap, but is never exposed by response helpers.
type HTTPError struct {
	status   int
	message  string
	details  any
	cause    error
	catalog  bool
	language Language
}

// ApplicationError is an HTTPError with a stable, application-defined error
// code. The code is intended for machine clients; the message and details are
// the public human-readable response fields.
type ApplicationError struct {
	*HTTPError
	errorCode string
}

const (
	// ErrorCodeValidation identifies invalid application input.
	ErrorCodeValidation = "validation_error"
	// ErrorCodeNotFound identifies a missing application resource.
	ErrorCodeNotFound = "resource_not_found"
	// ErrorCodeConflict identifies a business state conflict.
	ErrorCodeConflict = "conflict"
	// ErrorCodeInvalidState identifies an operation that is invalid in the
	// resource's current state.
	ErrorCodeInvalidState = "invalid_state"
	// ErrorCodeInternal identifies an unexpected application failure.
	ErrorCodeInternal = "internal_error"
)

// NewApplicationError creates an application error with a safe machine code.
// Invalid status codes fall back to 500, as they do for NewHTTPError. Invalid
// or empty application codes are omitted from the serialized response.
func NewApplicationError(status int, code string, message string, details any, cause error) *ApplicationError {
	return &ApplicationError{
		HTTPError: NewHTTPErrorWithDetails(status, message, details, cause),
		errorCode: normalizeErrorCode(code),
	}
}

// ErrorCode returns the validated application code, or an empty string when
// no valid application code was supplied.
func (e *ApplicationError) ErrorCode() string {
	if e == nil {
		return ""
	}
	return e.errorCode
}

// IsValidErrorCode reports whether code is safe for use as an application
// error code. Codes use lowercase ASCII letters, digits, and underscores,
// start with a letter, and are limited to 64 characters.
func IsValidErrorCode(code string) bool {
	if len(code) == 0 || len(code) > 64 || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	for i := 1; i < len(code); i++ {
		if !((code[i] >= 'a' && code[i] <= 'z') || (code[i] >= '0' && code[i] <= '9') || code[i] == '_') {
			return false
		}
	}
	return true
}

func normalizeErrorCode(code string) string {
	if IsValidErrorCode(code) {
		return code
	}
	return ""
}

// NewHTTPError creates an HTTPError with an optional internal cause.
// Invalid or non-error status codes become 500 Internal Server Error.
func NewHTTPError(status int, message string, cause error) *HTTPError {
	return NewHTTPErrorWithDetails(status, message, nil, cause)
}

// NewHTTPErrorWithDetails creates an HTTPError with caller-supplied details
// that are safe to expose in the response. The cause remains private.
func NewHTTPErrorWithDetails(status int, message string, details any, cause error) *HTTPError {
	useCatalog := message == "" && status >= http.StatusBadRequest && status <= 599
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusInternalServerError
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &HTTPError{status: status, message: message, details: details, cause: cause, catalog: useCatalog}
}

// NewLocalizedHTTPError creates an HTTPError using the catalog message for
// the requested language.
func NewLocalizedHTTPError(status int, language Language, cause error) *HTTPError {
	return NewLocalizedHTTPErrorWithDetails(status, language, nil, cause)
}

// NewLocalizedHTTPErrorWithDetails creates an HTTPError with explicit public
// details and a catalog message for the requested language.
func NewLocalizedHTTPErrorWithDetails(status int, language Language, details any, cause error) *HTTPError {
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusInternalServerError
	}
	err := NewHTTPErrorWithDetails(status, ResolveMessage(status, language), details, cause)
	err.catalog = true
	err.language = language
	return err
}

// NewBadRequest creates a catalog-backed 400 error in English.
func NewBadRequest(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusBadRequest, LanguageEnglish, cause)
}

// NewBadRequestWithLanguage creates a catalog-backed 400 error.
func NewBadRequestWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusBadRequest, language, cause)
}

// NewUnauthorized creates a catalog-backed 401 error in English.
func NewUnauthorized(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusUnauthorized, LanguageEnglish, cause)
}

// NewUnauthorizedWithLanguage creates a catalog-backed 401 error.
func NewUnauthorizedWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusUnauthorized, language, cause)
}

// NewForbidden creates a catalog-backed 403 error in English.
func NewForbidden(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusForbidden, LanguageEnglish, cause)
}

// NewForbiddenWithLanguage creates a catalog-backed 403 error.
func NewForbiddenWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusForbidden, language, cause)
}

// NewNotFound creates a catalog-backed 404 error in English.
func NewNotFound(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusNotFound, LanguageEnglish, cause)
}

// NewNotFoundWithLanguage creates a catalog-backed 404 error.
func NewNotFoundWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusNotFound, language, cause)
}

// NewConflict creates a catalog-backed 409 error in English.
func NewConflict(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusConflict, LanguageEnglish, cause)
}

// NewConflictWithLanguage creates a catalog-backed 409 error.
func NewConflictWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusConflict, language, cause)
}

// NewUnprocessableEntity creates a catalog-backed 422 error in English.
func NewUnprocessableEntity(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusUnprocessableEntity, LanguageEnglish, cause)
}

// NewUnprocessableEntityWithLanguage creates a catalog-backed 422 error.
func NewUnprocessableEntityWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusUnprocessableEntity, language, cause)
}

// NewTooManyRequests creates a catalog-backed 429 error in English.
func NewTooManyRequests(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusTooManyRequests, LanguageEnglish, cause)
}

// NewTooManyRequestsWithLanguage creates a catalog-backed 429 error.
func NewTooManyRequestsWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusTooManyRequests, language, cause)
}

// NewInternalServerError creates a catalog-backed 500 error in English.
func NewInternalServerError(cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusInternalServerError, LanguageEnglish, cause)
}

// NewInternalServerErrorWithLanguage creates a catalog-backed 500 error.
func NewInternalServerErrorWithLanguage(language Language, cause error) *HTTPError {
	return NewLocalizedHTTPError(http.StatusInternalServerError, language, cause)
}

// Error implements error without returning the internal cause.
func (e *HTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.message
}

// Unwrap exposes the cause to trusted server-side error handling.
func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// StatusCode returns the HTTP status associated with the error.
func (e *HTTPError) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.status
}

// Details returns the optional, caller-supplied public details.
func (e *HTTPError) Details() any {
	if e == nil {
		return nil
	}
	return e.details
}

// Handler is an HTTP handler whose returned error is converted to a safe
// response by Adapt.
type Handler func(http.ResponseWriter, *http.Request) error

// Adapt converts a Handler into the standard net/http Handler interface.
// A known HTTPError keeps its public status and message; all other errors
// become a generic 500 response.
func Adapt(handler Handler) http.Handler {
	return AdaptWithLanguage(handler, LanguageEnglish)
}

// AdaptWithLanguage converts a Handler and localizes unknown HTTPError
// messages generated by the handler's status code.
func AdaptWithLanguage(handler Handler, language Language) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler == nil {
			_ = WriteError(w, errors.New("statusflow: nil handler"))
			return
		}
		if err := handler(w, r); err != nil {
			_ = writeError(w, err, language)
		}
	})
}

// WriteJSON safely writes a response. It validates the HTTP status and fully
// encodes the JSON before committing headers, preventing partial responses.
// Statuses that must not have a body (1xx, 204, and 304) only write headers.
func WriteJSON(w http.ResponseWriter, response Response) error {
	if w == nil {
		return errors.New("statusflow: nil ResponseWriter")
	}
	if !validStatus(response.Code) {
		return errors.New("statusflow: invalid HTTP status code")
	}
	if statusHasNoBody(response.Code) {
		w.WriteHeader(response.Code)
		return nil
	}

	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(response.Code)
	n, err := w.Write(body)
	if err != nil {
		return err
	}
	if n != len(body) {
		return io.ErrShortWrite
	}
	return nil
}

// WriteError writes a safe error response. Unknown errors are deliberately
// reduced to a generic message so their contents cannot reach the client.
func WriteError(w http.ResponseWriter, err error) error {
	return writeError(w, err, LanguageEnglish)
}

func writeError(w http.ResponseWriter, err error, language Language) error {
	var applicationErr *ApplicationError
	if errors.As(err, &applicationErr) && applicationErr != nil && applicationErr.HTTPError != nil {
		message := applicationErr.Error()
		if applicationErr.catalog && applicationErr.language == "" {
			message = ResolveMessage(applicationErr.StatusCode(), language)
		}
		if message == "" {
			message = ResolveMessage(applicationErr.StatusCode(), language)
		}
		response := NewErrorResponseWithDetails(applicationErr.StatusCode(), message, applicationErr.Details())
		response.ErrorCode = applicationErr.ErrorCode()
		return WriteJSON(w, response)
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		message := httpErr.Error()
		if httpErr.catalog && httpErr.language == "" {
			message = ResolveMessage(httpErr.StatusCode(), language)
		}
		if message == "" {
			message = ResolveMessage(httpErr.StatusCode(), language)
		}
		return WriteJSON(w, NewErrorResponseWithDetails(httpErr.StatusCode(), message, httpErr.Details()))
	}
	return WriteJSON(w, NewErrorResponse(http.StatusInternalServerError, genericInternalMessage))
}

func validStatus(code int) bool { return code >= 100 && code <= 599 }

func statusHasNoBody(code int) bool {
	return (code >= 100 && code < 200) || code == http.StatusNoContent || code == http.StatusNotModified
}
