package app

import (
	"testing"

	"github.com/xuri/excelize/v2"
	"google.golang.org/api/sheets/v4"
)

func TestParseXLSXCommentsLoadsCellComments(t *testing.T) {
	file := excelize.NewFile()
	defer file.Close()
	const sheetName = "Notas AB1"
	if err := file.SetSheetName("Sheet1", sheetName); err != nil {
		t.Fatal(err)
	}
	if err := file.AddComment(sheetName, excelize.Comment{
		Cell:   "B4",
		Author: "Professor",
		Text:   "feedback exportado do Google Sheets",
	}); err != nil {
		t.Fatal(err)
	}

	buf, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	comments, err := parseXLSXComments(buf.Bytes(), []string{sheetName})
	if err != nil {
		t.Fatal(err)
	}

	comment := comments[sheetName]["B4"]
	if comment.Text != "feedback exportado do Google Sheets" {
		t.Fatalf("comment text = %q", comment.Text)
	}
	if comment.Author != "Professor" {
		t.Fatalf("comment author = %q", comment.Author)
	}
}

func TestParseGridAppliesXLSXCommentsToActivitySubtopics(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("Modelagem", "")),
		rowData(cellData("Nota maxima", ""), cellData("2", "")),
		rowData(cellData("Alice", ""), cellData("1,5", "")),
	}, nil)
	grid.applyComments(map[string]cellComment{
		"B2": {Text: "comentario vindo do xlsx", Author: "Professor"},
	})

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
	detail := table.Cards[0].Details[0]
	if detail.Comment != "comentario vindo do xlsx" {
		t.Fatalf("detail comment = %q", detail.Comment)
	}
	if detail.CommentAuthor != "Professor" {
		t.Fatalf("detail author = %q", detail.CommentAuthor)
	}
}

func TestSheetGridFeedbacksReturnsAbsoluteCells(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "feedback do cabecalho")),
		rowData(cellData("Subtópico", ""), cellData("Modelagem", "")),
		rowData(cellData("Alice", ""), cellData("1,5", "feedback da linha")),
	}, nil)
	grid.noteAuthors[1] = "Professor"
	grid.rowNoteAuthors[1][1] = "Monitor"

	feedbacks := grid.feedbacks()
	if len(feedbacks) != 2 {
		t.Fatalf("feedbacks len = %d, want 2", len(feedbacks))
	}
	if feedbacks[0].Cell != "B1" || feedbacks[0].Author != "Professor" {
		t.Fatalf("first feedback = %#v", feedbacks[0])
	}
	if feedbacks[1].Cell != "B3" || feedbacks[1].Author != "Monitor" {
		t.Fatalf("second feedback = %#v", feedbacks[1])
	}
}

func TestApplyDriveCommentsMatchesUniqueQuotedCell(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("Modelagem", "")),
		rowData(cellData("Alice", ""), cellData("1,5", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{QuotedText: "Modelagem", Text: "comentario do Drive", Author: "Professor", SheetID: 123, HasSheetID: true},
		{QuotedText: "Alice", Text: "outra aba", Author: "Professor", SheetID: 456, HasSheetID: true},
	}, 123)

	if got := noteAt(grid.rowNotes[0], 1); got != "comentario do Drive" {
		t.Fatalf("drive comment = %q", got)
	}
	if got := noteAt(grid.rowNotes[1], 0); got != "" {
		t.Fatalf("comment from another sheet should be ignored, got %q", got)
	}
}

func TestApplyDriveCommentsIgnoresAmbiguousQuotedCell(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Aluno", ""), cellData("Critério", "")),
		rowData(cellData("Subtópico", ""), cellData("1", "")),
		rowData(cellData("Alice", ""), cellData("1", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{QuotedText: "1", Text: "comentario ambiguo", Author: "Professor"},
	}, 0)

	if got := noteAt(grid.rowNotes[0], 1); got != "" {
		t.Fatalf("ambiguous detail comment = %q", got)
	}
	if got := noteAt(grid.rowNotes[1], 1); got != "" {
		t.Fatalf("ambiguous student comment = %q", got)
	}
}

func TestDriveCommentSheetIDParsesWorkbookAnchor(t *testing.T) {
	sheetID, ok := driveCommentSheetID(`{"type":"workbook-range","uid":123,"range":"456"}`)
	if !ok || sheetID != 123 {
		t.Fatalf("sheetID = %d, ok = %v", sheetID, ok)
	}
}
