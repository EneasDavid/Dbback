package app

import (
	"fmt"
	"strings"

	"google.golang.org/api/sheets/v4"
)

func parseGrid(rows []*sheets.RowData, merges []*sheets.GridRange) *sheetGrid {
	allValues := make([][]string, 0, len(rows))
	allNotes := make([][]string, 0, len(rows))
	for _, row := range rows {
		values := make([]string, len(row.Values))
		notes := make([]string, len(row.Values))
		for idx, cell := range row.Values {
			values[idx] = cellText(cell)
			notes[idx] = strings.TrimSpace(cell.Note)
		}
		allValues = append(allValues, values)
		allNotes = append(allNotes, notes)
	}
	applyMergedRanges(allValues, allNotes, merges)

	headerIdx := bestHeaderIndex(allValues)
	if headerIdx < 0 {
		return &sheetGrid{}
	}

	grid := &sheetGrid{headers: allValues[headerIdx], notes: allNotes[headerIdx], noteAuthors: make([]string, len(allNotes[headerIdx])), headerRow: headerIdx}
	for idx, values := range allValues[headerIdx+1:] {
		if hasAny(values) {
			grid.rows = append(grid.rows, values)
			grid.rowNotes = append(grid.rowNotes, allNotes[headerIdx+1+idx])
			grid.rowNoteAuthors = append(grid.rowNoteAuthors, make([]string, len(allNotes[headerIdx+1+idx])))
			grid.rowIndices = append(grid.rowIndices, headerIdx+1+idx)
		}
	}
	return grid
}

func applyMergedRanges(values [][]string, notes [][]string, merges []*sheets.GridRange) {
	for _, merged := range merges {
		startRow := int(merged.StartRowIndex)
		endRow := int(merged.EndRowIndex)
		startCol := int(merged.StartColumnIndex)
		endCol := int(merged.EndColumnIndex)
		if startRow < 0 || startCol < 0 || endRow <= startRow || endCol <= startCol {
			continue
		}
		value := matrixValue(values, startRow, startCol)
		note := matrixValue(notes, startRow, startCol)
		for rowIdx := startRow; rowIdx < endRow; rowIdx++ {
			for colIdx := startCol; colIdx < endCol; colIdx++ {
				if value != "" {
					setMatrixValue(values, rowIdx, colIdx, value)
				}
				if note != "" {
					setMatrixValue(notes, rowIdx, colIdx, note)
				}
			}
		}
	}
}

func (g *sheetGrid) applyDriveComments(comments []driveCellComment, sheetID int64, merges []*sheets.GridRange) {
	g.driveComments = nil
	for _, comment := range comments {
		// Drive anchors for Sheets comments can expose uid:0 even when the target sheet has another sheetId.
		if comment.HasSheetID && comment.SheetID != 0 && comment.SheetID != sheetID {
			continue
		}
		comment.Text = visibleFeedbackComment(comment.Text)
		if strings.TrimSpace(comment.Text) == "" {
			continue
		}
		g.driveComments = append(g.driveComments, comment)
		if strings.TrimSpace(comment.Text) == "" || strings.TrimSpace(comment.QuotedText) == "" {
			continue
		}
		if comment.HasSheetID && comment.SheetID == 0 && isNumericCellText(comment.QuotedText) {
			continue
		}
		rowIdx, colIdx, ok := g.uniqueCellForQuotedText(comment.QuotedText, merges)
		if !ok || g.noteAtAbsolute(rowIdx, colIdx) != "" {
			continue
		}
		g.setNoteAtAbsolute(rowIdx, colIdx, comment.Text, comment.Author)
	}
}

func (g *sheetGrid) uniqueCellForQuotedText(value string, merges []*sheets.GridRange) (int, int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, false
	}

	found := map[string][2]int{}
	check := func(rowIdx int, row []string) {
		for colIdx, cell := range row {
			if strings.TrimSpace(cell) != value {
				continue
			}
			logicalRow, logicalCol := logicalMergedCell(rowIdx, colIdx, merges)
			key := fmt.Sprintf("%d:%d", logicalRow, logicalCol)
			found[key] = [2]int{logicalRow, logicalCol}
		}
	}

	check(g.headerRow, g.headers)
	for idx, row := range g.rows {
		check(g.rowIndices[idx], row)
	}
	if len(found) != 1 {
		return 0, 0, false
	}
	for _, cell := range found {
		return cell[0], cell[1], true
	}
	return 0, 0, false
}

