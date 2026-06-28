package app

import (
	"testing"
	"time"
)

func TestV2ABStateUsesActiveColumn(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Nome", "Ativa"},
		rows: [][]string{
			{"AB1", "AB1", "0"},
			{"AB2", "AB2", "1"},
		},
	}

	label, active := v2ABState(grid, "ab2")

	if label != "AB2" || !active {
		t.Fatalf("v2ABState() = %q/%v, want AB2/true", label, active)
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

func TestGradeForV2MarksInactiveAB(t *testing.T) {
	client := &SheetsClient{
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "Status"},
					rows:    [][]string{{"AB1", "0"}},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab1", SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	if result.Active == nil || *result.Active {
		t.Fatalf("gradeForV2().Active = %#v, want false", result.Active)
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

func TestV2ABStateTreatsTextualProgressStatusAsInactive(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Status"},
		rows: [][]string{
			{"AB1", "Em correção"},
			{"AB2", "Não encerrado"},
		},
	}

	_, ab1Active := v2ABState(grid, "ab1")
	_, ab2Active := v2ABState(grid, "ab2")

	if ab1Active || ab2Active {
		t.Fatalf("v2ABState() active states = %v/%v, want both inactive for non-1 statuses", ab1Active, ab2Active)
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

func TestV2ActivitiesForABPreservesMissingWeight(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"atividade", "peso", "ab"},
		rows: [][]string{
			{"sem peso", "", "Ab. 1"},
			{"peso zero", "0", "Ab. 1"},
			{"com peso", "2", "Ab. 1"},
		},
	}

	activities := v2ActivitiesForAB(grid, "ab1")

	if len(activities) != 3 {
		t.Fatalf("activities len = %d, want 3: %#v", len(activities), activities)
	}
	if activities[0].HasWeight || activities[0].Weight != 0 || activities[1].HasWeight || activities[1].Weight != 0 {
		t.Fatalf("missing/zero weights should stay scoreless: %#v", activities)
	}
	if !activities[2].HasWeight || activities[2].Weight != 2 {
		t.Fatalf("weighted activity = %#v, want weight 2", activities[2])
	}
}

func TestV2ActivitiesForABOnlyKeepsStatusOneWhenStatusColumnExists(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Atividade", "AB", "Status"},
		rows: [][]string{
			{"Atividade 1", "AB1", "0"},
			{"Atividade 2", "AB1", "Em correção"},
			{"Atividade 3", "AB1", "1"},
		},
	}

	activities := v2ActivitiesForAB(grid, "ab1")

	if len(activities) != 1 || activities[0].Label != "Atividade 3" {
		t.Fatalf("activities = %#v, want only status 1 activity", activities)
	}
}

func TestV2ActivityItemsNormalizesCriteriaToActivityWeightAndKeepsComments(t *testing.T) {
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
		t.Fatalf("first rubric item = %#v, want normalized max 0,8 and value 0,4", items[0])
	}
	if items[1].NotaMaxima != "1,2" || items[1].NotaAlcancada != "1,2" {
		t.Fatalf("second rubric item = %#v, want normalized max/value 1,2", items[1])
	}
	if items[1].Comment != "comentário B" || items[1].CommentAuthor != "Prof" {
		t.Fatalf("second item comment = %#v", items[1])
	}
}

func TestV2ActivityItemsUsesMaximoPossivelRowForQuestionWeights(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"grupo", "Questão 1", "Questão 2", "Questão 3", "Questão 4", "Questão 5", "Questão 6", "Adequação", "Organização", "nota final"},
		rows: [][]string{
			{"maximo possivel", "1", "1,5", "1,5", "2", "1", "1,5", "1", "0,5", "10"},
			{"Grupo A", "1", "1", "1,5", "1", "0,5", "1", "1", "0,5", "8"},
		},
	}

	maxRowIdx := findMaxRow(grid.rows)
	items := v2ActivityItems(grid, maxRowIdx, 1, 0.33)

	if maxRowIdx != 0 {
		t.Fatalf("findMaxRow() = %d, want maximo possivel row", maxRowIdx)
	}
	if len(items) != 8 {
		t.Fatalf("items len = %d, want 8 criteria without group/final grade: %#v", len(items), items)
	}
	wantMaxima := []string{"0,03", "0,05", "0,05", "0,07", "0,03", "0,05", "0,03", "0,02"}
	for idx, want := range wantMaxima {
		if items[idx].NotaMaxima != want {
			t.Fatalf("items[%d].NotaMaxima = %q, want %q: %#v", idx, items[idx].NotaMaxima, want, items[idx])
		}
	}
}

