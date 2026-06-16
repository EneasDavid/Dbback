package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"feedback/pkg/app"
)

type AuthController struct{}

const maxLoginBodyBytes = 4096

type loginRequest struct {
	Matricula      string `json:"matricula"`
	TurnstileToken string `json:"turnstileToken"`
}

func (AuthController) Login(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg := app.LoadConfig()
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
	if err := app.ValidateTurnstile(r.Context(), cfg.TurnstileSecret, req.TurnstileToken, turnstileRemoteIP(r)); err != nil {
		app.Error(w, err)
		return
	}
	_, sessions, sheetsClient, err := app.Bootstrap(r)
	if err != nil {
		app.Error(w, err)
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
	if len(value) < 1 || len(value) > 32 {
		return "", false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	return value, true
}

func turnstileRemoteIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		firstIP, _, _ := strings.Cut(forwardedFor, ",")
		return strings.TrimSpace(firstIP)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func warmGradesAfterLogin(sheetsClient *app.SheetsClient, user app.SessionUser) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_, _ = sheetsClient.GradeFor(ctx, "ab1", user)
		if ctx.Err() != nil {
			return
		}
		_, _ = sheetsClient.GradesFor(ctx, []string{"ab1", "ab2"}, user)
	}()
}
