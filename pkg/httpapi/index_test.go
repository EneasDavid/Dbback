package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setDocsCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("DOCS_USERNAME", "docs-test")
	t.Setenv("DOCS_PASSWORD", "docs-password-test")
}

func setDocsBasicAuth(req *http.Request) {
	req.SetBasicAuth("docs-test", "docs-password-test")
}

func TestDocsRoute(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	setDocsBasicAuth(req)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Name   string           `json:"name"`
		Routes []map[string]any `json:"routes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "dbBack" {
		t.Fatalf("name = %q, want dbBack", payload.Name)
	}
	if len(payload.Routes) == 0 {
		t.Fatal("routes should not be empty")
	}
}

func TestDocsRouteRendersHTMLForBrowser(t *testing.T) {
	setDocsCredentials(t)

	// Definimos os cenários que queremos testar, garantindo a ordem: ab1 e depois ab2
	cenarios := []struct {
		nome  string
		busca string
	}{
		{nome: "Cenário AB1", busca: "/api/grades?exam=ab1"},
		{nome: "Cenário AB2", busca: "/api/grades?exam=ab2"},
	}

	for _, c := range cenarios {
		// t.Run isola a execução de cada cenário sequencialmente
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			setDocsBasicAuth(req)
			rec := httptest.NewRecorder()

			Handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", got)
			}

			body := rec.Body.String()

			// Validações fixas que devem existir em ambos
			stringsFixas := []string{"dbBack Documentação da API", "Abrir JSON"}
			for _, want := range stringsFixas {
				if !strings.Contains(body, want) {
					t.Fatalf("HTML body should contain %q", want)
				}
			}

			// Validação específica do cenário atual (ab1 ou ab2)
			if !strings.Contains(body, c.busca) {
				t.Fatalf("HTML body should contain %q", c.busca)
			}
		})
	}
}
func TestDocsRouteFormatJSONOverridesHTMLAccept(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs?format=json", nil)
	req.Header.Set("Accept", "text/html")
	setDocsBasicAuth(req)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "dbBack" {
		t.Fatalf("name = %q, want dbBack", payload.Name)
	}
}

func TestDocsRouteSupportsVercelFunctionPath(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/index.go/docs", nil)
	setDocsBasicAuth(req)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDocsRouteSupportsVercelFunctionPathWithoutExtension(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/index/docs", nil)
	setDocsBasicAuth(req)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDocsRouteSupportsDirectFunctionGet(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/index.go", nil)
	setDocsBasicAuth(req)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIPathUsesOriginalURLHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/index.go", nil)
	req.Header.Set("X-Vercel-Original-Url", "/api/docs")

	if got := apiPath(req); got != "/api/docs" {
		t.Fatalf("apiPath = %q, want /api/docs", got)
	}
}

func TestGradeExamSupportsPathAliases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		raw      string
		original string
		want     string
	}{
		{name: "query", path: "/api/grades", raw: "exam=ab1", want: "ab1"},
		{name: "exam path", path: "/api/grades/exam=ab2", want: "ab2"},
		{name: "exam path encoded pipe", path: "/api/grades/exam=ab1%7Cab2", want: "ab1"},
		{name: "short path", path: "/api/grades/ab1", want: "ab1"},
		{name: "pipe query uses first valid exam", path: "/api/grades", raw: "exam=ab1|ab2", want: "ab1"},
		{name: "pipe query accepts second valid exam", path: "/api/grades", raw: "exam=invalid|ab2", want: "ab2"},
		{name: "double encoded pipe query still finds first exam", path: "/api/grades", raw: "exam=ab1%257Cab2", want: "ab1"},
		{name: "query wins", path: "/api/grades/ab1", raw: "exam=ab2", want: "ab2"},
		{name: "vercel original url encoded query", path: "/api/index.go", original: "/api/grades?exam=ab1%7Cab2", want: "ab1"},
		{name: "vercel original url plain query", path: "/api/index.go", original: "/api/grades?exam=ab2|ab1", want: "ab2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.path
			if tt.raw != "" {
				target += "?" + tt.raw
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			if tt.original != "" {
				req.Header.Set("X-Vercel-Original-Url", tt.original)
			}
			if got := gradeExam(req, apiPath(req)); got != tt.want {
				t.Fatalf("gradeExam = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocsRouteRequiresBasicAuth(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Fatalf("WWW-Authenticate = %q, want Basic challenge", got)
	}
}

func TestDocsRouteRejectsInvalidBasicAuth(t *testing.T) {
	setDocsCredentials(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	req.SetBasicAuth("docs-test", "errada")
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDocsRouteUsesEnvCredentials(t *testing.T) {
	t.Setenv("DOCS_USERNAME", "doc")
	t.Setenv("DOCS_PASSWORD", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	req.SetBasicAuth("doc", "secret")
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDocsRouteRequiresConfiguredCredentials(t *testing.T) {
	t.Setenv("DOCS_USERNAME", "")
	t.Setenv("DOCS_PASSWORD", "")
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	req.SetBasicAuth("docs-test", "docs-password-test")
	rec := httptest.NewRecorder()

	Handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
