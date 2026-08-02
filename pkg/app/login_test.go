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
					rowSources: []string{"v2-sheet-a", "v2-sheet-b"},
					rowSchemas: []string{"v2", "v2"},
				},
			},
		},
	}

	identity, err := client.LoginIdentity(t.Context(), "6")
	if err != nil {
		t.Fatalf("LoginIdentity() error = %v", err)
	}
	if identity.Name != "Nova" || identity.SpreadsheetID != "v2-sheet-a" || identity.SchemaStatus != "v2" {
		t.Fatalf("LoginIdentity() = %#v, want first occurrence", identity)
	}
}

func TestLoginIdentityReturnsRowSourceAndSchemaStatus(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{
			LoginSheet:       "Base de dados",
			V2SpreadsheetIDs: []string{"v2-sheet-a", "v2-sheet-b"},
			SpreadsheetIDs:   []string{"v2-sheet-a", "v2-sheet-b"},
		},
		cache: map[string]cachedGrid{
			"Base de dados": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:    []string{"Matricula", "Nome"},
					rows:       [][]string{{"23210542", "Aluno"}},
					rowSources: []string{"v2-sheet-b"},
					rowSchemas: []string{"v2"},
				},
			},
		},
	}

	identity, err := client.LoginIdentity(t.Context(), "23210542")
	if err != nil {
		t.Fatalf("LoginIdentity() error = %v", err)
	}
	if identity.Name != "Aluno" || identity.SpreadsheetID != "v2-sheet-b" || identity.SchemaStatus != "v2" {
		t.Fatalf("LoginIdentity() = %#v, want row's own spreadsheet source and schema status", identity)
	}
}
