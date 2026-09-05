package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDemoRoutes(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		status     int
		message    string
		errorCode  string
		details    map[string]any
		mustNotSee string
	}{
		{name: "success data", path: "/success", status: http.StatusOK, message: "users listed"},
		{name: "not found", path: "/not-found", status: http.StatusNotFound, message: "resource not found", mustNotSee: "database record 42"},
		{
			name:       "application conflict",
			path:       "/conflict",
			status:     http.StatusConflict,
			message:    "resource cannot be modified in its current state",
			errorCode:  "conflict",
			details:    map[string]any{"resource": "order-42", "state": "already_processed"},
			mustNotSee: "worker-7",
		},
		{name: "unknown error is safe", path: "/unknown", status: http.StatusInternalServerError, message: "internal server error", mustNotSee: "demo-secret"},
		{name: "localized english", path: "/localized/en", status: http.StatusNotFound, message: "resource not found", mustNotSee: "demo-secret"},
		{name: "localized spanish", path: "/localized/es", status: http.StatusNotFound, message: "recurso no encontrado", mustNotSee: "demo-secret"},
	}

	server := httptest.NewServer(demoHandler())
	defer server.Close()

	client := server.Client()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, tt.status, body)
			}
			if contentType := response.Header.Get("Content-Type"); contentType != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", contentType)
			}

			var payload struct {
				Success   bool             `json:"success"`
				Message   string           `json:"message"`
				Code      int              `json:"code"`
				Data      []map[string]any `json:"data"`
				Details   map[string]any   `json:"details"`
				ErrorCode string           `json:"error_code"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode JSON: %v; body = %s", err, body)
			}
			if payload.Success != (tt.status < 300) || payload.Code != tt.status || payload.Message != tt.message {
				t.Fatalf("payload = %+v", payload)
			}
			if payload.ErrorCode != tt.errorCode {
				t.Fatalf("error_code = %q, want %q", payload.ErrorCode, tt.errorCode)
			}
			if tt.details != nil {
				for key, value := range tt.details {
					if payload.Details[key] != value {
						t.Errorf("details[%q] = %v, want %v", key, payload.Details[key], value)
					}
				}
			}
			if len(payload.Data) == 0 && tt.path == "/success" {
				t.Fatal("success response has no data")
			}
			if tt.mustNotSee != "" && strings.Contains(string(body), tt.mustNotSee) {
				t.Errorf("response exposed private cause %q: %s", tt.mustNotSee, body)
			}
		})
	}
}
