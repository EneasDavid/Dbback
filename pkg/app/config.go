package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Config struct {
	SpreadsheetID        string
	SpreadsheetIDs       []string
	LegacySpreadsheetIDs []string
	V2SpreadsheetIDs     []string
	RuntimeVersion       string
	MetadataKey          string
	MetadataValue        string
	LoginSheet           string
	AB1Tables            []TableConfig
	AB2Tables            []TableConfig
	SessionSecret        string
	CookieSecure         bool
	DocsUsername         string
	DocsPassword         string
	TurnstileSecret      string
	ServiceJSON          string
	ServiceFile          string
	CacheTTL             time.Duration
}

type TableConfig struct {
	Key          string
	Label        string
	SheetName    string
	Kind         string
	ScoreDivisor float64
}

func LoadConfig() Config {
	legacySpreadsheetIDs := splitSpreadsheetIDs(os.Getenv("GOOGLE_SHEET_LEGACY_IDS"))
	v2SpreadsheetIDs := splitSpreadsheetIDs(os.Getenv("GOOGLE_SHEET_V2_IDS"))
	spreadsheetIDs := spreadsheetIDsFromEnv(legacySpreadsheetIDs, v2SpreadsheetIDs)
	return Config{
		SpreadsheetID:        firstString(spreadsheetIDs),
		SpreadsheetIDs:       spreadsheetIDs,
		LegacySpreadsheetIDs: legacySpreadsheetIDs,
		V2SpreadsheetIDs:     v2SpreadsheetIDs,
		RuntimeVersion:       strings.ToLower(firstNonEmpty(os.Getenv("SHEETS_RUNTIME_VERSION"), "auto")),
		MetadataKey:          firstNonEmpty(os.Getenv("GOOGLE_SHEET_METADATA_KEY"), "dbback_schema"),
		MetadataValue:        firstNonEmpty(os.Getenv("GOOGLE_SHEET_METADATA_VALUE"), "v2"),
		LoginSheet:           firstNonEmpty(os.Getenv("LOGIN_SHEET_NAME"), "Base de dados"),
		AB1Tables: []TableConfig{
			tableFromEnv("at1", "Atividade 1", "SHEET_AB1_PESQUISA", "AT. 1", "activity", 10),
			tableFromEnv("at2", "Atividade 2", "SHEET_AB1_ARTIGO", "AT. 2", "activity", 10),
			tableFromEnv("at3", "Atividade 3", "SHEET_AB1_LISTA", "AT. 3", "activity", 10),
			tableFromEnv("prova", "Prova AB1", "SHEET_AB1_PROVA", firstNonEmpty(os.Getenv("SHEET_AB1_NAME"), "Notas AB1"), "summary", 1),
		},
		AB2Tables: []TableConfig{
			tableFromEnv("at4", "Atividade 4", "SHEET_AB2_LISTA", "AT. 4", "activity", 10),
			tableFromEnv("projeto", "Projeto AB2", "SHEET_AB2_PROJETO", firstNonEmpty(os.Getenv("SHEET_AB2_NAME"), "Projeto AB2"), "project", 1),
		},
		SessionSecret: os.Getenv("SESSION_SECRET"),
		CookieSecure:  strings.EqualFold(firstNonEmpty(os.Getenv("COOKIE_SECURE"), "true"), "true"),
		DocsUsername:  firstNonEmpty(os.Getenv("DOCS_USERNAME"), os.Getenv("DOCS_USER")),
		DocsPassword:  firstNonEmpty(os.Getenv("DOCS_PASSWORD"), os.Getenv("DOCS_PASS")),
		TurnstileSecret: firstNonEmpty(
			os.Getenv("TURNSTILE_SECRET_KEY"),
			os.Getenv("CF_TURNSTILE_SECRET_KEY"),
		),
		ServiceJSON: serviceAccountJSON(),
		ServiceFile: os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"),
		CacheTTL:    7 * time.Hour,
	}
}

func (c Config) Validate() error {
	if c.SessionSecret == "" {
		return NewHTTPError(500, "SESSION_SECRET nao configurado")
	}
	if len(c.SpreadsheetIDs) == 0 {
		return NewHTTPError(500, "GOOGLE_SHEET_ID, GOOGLE_SHEET_IDS, GOOGLE_SHEET_LEGACY_IDS ou GOOGLE_SHEET_V2_IDS nao configurado")
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

func spreadsheetIDsFromEnv(groups ...[]string) []string {
	seen := map[string]bool{}
	var result []string
	add := func(item string) {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			return
		}
		seen[item] = true
		result = append(result, item)
	}
	// Ordem de busca: 1) LEGACY_IDS, 2) V2_IDS, 3) GOOGLE_SHEET_ID, 4) GOOGLE_SHEET_IDS (mista)
	for _, group := range groups {
		for _, item := range group {
			add(item)
		}
	}
	for _, item := range splitSpreadsheetIDs(os.Getenv("GOOGLE_SHEET_ID")) {
		add(item)
	}
	for _, item := range splitSpreadsheetIDs(os.Getenv("GOOGLE_SHEET_IDS")) {
		add(item)
	}
	return result
}

func splitSpreadsheetIDs(raw string) []string {
	var result []string
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
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
