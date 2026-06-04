package app

import (
	"testing"

	"google.golang.org/api/sheets/v4"
)

func TestActivityIdentityColumnCommentBecomesCardComment(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Total", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", "")),
		rowData(cellData("Alice", "comentario geral da linha"), cellData("0,7", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Comment; got != "comentario geral da linha" {
		t.Fatalf("card comment = %q, want identity column comment", got)
	}
}

func TestActivityDriveCommentOnIdentityCellBecomesCardComment(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", "")),
		rowData(cellData("Alice", ""), cellData("0,7", "")),
	}, nil)
	grid.applyDriveComments([]driveCellComment{
		{Text: "comentario na celula do nome", Author: "Professor", QuotedText: "Alice", SheetID: 123, HasSheetID: true},
	}, 123, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Comment; got != "comentario na celula do nome" {
		t.Fatalf("card comment = %q, want Drive identity comment", got)
	}
}

func TestActivityDetailsNormalizeToActivityWeight(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("7", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at1",
		Label:        "AT. 1",
		SheetName:    "AT. 1",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Value; got != "0,7" {
		t.Fatalf("card value = %q, want 0,7", got)
	}
	if got := table.Cards[0].DisplayValue; got != "0,70 de 1,00" {
		t.Fatalf("card display value = %q, want 0,70 de 1,00", got)
	}
	detail := table.Cards[0].Details[0]
	if detail.Value != "0,7" || detail.Max != 1 || detail.DisplayScore != "0,70 de 1,00" || detail.Ratio != 70 {
		t.Fatalf("unexpected scaled detail: %#v", detail)
	}
}

func TestActivityCapsNormalizedScoresAtActivityWeight(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("12", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at1",
		Label:        "AT. 1",
		SheetName:    "AT. 1",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Value; got != "1" {
		t.Fatalf("card value = %q, want capped 1", got)
	}
	detail := table.Cards[0].Details[0]
	if detail.Value != "1" || detail.Max != 1 || detail.Ratio != 100 {
		t.Fatalf("unexpected capped detail: %#v", detail)
	}
}

func TestActivityQuestionRubricUsesOfficialCriterionWeights(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Questão 1", ""), cellData("Questão 2", ""), cellData("Questão 3", ""), cellData("Questão 4", ""), cellData("Questão 5", ""), cellData("Questão 6", ""), cellData("Adequação", ""), cellData("Organização", "")),
		rowData(cellData("Nota maxima", ""), cellData("1,5", ""), cellData("10", ""), cellData("1,5", ""), cellData("2", ""), cellData("1,5", ""), cellData("2", ""), cellData("2", ""), cellData("5", "")),
		rowData(cellData("Alice", ""), cellData("1", ""), cellData("7,5", ""), cellData("1,5", ""), cellData("1", ""), cellData("1,1", ""), cellData("1,3", ""), cellData("1", ""), cellData("3", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at3",
		Label:        "AT. 3",
		SheetName:    "AT. 3",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}

	details := table.Cards[0].Details
	wantScores := []string{"0,07 de 0,10", "0,11 de 0,15", "0,15 de 0,15", "0,10 de 0,20", "0,07 de 0,10", "0,10 de 0,15", "0,05 de 0,10", "0,03 de 0,05"}
	for idx, want := range wantScores {
		if details[idx].DisplayScore != want {
			t.Fatalf("details[%d].DisplayScore = %q, want %q: %#v", idx, details[idx].DisplayScore, want, details[idx])
		}
	}
}

func TestActivityDetailsKeepSpreadsheetSubtopicLabel(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("organização e qualidade do texto", "")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("7", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at1",
		Label:        "Atividade 1",
		SheetName:    "AT. 1",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Details[0].Label; got != "organização e qualidade do texto" {
		t.Fatalf("detail label = %q, want spreadsheet label", got)
	}
}

func TestActivityStartsAtFirstSubtopicColumn(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Matrícula", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("123", ""), cellData("7", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards[0].Details) != 1 || table.Cards[0].Details[0].Label != "Critério" {
		t.Fatalf("details = %#v, want first subtopic Critério", table.Cards[0].Details)
	}
}

func TestActivityCriterionCommentBecomesDetailComment(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("7", "comentario do criterio")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at3",
		Label:        "AT. 3",
		SheetName:    "AT. 3",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "18113089"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	detail := table.Cards[0].Details[0]
	if detail.Comment != "comentario do criterio" {
		t.Fatalf("detail comment = %q, want criterion comment", detail.Comment)
	}
}

func TestActivitySubtopicCommentBecomesDetailComment(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("Modelagem", "comentario do subtopico")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("7", "")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at3",
		Label:        "AT. 3",
		SheetName:    "AT. 3",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "18113089"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	detail := table.Cards[0].Details[0]
	if detail.Comment != "comentario do subtopico" {
		t.Fatalf("detail comment = %q, want subtopic comment", detail.Comment)
	}
}

func TestActivityTotalCommentBecomesCardComment(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Total", "")),
		rowData(cellData("Nota maxima", ""), cellData("10", "")),
		rowData(cellData("Alice", ""), cellData("8", "comentario total")),
	}, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:          "at3",
		Label:        "AT. 3",
		SheetName:    "AT. 3",
		Kind:         "activity",
		ScoreDivisor: 10,
	}, SessionUser{Name: "Alice", Matricula: "18113089"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if table.Cards[0].Comment != "comentario total" {
		t.Fatalf("card comment = %q, want total comment", table.Cards[0].Comment)
	}
}

func TestHumanizeLabelDoesNotTreatQualidadeAsQuestion(t *testing.T) {
	if got := humanizeLabel("organização e qualidade do texto"); got != "Organização e qualidade do texto" {
		t.Fatalf("humanizeLabel qualidade = %q", got)
	}
	if got := humanizeLabel("q.1"); got != "Q.1" {
		t.Fatalf("humanizeLabel q.1 = %q", got)
	}
}

func TestQuestionLetterHeadersAreRecognized(t *testing.T) {
	if !isQuestionLabel("Questão C") {
		t.Fatal("Questão C should be recognized as a question detail")
	}
	if got := cardLabel("Questão C"); got != "Questão C" {
		t.Fatalf("cardLabel Questão C = %q", got)
	}
	if got := inferMaxForLabel("Questão C"); got != 1.5 {
		t.Fatalf("inferMaxForLabel Questão C = %v, want 1.5", got)
	}
	if got := inferMaxForLabel("Adequação"); got != 1 {
		t.Fatalf("inferMaxForLabel Adequação = %v, want 1", got)
	}
	if got := compareDetailLabels("Questão C", "Questão B"); got <= 0 {
		t.Fatalf("compareDetailLabels Questão C vs Questão B = %d, want positive", got)
	}
}

func TestActivityIdentityColumnCommentCanExcludeStudent(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", "")),
		rowData(cellData("Alice", "NOME NÃO CONSTA NA ATIVIDADE"), cellData("0,7", "")),
	}, nil)

	_, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("student row with exclusion comment should not return grade data")
	}
}

