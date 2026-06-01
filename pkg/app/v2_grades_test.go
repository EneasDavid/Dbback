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

func TestV2ExamKeysReturnsOnlyActiveABs(t *testing.T) {
	client := &SheetsClient{
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Ab", "status"},
					rows: [][]string{
						{"Ab. 1", "0"},
						{"Ab. 2", "1"},
						{"reav", "0"},
						{"final", "0"},
					},
				},
			},
		},
	}

	keys, err := client.v2ExamKeys(t.Context())
	if err != nil {
		t.Fatalf("v2ExamKeys() error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "ab2" {
		t.Fatalf("v2ExamKeys() = %#v, want only ab2", keys)
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

func TestV2ActivityItemsKeepsRubricScaleAndComments(t *testing.T) {
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
	if items[0].NotaMaxima != "2" || items[0].NotaAlcancada != "1" {
		t.Fatalf("first rubric item = %#v, want max 2 and value 1", items[0])
	}
	if items[1].Comment != "comentário B" || items[1].CommentAuthor != "Prof" {
		t.Fatalf("second item comment = %#v", items[1])
	}
}

func TestV2ActivityItemsCapsScoresAtCriterionMaximum(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Critério A", "Critério B"},
		rows: [][]string{
			{"Nota máxima", "1", "2"},
			{"123", "2", "3"},
		},
	}

	items := v2ActivityItems(grid, 0, 1, 1)

	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %#v", len(items), items)
	}
	if items[0].NotaMaxima != "1" || items[0].NotaAlcancada != "1" {
		t.Fatalf("first capped item = %#v, want max/value 1", items[0])
	}
	if items[1].NotaMaxima != "2" || items[1].NotaAlcancada != "2" {
		t.Fatalf("second capped item = %#v, want max/value 2", items[1])
	}
}

