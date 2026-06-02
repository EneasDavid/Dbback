package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"feedback/pkg/app"
)

type AuthController struct{}

const maxLoginBodyBytes = 1024

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
	defer r.Body.Close()
	var req loginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		app.Error(w, app.NewHTTPError(400, "json invalido"))
		return
	}
	matricula, ok := normalizeMatricula(req.Matricula)
	if !ok {
		app.Error(w, app.NewHTTPError(400, "informe a matricula"))
		return
	}
	identity, err := sheetsClient.LoginIdentity(r.Context(), matricula)
	if err != nil {
		app.Error(w, err)
		return
	}
	user := app.SessionUser{Matricula: identity.Matricula, Name: identity.Name, SpreadsheetID: identity.SpreadsheetID, SchemaStatus: identity.SchemaStatus}
	sessions.Set(w, user)
	warmGradesAfterLogin(sheetsClient, user)
	app.JSON(w, http.StatusOK, publicUser(user))
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
	app.JSON(w, http.StatusOK, publicUser(user))
}

func normalizeMatricula(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 32 {
		return "", false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	return value, true
}

func warmGradesAfterLogin(sheetsClient *app.SheetsClient, user app.SessionUser) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_, _ = sheetsClient.GradesFor(ctx, []string{"ab1", "ab2"}, user)
	}()
}
