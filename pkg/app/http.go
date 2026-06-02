package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var runtimeCache struct {
	sync.Mutex
	key    string
	client *SheetsClient
}

func JSON(w http.ResponseWriter, status int, body any) {
	status, payload := jsonPayload(status, body)
	SecureHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setIntegrityHeaders(w, payload)
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func CacheableJSON(w http.ResponseWriter, r *http.Request, status int, body any, maxAge time.Duration, staleWhileRevalidate time.Duration) {
	status, payload := jsonPayload(status, body)
	SecureHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setIntegrityHeaders(w, payload)
	if status == http.StatusOK {
		etag := etagForPayload(payload)
		w.Header().Set("Cache-Control", cacheControlValue(maxAge, staleWhileRevalidate))
		w.Header().Set("ETag", etag)
		w.Header().Set("Vary", "Cookie, Accept-Encoding")
		if etagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func jsonPayload(status int, body any) (int, []byte) {
	payload, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		payload = []byte(`{"error":"erro interno"}`)
	}
	payload = append(payload, '\n')
	return status, payload
}

func setIntegrityHeaders(w http.ResponseWriter, payload []byte) {
	digest := sha256.Sum256(payload)
	encodedDigest := base64.StdEncoding.EncodeToString(digest[:])
	w.Header().Set("Digest", "sha-256=:"+encodedDigest+":")
	w.Header().Set("X-Dbback-Content-SHA256", encodedDigest)
}

func SecureHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
}

func etagForPayload(payload []byte) string {
	digest := sha256.Sum256(payload)
	return `"` + base64.RawURLEncoding.EncodeToString(digest[:]) + `"`
}

func cacheControlValue(maxAge time.Duration, staleWhileRevalidate time.Duration) string {
	return "private, max-age=" + strconv.Itoa(max(0, int(maxAge.Seconds()))) +
		", stale-while-revalidate=" + strconv.Itoa(max(0, int(staleWhileRevalidate.Seconds())))
}

func etagMatches(header string, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
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
		if allowed == http.MethodPost && !sameOrigin(r) {
			JSON(w, http.StatusForbidden, map[string]string{"error": "origem nao permitida"})
			return false
		}
		return true
	}
	w.Header().Set("Allow", allowed)
	JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo nao permitido"})
	return false
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.TrimSpace(r.Host)
	forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	return strings.EqualFold(parsed.Host, host) || (forwardedHost != "" && strings.EqualFold(parsed.Host, forwardedHost))
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
