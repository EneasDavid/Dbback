package app

import (
	"testing"
	"time"
)

func TestSameLookupValueMatchesFormattedNumericMatricula(t *testing.T) {
	cases := [][2]string{
		{"6", "6,0"},
		{"6", "6.00"},
		{"0006", "6"},
		{"21112993", "21112993,0"},
	}

	for _, tc := range cases {
		if !sameLookupValue(tc[0], tc[1], false) {
			t.Fatalf("sameLookupValue(%q, %q) = false, want true", tc[0], tc[1])
		}
	}
}

func TestLoginIdentityUsesFirstConfiguredBaseOccurrence(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{LoginSheet: "Base de dados"},
		cache: map[string]cachedGrid{
			"Base de dados": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:    []string{"Matricula", "Nome"},
					rows:       [][]string{{"6,0", "Nova"}, {"6", "Antiga"}},
					rowSources: []string{"v2-sheet", "v1-sheet"},
					rowSchemas: []string{"v2", "legacy"},
				},
			},
		},
	}

	identity, err := client.LoginIdentity(t.Context(), "6")
	if err != nil {
		t.Fatalf("LoginIdentity() error = %v", err)
	}
	if identity.Name != "Nova" || identity.SpreadsheetID != "v2-sheet" || identity.SchemaStatus != "v2" {
		t.Fatalf("LoginIdentity() = %#v, want first v2 occurrence", identity)
	}
}

func TestLoginIdentitySearchesLegacyFirst(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{
			LoginSheet:           "Base de dados",
			LegacySpreadsheetIDs: []string{"legacy-sheet"},
			V2SpreadsheetIDs:     []string{"v2-sheet"},
			SpreadsheetIDs:       []string{"legacy-sheet", "v2-sheet"},
		},
		cache: map[string]cachedGrid{
			"Base de dados": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:    []string{"Matricula", "Nome"},
					rows:       [][]string{{"23210542", "Legado User"}},
					rowSources: []string{"legacy-sheet"},
					rowSchemas: []string{"legacy"},
				},
			},
		},
	}

	identity, err := client.LoginIdentity(t.Context(), "23210542")
	if err != nil {
		t.Fatalf("LoginIdentity() error = %v", err)
	}
	if identity.Name != "Legado User" || identity.SpreadsheetID != "legacy-sheet" || identity.SchemaStatus != "legacy" {
		t.Fatalf("LoginIdentity() = %#v, want legacy base occurrence", identity)
	}
}
