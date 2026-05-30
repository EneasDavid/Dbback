package app

import (
	"testing"

	"google.golang.org/api/sheets/v4"
)

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

func TestCellName(t *testing.T) {
	tests := []struct {
		row  int
		col  int
		want string
	}{
		{row: 0, col: 0, want: "A1"},
		{row: 2, col: 1, want: "B3"},
		{row: 0, col: 25, want: "Z1"},
		{row: 0, col: 26, want: "AA1"},
	}

	for _, tt := range tests {
		if got := cellName(tt.row, tt.col); got != tt.want {
			t.Fatalf("cellName(%d, %d) = %q, want %q", tt.row, tt.col, got, tt.want)
		}
	}
}
