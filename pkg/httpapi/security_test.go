package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"feedback/pkg/app"
)

func TestPublicUserDoesNotExposeSpreadsheetID(t *testing.T) {
	payload, err := json.Marshal(publicUser(app.SessionUser{
		Matricula:     "123456",
		Name:          "Alice",
		SchemaStatus:  "v2",
		SpreadsheetID: "secret-sheet",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret-sheet") || strings.Contains(string(payload), "spreadsheetId") {
		t.Fatalf("public user leaked spreadsheet id: %s", payload)
	}
}

func TestPublicGradeResultDoesNotExposeSpreadsheetID(t *testing.T) {
	result := publicGradeResult(app.GradeResult{
		SpreadsheetID: "secret-sheet",
		Tables: []app.TableResult{
			{SpreadsheetID: "secret-table", Cards: []app.CardResult{{Key: "nota", Label: "Nota"}}},
		},
	})
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret-sheet") || strings.Contains(string(payload), "secret-table") || strings.Contains(string(payload), "spreadsheetId") {
		t.Fatalf("public grade leaked spreadsheet id: %s", payload)
	}
}

func TestJSONAddsIntegrityDigest(t *testing.T) {
	rec := httptest.NewRecorder()

	app.JSON(rec, http.StatusOK, map[string]string{"ok": "true"})

	if rec.Header().Get("Digest") == "" {
		t.Fatal("Digest header should be set")
	}
	if rec.Header().Get("X-Dbback-Content-SHA256") == "" {
		t.Fatal("X-Dbback-Content-SHA256 header should be set")
	}
}

func TestCacheableJSONAddsETagAndSupportsConditionalRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://dbback.test/api/grades/all", nil)
	rec := httptest.NewRecorder()

	app.CacheableJSON(rec, req, http.StatusOK, map[string]string{"ok": "true"}, 30*time.Second, 5*time.Minute)

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header should be set")
	}
	cacheControl := rec.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "private") || !strings.Contains(cacheControl, "max-age=30") || !strings.Contains(cacheControl, "stale-while-revalidate=300") {
		t.Fatalf("Cache-Control = %q, want private SWR policy", cacheControl)
	}
	if got := rec.Header().Get("Vary"); got != "Cookie, Accept-Encoding" {
		t.Fatalf("Vary = %q, want Cookie, Accept-Encoding", got)
	}

	req = httptest.NewRequest(http.MethodGet, "https://dbback.test/api/grades/all", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()

	app.CacheableJSON(rec, req, http.StatusOK, map[string]string{"ok": "true"}, 30*time.Second, 5*time.Minute)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotModified)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "" {
		t.Fatalf("304 body = %q, want empty", body)
	}
}

func TestPostRejectsCrossOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://dbback.test/api/login", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	if app.Method(rec, req, http.MethodPost) {
		t.Fatal("cross-origin POST should be rejected")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