func TestV2ActivityItemsDoesNotExposeFinalGradeAsCriterion(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Critério A", "nota final"},
		rows: [][]string{
			{"Nota máxima", "1", "1"},
			{"123", "0,8", "0,8"},
		},
	}

	items := v2ActivityItems(grid, 0, 1, 1)

	if len(items) != 1 {
		t.Fatalf("items len = %d, want only real criteria: %#v", len(items), items)
	}
	if items[0].Subtopic != "Critério A" {
		t.Fatalf("item = %#v, want Critério A only", items[0])
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
	if items[0].Subtopic != "Originalidade" || items[0].NotaMaxima != "1" {
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

func TestRuntimeForUserUsesForcedV2Config(t *testing.T) {
	got := runtimeForUser(Config{RuntimeVersion: "v2"}, SessionUser{})

	if got != "v2" {
		t.Fatalf("runtimeForUser() = %q, want v2", got)
	}
}

func TestCandidateSpreadsheetIDsPrioritizesLegacyBases(t *testing.T) {
	client := &SheetsClient{cfg: Config{
		SpreadsheetIDs:       []string{"mixed-a", "v2-a", "legacy-a"},
		LegacySpreadsheetIDs: []string{"legacy-a"},
		V2SpreadsheetIDs:     []string{"v2-a"},
		RuntimeVersion:       "v2",
	}}

	got := client.candidateSpreadsheetIDs(SessionUser{})
	want := []string{"legacy-a", "v2-a", "mixed-a"}
	if len(got) != len(want) {
		t.Fatalf("candidateSpreadsheetIDs() = %#v, want %#v", got, want)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("candidateSpreadsheetIDs() = %#v, want %#v", got, want)
		}
	}
}

func TestGradesForRuntimeV2UsesAbsKeysWhenAbsExists(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "auto"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "status"},
					rows:    [][]string{{"AB1", "0"}, {"AB2", "1"}},
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
		t.Fatalf("gradesForRuntimeV2() keys = %#v, want v2 ab2 exam", results)
	}
	if _, ok := results["ab1"]; ok {
		t.Fatalf("gradesForRuntimeV2() returned inactive ab1: %#v", results)
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

func TestV2ActivityTableIncludesTopicsAndComments(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:       []string{"AB", "status"},
					rows:          [][]string{{"AB1", "1"}},
					schemaStatus:  "v2",
					spreadsheetID: "sheet-v2",
				},
			},
			v2ActivitiesSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Atividade", "AB", "Peso", "Aba"},
					rows:    [][]string{{"Critérios de Aceite", "AB1", "3", "AT. Aceite"}},
				},
			},
			"nota ab1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critérios de Aceite"},
					rows:    [][]string{{"6342342", "3,0"}},
				},
			},
			"AT. Aceite": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Cobertura", "Manutenibilidade", "Design"},
					rows: [][]string{
						{"Nota máxima", "1", "1", "1"},
						{"6342342", "1", "0,8", "0,9"},
					},
					rowNotes: [][]string{
						{"", "", "", ""},
						{"", "Ótima cobertura", "Código bem estruturado", "Bom design patterns"},
					},
					rowNoteAuthors: [][]string{
						{"", "", "", ""},
						{"", "Prof. Silva", "Prof. Silva", "Prof. Santos"},
					},
					schemaStatus:  "v2",
					spreadsheetID: "sheet-v2",
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab1", SessionUser{Matricula: "6342342", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}

	if len(result.Tables) != 1 {
		t.Fatalf("gradeForV2() tables len = %d, want 1", len(result.Tables))
	}

	table := result.Tables[0]
	if len(table.Cards) != 1 {
		t.Fatalf("table.Cards len = %d, want 1", len(table.Cards))
	}

	card := table.Cards[0]
	if len(card.Details) != 3 {
		t.Fatalf("card.Details len = %d, want 3 (Cobertura, Manutenibilidade, Design)", len(card.Details))
	}

	// Verify topics and comments
	if card.Details[0].Label != "Cobertura" || card.Details[0].Value != "1" {
		t.Fatalf("Details[0] = %#v, want Cobertura with value 1", card.Details[0])
	}
	if card.Details[0].Comment != "Ótima cobertura" || card.Details[0].CommentAuthor != "Prof. Silva" {
		t.Fatalf("Details[0] comment = %q/%q, want 'Ótima cobertura'/'Prof. Silva'", card.Details[0].Comment, card.Details[0].CommentAuthor)
	}

	if card.Details[1].Label != "Manutenibilidade" || card.Details[1].Value != "0,8" {
		t.Fatalf("Details[1] = %#v, want Manutenibilidade with value 0,8", card.Details[1])
	}
	if card.Details[1].Comment != "Código bem estruturado" || card.Details[1].CommentAuthor != "Prof. Silva" {
		t.Fatalf("Details[1] comment = %q/%q, want 'Código bem estruturado'/'Prof. Silva'", card.Details[1].Comment, card.Details[1].CommentAuthor)
	}

	if card.Details[2].Label != "Design" || card.Details[2].Value != "0,9" {
		t.Fatalf("Details[2] = %#v, want Design with value 0,9", card.Details[2])
	}
	if card.Details[2].Comment != "Bom design patterns" || card.Details[2].CommentAuthor != "Prof. Santos" {
		t.Fatalf("Details[2] comment = %q/%q, want 'Bom design patterns'/'Prof. Santos'", card.Details[2].Comment, card.Details[2].CommentAuthor)
	}
}