func TestV2ActivityItemsUsesSubtopicRowForOfficialWeights(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"grupo", "Critério 1", "Critério 2", "Critério 3", "Critério 4", "Critério 5", "Critério 6", "Critério 7", "Critério 8"},
		rows: [][]string{
			{"subtópico", "Questão 1", "Questão 2", "Questão 3", "Questão 4", "Questão 5", "Questão 6", "Adequação", "Organização"},
			{"maximo possivel", "10", "10", "10", "10", "10", "10", "10", "10"},
			{"Grupo A", "10", "10", "10", "10", "10", "10", "10", "10"},
		},
	}

	items := v2ActivityItems(grid, 1, 2, 0.33)

	wantMaxima := []string{"0,03", "0,05", "0,05", "0,07", "0,03", "0,05", "0,03", "0,02"}
	for idx, want := range wantMaxima {
		if items[idx].NotaMaxima != want {
			t.Fatalf("items[%d].NotaMaxima = %q, want %q: %#v", idx, items[idx].NotaMaxima, want, items[idx])
		}
	}
}

func TestV2ActivityItemsPreservesRatioWhenCorrectingQuestionWeights(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"grupo", "Questão 1", "Questão 2", "Questão 3", "Questão 4", "Adequação", "Organização"},
		rows: [][]string{
			{"maximo possivel", "1,5", "10", "1,5", "2", "2", "5"},
			{"Grupo A", "1", "7,5", "1,5", "1", "1", "3"},
		},
	}

	items := v2ActivityItems(grid, 0, 1, 1)

	wantMaxima := []string{"0,1", "0,15", "0,15", "0,2", "0,1", "0,05"}
	wantValues := []string{"0,07", "0,11", "0,15", "0,1", "0,05", "0,03"}
	for idx := range wantMaxima {
		if items[idx].NotaMaxima != wantMaxima[idx] || items[idx].NotaAlcancada != wantValues[idx] {
			t.Fatalf("items[%d] = %#v, want value/max %s/%s", idx, items[idx], wantValues[idx], wantMaxima[idx])
		}
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
	if items[0].NotaMaxima != "0,33" || items[0].NotaAlcancada != "0,33" {
		t.Fatalf("first capped item = %#v, want normalized max/value 0,33", items[0])
	}
	if items[1].NotaMaxima != "0,67" || items[1].NotaAlcancada != "0,67" {
		t.Fatalf("second capped item = %#v, want normalized max/value 0,67", items[1])
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
	if items[0].Subtopic != "Originalidade" || items[0].NotaMaxima != "0,5" {
		t.Fatalf("unexpected generic criterion: %#v", items[0])
	}
}

func TestV2ActivitiesCompleteRequiresEveryActivityToEnd(t *testing.T) {
	activities := []v2ActivityConfig{
		{Key: "at1", HasWeight: true, SummaryCol: 1},
		{Key: "at2", HasWeight: true, SummaryCol: 2},
	}
	tables := []TableResult{
		{Key: "at1", Kind: "activity", Complete: true, Status: "Encerrado"},
		{Key: "at2", Kind: "activity", Complete: false, Status: "Não encerrado"},
	}

	if v2ActivitiesComplete(activities, tables) {
		t.Fatal("v2ActivitiesComplete() = true with a pending activity")
	}

	tables[1].Complete = true
	tables[1].Status = "Encerrado"
	if !v2ActivitiesComplete(activities, tables) {
		t.Fatal("v2ActivitiesComplete() = false after every activity ended")
	}
}

func TestV2ActivitiesCompleteIgnoresScorelessActivities(t *testing.T) {
	activities := []v2ActivityConfig{
		{Key: "weighted", HasWeight: true},
		{Key: "scoreless", HasWeight: false},
	}
	tables := []TableResult{
		{Key: "weighted", Kind: "activity", Complete: true, Status: "Encerrado"},
		{Key: "scoreless", Kind: "activity", Complete: false, Scoreless: true, Status: "Não pontua"},
	}

	if !v2ActivitiesComplete(activities, tables) {
		t.Fatal("v2ActivitiesComplete() = false, scoreless activity should not block the average")
	}
}

func TestGradeForV2RendersScorelessActivityAsPercentagesWithoutGrade(t *testing.T) {
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
					headers: []string{"Atividade", "AB", "Peso", "Aba", "Status"},
					rows:    [][]string{{"Pré entrega", "AB2", "", "Pré entrega", "1"}},
				},
			},
			"nota ab2": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula"},
					rows:    [][]string{{"123"}},
				},
			},
			"Pré entrega": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critério A", "Critério B", "Critério C"},
					rows: [][]string{
						{"Nota máxima", "2", "4", "1"},
						{"123", "1", "4", "0"},
					},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab2", SessionUser{Matricula: "123", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	if len(result.Tables) != 1 {
		t.Fatalf("tables = %#v, want scoreless activity only", result.Tables)
	}
	table := result.Tables[0]
	if !table.Scoreless || table.Complete || table.Status != "Não pontua" {
		t.Fatalf("table = %#v, want always scoreless/Não pontua", table)
	}
	if len(table.Cards) != 1 || table.Cards[0].Label != "Critérios avaliados" || table.Cards[0].Value != "" || table.Cards[0].DisplayValue != "" {
		t.Fatalf("cards = %#v, want criteria transport without grade", table.Cards)
	}
	details := table.Cards[0].Details
	if len(details) != 3 {
		t.Fatalf("details = %#v, want 3 criteria", details)
	}
	want := []string{"50%", "100%", "0%"}
	for idx := range want {
		if !details[idx].Percentage || details[idx].DisplayScore != want[idx] || details[idx].Ratio < 0 || details[idx].Ratio > 100 {
			t.Fatalf("details[%d] = %#v, want percentage %s", idx, details[idx], want[idx])
		}
	}
}

