package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"feedback/pkg/app"
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
	identity, err := sheetsClient.LoginIdentity(r.Context(), matricula)
	if err != nil {
		app.Error(w, err)
		return
	}
	user := app.SessionUser{Matricula: identity.Matricula, Name: identity.Name}
	sessions.Set(w, user)
	app.JSON(w, http.StatusOK, user)
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
	user, ok := app.NewSessionManager(cfg).User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	app.JSON(w, http.StatusOK, user)
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
	user, ok := sessions.User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(401, "sessao expirada"))
		return
	}
	
	// Validate exam parameter strictly
	exam := strings.TrimSpace(r.URL.Query().Get("exam"))
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
