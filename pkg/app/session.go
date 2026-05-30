package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const cookieName = "feedback_session"

type SessionManager struct {
	secret []byte
	secure bool
}

type SessionUser struct {
	Matricula string `json:"matricula"`
	Name      string `json:"name"`
}

func NewSessionManager(cfg Config) SessionManager {
	return SessionManager{secret: []byte(cfg.SessionSecret), secure: cfg.CookieSecure}
}

func (s SessionManager) Set(w http.ResponseWriter, user SessionUser) {
	expires := time.Now().Add(8 * time.Hour)
	payload := fmt.Sprintf("%s|%s|%d", url.QueryEscape(user.Matricula), url.QueryEscape(user.Name), expires.Unix())
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

func (s SessionManager) User(r *http.Request) (SessionUser, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return SessionUser{}, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return SessionUser{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return SessionUser{}, false
	}
	payload := string(raw)
	if !hmac.Equal([]byte(parts[1]), []byte(sign(payload, s.secret))) {
		return SessionUser{}, false
	}
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) < 2 {
		return SessionUser{}, false
	}
	expiresValue := payloadParts[len(payloadParts)-1]
	expires, err := strconv.ParseInt(expiresValue, 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return SessionUser{}, false
	}
	matricula, err := url.QueryUnescape(payloadParts[0])
	if err != nil {
		return SessionUser{}, false
	}
	name := ""
	if len(payloadParts) >= 3 {
		name, err = url.QueryUnescape(payloadParts[1])
		if err != nil {
			return SessionUser{}, false
		}
	}
	return SessionUser{Matricula: matricula, Name: name}, true
}

func (s SessionManager) Matricula(r *http.Request) (string, bool) {
	user, ok := s.User(r)
	return user.Matricula, ok
}

func sign(payload string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