func TestNotLaunchedActivityValueIsPending(t *testing.T) {
	value := "Essa atividade não foi lançada"

	if !isPendingValue(value) {
		t.Fatalf("isPendingValue(%q) = false, want true", value)
	}
	if got := v2ActivityStatusFromScore(value); got != "Não encerrado" {
		t.Fatalf("v2ActivityStatusFromScore(%q) = %q, want Não encerrado", value, got)
	}
}

func TestGradeForV2IncludesEveryActiveRegisteredActivity(t *testing.T) {
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
					headers: []string{"Atividade", "AB", "Peso", "Aba", "Status"},
					rows: [][]string{
						{"Atividade 1", "AB2", "1", "AT. 1", "1"},
						{"Pré entrega", "AB2", "1", "Pré entrega", "1"},
						{"Removida", "AB2", "1", "Removida", "0"},
					},
				},
			},
			"nota ab2": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Atividade 1"},
					rows:    [][]string{{"123", "1"}},
				},
			},
			"AT. 1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critério"},
					rows:    [][]string{{"Nota máxima", "1"}, {"123", "1"}},
				},
			},
			"Pré entrega": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Matrícula", "Critério"},
					rows:    [][]string{{"Nota máxima", "1"}, {"123", ""}},
				},
			},
		},
	}

	result, err := client.gradeForV2(t.Context(), "ab2", SessionUser{Matricula: "123", Name: "Student"})
	if err != nil {
		t.Fatalf("gradeForV2() error = %v", err)
	}
	if len(result.Tables) != 2 {
		t.Fatalf("gradeForV2() tables = %#v, want every active registered activity", result.Tables)
	}
	if result.Tables[1].Label != "Pré entrega" || result.Tables[1].Status != "Não encerrado" {
		t.Fatalf("second table = %#v, want pending Pré entrega", result.Tables[1])
	}
}

