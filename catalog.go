package statusflow

import "net/http"

// Language identifies a supported response message language.
type Language string

const (
	// LanguageEnglish selects English messages.
	LanguageEnglish Language = "en"
	// LanguageSpanish selects Spanish messages.
	LanguageSpanish Language = "es"
)

const genericInternalMessage = "internal server error"

var messages = map[int]map[Language]string{
	http.StatusBadRequest:          {LanguageEnglish: "bad request", LanguageSpanish: "solicitud incorrecta"},
	http.StatusUnauthorized:        {LanguageEnglish: "unauthorized", LanguageSpanish: "no autorizado"},
	http.StatusForbidden:           {LanguageEnglish: "forbidden", LanguageSpanish: "prohibido"},
	http.StatusNotFound:            {LanguageEnglish: "resource not found", LanguageSpanish: "recurso no encontrado"},
	http.StatusConflict:            {LanguageEnglish: "conflict", LanguageSpanish: "conflicto"},
	http.StatusUnprocessableEntity: {LanguageEnglish: "unprocessable entity", LanguageSpanish: "entidad no procesable"},
	http.StatusTooManyRequests:     {LanguageEnglish: "too many requests", LanguageSpanish: "demasiadas solicitudes"},
	http.StatusInternalServerError: {LanguageEnglish: genericInternalMessage, LanguageSpanish: "error interno del servidor"},
	http.StatusOK:                  {LanguageEnglish: "ok", LanguageSpanish: "correcto"},
	http.StatusCreated:             {LanguageEnglish: "created", LanguageSpanish: "creado"},
	http.StatusNoContent:           {LanguageEnglish: "no content", LanguageSpanish: "sin contenido"},
}

// ResolveMessage returns the catalog message for code and language.
// Unsupported languages fall back to English. Unknown valid HTTP codes use
// net/http's status text; invalid codes use a safe generic internal message.
func ResolveMessage(code int, language Language) string {
	if !validStatus(code) {
		return genericInternalMessage
	}
	if localized, ok := messages[code]; ok {
		if message, ok := localized[language]; ok {
			return message
		}
		return localized[LanguageEnglish]
	}
	if message := http.StatusText(code); message != "" {
		return message
	}
	return genericInternalMessage
}

// Message is a concise alias for ResolveMessage.
func Message(code int, language Language) string { return ResolveMessage(code, language) }