func TestActivityDoesNotMatchNameOutsideIdentityColumns(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Grupo", ""), cellData("Critério", ""), cellData("Observação", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", ""), cellData("", "")),
		rowData(cellData("Bob", ""), cellData("0,7", ""), cellData("Alice", "")),
	}, nil)

	_, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("name outside identity columns should not select a grade row")
	}
}

func TestProjectPayloadKeepsAllSubtopicsInDropdown(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Total", "CRUD", "Referências", "Discussão em aula", "Funcionalidades gerais", "Apresentação do projeto"},
		notes:   []string{"", "", "comentario total", "comentario crud"},
		rows: [][]string{
			{"Alice", "123", "8", "1", "0,5", "0,5", "0,8", "1"},
		},
		rowNotes: [][]string{
			{"", "", "", "", "", "", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"", "", "", "", "", "", "", ""},
		},
	}

	table, found, err := parseStudentTable(grid, TableConfig{
		Key:       "projeto",
		Label:     "Projeto AB2",
		SheetName: "Projeto AB2",
		Kind:      "project",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 {
		t.Fatalf("cards len = %d, want 1: %#v", len(table.Cards), table.Cards)
	}
	if table.Cards[0].Label != "Total" {
		t.Fatalf("card label = %q, want Total", table.Cards[0].Label)
	}
	details := table.Cards[0].Details
	if len(details) != 5 {
		t.Fatalf("details len = %d, want 5: %#v", len(details), details)
	}
	wantLabels := map[string]bool{
		"CRUD":                   false,
		"Referências":            false,
		"Discussão em aula":      false,
		"Funcionalidades gerais": false,
		"Apresentação":           false,
	}
	for _, detail := range details {
		if _, ok := wantLabels[detail.Label]; ok {
			wantLabels[detail.Label] = true
		}
	}
	for label, found := range wantLabels {
		if !found {
			t.Fatalf("missing project detail %q in %#v", label, details)
		}
	}
	for _, detail := range details {
		if detail.Label == "CRUD" && detail.Comment != "comentario crud" {
			t.Fatalf("project detail comment = %q, want comentario crud", detail.Comment)
		}
	}
}

func TestSummaryPayloadKeepsCommonGradeColumns(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Prova", "Média"},
		rows: [][]string{
			{"Alice", "123", "8", "Em correção"},
		},
		rowNotes: [][]string{
			{"", "", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"", "", "", ""},
		},
	}

	table, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 {
		t.Fatalf("cards len = %d, want 1: %#v", len(table.Cards), table.Cards)
	}
	if table.Cards[0].Label != "Prova AB" || table.Cards[0].Value != "8" {
		t.Fatalf("unexpected summary card: %#v", table.Cards[0])
	}
}

