package handler

import (
	"net/http"

	"feedback/internal/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg := app.LoadConfig()
	app.NewSessionManager(cfg).Clear(w)
	app.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
