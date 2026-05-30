package app

import "testing"

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
		headers: []string{"Nome", "Matricula", "Nota", "Média"},
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
	if table.Cards[0].Label != "Nota" || table.Cards[0].Value != "8" {
		t.Fatalf("unexpected summary card: %#v", table.Cards[0])
	}
}

func TestSummaryPayloadKeepsUnexpectedGradeHeaders(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Nome", "Matricula", "Banco de Dados"},
		rows: [][]string{
			{"Alice", "123", "9,5"},
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
	if table.Cards[0].Label != "Banco de Dados" || table.Cards[0].Value != "9,5" {
		t.Fatalf("unexpected fallback summary card: %#v", table.Cards[0])
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
