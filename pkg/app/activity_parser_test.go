package app

import (
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

func TestProjectPayloadHidesIdentityColumnsAndKeepsDetails(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Total", "CRUD"},
		notes:   []string{"", "", "comentario total", "comentario crud"},
		rows: [][]string{
			{"Alice", "123", "8", "1"},
		},
		rowNotes: [][]string{
			{"", "", "", ""},
		},
		rowNoteAuthors: [][]string{
			{"", "", "", ""},
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
	if len(table.Cards[0].Details) != 1 || table.Cards[0].Details[0].Label != "CRUD" {
		t.Fatalf("details should only include CRUD, got %#v", table.Cards[0].Details)
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
