package handler

import (
	"net/http"
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

	exam := gradeExam(r, path)
	if exam != "ab1" && exam != "ab2" {
		app.Error(w, app.NewHTTPError(400, "parametro exam invalido: deve ser 'ab1' ou 'ab2'"))
		return
	}

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

func gradeExam(r *http.Request, path string) string {
	if exam := normalizeGradeExam(r.URL.Query().Get("exam")); exam != "" {
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
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	for _, part := range strings.Split(value, "|") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "ab1" || part == "ab2" {
			return part
		}
	}
	return value
}
