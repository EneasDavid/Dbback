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
