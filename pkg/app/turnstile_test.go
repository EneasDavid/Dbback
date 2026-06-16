package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateTurnstileRequiresConfiguredSecret(t *testing.T) {
	err := ValidateTurnstile(context.Background(), "", "", "")
	if err == nil {
		t.Fatal("ValidateTurnstile() error = nil, want missing secret error")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusInternalServerError {
		t.Fatalf("ValidateTurnstile() error = %#v, want 500 HTTPError", err)
	}
}

func TestValidateTurnstileRequiresTokenWhenSecretIsConfigured(t *testing.T) {
	err := ValidateTurnstile(context.Background(), "secret", "", "")
	if err == nil {
		t.Fatal("ValidateTurnstile() error = nil, want token error")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("ValidateTurnstile() error = %#v, want 400 HTTPError", err)
	}
}

func TestValidateTurnstileCallsSiteverify(t *testing.T) {
	var got turnstileVerifyRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if gotContentType := r.Header.Get("Content-Type"); gotContentType != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", gotContentType)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()
	withTurnstileVerifier(t, server)

	if err := ValidateTurnstile(context.Background(), "secret", "token", "203.0.113.10"); err != nil {
		t.Fatalf("ValidateTurnstile() error = %v, want nil", err)
	}
	if got.Secret != "secret" || got.Response != "token" || got.RemoteIP != "203.0.113.10" {
		t.Fatalf("siteverify request = %#v, want secret/token/remoteip", got)
	}
}

func TestValidateTurnstileRejectsFailedChallenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer server.Close()
	withTurnstileVerifier(t, server)

	err := ValidateTurnstile(context.Background(), "secret", "token", "")
	if err == nil {
		t.Fatal("ValidateTurnstile() error = nil, want challenge failure")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusForbidden {
		t.Fatalf("ValidateTurnstile() error = %#v, want 403 HTTPError", err)
	}
}

func withTurnstileVerifier(t *testing.T, server *httptest.Server) {
	t.Helper()
	originalURL := turnstileSiteverifyURL
	originalClient := turnstileHTTPClient
	turnstileSiteverifyURL = server.URL
	turnstileHTTPClient = server.Client()
	t.Cleanup(func() {
		turnstileSiteverifyURL = originalURL
		turnstileHTTPClient = originalClient
	})
}
