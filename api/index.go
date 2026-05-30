package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"feedback/internal/app"
)

type loginRequest struct {
	Matricula string `json:"matricula"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimSuffix(r.URL.Path, "/") {
	case "/api/login":
		login(w, r)
	case "/api/logout":
		logout(w, r)
	case "/api/me":
		me(w, r)
	case "/api/grades":
		grades(w, r)
	default:
		app.Error(w, app.NewHTTPError(http.StatusNotFound, "rota nao encontrada"))
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	_, sessions, sheetsClient, err := app.Bootstrap(r)
	if err != nil {
		app.Error(w, err)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.Error(w, app.NewHTTPError(400, "json invalido"))
		return
	}
	matricula := strings.TrimSpace(req.Matricula)
	if matricula == "" {
		app.Error(w, app.NewHTTPError(400, "informe a matricula"))
		return
	}
	ok, err := sheetsClient.MatriculaExists(r.Context(), matricula)
	if err != nil {
		app.Error(w, err)
		return
	}
	if !ok {
		app.Error(w, app.NewHTTPError(401, "matricula nao autorizada"))
		return
	}
	sessions.Set(w, matricula)
	app.JSON(w, http.StatusOK, map[string]any{"matricula": matricula})
}

func logout(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg := app.LoadConfig()
	app.NewSessionManager(cfg).Clear(w)
	app.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func me(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	cfg := app.LoadConfig()
	matricula, ok := app.NewSessionManager(cfg).Matricula(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	app.JSON(w, http.StatusOK, map[string]string{"matricula": matricula})
}

func grades(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	_, sessions, sheetsClient, err := app.Bootstrap(r)
	if err != nil {
		app.Error(w, err)
		return
	}
	matricula, ok := sessions.Matricula(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	result, err := sheetsClient.GradeFor(r.Context(), r.URL.Query().Get("exam"), matricula)
	if err != nil {
		app.Error(w, err)
		return
	}
	app.JSON(w, http.StatusOK, result)
}
