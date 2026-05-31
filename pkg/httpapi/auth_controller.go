package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"feedback/pkg/app"
)

type AuthController struct{}

type loginRequest struct {
	Matricula string `json:"matricula"`
}

func (AuthController) Login(w http.ResponseWriter, r *http.Request) {
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

func (AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg := app.LoadConfig()
	app.NewSessionManager(cfg).Clear(w)
	app.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (AuthController) Me(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	cfg := app.LoadConfig()
	user, ok := app.NewSessionManager(cfg).User(r)
	if !ok {
		app.JSON(w, http.StatusOK, nil)
		return
	}
	app.JSON(w, http.StatusOK, user)
}
