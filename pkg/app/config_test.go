package app

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const testServiceAccountJSON = `{"type":"service_account","project_id":"test","private_key_id":"key","private_key":"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----\n","client_email":"svc@example.test","client_id":"1"}`

func TestServiceAccountJSONAcceptsQuotedRawJSON(t *testing.T) {
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON", "'"+testServiceAccountJSON+"'")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64", "")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_FILE", "")

	if got := serviceAccountJSON(); got != testServiceAccountJSON {
		t.Fatalf("serviceAccountJSON() = %q, want raw JSON", got)
	}
}

func TestServiceAccountJSONAcceptsWrappedBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(testServiceAccountJSON))
	wrapped := encoded[:16] + "\n" + encoded[16:48] + "\r\n" + encoded[48:]
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON", "")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64", wrapped)
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_FILE", "")

	if got := serviceAccountJSON(); got != testServiceAccountJSON {
		t.Fatalf("serviceAccountJSON() = %q, want decoded JSON", got)
	}
}

func TestValidateExplainsMissingServiceAccountFileOnVercel(t *testing.T) {
	t.Setenv("GOOGLE_SHEET_IDS", "sheet-test-id")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON", "")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64", "")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_FILE", "missing-service-account.json")

	err := LoadConfig().Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing credential error")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Validate() error type = %T, want HTTPError", err)
	}
	if !strings.Contains(httpErr.Message, "GOOGLE_SERVICE_ACCOUNT_FILE") || !strings.Contains(httpErr.Message, "Vercel") {
		t.Fatalf("Validate() message = %q, want file and Vercel guidance", httpErr.Message)
	}
}

func TestValidateExplainsMissingSpreadsheetID(t *testing.T) {
	t.Setenv("GOOGLE_SHEET_IDS", "")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON", testServiceAccountJSON)
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64", "")
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_FILE", "")

	err := LoadConfig().Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing spreadsheet ID error")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Validate() error type = %T, want HTTPError", err)
	}
	if !strings.Contains(httpErr.Message, "GOOGLE_SHEET_IDS") {
		t.Fatalf("Validate() message = %q, want GOOGLE_SHEET_IDS guidance", httpErr.Message)
	}
}

func TestLoadConfigAcceptsMultipleSpreadsheetIDs(t *testing.T) {
	t.Setenv("GOOGLE_SHEET_IDS", " sheet-a, sheet-b ;sheet-a\nsheet-c ")

	cfg := LoadConfig()

	want := []string{"sheet-a", "sheet-b", "sheet-c"}
	if strings.Join(cfg.SpreadsheetIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("SpreadsheetIDs = %#v, want %#v", cfg.SpreadsheetIDs, want)
	}
	if cfg.SpreadsheetID != "sheet-a" {
		t.Fatalf("SpreadsheetID = %q, want first id", cfg.SpreadsheetID)
	}
}

func TestLoadConfigDocsCredentials(t *testing.T) {
	t.Setenv("DOCS_USERNAME", "docs")
	t.Setenv("DOCS_PASSWORD", "senha")

	cfg := LoadConfig()

	if cfg.DocsUsername != "docs" || cfg.DocsPassword != "senha" {
		t.Fatalf("docs credentials = %q/%q, want docs/senha", cfg.DocsUsername, cfg.DocsPassword)
	}
}

func TestLoadConfigTurnstileSecret(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET_KEY", "turnstile-secret")
	t.Setenv("CF_TURNSTILE_SECRET_KEY", "fallback-secret")

	cfg := LoadConfig()

	if cfg.TurnstileSecret != "turnstile-secret" {
		t.Fatalf("TurnstileSecret = %q, want TURNSTILE_SECRET_KEY", cfg.TurnstileSecret)
	}
}
