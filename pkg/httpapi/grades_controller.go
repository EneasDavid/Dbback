package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"feedback/pkg/app"
)

type GradesController struct{}

func (GradesController) Show(w http.ResponseWriter, r *http.Request, path string) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	_, sessions, sheetsClient, err := app.Bootstrap(r)
	if err != nil {
		app.Error(w, err)
		return
	}
	user, ok := sessions.User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	user = ensureUserSpreadsheet(w, r, sessions, sheetsClient, user)

	exam := gradeExam(r, path)

	if r.URL.Query().Get("refresh") == "1" {
		sheetsClient.ClearCache()
	}
	result, err := sheetsClient.GradeFor(r.Context(), exam, user)
	if err != nil {
		app.Error(w, err)
		return
	}
	app.JSON(w, http.StatusOK, result)
}

func (GradesController) All(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	_, sessions, sheetsClient, err := app.Bootstrap(r)
	if err != nil {
		app.Error(w, err)
		return
	}
	user, ok := sessions.User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	user = ensureUserSpreadsheet(w, r, sessions, sheetsClient, user)

	if r.URL.Query().Get("refresh") == "1" {
		sheetsClient.ClearCache()
	}
	result, err := sheetsClient.GradesFor(r.Context(), []string{"ab1", "ab2"}, user)
	if err != nil {
		app.Error(w, err)
		return
	}
	app.JSON(w, http.StatusOK, result)
}

func ensureUserSpreadsheet(w http.ResponseWriter, r *http.Request, sessions app.SessionManager, sheetsClient *app.SheetsClient, user app.SessionUser) app.SessionUser {
	if strings.TrimSpace(user.SpreadsheetID) != "" {
		return user
	}
	identity, err := sheetsClient.LoginIdentity(r.Context(), user.Matricula)
	if err != nil {
		return user
	}
	user.SpreadsheetID = identity.SpreadsheetID
	user.SchemaStatus = identity.SchemaStatus
	if strings.TrimSpace(identity.Name) != "" {
		user.Name = identity.Name
	}
	sessions.Set(w, user)
	return user
}

func gradeExam(r *http.Request, path string) string {
	if exam := normalizeGradeExam(requestQueryValue(r, "exam")); exam != "" {
		return exam
	}
	suffix := strings.Trim(strings.TrimPrefix(path, "/api/grades"), "/")
	if suffix == "" {
		return ""
	}
	if value, ok := strings.CutPrefix(suffix, "exam="); ok {
		return normalizeGradeExam(value)
	}
	return normalizeGradeExam(suffix)
}

func normalizeGradeExam(value string) string {
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return value
}

func requestQueryValue(r *http.Request, key string) string {
	if value := r.URL.Query().Get(key); strings.TrimSpace(value) != "" {
		return value
	}
	for _, header := range []string{"X-Vercel-Original-Url", "X-Original-URL", "X-Rewrite-URL", "X-Forwarded-Uri"} {
		if value := queryValueFromHeader(r.Header.Get(header), key); value != "" {
			return value
		}
	}
	return ""
}

func queryValueFromHeader(header string, key string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parsed, err := url.Parse(header)
	if err != nil || !strings.HasPrefix(parsed.Path, "/api/") {
		return ""
	}
	return parsed.Query().Get(key)
}
