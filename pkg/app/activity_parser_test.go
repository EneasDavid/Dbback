package app

import (
	"strings"
	"testing"

	"google.golang.org/api/sheets/v4"
)

func TestActivityCommentPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		studentNote string
		detailNote  string
		headerNote  string
		want        string
	}{
		{name: "student note wins", studentNote: "aluno", detailNote: "subtopico", headerNote: "cabecalho", want: "aluno"},
		{name: "detail note wins without student note", detailNote: "subtopico", headerNote: "cabecalho", want: "subtopico"},
		{name: "header note is fallback", headerNote: "cabecalho", want: "cabecalho"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, found, err := parseActivityRubric(activityGrid(tt.studentNote, tt.detailNote, tt.headerNote), TableConfig{
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
			if len(table.Cards) != 1 || len(table.Cards[0].Details) != 1 {
				t.Fatalf("unexpected cards/details: %#v", table.Cards)
			}
			if got := table.Cards[0].Details[0].Comment; got != tt.want {
				t.Fatalf("comment = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActivitySubtopicCommentsComeFromSheetsNotes(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("Modelagem", "comentario do subtopico")),
		rowData(cellData("Nota maxima", ""), cellData("2", "")),
		rowData(cellData("Alice", ""), cellData("1,5", "")),
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
	if len(table.Cards) != 1 || len(table.Cards[0].Details) != 1 {
		t.Fatalf("unexpected cards/details: %#v", table.Cards)
	}
	if got := table.Cards[0].Details[0].Comment; got != "comentario do subtopico" {
		t.Fatalf("detail comment = %q, want Sheets note", got)
	}
}

func TestActivitySubtopicCommentsComeFromDriveMergedCells(t *testing.T) {
	merges := []*sheets.GridRange{
		{StartRowIndex: 1, EndRowIndex: 2, StartColumnIndex: 1, EndColumnIndex: 3},
	}
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("Organização", ""), cellData("", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", ""), cellData("1", "")),
		rowData(cellData("Alice", ""), cellData("0,8", ""), cellData("0,7", "")),
	}, merges)
	grid.applyDriveComments([]driveCellComment{
		{Text: "comentario no criterio mesclado", Author: "Professor", QuotedText: "Organização", SheetID: 7, HasSheetID: true},
	}, 7, merges)
	grid.applyCommentMerges(merges)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at3",
		Label:     "AT. 3",
		SheetName: "AT. 3",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "2024001339"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 || len(table.Cards[0].Details) != 2 {
		t.Fatalf("unexpected cards/details: %#v", table.Cards)
	}
	for _, detail := range table.Cards[0].Details {
		if got := detail.Comment; got != "comentario no criterio mesclado" {
			t.Fatalf("detail %q comment = %q, want merged Drive comment", detail.Label, got)
		}
	}
}

func TestActivityCommentsPropagateThroughThreeRowMergedGroup(t *testing.T) {
	merges := []*sheets.GridRange{
		{StartRowIndex: 2, EndRowIndex: 5, StartColumnIndex: 1, EndColumnIndex: 2},
	}
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "")),
		rowData(cellData("Nota maxima", ""), cellData("1", "")),
		rowData(cellData("Colega 1", ""), cellData("0,7", "comentario do grupo")),
		rowData(cellData("Colega 2", ""), cellData("", "")),
		rowData(cellData("Alice", ""), cellData("", "")),
	}, merges)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at3",
		Label:     "AT. 3",
		SheetName: "AT. 3",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "2024001339"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 || len(table.Cards[0].Details) != 1 {
		t.Fatalf("unexpected cards/details: %#v", table.Cards)
	}
	if got := table.Cards[0].Details[0].Comment; got != "comentario do grupo" {
		t.Fatalf("detail comment = %q, want merged group comment", got)
	}
}

func TestActivityCommentsUseDriveRowSequenceWhenScoresRepeat(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Organização", ""), cellData("Q.1", ""), cellData("Q.2", ""), cellData("Q.3", ""), cellData("Q.4", ""), cellData("Q.5", ""), cellData("Q.6", "")),
		rowData(cellData("Nota maxima", ""), cellData("0,5", ""), cellData("1,5", ""), cellData("1", ""), cellData("1,5", ""), cellData("2", ""), cellData("1,5", ""), cellData("2", "")),
		rowData(cellData("Bob", ""), cellData("0,3", ""), cellData("1", ""), cellData("0,75", ""), cellData("0,85", ""), cellData("1,2", ""), cellData("1,1", ""), cellData("1,3", "")),
		rowData(cellData("Alice", ""), cellData("0,3", ""), cellData("1", ""), cellData("0,75", ""), cellData("0,85", ""), cellData("1,2", ""), cellData("1,1", ""), cellData("1,3", "")),
	}, nil)
	grid.applyDriveComments([]driveCellComment{
		{Text: "comentario de outro aluno", Author: "Professor", QuotedText: "0,3", SheetID: 0, HasSheetID: true},
		{Text: "comentario q6", Author: "Professor", QuotedText: "1,3", SheetID: 0, HasSheetID: true},
		{Text: "comentario q5", Author: "Professor", QuotedText: "1,1", SheetID: 0, HasSheetID: true},
		{Text: "comentario q4", Author: "Professor", QuotedText: "1,2", SheetID: 0, HasSheetID: true},
		{Text: "comentario q3", Author: "Professor", QuotedText: "0,85", SheetID: 0, HasSheetID: true},
		{Text: "comentario q2", Author: "Professor", QuotedText: "0,75", SheetID: 0, HasSheetID: true},
		{Text: "comentario q1", Author: "Professor", QuotedText: "1", SheetID: 0, HasSheetID: true},
		{Text: "comentario organizacao", Author: "Professor", QuotedText: "0,3", SheetID: 0, HasSheetID: true},
	}, 123, nil)

	table, found, err := parseActivityRubric(grid, TableConfig{
		Key:       "at3",
		Label:     "AT. 3",
		SheetName: "AT. 3",
		Kind:      "activity",
	}, SessionUser{Name: "Alice", Matricula: "2024001339"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("student row was not found")
	}
	if len(table.Cards) != 1 || len(table.Cards[0].Details) != 7 {
		t.Fatalf("unexpected cards/details: %#v", table.Cards)
	}
	for _, detail := range table.Cards[0].Details {
		if strings.TrimSpace(detail.Comment) == "" {
			t.Fatalf("detail %q has no comment: %#v", detail.Label, table.Cards[0].Details)
		}
	}
	want := map[string]string{
		"Organização": "comentario organizacao",
		"Q.1":         "comentario q1",
		"Q.2":         "comentario q2",
		"Q.3":         "comentario q3",
		"Q.4":         "comentario q4",
		"Q.5":         "comentario q5",
		"Q.6":         "comentario q6",
	}
	for _, detail := range table.Cards[0].Details {
		if got := detail.Comment; got != want[detail.Label] {
			t.Fatalf("detail %q comment = %q, want %q", detail.Label, got, want[detail.Label])
		}
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
}

func TestSummaryPayloadKeepsCommonGradeColumns(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Prova", "Média"},
		rows: [][]string{
			{"Alice", "123", "8", "Não corrigida ainda"},
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
