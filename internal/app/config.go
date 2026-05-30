package app

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Config struct {
	SpreadsheetID string
	LoginSheet    string
	AB1Sheet      string
	AB2Sheet      string
	SessionSecret string
	CookieSecure  bool
	ServiceJSON   string
	CacheTTL      time.Duration
}

func LoadConfig() Config {
	return Config{
		SpreadsheetID: firstNonEmpty(os.Getenv("GOOGLE_SHEET_ID"), "12zXd1oCQOdBhI88JWMrZ2req0c3XfFLJcVPXQ9CaKT8"),
		LoginSheet:    firstNonEmpty(os.Getenv("LOGIN_SHEET_NAME"), "Base de dados"),
		AB1Sheet:      firstNonEmpty(os.Getenv("SHEET_AB1_NAME"), "Notas AB1"),
		AB2Sheet:      firstNonEmpty(os.Getenv("SHEET_AB2_NAME"), "Projeto AB2"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		CookieSecure:  strings.EqualFold(firstNonEmpty(os.Getenv("COOKIE_SECURE"), "true"), "true"),
		ServiceJSON:   os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"),
		CacheTTL:      90 * time.Second,
	}
}

func (c Config) Validate() error {
	if c.SessionSecret == "" {
		return NewHTTPError(500, "SESSION_SECRET nao configurado")
	}
	if c.ServiceJSON == "" {
		return NewHTTPError(500, "GOOGLE_SERVICE_ACCOUNT_JSON nao configurado")
	}
	if !json.Valid([]byte(c.ServiceJSON)) {
		return NewHTTPError(500, "GOOGLE_SERVICE_ACCOUNT_JSON invalido")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