func TestV2ActivityStatusWaitsForTopScore(t *testing.T) {
	items := []activityItem{{NotaAlcancada: "1"}}

	if got := v2ActivityStatus(items, "Em correção"); got != "Não encerrado" {
		t.Fatalf("v2ActivityStatus() = %q, want Não encerrado", got)
	}
}

func TestV2BindSummaryColumnsDoesNotUseFinalGradeForMultipleActivities(t *testing.T) {
	activities := []v2ActivityConfig{
		{Label: "Atividade 1", SheetName: "AT. 1", HasWeight: true},
		{Label: "Atividade 2", SheetName: "AT. 2", HasWeight: true},
	}

	v2BindSummaryColumns([]string{"Matrícula", "nota final"}, activities)

	if activities[0].SummaryCol >= 0 || activities[1].SummaryCol >= 0 {
		t.Fatalf("activities = %#v, want no ambiguous nota final fallback for multiple activities", activities)
	}
}

func TestV2BindSummaryColumnsAllowsFinalGradeForSingleActivity(t *testing.T) {
	activities := []v2ActivityConfig{{Label: "Projeto", SheetName: "Projeto", HasWeight: true}}

	v2BindSummaryColumns([]string{"Matrícula", "nota final"}, activities)

	if activities[0].SummaryCol != 1 {
		t.Fatalf("activity = %#v, want nota final fallback for single activity", activities[0])
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

func TestRuntimeForUserPreservesLegacySessionWithAutoConfig(t *testing.T) {
	got := runtimeForUser(Config{RuntimeVersion: "auto"}, SessionUser{SchemaStatus: "legacy"})

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

func TestConfiguredRuntimeKeepsExplicitLegacyUserOnLegacyTables(t *testing.T) {
	client := autoClientWithEmptyV2AndLegacyGrade()

	result, err := client.gradeForConfiguredRuntime(t.Context(), "ab1", SessionUser{Matricula: "123", Name: "Alice", SchemaStatus: "legacy"})
	if err != nil {
		t.Fatalf("gradeForConfiguredRuntime() error = %v", err)
	}
	if len(result.Tables) == 0 || result.Tables[0].Key != "legacy-ab1" {
		t.Fatalf("gradeForConfiguredRuntime() = %#v, want legacy table", result)
	}
}

func TestConfiguredRuntimeFallsBackToLegacyWhenAutoV2IsEmpty(t *testing.T) {
	client := autoClientWithEmptyV2AndLegacyGrade()

	result, err := client.gradeForConfiguredRuntime(t.Context(), "ab1", SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradeForConfiguredRuntime() error = %v", err)
	}
	if len(result.Tables) == 0 || result.Tables[0].Key != "legacy-ab1" {
		t.Fatalf("gradeForConfiguredRuntime() = %#v, want legacy table", result)
	}
}

func TestConfiguredRuntimeAllFallsBackToLegacyWhenAutoV2IsEmpty(t *testing.T) {
	client := autoClientWithEmptyV2AndLegacyGrade()

	results, err := client.gradesForConfiguredRuntime(t.Context(), []string{"ab1", "ab2"}, SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradesForConfiguredRuntime() error = %v", err)
	}
	if len(results["ab1"].Tables) == 0 || results["ab1"].Tables[0].Key != "legacy-ab1" {
		t.Fatalf("gradesForConfiguredRuntime() = %#v, want legacy ab1 table", results)
	}
}

func autoClientWithEmptyV2AndLegacyGrade() *SheetsClient {
	return &SheetsClient{
		cfg: Config{
			RuntimeVersion: "auto",
			AB1Tables:      []TableConfig{{Key: "legacy-ab1", Label: "Legacy AB1", SheetName: "Legacy AB1", Kind: "activity"}},
		},
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
				grid:    &sheetGrid{headers: []string{"atividade", "AB"}, rows: [][]string{{"Pesquisa", "AB1"}}},
			},
			"nota ab1": {
				expires: time.Now().Add(time.Hour),
				grid:    &sheetGrid{headers: []string{"Matrícula", "Pesquisa"}, rows: [][]string{}},
			},
			"Legacy AB1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Grupo", "Critério"},
					rows:    [][]string{{"Nota máxima", "1"}, {"Alice", "1"}},
				},
			},
		},
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

