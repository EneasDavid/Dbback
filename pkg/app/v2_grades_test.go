package app

import (
	"testing"
	"time"
)

func TestV2ABStateUsesActiveColumn(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Nome", "Ativa"},
		rows: [][]string{
			{"AB1", "AB1", "sim"},
			{"AB2", "AB2", "não"},
		},
	}

	label, active := v2ABState(grid, "ab2")

	if label != "AB2" || active {
		t.Fatalf("v2ABState() = %q/%v, want AB2/false", label, active)
	}
}

func TestV2ABStateTreatsZeroStatusAsInactive(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Status"},
		rows: [][]string{
			{"AB1", "0"},
			{"AB2", "1"},
		},
	}

	_, active := v2ABState(grid, "ab1")

	if active {
		t.Fatal("v2ABState() active = true, want false for status 0")
	}
}

func TestV2ABStateTreatsBlankStatusAsInactiveWhenStatusColumnExists(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Status"},
		rows:    [][]string{{"AB1", ""}},
	}

	_, active := v2ABState(grid, "ab1")

	if active {
		t.Fatal("v2ABState() active = true, want false for blank status")
	}
}

func TestV2ABStateInfersStatusColumnFromZeroOneValues(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", ""},
		rows: [][]string{
			{"Ab. 1", "0"},
			{"Ab. 2", "1"},
		},
	}

	_, ab1Active := v2ABState(grid, "ab1")
	_, ab2Active := v2ABState(grid, "ab2")

	if ab1Active || !ab2Active {
		t.Fatalf("v2ABState() active states = %v/%v, want false/true", ab1Active, ab2Active)
	}
}

func TestV2ResolveABUsesGenericABColumnValues(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Nome", "Status"},
		rows: [][]string{
			{"AV2", "Avaliação 2", "1"},
			{"AV1", "Avaliação 1", "1"},
		},
	}

	ab, found := v2ResolveAB(grid, "av1|ab2")

	if !found {
		t.Fatal("v2ResolveAB() found = false, want true")
	}
	if ab.Key != "av1" || ab.Label != "Avaliação 1" || !ab.Active {
		t.Fatalf("v2ResolveAB() = %#v, want AV1 config", ab)
	}
}

func TestV2ResolveABDefaultsToFirstABWhenRouteIsEmpty(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Status"},
		rows:    [][]string{{"AV1", "1"}, {"AV2", "1"}},
	}

	ab, found := v2ResolveAB(grid, "")

	if !found || ab.Key != "av1" {
		t.Fatalf("v2ResolveAB() = %#v/%v, want first AB", ab, found)
	}
}

func TestV2ActivitiesForABUsesABAndWeight(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Atividade", "AB", "Peso Máximo", "Aba", "Status"},
		rows: [][]string{
			{"Modelo", "AB2", "3", "Projeto", "1"},
			{"Pesquisa", "AB1", "2", "", "1"},
			{"Lista", "AB1", "1,5", "AT. Lista", "0"},
		},
		schemaStatus:  "v2",
		spreadsheetID: "sheet-a",
	}

	activities := v2ActivitiesForAB(grid, "ab1")

	if len(activities) != 1 {
		t.Fatalf("activities len = %d, want 1: %#v", len(activities), activities)
	}
	if activities[0].Label != "Pesquisa" || activities[0].SheetName != "Pesquisa" || activities[0].Weight != 2 {
		t.Fatalf("unexpected activity: %#v", activities[0])
	}
}

func TestV2ActivitiesForABKeepsActivitiesWhenStatusColumnIsAbsent(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"atividade", "peso", "ab"},
		rows:    [][]string{{"pesquisa", "1", "Ab. 1"}, {"artigo", "1", "Ab. 1"}},
	}

	activities := v2ActivitiesForAB(grid, "ab1")

	if len(activities) != 2 {
		t.Fatalf("activities len = %d, want 2: %#v", len(activities), activities)
	}
}