func TestV2ActivityTableKeepsCriteriaAndCommentsWhenNoSimilarGroup(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:       []string{"AB", "status"},
					rows:          [][]string{{"AB1", "1"}},
					schemaStatus:  "v2",
					spreadsheetID: "sheet-v2",
				},
			},
			v2ActivitiesSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Atividade", "AB", "Peso", "Aba"},
					rows:    [][]string{{"Critérios de Aceite", "AB1", "3", "AT. Aceite"}},
				},
			},
			"nota ab1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Grupo", "Critérios de Aceite"},
					rows:    [][]string{{"6342342", "G99", "2,5"}},
				},
			},
			"AT. Aceite": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Grupo", "Cobertura", "Manutenibilidade"},
					rows: [][]string{
						{"Nota máxima", "1", "2"},
						{"G01", "1", "1,5"},
					},
					rowNotes: [][]string{
						{"", "Comentário do critério cobertura", "Comentário do critério manutenção"},
						{"", "", ""},
					},
					rowNoteAuthors: [][]string{
						{"", "Prof. Silva", "Prof. Santos"},
						{"", "", ""},
					},
					schemaStatus:  "v2",
					spreadsheetID: "sheet-v2",
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab1", SessionUser{Matricula: "6342342", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}

	if len(result.Tables) != 1 {
		t.Fatalf("gradeForV2() tables len = %d, want 1", len(result.Tables))
	}
	card := result.Tables[0].Cards[0]
	if card.Value != "2,5" || card.DisplayValue != "2,5/3" {
		t.Fatalf("card score = %q/%q, want summary score 2,5/3", card.Value, card.DisplayValue)
	}
	if len(card.Details) != 2 {
		t.Fatalf("card.Details len = %d, want criteria without matching group: %#v", len(card.Details), card.Details)
	}
	if card.Details[0].Value != "" || !card.Details[0].Pending {
		t.Fatalf("first detail = %#v, want pending blank score", card.Details[0])
	}
	if card.Details[0].Comment != "Comentário do critério cobertura" || card.Details[0].CommentAuthor != "Prof. Silva" {
		t.Fatalf("first detail comment = %q/%q", card.Details[0].Comment, card.Details[0].CommentAuthor)
	}
	if card.Details[1].Comment != "Comentário do critério manutenção" || card.Details[1].CommentAuthor != "Prof. Santos" {
		t.Fatalf("second detail comment = %q/%q", card.Details[1].Comment, card.Details[1].CommentAuthor)
	}
}

func TestV2ActivityTableUsesSummaryScoreWithoutRubricNormalization(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "status"},
					rows:    [][]string{{"AB1", "1"}},
				},
			},
			v2ActivitiesSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Atividade", "AB", "Peso", "Aba"},
					rows:    [][]string{{"Escala Dez", "AB1", "1", "AT. Dez"}},
				},
			},
			"nota ab1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Escala Dez"},
					rows:    [][]string{{"123", "12"}},
				},
			},
			"AT. Dez": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critério"},
					rows: [][]string{
						{"Nota máxima", "10"},
						{"123", "12"},
					},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab1", SessionUser{Matricula: "123", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	card := result.Tables[0].Cards[0]
	if card.Value != "12" || card.DisplayValue != "12/1" {
		t.Fatalf("card score = %q/%q, want raw summary score 12/1", card.Value, card.DisplayValue)
	}
	if card.Details[0].Value != "10" || card.Details[0].Max != 10 {
		t.Fatalf("detail = %#v, want rubric value/max 10", card.Details[0])
	}
}

func TestV2AverageCardUsesNotaLabel(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Média"},
		rows:    [][]string{{"123", "0,5"}},
	}

	card := v2AverageCard(grid, grid.rows[0])

	if card == nil {
		t.Fatal("v2AverageCard() = nil, want card")
	}
	if card.Label != "Nota" || card.Value != "0,5" {
		t.Fatalf("v2AverageCard() = %#v, want Nota 0,5", card)
	}
}

func TestV2AverageCardCapsAtTen(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Média"},
		rows:    [][]string{{"123", "12"}},
	}

	card := v2AverageCard(grid, grid.rows[0])

	if card == nil {
		t.Fatal("v2AverageCard() = nil, want card")
	}
	if card.Value != "10" || card.DisplayValue != "10" {
		t.Fatalf("v2AverageCard() = %#v, want capped 10", card)
	}
}

