package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Config struct {
	SpreadsheetID string
	LoginSheet    string
	AB1Tables     []TableConfig
	AB2Tables     []TableConfig
	SessionSecret string
	CookieSecure  bool
	DocsUsername  string
	DocsPassword  string
	ServiceJSON   string
	ServiceFile   string
	CacheTTL      time.Duration
}

type TableConfig struct {
	Key          string
	Label        string
	SheetName    string
	Kind         string
	ScoreDivisor float64
}

func LoadConfig() Config {
	return Config{
		SpreadsheetID: strings.TrimSpace(os.Getenv("GOOGLE_SHEET_ID")),
		LoginSheet:    firstNonEmpty(os.Getenv("LOGIN_SHEET_NAME"), "Base de dados"),
		AB1Tables: []TableConfig{
			tableFromEnv("at1", "AT. 1", "SHEET_AB1_PESQUISA", "AT. 1", "activity", 10),
			tableFromEnv("at2", "AT. 2", "SHEET_AB1_ARTIGO", "AT. 2", "activity", 10),
			tableFromEnv("at3", "AT. 3", "SHEET_AB1_LISTA", "AT. 3", "activity", 10),
			tableFromEnv("prova", "Prova AB1", "SHEET_AB1_PROVA", firstNonEmpty(os.Getenv("SHEET_AB1_NAME"), "Notas AB1"), "summary", 1),
		},
		AB2Tables: []TableConfig{
			tableFromEnv("at4", "AT. 4", "SHEET_AB2_LISTA", "AT. 4", "activity", 10),
			tableFromEnv("projeto", "Projeto AB2", "SHEET_AB2_PROJETO", firstNonEmpty(os.Getenv("SHEET_AB2_NAME"), "Projeto AB2"), "project", 1),
		},
		SessionSecret: os.Getenv("SESSION_SECRET"),
		CookieSecure:  strings.EqualFold(firstNonEmpty(os.Getenv("COOKIE_SECURE"), "true"), "true"),
		DocsUsername:  firstNonEmpty(os.Getenv("DOCS_USERNAME"), os.Getenv("DOCS_USER"), "adão"),
		DocsPassword:  firstNonEmpty(os.Getenv("DOCS_PASSWORD"), os.Getenv("DOCS_PASS"), "primeiro"),
		ServiceJSON:   serviceAccountJSON(),
		ServiceFile:   os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"),
		CacheTTL:      7 * time.Hour,
	}
}

func (c Config) Validate() error {
	if c.SessionSecret == "" {
		return NewHTTPError(500, "SESSION_SECRET nao configurado")
	}
	if c.SpreadsheetID == "" {
		return NewHTTPError(500, "GOOGLE_SHEET_ID nao configurado")
	}
	if c.ServiceJSON == "" {
		if strings.TrimSpace(c.ServiceFile) != "" {
			return NewHTTPError(500, "credencial Google nao encontrada em GOOGLE_SERVICE_ACCOUNT_FILE; no Vercel use GOOGLE_SERVICE_ACCOUNT_JSON_BASE64")
		}
		return NewHTTPError(500, "GOOGLE_SERVICE_ACCOUNT_JSON ou GOOGLE_SERVICE_ACCOUNT_JSON_BASE64 nao configurado")
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

func tableFromEnv(key string, label string, envName string, fallback string, kind string, scoreDivisor float64) TableConfig {
	return TableConfig{
		Key:          key,
		Label:        label,
		SheetName:    firstNonEmpty(os.Getenv(envName), fallback),
		Kind:         kind,
		ScoreDivisor: scoreDivisor,
	}
}

func serviceAccountJSON() string {
	if raw := cleanEnvValue(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")); raw != "" {
		return raw
	}
	if encoded := cleanEnvValue(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64")); encoded != "" {
		decoded, err := decodeBase64Env(encoded)
		if err == nil {
			return string(decoded)
		}
	}
	if path := strings.TrimSpace(os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE")); path != "" {
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content)
		}
	}
	return ""
}

func cleanEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func decodeBase64Env(value string) ([]byte, error) {
	value = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, value)

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = base64.CorruptInputError(0)
	}
	return nil, lastErr
}