func logicalMergedCell(rowIdx int, colIdx int, merges []*sheets.GridRange) (int, int) {
	for _, merged := range merges {
		startRow := int(merged.StartRowIndex)
		endRow := int(merged.EndRowIndex)
		startCol := int(merged.StartColumnIndex)
		endCol := int(merged.EndColumnIndex)
		if rowIdx >= startRow && rowIdx < endRow && colIdx >= startCol && colIdx < endCol {
			return startRow, startCol
		}
	}
	return rowIdx, colIdx
}

func matrixValue(values [][]string, rowIdx int, colIdx int) string {
	if rowIdx < 0 || rowIdx >= len(values) || colIdx < 0 || colIdx >= len(values[rowIdx]) {
		return ""
	}
	return strings.TrimSpace(values[rowIdx][colIdx])
}

func setMatrixValue(values [][]string, rowIdx int, colIdx int, value string) {
	if rowIdx < 0 || rowIdx >= len(values) || colIdx < 0 {
		return
	}
	for len(values[rowIdx]) <= colIdx {
		values[rowIdx] = append(values[rowIdx], "")
	}
	values[rowIdx][colIdx] = value
}

func (g *sheetGrid) applyCommentMerges(merges []*sheets.GridRange) {
	for _, merged := range merges {
		startRow := int(merged.StartRowIndex)
		endRow := int(merged.EndRowIndex)
		startCol := int(merged.StartColumnIndex)
		endCol := int(merged.EndColumnIndex)
		if startRow < 0 || startCol < 0 || endRow <= startRow || endCol <= startCol {
			continue
		}
		comment := g.noteAtAbsolute(startRow, startCol)
		author := g.noteAuthorAtAbsolute(startRow, startCol)
		if comment == "" {
			continue
		}
		for rowIdx := startRow; rowIdx < endRow; rowIdx++ {
			for colIdx := startCol; colIdx < endCol; colIdx++ {
				g.setNoteAtAbsolute(rowIdx, colIdx, comment, author)
			}
		}
	}
}

func (g *sheetGrid) noteAtAbsolute(rowIdx int, colIdx int) string {
	if rowIdx == g.headerRow {
		return noteAt(g.notes, colIdx)
	}
	for idx, actualRow := range g.rowIndices {
		if actualRow == rowIdx {
			return noteAt(g.rowNotes[idx], colIdx)
		}
	}
	return ""
}

func (g *sheetGrid) noteAuthorAtAbsolute(rowIdx int, colIdx int) string {
	if rowIdx == g.headerRow {
		return noteAt(g.noteAuthors, colIdx)
	}
	for idx, actualRow := range g.rowIndices {
		if actualRow == rowIdx {
			return noteAt(g.rowNoteAuthors[idx], colIdx)
		}
	}
	return ""
}

func (g *sheetGrid) setNoteAtAbsolute(rowIdx int, colIdx int, comment string, author string) {
	comment = visibleFeedbackComment(comment)
	if comment == "" {
		return
	}
	author = authorDisplayName(author)
	if rowIdx == g.headerRow {
		g.notes = setAt(g.notes, colIdx, comment)
		g.noteAuthors = setAt(g.noteAuthors, colIdx, author)
		return
	}
	for idx, actualRow := range g.rowIndices {
		if actualRow == rowIdx {
			g.rowNotes[idx] = setAt(g.rowNotes[idx], colIdx, comment)
			g.rowNoteAuthors[idx] = setAt(g.rowNoteAuthors[idx], colIdx, author)
			return
		}
	}
}

func setAt(values []string, idx int, value string) []string {
	for len(values) <= idx {
		values = append(values, "")
	}
	values[idx] = value
	return values
}

func bestHeaderIndex(rows [][]string) int {
	headerIdx := -1
	bestScore := 0
	for idx, values := range rows {
		score := headerScore(values)
		if score > bestScore {
			bestScore = score
			headerIdx = idx
		}
	}
	if headerIdx >= 0 {
		return headerIdx
	}
	for idx, values := range rows {
		if hasAny(values) {
			return idx
		}
	}
	return -1
}

func cellText(cell *sheets.CellData) string {
	if cell == nil {
		return ""
	}
	if cell.FormattedValue != "" {
		return strings.TrimSpace(cell.FormattedValue)
	}
	if cell.UserEnteredValue == nil {
		return ""
	}
	if cell.UserEnteredValue.StringValue != nil {
		return strings.TrimSpace(*cell.UserEnteredValue.StringValue)
	}
	if cell.UserEnteredValue.NumberValue != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", *cell.UserEnteredValue.NumberValue))
	}
	if cell.UserEnteredValue.BoolValue != nil {
		if *cell.UserEnteredValue.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}

func hasAny(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
