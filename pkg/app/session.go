package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const cookieName = "feedback_session"

type SessionManager struct {
	secret []byte
	secure bool
}

func NewSessionManager(cfg Config) SessionManager {
	return SessionManager{secret: []byte(cfg.SessionSecret), secure: cfg.CookieSecure}
}

func (s SessionManager) Set(w http.ResponseWriter, matricula string) {
	expires := time.Now().Add(8 * time.Hour)
	payload := fmt.Sprintf("%s|%d", matricula, expires.Unix())
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sign(payload, s.secret)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s SessionManager) Matricula(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	payload := string(raw)
	if !hmac.Equal([]byte(parts[1]), []byte(sign(payload, s.secret))) {
		return "", false
	}
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 2 {
		return "", false
	}
	expires, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return "", false
	}
	return payloadParts[0], true
}

func sign(payload string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
