package handler

import (
	"net/http"

	"feedback/internal/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
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
