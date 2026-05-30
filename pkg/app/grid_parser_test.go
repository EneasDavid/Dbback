package app

import (
	"errors"
	"os"
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

func TestXLSXCommentParserIsRemoved(t *testing.T) {
	if _, err := os.Stat("xlsx_comments.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("xlsx comment parser should not exist, stat err = %v", err)
	}
}

func rowData(cells ...*sheets.CellData) *sheets.RowData {
	return &sheets.RowData{Values: cells}
}

func cellData(value string, note string) *sheets.CellData {
	return &sheets.CellData{FormattedValue: value, Note: note}
}
