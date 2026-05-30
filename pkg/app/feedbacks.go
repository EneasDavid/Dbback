package app

import (
	"context"
	"strconv"
)

func (c *SheetsClient) LoadSheetFeedbacks(ctx context.Context, sheetNames []string) (map[string][]SheetComment, error) {
	result := map[string][]SheetComment{}
	for _, sheetName := range sheetNames {
		grid, err := c.loadSheet(ctx, sheetName)
		if err != nil {
			return nil, err
		}
		result[sheetName] = grid.feedbacks()
	}
	return result, nil
}

func (g *sheetGrid) feedbacks() []SheetComment {
	var comments []SheetComment
	appendRow := func(rowIdx int, notes []string, authors []string) {
		for colIdx, note := range notes {
			note = visibleFeedbackComment(note)
			if note == "" {
				continue
			}
			comments = append(comments, SheetComment{
				Cell:   cellName(rowIdx, colIdx),
				Text:   note,
				Author: noteAt(authors, colIdx),
			})
		}
	}

	appendRow(g.headerRow, g.notes, g.noteAuthors)
	for idx, rowIdx := range g.rowIndices {
		appendRow(rowIdx, g.rowNotes[idx], g.rowNoteAuthors[idx])
	}
	return comments
}

func cellName(rowIdx int, colIdx int) string {
	col := colIdx + 1
	var letters []byte
	for col > 0 {
		col--
		letters = append([]byte{byte('A' + col%26)}, letters...)
		col /= 26
	}
	return string(letters) + strconv.Itoa(rowIdx+1)
}
