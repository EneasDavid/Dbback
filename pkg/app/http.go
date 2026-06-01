package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var runtimeCache struct {
	sync.Mutex
	key    string
	client *SheetsClient
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
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
	key := bootstrapKey(cfg)
	runtimeCache.Lock()
	client := runtimeCache.client
	if client == nil || runtimeCache.key != key {
		var err error
		client, err = NewSheetsClient(r.Context(), cfg)
		if err != nil {
			runtimeCache.Unlock()
			return cfg, SessionManager{}, nil, err
		}
		runtimeCache.key = key
		runtimeCache.client = client
	}
	runtimeCache.Unlock()
	return cfg, NewSessionManager(cfg), client, nil
}

func bootstrapKey(cfg Config) string {
	parts := []string{
		strings.Join(cfg.SpreadsheetIDs, "\x01"),
		cfg.RuntimeVersion,
		cfg.MetadataKey,
		cfg.MetadataValue,
		cfg.LoginSheet,
		cfg.SessionSecret,
		cfg.ServiceJSON,
		cfg.ServiceFile,
	}
	for _, table := range append(cfg.AB1Tables, cfg.AB2Tables...) {
		parts = append(parts, table.Key, table.SheetName, table.Kind, strconv.FormatFloat(table.ScoreDivisor, 'f', -1, 64))
	}
	return strings.Join(parts, "\x00")
}
