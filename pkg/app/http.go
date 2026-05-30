package app

import (
	"encoding/json"
	"errors"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func Error(w http.ResponseWriter, err error) {
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		JSON(w, httpErr.Status, map[string]string{"error": httpErr.Message})
		return
	}
	JSON(w, http.StatusInternalServerError, map[string]string{"error": "erro interno"})
}

func Method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo nao permitido"})
	return false
}

func Bootstrap(r *http.Request) (Config, SessionManager, *SheetsClient, error) {
	cfg := LoadConfig()
	if err := cfg.Validate(); err != nil {
		return cfg, SessionManager{}, nil, err
	}
	client, err := NewSheetsClient(r.Context(), cfg)
	if err != nil {
		return cfg, SessionManager{}, nil, err
	}
	return cfg, NewSessionManager(cfg), client, nil
}