func TestV2ActivityTableUsesCriteriaScoreWhenSummaryIsPending(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
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
				grid: &sheetGrid{
					headers: []string{"Atividade", "AB", "Peso", "Aba"},
					rows:    [][]string{{"Projeto", "AB2", "3", "Projeto"}},
				},
			},
			"nota ab2": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Projeto"},
					rows:    [][]string{{"123", "Não corrigida ainda"}},
					rowNotes: [][]string{
						{"", "Comentário geral do projeto"},
					},
					rowNoteAuthors: [][]string{
						{"", "Prof. Geral"},
					},
				},
			},
			"Projeto": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critério 1", "Critério 2", "Critério 3"},
					rows: [][]string{
						{"Nota máxima", "1", "1", "1"},
						{"123", "1", "0,5", "0,5"},
					},
					rowNotes: [][]string{
						{"", "", "", ""},
						{"", "Comentário 1", "Comentário 2", "Comentário 3"},
					},
					rowNoteAuthors: [][]string{
						{"", "", "", ""},
						{"", "Prof. 1", "Prof. 2", "Prof. 3"},
					},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab2", SessionUser{Matricula: "123", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	card := result.Tables[0].Cards[0]
	if card.Value != "Não corrigida ainda" || card.DisplayValue != "Não corrigida ainda" {
		t.Fatalf("card score = %q/%q, want pending summary score", card.Value, card.DisplayValue)
	}
	if card.Comment != "Comentário geral do projeto" || card.CommentAuthor != "Prof. Geral" {
		t.Fatalf("card comment = %q/%q", card.Comment, card.CommentAuthor)
	}
	if len(card.Details) != 3 {
		t.Fatalf("details len = %d, want 3", len(card.Details))
	}
	if card.Details[1].Comment != "Comentário 2" || card.Details[1].CommentAuthor != "Prof. 2" {
		t.Fatalf("detail comment = %q/%q", card.Details[1].Comment, card.Details[1].CommentAuthor)
	}
}

func TestV2ActivityTableFallsBackToSummaryFinalGradeColumn(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "v2"},
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
				grid: &sheetGrid{
					headers: []string{"Atividade", "AB", "Peso", "Aba"},
					rows:    [][]string{{"Projeto", "AB2", "9", "Projeto"}},
				},
			},
			"nota ab2": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "nota final"},
					rows:    [][]string{{"2025026109", "4,5"}},
					rowNotes: [][]string{
						{"", "Comentário na nota final"},
					},
					rowNoteAuthors: [][]string{
						{"", "Prof. Final"},
					},
				},
			},
			"Projeto": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"grupo", "criterio 1", "criterio 2", "nota final"},
					rows: [][]string{
						{"maximo possivel", "1", "1", "10"},
						{"2025026109", "0", "1", "5"},
					},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab2", SessionUser{Matricula: "2025026109", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	card := result.Tables[0].Cards[0]
	if card.Value != "4,5" || card.DisplayValue != "4,5/9" {
		t.Fatalf("card score = %q/%q, want summary final grade 4,5/9", card.Value, card.DisplayValue)
	}
	if card.Tone != "score-warning" {
		t.Fatalf("card tone = %q, want warning for 50%% score", card.Tone)
	}
	if card.Comment != "Comentário na nota final" || card.CommentAuthor != "Prof. Final" {
		t.Fatalf("card comment = %q/%q", card.Comment, card.CommentAuthor)
	}
	if len(card.Details) != 2 {
		t.Fatalf("details len = %d, want nota final excluded from criteria: %#v", len(card.Details), card.Details)
	}
	if card.Details[0].DisplayScore != "0 / 1" || card.Details[1].DisplayScore != "1 / 1" {
		t.Fatalf("details = %#v, want rubric maxima /1", card.Details)
	}
}