func TestSummaryPayloadHidesActivityColumns(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Pesquisa", "Artigo", "Lista", "AT. Total Atividades", "Prova", "Média"},
		rows: [][]string{
			{"Alice", "123", "0,98", "0,85", "0,65", "2,48", "7", "8"},
		},
		rowNotes: [][]string{
			{"", "", "", "", "", "", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"", "", "", "", "", "", "", ""},
		},
	}

	table, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 2 {
		t.Fatalf("cards len = %d, want 2: %#v", len(table.Cards), table.Cards)
	}
	if table.Cards[0].Label != "Prova AB" || table.Cards[1].Label != "Média AB" {
		t.Fatalf("unexpected summary cards: %#v", table.Cards)
	}
}

func TestSummaryPayloadHidesGeneralFormulaComment(t *testing.T) {
	grid := &sheetGrid{
		headers:     []string{"Nome", "Matricula", "Prova"},
		notes:       []string{"", "", "o montante maximo das atividades são de até 2 pontos, e a prova vale 8"},
		noteAuthors: []string{"", "", "Professor"},
		rows: [][]string{
			{"Alice", "123", "6"},
		},
		rowNotes: [][]string{
			{"", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"", "", ""},
		},
	}

	table, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 {
		t.Fatalf("cards len = %d, want 1: %#v", len(table.Cards), table.Cards)
	}
	if got := table.Cards[0].Comment; got != "" {
		t.Fatalf("summary comment = %q, want empty", got)
	}
}

func TestSummaryIdentityColumnCommentBecomesCardComment(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Prova"},
		rows: [][]string{
			{"Alice", "123", "6"},
		},
		rowNotes: [][]string{
			{"comentario geral", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"Professor", "", ""},
		},
	}

	table, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if got := table.Cards[0].Comment; got != "comentario geral" {
		t.Fatalf("summary card comment = %q, want identity column comment", got)
	}
	if got := table.Cards[0].CommentAuthor; got != "Professor" {
		t.Fatalf("summary card author = %q, want Professor", got)
	}
}

func TestSummaryIdentityColumnCommentCanExcludeStudent(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Prova"},
		rows: [][]string{
			{"Alice", "123", "6"},
		},
		rowNotes: [][]string{
			{"NOME NÃO CONSTA NA ATIVIDADE", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"Professor", "", ""},
		},
	}

	_, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("summary row with exclusion comment should not return grade data")
	}
}

func TestActivityWorkbookCommentDoesNotLeakBetweenStudents(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", "")),
		rowData(cellData("Alice", ""), cellData("0,8", "")),
		rowData(cellData("Bob", ""), cellData("0,7", "")),
	}, nil)
	grid.applyWorkbookComments([]workbookCellComment{
		{SheetName: "AT. 1", RowIndex: 3, ColumnIndex: 1, Text: "comentario privado do Bob", Author: "Professor"},
	}, nil)

	aliceTable, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Alice row was not found")
	}
	if got := aliceTable.Cards[0].Details[0].Comment; got != "" {
		t.Fatalf("Alice detail comment = %q, want empty", got)
	}

	bobTable, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at1",
		Label:     "AT. 1",
		SheetName: "AT. 1",
		Kind:      "activity",
	}, SessionUser{Name: "Bob", Matricula: "456"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Bob row was not found")
	}
	if got := bobTable.Cards[0].Details[0].Comment; got != "comentario privado do Bob" {
		t.Fatalf("Bob detail comment = %q, want private workbook comment", got)
	}
}

func TestSummaryWorkbookCommentDoesNotLeakBetweenStudents(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Matricula", ""), cellData("Prova", "")),
		rowData(cellData("Alice", ""), cellData("123", ""), cellData("8", "")),
		rowData(cellData("Bob", ""), cellData("456", ""), cellData("7", "")),
	}, nil)
	grid.applyWorkbookComments([]workbookCellComment{
		{SheetName: "Notas AB1", RowIndex: 2, ColumnIndex: 2, Text: "comentario privado do Bob", Author: "Professor"},
	}, nil)

	aliceTable, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Alice", Matricula: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Alice row was not found")
	}
	if got := aliceTable.Cards[0].Comment; got != "" {
		t.Fatalf("Alice card comment = %q, want empty", got)
	}

	bobTable, found, err := parseStudentTable(grid, TableConfig{
		Key:       "prova",
		Label:     "Prova AB1",
		SheetName: "Notas AB1",
		Kind:      "summary",
	}, SessionUser{Name: "Bob", Matricula: "456"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Bob row was not found")
	}
	if got := bobTable.Cards[0].Comment; got != "comentario privado do Bob" {
		t.Fatalf("Bob card comment = %q, want private workbook comment", got)
	}
}

func activityGrid(studentNote string, detailNote string, headerNote string) *sheetGrid {
	return &sheetGrid{
		headers:     []string{"Aluno", "Critério"},
		notes:       []string{"", headerNote},
		noteAuthors: []string{"", ""},
		rows: [][]string{
			{"Subtópico", "Modelagem"},
			{"Nota maxima", "2"},
			{"Alice", "1,5"},
		},
		rowNotes: [][]string{
			{"", detailNote},
			{"", ""},
			{"", studentNote},
		},
		rowNoteAuthors: [][]string{
			{"", ""},
			{"", ""},
			{"", ""},
		},
	}
}
