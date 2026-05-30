package handler

import (
	"net/http"

	"feedback/internal/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
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
