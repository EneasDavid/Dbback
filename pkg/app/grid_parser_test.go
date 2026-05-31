package app

import (
	"testing"

	"google.golang.org/api/sheets/v4"
)

func TestParseGridLoadsCellNotes(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Nota", "feedback do cabecalho")),
		rowData(cellData("Alice", ""), cellData("8", "feedback do aluno")),
	}, nil)

	if got := noteAt(grid.notes, 1); got != "feedback do cabecalho" {
		t.Fatalf("header note = %q, want %q", got, "feedback do cabecalho")
	}
	if got := noteAt(grid.rowNotes[0], 1); got != "feedback do aluno" {
		t.Fatalf("student note = %q, want %q", got, "feedback do aluno")
	}
}

func TestParseGridPropagatesMergedCellNotes(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "feedback mesclado"), cellData("", "")),
		rowData(cellData("Alice", ""), cellData("8", ""), cellData("9", "")),
	}, []*sheets.GridRange{
		{StartRowIndex: 0, EndRowIndex: 1, StartColumnIndex: 1, EndColumnIndex: 3},
	})

	if got := noteAt(grid.notes, 2); got != "feedback mesclado" {
		t.Fatalf("merged note = %q, want %q", got, "feedback mesclado")
	}
	if got := valueAt(grid.headers, 2); got != "Critério" {
		t.Fatalf("merged value = %q, want %q", got, "Critério")
	}
}

func TestDriveCommentOnMergedCellPropagatesToMergedColumns(t *testing.T) {
	merges := []*sheets.GridRange{
		{StartRowIndex: 0, EndRowIndex: 1, StartColumnIndex: 1, EndColumnIndex: 3},
	}
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", ""), cellData("", "")),
		rowData(cellData("Alice", ""), cellData("0,5", ""), cellData("0,7", "")),
	}, merges)

	grid.applyDriveComments([]driveCellComment{
		{Text: "feedback do Drive", Author: "Professor (Prof)", QuotedText: "Critério", SheetID: 42, HasSheetID: true},
	}, 42, merges)
	grid.applyCommentMerges(merges)

	if got := noteAt(grid.notes, 1); got != "feedback do Drive" {
		t.Fatalf("merged drive note at B1 = %q, want feedback do Drive", got)
	}
	if got := noteAt(grid.notes, 2); got != "feedback do Drive" {
		t.Fatalf("merged drive note at C1 = %q, want feedback do Drive", got)
	}
	if got := noteAt(grid.noteAuthors, 2); got != "Prof" {
		t.Fatalf("merged drive author = %q, want Prof", got)
	}
}

func TestDriveCommentWithZeroSheetIDUsesNonNumericQuotedText(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "")),
		rowData(cellData("Alice", ""), cellData("0,3", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{Text: "feedback do Drive", Author: "Professor", QuotedText: "Critério", SheetID: 0, HasSheetID: true},
	}, 987654321, nil)

	if got := noteAt(grid.notes, 1); got != "feedback do Drive" {
		t.Fatalf("drive note = %q, want feedback do Drive", got)
	}
}

func TestDriveCommentWithZeroSheetIDUsesUniqueNumericQuotedText(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "")),
		rowData(cellData("Alice", ""), cellData("0,3", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{Text: "feedback do Drive", Author: "Professor", QuotedText: "0,3", SheetID: 0, HasSheetID: true},
	}, 987654321, nil)

	if got := noteAt(grid.rowNotes[0], 1); got != "feedback do Drive" {
		t.Fatalf("drive note = %q, want feedback do Drive", got)
	}
}

func TestDriveCommentWithZeroSheetIDSkipsAmbiguousNumericQuotedText(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "")),
		rowData(cellData("Alice", ""), cellData("0,3", "")),
		rowData(cellData("Bob", ""), cellData("0,3", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{Text: "feedback do Drive", Author: "Professor", QuotedText: "0,3", SheetID: 0, HasSheetID: true},
	}, 987654321, nil)

	if got := noteAt(grid.rowNotes[0], 1); got != "" {
		t.Fatalf("first drive note = %q, want empty for ambiguous numeric quoted text", got)
	}
	if got := noteAt(grid.rowNotes[1], 1); got != "" {
		t.Fatalf("second drive note = %q, want empty for ambiguous numeric quoted text", got)
	}
}

func TestDriveCommentWithCellAnchorUsesExactRepeatedValue(t *testing.T) {
	grid := parseGrid([]*sheets.RowData{
		rowData(cellData("Nome", ""), cellData("Critério", "")),
		rowData(cellData("Alice", ""), cellData("0,3", "")),
		rowData(cellData("Bob", ""), cellData("0,3", "")),
	}, nil)

	grid.applyDriveComments([]driveCellComment{
		{
			Text:        "feedback do Bob",
			Author:      "Professor",
			QuotedText:  "0,3",
			SheetID:     0,
			HasSheetID:  true,
			RowIndex:    2,
			ColumnIndex: 1,
			HasCell:     true,
		},
	}, 987654321, nil)

	if got := noteAt(grid.rowNotes[0], 1); got != "" {
		t.Fatalf("first drive note = %q, want empty", got)
	}
	if got := noteAt(grid.rowNotes[1], 1); got != "feedback do Bob" {
		t.Fatalf("second drive note = %q, want feedback do Bob", got)
	}
}

func rowData(cells ...*sheets.CellData) *sheets.RowData {
	return &sheets.RowData{Values: cells}
}

func cellData(value string, note string) *sheets.CellData {
	return &sheets.CellData{FormattedValue: value, Note: note}
}