func TestGradesForRuntimeV2ReturnsNoExamsWhenNoABStatusIsOne(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{RuntimeVersion: "auto"},
		cache: map[string]cachedGrid{
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "status"},
					rows:    [][]string{{"AB1", "0"}, {"AB2", "0"}, {"reav", "0"}, {"final", "0"}},
				},
			},
		},
	}

	results, err := client.gradesForRuntimeV2(t.Context(), []string{"ab1", "ab2"}, SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradesForRuntimeV2() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("gradesForRuntimeV2() = %#v, want no exams when all abs statuses are 0", results)
	}
}

func TestConfiguredRuntimeV2DoesNotFallbackInactiveABToLegacy(t *testing.T) {
	client := v2InactiveClientWithLegacyGrades()

	result, err := client.gradeForConfiguredRuntime(t.Context(), "ab1", SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradeForConfiguredRuntime() error = %v", err)
	}
	if result.Active == nil || *result.Active || len(result.Tables) != 0 {
		t.Fatalf("gradeForConfiguredRuntime() = %#v, want inactive empty v2 result", result)
	}
}

func TestConfiguredRuntimeV2DoesNotFallbackInactiveABsToLegacy(t *testing.T) {
	client := v2InactiveClientWithLegacyGrades()

	results, err := client.gradesForConfiguredRuntime(t.Context(), []string{"ab1", "ab2"}, SessionUser{Matricula: "123", Name: "Alice"})
	if err != nil {
		t.Fatalf("gradesForConfiguredRuntime() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("gradesForConfiguredRuntime() = %#v, want no inactive v2 exams", results)
	}
}

func TestGradeForStopsAtInactiveV2Spreadsheet(t *testing.T) {
	client := v2InactiveClientWithLegacyGrades()
	user := SessionUser{Matricula: "123", Name: "Alice", SpreadsheetID: "v2-sheet", SchemaStatus: "v2"}

	result, err := client.GradeFor(t.Context(), "ab1", user)
	if err != nil {
		t.Fatalf("GradeFor() error = %v", err)
	}
	if result.Active == nil || *result.Active || len(result.Tables) != 0 {
		t.Fatalf("GradeFor() = %#v, want authoritative inactive v2 result", result)
	}
}

func TestGradesForStopsAtInactiveV2Spreadsheet(t *testing.T) {
	client := v2InactiveClientWithLegacyGrades()
	user := SessionUser{Matricula: "123", Name: "Alice", SpreadsheetID: "v2-sheet", SchemaStatus: "v2"}

	results, err := client.GradesFor(t.Context(), []string{"ab1", "ab2"}, user)
	if err != nil {
		t.Fatalf("GradesFor() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("GradesFor() = %#v, want no inactive v2 exams", results)
	}
}

func v2InactiveClientWithLegacyGrades() *SheetsClient {
	return &SheetsClient{
		cfg: Config{
			SpreadsheetIDs:       []string{"v2-sheet", "legacy-sheet"},
			LegacySpreadsheetIDs: []string{"legacy-sheet"},
			V2SpreadsheetIDs:     []string{"v2-sheet"},
			RuntimeVersion:       "v2",
			LoginSheet:           "Base de dados",
			AB1Tables:            []TableConfig{{Key: "legacy-ab1", Label: "Legacy AB1", SheetName: "Legacy AB1", Kind: "activity"}},
			AB2Tables:            []TableConfig{{Key: "legacy-ab2", Label: "Legacy AB2", SheetName: "Legacy AB2", Kind: "activity"}},
		},
		cache: map[string]cachedGrid{
			"Base de dados": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers:    []string{"Matricula", "Nome"},
					rows:       [][]string{{"123", "Alice"}},
					rowSources: []string{"legacy-sheet"},
					rowSchemas: []string{"legacy"},
				},
			},
			v2ABsSheet: {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"AB", "status"},
					rows:    [][]string{{"AB1", "0"}, {"AB2", "0"}},
				},
			},
			"Legacy AB1": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Grupo", "Critério"},
					rows:    [][]string{{"Nota máxima", "1"}, {"Alice", "1"}},
				},
			},
			"Legacy AB2": {
				expires: time.Now().Add(time.Hour),
				grid: &sheetGrid{
					headers: []string{"Grupo", "Critério"},
					rows:    [][]string{{"Nota máxima", "1"}, {"Alice", "1"}},
				},
			},
		},
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
	if result.Active == nil || !*result.Active {
		t.Fatalf("gradeForV2().Active = %#v, want true", result.Active)
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

func TestV2ActivityTableKeepsCriteriaWithoutHeaderCommentsWhenNoSimilarGroup(t *testing.T) {
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
	if card.Value != "2,5" || card.DisplayValue != "2,50 de 3,00" {
		t.Fatalf("card score = %q/%q, want summary score 2,50 de 3,00", card.Value, card.DisplayValue)
	}
	if len(card.Details) != 2 {
		t.Fatalf("card.Details len = %d, want criteria without matching group: %#v", len(card.Details), card.Details)
	}
	if card.Details[0].Value != "" || !card.Details[0].Pending {
		t.Fatalf("first detail = %#v, want pending blank score", card.Details[0])
	}
	if card.Details[0].Comment != "" || card.Details[0].CommentAuthor != "" {
		t.Fatalf("first detail comment = %q/%q, want empty header comment ignored", card.Details[0].Comment, card.Details[0].CommentAuthor)
	}
	if card.Details[1].Comment != "" || card.Details[1].CommentAuthor != "" {
		t.Fatalf("second detail comment = %q/%q, want empty header comment ignored", card.Details[1].Comment, card.Details[1].CommentAuthor)
	}
	if result.Tables[0].Status != "Não encerrado" || card.Tone != "score-pending" {
		t.Fatalf("pending table/card = %q/%q, want Não encerrado/score-pending", result.Tables[0].Status, card.Tone)
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
	if card.Value != "12" || card.DisplayValue != "12,00 de 1,00" {
		t.Fatalf("card score = %q/%q, want raw summary score 12,00 de 1,00", card.Value, card.DisplayValue)
	}
	if card.Details[0].Value != "1" || card.Details[0].Max != 1 || card.Details[0].DisplayScore != "1,00 de 1,00" {
		t.Fatalf("detail = %#v, want criterion normalized to activity weight", card.Details[0])
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
	if card.Label != "Média" || card.Value != "0,5" {
		t.Fatalf("v2AverageCard() = %#v, want Média 0,5", card)
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
	if card.Value != "10" || card.DisplayValue != "10,00" {
		t.Fatalf("v2AverageCard() = %#v, want capped display 10,00", card)
	}
}

func TestV2AverageCardHidesPendingValue(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Média"},
		rows:    [][]string{{"123", "Em correção"}},
	}

	if card := v2AverageCard(grid, grid.rows[0]); card != nil {
		t.Fatalf("v2AverageCard() = %#v, want nil pending average", card)
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
					rows:    [][]string{{"123", "Em correção"}},
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
	if card.Value != "Em correção" || card.DisplayValue != "Em correção" {
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
	if card.Value != "4,5" || card.DisplayValue != "4,50 de 9,00" {
		t.Fatalf("card score = %q/%q, want summary final grade 4,50 de 9,00", card.Value, card.DisplayValue)
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
	if card.Details[0].DisplayScore != "0,00 de 4,50" || card.Details[1].DisplayScore != "4,50 de 4,50" {
		t.Fatalf("details = %#v, want criteria normalized to project weight", card.Details)
	}
}
