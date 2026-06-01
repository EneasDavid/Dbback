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
	t.Setenv("GOOGLE_SHEET_ID", "sheet-test-id")
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
	t.Setenv("GOOGLE_SHEET_ID", "")
	t.Setenv("GOOGLE_SHEET_IDS", "")
	t.Setenv("GOOGLE_SHEET_LEGACY_IDS", "")
	t.Setenv("GOOGLE_SHEET_V2_IDS", "")
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
	if !strings.Contains(httpErr.Message, "GOOGLE_SHEET_ID") {
		t.Fatalf("Validate() message = %q, want GOOGLE_SHEET_ID guidance", httpErr.Message)
	}
}

func TestLoadConfigAcceptsMultipleSpreadsheetIDs(t *testing.T) {
	t.Setenv("GOOGLE_SHEET_ID", "legacy-id")
	t.Setenv("GOOGLE_SHEET_IDS", " sheet-a, sheet-b ;sheet-a\nsheet-c ")
	t.Setenv("GOOGLE_SHEET_LEGACY_IDS", "legacy-extra")
	t.Setenv("GOOGLE_SHEET_V2_IDS", "v2-a; sheet-b")

	cfg := LoadConfig()

	want := []string{"legacy-extra", "v2-a", "sheet-b", "legacy-id", "sheet-a", "sheet-c"}
	if strings.Join(cfg.SpreadsheetIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("SpreadsheetIDs = %#v, want %#v (order: LEGACY_IDS, V2_IDS, GOOGLE_SHEET_ID, GOOGLE_SHEET_IDS)", cfg.SpreadsheetIDs, want)
	}
	if cfg.SpreadsheetID != "legacy-extra" {
		t.Fatalf("SpreadsheetID = %q, want first legacy id", cfg.SpreadsheetID)
	}
	if strings.Join(cfg.LegacySpreadsheetIDs, ",") != "legacy-extra" {
		t.Fatalf("LegacySpreadsheetIDs = %#v", cfg.LegacySpreadsheetIDs)
	}
	if strings.Join(cfg.V2SpreadsheetIDs, ",") != "v2-a,sheet-b" {
		t.Fatalf("V2SpreadsheetIDs = %#v", cfg.V2SpreadsheetIDs)
	}
}

func TestLoadConfigScoreDivisors(t *testing.T) {
	cfg := LoadConfig()

	for _, table := range cfg.AB1Tables {
		if table.Kind == "activity" && table.ScoreDivisor != 10 {
			t.Fatalf("%s ScoreDivisor = %v, want 10", table.Key, table.ScoreDivisor)
		}
	}
	for _, table := range cfg.AB2Tables {
		switch table.Key {
		case "at4":
			if table.ScoreDivisor != 10 {
				t.Fatalf("at4 ScoreDivisor = %v, want 10", table.ScoreDivisor)
			}
		case "projeto":
			if table.ScoreDivisor != 1 {
				t.Fatalf("projeto ScoreDivisor = %v, want 1", table.ScoreDivisor)
			}
		}
	}
}

func TestLoadConfigActivityLabelsAreUserFacing(t *testing.T) {
	cfg := LoadConfig()
	want := map[string]string{
		"at1": "Atividade 1",
		"at2": "Atividade 2",
		"at3": "Atividade 3",
		"at4": "Atividade 4",
	}

	for _, table := range append(cfg.AB1Tables, cfg.AB2Tables...) {
		expected, ok := want[table.Key]
		if !ok {
			continue
		}
		if table.Label != expected {
			t.Fatalf("%s label = %q, want %q", table.Key, table.Label, expected)
		}
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