func TestV2ActivityItemsNormalizesCriteriaByWeightAndKeepsComments(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Critério A", "Critério B"},
		rows: [][]string{
			{"Nota máxima", "2", "3"},
			{"123", "1", "3"},
		},
		rowNotes: [][]string{
			{"", "", ""},
			{"", "comentário A", "comentário B"},
		},
		rowNoteAuthors: [][]string{
			{"", "", ""},
			{"", "Prof", "Prof"},
		},
	}

	items := v2ActivityItems(grid, 0, 1, 2)

	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %#v", len(items), items)
	}
	if items[0].NotaMaxima != "0,8" || items[0].NotaAlcancada != "0,4" {
		t.Fatalf("first normalized item = %#v, want max 0,8 and value 0,4", items[0])
	}
	if items[1].Comment != "comentário B" || items[1].CommentAuthor != "Prof" {
		t.Fatalf("second item comment = %#v", items[1])
	}
}

func TestV2ActivityItemsIncludesGenericCriteriaWithoutMaxRow(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Originalidade", "Entrega"},
		rows:    [][]string{{"123", "0,8", "0,7"}},
	}

	items := v2ActivityItems(grid, -1, 0, 1)

	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %#v", len(items), items)
	}
	if items[0].Subtopic != "Originalidade" || items[0].NotaMaxima != "0,5" {
		t.Fatalf("unexpected generic criterion: %#v", items[0])
	}
}

func TestV2ActivityLaunchedSkipsNotLaunchedText(t *testing.T) {
	activity := v2ActivityConfig{SummaryCol: 1}
	row := []string{"123", "Essa atividade não foi lançada"}

	if v2ActivityLaunched(row, activity) {
		t.Fatal("v2ActivityLaunched() = true, want false")
	}
}

func TestV2SummarySheetNameUsesNormalizedABKey(t *testing.T) {
	if got := v2SummarySheetName("AV 1"); got != "nota av1" {
		t.Fatalf("v2SummarySheetName() = %q, want nota av1", got)
	}
}

func TestLegacyExamKeyKeepsLegacyRouteAliases(t *testing.T) {
	if got := legacyExamKey("invalid|ab2"); got != "ab2" {
		t.Fatalf("legacyExamKey() = %q, want ab2", got)
	}
}

func TestRuntimeForUserPreservesLegacySessionWithV2Config(t *testing.T) {
	got := runtimeForUser(Config{RuntimeVersion: "v2"}, SessionUser{SchemaStatus: "legacy"})

	if got != "legacy" {
		t.Fatalf("runtimeForUser() = %q, want legacy", got)
	}
}

func TestGradesForRuntimeV2DoesNotFallbackToLegacyWhenAbsExists(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "auto"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "status"},
					rows:    [][]string{{"AB2", "1"}},
				},
			},
			v2ActivitiesSheet: {
				expires: time.Now().Add(time.Hour),
				grid:    &sheetGrid{headers: []string{"atividade", "AB"}, rows: [][]string{{"Pesquisa", "AB2"}}},
			},
			"nota ab2": {
				expires: time.Now().Add(time.Hour),
				grid:    &sheetGrid{headers: []string{"Matrícula", "Pesquisa"}, rows: [][]string{}},
			},
		},
	}

	results, err := client.gradesForRuntimeV2(t.Context(), []string{"ab1", "ab2"}, SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradesForRuntimeV2() error = %v", err)
	}
	if _, ok := results["ab2"]; !ok {
		t.Fatalf("gradesForRuntimeV2() keys = %#v, want only active v2 exam", results)
	}
	if _, ok := results["ab1"]; ok {
		t.Fatalf("gradesForRuntimeV2() unexpectedly returned legacy ab1: %#v", results)
	}
}

func TestGradeForV2ReturnsEmptyWhenSummarySheetDoesNotExist(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:       []string{"AB", "status"},
					rows:          [][]string{{"AB2", "1"}},
					schemaStatus:  "v2",
					spreadsheetID: "sheet-v2",
				},
			},
			v2ActivitiesSheet: {
				expires: time.Now().Add(time.Hour),
				grid:    &sheetGrid{headers: []string{"atividade", "AB"}, rows: [][]string{{"Pesquisa", "AB2"}}},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab2", SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	if result.Exam != "AB2" || len(result.Tables) != 0 {
		t.Fatalf("gradeForV2() = %#v, want empty AB2 result", result)
	}
}
