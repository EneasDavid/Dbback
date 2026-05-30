package app

import (
	"fmt"
	"strings"
)

func parseActivityRubric(grid *sheetGrid, table TableConfig, user SessionUser) (TableResult, bool, error) {
	maxRowIdx := findMaxRow(grid.rows)
	if maxRowIdx < 0 {
		return TableResult{}, false, NewHTTPError(500, "erro de execução: linha de nota máxima não encontrada na aba "+table.SheetName)
	}
	studentRowIdx := findStudentRow(grid.rows, maxRowIdx+1, user)
	if studentRowIdx < 0 {
		return TableResult{}, false, nil
	}
	rowComment, rowCommentAuthor := rowIdentityComment(grid, studentRowIdx)
	if excludesStudentFromGrades(rowComment) {
		return TableResult{}, false, nil
	}

	items := make([]activityItem, 0, len(grid.headers))
	for colIdx := 1; colIdx < len(grid.headers); colIdx++ {
		subtopic := rubricLabel(grid, maxRowIdx, colIdx)
		if subtopic == "" {
			continue
		}
		maximum := valueAt(grid.rows[maxRowIdx], colIdx)
		if maximum == "" {
			value := valueAt(grid.rows[studentRowIdx], colIdx)
			if value == "" && noteAt(grid.notes, colIdx) == "" {
				continue
			}
			return TableResult{}, false, NewHTTPError(500, "erro de execução: nota máxima ausente em "+table.SheetName+" / "+subtopic)
		}
		if _, ok := parseNumber(maximum); !ok {
			continue
		}
		comment, commentAuthor := activityComment(grid, maxRowIdx, studentRowIdx, colIdx)
		items = append(items, activityItem{
			ColIdx:          colIdx,
			Key:             fmt.Sprintf("i%d", colIdx),
			Subtopic:        subtopic,
			NotaMaxima:      maximum,
			NotaAlcancada:   valueAt(grid.rows[studentRowIdx], colIdx),
			Comentario:      comment,
			ComentarioAutor: commentAuthor,
		})
	}
	if len(items) == 0 {
		return TableResult{}, false, NewHTTPError(500, "erro de execução: lista de sub tópicos vazia na aba "+table.SheetName)
	}

	// Detectar se há subtópicos incompletos (sem nota)
	incompleteCount := 0
	for _, item := range items {
		if strings.TrimSpace(item.NotaAlcancada) == "" {
			incompleteCount++
		}
	}
	fillActivityCommentsFromDriveSequence(items, grid, maxRowIdx, studentRowIdx)
	if rowComment == "" {
		rowComment, rowCommentAuthor = peerIdentityComment(grid, maxRowIdx, studentRowIdx, items)
	}

	status := "Encerrado"
	if incompleteCount > 0 {
		status = "Não encerrado"
	}

	details := activityDetails(items)
	card := activityTotalCard(items, details)
	if card.Comment == "" && rowComment != "" {
		card.Comment = rowComment
		card.CommentAuthor = rowCommentAuthor
	}
	return TableResult{
		Key:       table.Key,
		Label:     table.Label,
		SheetName: table.SheetName,
		Kind:      table.Kind,
		Complete:  true,
		Status:    status,
		Cards:     []CardResult{card},
	}, true, nil
}

func activityComment(grid *sheetGrid, maxRowIdx int, studentRowIdx int, colIdx int) (string, string) {
	if studentRowIdx < len(grid.rowNotes) {
		if comment := noteAt(grid.rowNotes[studentRowIdx], colIdx); comment != "" {
			return comment, noteAt(grid.rowNoteAuthors[studentRowIdx], colIdx)
		}
	}
	detailRowIdx := maxRowIdx - 1
	if detailRowIdx >= 0 && detailRowIdx < len(grid.rowNotes) {
		if comment := noteAt(grid.rowNotes[detailRowIdx], colIdx); comment != "" {
			return comment, noteAt(grid.rowNoteAuthors[detailRowIdx], colIdx)
		}
	}
	return noteAt(grid.notes, colIdx), noteAt(grid.noteAuthors, colIdx)
}

func peerIdentityComment(grid *sheetGrid, maxRowIdx int, studentRowIdx int, items []activityItem) (string, string) {
	signature := activityScoreSignature(grid, studentRowIdx, items)
	if signature == "" {
		return "", ""
	}

	start := studentRowIdx
	for start > maxRowIdx+1 && activityScoreSignature(grid, start-1, items) == signature {
		start--
	}
	end := studentRowIdx + 1
	for end < len(grid.rows) && activityScoreSignature(grid, end, items) == signature {
		end++
	}
	if end-start <= 1 {
		return "", ""
	}

	type commentKey struct {
		text   string
		author string
	}
	seen := map[commentKey]bool{}
	var comments []commentKey
	for rowIdx := start; rowIdx < end; rowIdx++ {
		comment, author := rowIdentityComment(grid, rowIdx)
		if comment == "" || excludesStudentFromGrades(comment) {
			continue
		}
		key := commentKey{text: comment, author: author}
		if seen[key] {
			continue
		}
		seen[key] = true
		comments = append(comments, key)
	}
	if len(comments) != 1 {
		return "", ""
	}
	return comments[0].text, comments[0].author
}

func activityScoreSignature(grid *sheetGrid, rowIdx int, items []activityItem) string {
	if rowIdx < 0 || rowIdx >= len(grid.rows) {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, normalizeID(valueAt(grid.rows[rowIdx], item.ColIdx)))
	}
	return strings.Join(parts, "|")
}

func fillActivityCommentsFromDriveSequence(items []activityItem, grid *sheetGrid, maxRowIdx int, studentRowIdx int) {
	rows := activityCommentRows(items, grid, maxRowIdx)
	studentRow, ok := activityCommentRowByIndex(rows, studentRowIdx)
	if !ok || len(studentRow.ItemIndexes) < 2 || len(grid.driveComments) < 2 {
		return
	}

	usedItems := map[int]bool{}
	usedComments := map[int]bool{}
	maxLength := minInt(len(studentRow.ItemIndexes), len(grid.driveComments))
	for length := maxLength; length >= minActivityCommentSequenceLength(maxLength); length-- {
		matches := activityCommentSequenceMatches(rows, grid.driveComments, length)
		for _, match := range matches {
			if match.RowIdx != studentRowIdx || !activityCommentMatchIsUnique(match, matches) {
				continue
			}
			if activityCommentMatchUsed(match, usedItems, usedComments) {
				continue
			}
			applyActivityCommentMatch(items, match)
			for _, itemIdx := range match.ItemIndexes {
				usedItems[itemIdx] = true
			}
			for commentIdx := match.CommentStart; commentIdx < match.CommentStart+match.Length; commentIdx++ {
				usedComments[commentIdx] = true
			}
		}
	}
}

type activityCommentRow struct {
	RowIdx      int
	ItemIndexes []int
	Scores      []string
}

type activityCommentMatch struct {
	RowIdx       int
	CommentStart int
	Length       int
	ItemIndexes  []int
	Comments     []driveCellComment
}

func activityCommentRows(items []activityItem, grid *sheetGrid, maxRowIdx int) []activityCommentRow {
	rows := make([]activityCommentRow, 0, len(grid.rows))
	for rowIdx := maxRowIdx + 1; rowIdx < len(grid.rows); rowIdx++ {
		row := activityCommentRow{RowIdx: rowIdx}
		for itemIdx, item := range items {
			if !activityItemCanReceiveDriveComment(item) {
				continue
			}
			score := valueAt(grid.rows[rowIdx], item.ColIdx)
			if strings.TrimSpace(score) == "" {
				continue
			}
			row.ItemIndexes = append(row.ItemIndexes, itemIdx)
			row.Scores = append(row.Scores, score)
		}
		if len(row.ItemIndexes) >= 2 {
			rows = append(rows, row)
		}
	}
	return rows
}

func activityItemCanReceiveDriveComment(item activityItem) bool {
	return normalizeHeader(item.Subtopic) != "total" && strings.TrimSpace(item.NotaAlcancada) != ""
}

func activityCommentRowByIndex(rows []activityCommentRow, rowIdx int) (activityCommentRow, bool) {
	for _, row := range rows {
		if row.RowIdx == rowIdx {
			return row, true
		}
	}
	return activityCommentRow{}, false
}

func activityCommentSequenceMatches(rows []activityCommentRow, comments []driveCellComment, length int) []activityCommentMatch {
	if length < minActivityCommentSequenceLength(length) {
		return nil
	}
	var matches []activityCommentMatch
	for _, row := range rows {
		for segmentStart := 0; segmentStart+length <= len(row.ItemIndexes); segmentStart++ {
			scores := row.Scores[segmentStart : segmentStart+length]
			itemIndexes := row.ItemIndexes[segmentStart : segmentStart+length]
			for commentStart := 0; commentStart+length <= len(comments); commentStart++ {
				window := comments[commentStart : commentStart+length]
				if matched, ok := activityCommentWindowMatch(scores, itemIndexes, window, false); ok {
					matched.RowIdx = row.RowIdx
					matched.CommentStart = commentStart
					matched.Length = length
					matches = append(matches, matched)
				}
				if matched, ok := activityCommentWindowMatch(scores, itemIndexes, window, true); ok {
					matched.RowIdx = row.RowIdx
					matched.CommentStart = commentStart
					matched.Length = length
					matches = append(matches, matched)
				}
			}
		}
	}
	return matches
}

func activityCommentWindowMatch(scores []string, itemIndexes []int, comments []driveCellComment, reverse bool) (activityCommentMatch, bool) {
	if len(scores) != len(comments) || len(scores) != len(itemIndexes) {
		return activityCommentMatch{}, false
	}
	matched := activityCommentMatch{
		ItemIndexes: append([]int(nil), itemIndexes...),
		Comments:    make([]driveCellComment, len(comments)),
	}
	for offset, score := range scores {
		commentIdx := offset
		if reverse {
			commentIdx = len(comments) - 1 - offset
		}
		comment := comments[commentIdx]
		if !sameQuotedCellValue(comment.QuotedText, score) || visibleFeedbackComment(comment.Text) == "" {
			return activityCommentMatch{}, false
		}
		matched.Comments[offset] = comment
	}
	return matched, true
}

func activityCommentMatchIsUnique(match activityCommentMatch, matches []activityCommentMatch) bool {
	sameCommentWindowForRow := 0
	sameTargetSegment := 0
	for _, candidate := range matches {
		if candidate.RowIdx == match.RowIdx && candidate.CommentStart == match.CommentStart && candidate.Length == match.Length {
			sameCommentWindowForRow++
		}
		if candidate.RowIdx == match.RowIdx && sameIntSlice(candidate.ItemIndexes, match.ItemIndexes) {
			sameTargetSegment++
		}
	}
	return sameCommentWindowForRow == 1 && sameTargetSegment == 1
}

func activityCommentMatchUsed(match activityCommentMatch, usedItems map[int]bool, usedComments map[int]bool) bool {
	for _, itemIdx := range match.ItemIndexes {
		if usedItems[itemIdx] {
			return true
		}
	}
	for commentIdx := match.CommentStart; commentIdx < match.CommentStart+match.Length; commentIdx++ {
		if usedComments[commentIdx] {
			return true
		}
	}
	return false
}

func applyActivityCommentMatch(items []activityItem, match activityCommentMatch) {
	for offset, itemIdx := range match.ItemIndexes {
		comment := match.Comments[offset]
		items[itemIdx].Comentario = visibleFeedbackComment(comment.Text)
		items[itemIdx].ComentarioAutor = authorDisplayName(comment.Author)
	}
}

func sameIntSlice(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func minActivityCommentSequenceLength(maxLength int) int {
	if maxLength < 3 {
		return 2
	}
	return 3
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func isNumericCellText(value string) bool {
	_, ok := parseNumber(value)
	return ok
}

func sameQuotedCellValue(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if normalizeID(left) == normalizeID(right) {
		return true
	}
	leftScore, leftOK := parseScore(left)
	rightScore, rightOK := parseScore(right)
	return leftOK && rightOK && formatScore(leftScore) == formatScore(rightScore)
}

func findMaxRow(rows [][]string) int {
	for rowIdx, row := range rows {
		if strings.Contains(normalizeHeader(valueAt(row, 0)), "nota maxima") || strings.Contains(normalizeHeader(valueAt(row, 0)), "exemplo nota maxima") {
			return rowIdx
		}
	}
	return -1
}

func findStudentRow(rows [][]string, start int, user SessionUser) int {
	for rowIdx := start; rowIdx < len(rows); rowIdx++ {
		for _, value := range rows[rowIdx] {
			if sameLookupValue(value, user.Name, true) || sameLookupValue(value, user.Matricula, false) {
				return rowIdx
			}
		}
	}
	return -1
}

func rubricLabel(grid *sheetGrid, maxRowIdx int, colIdx int) string {
	main := valueAt(grid.headers, colIdx)
	detail := ""
	if maxRowIdx > 0 {
		detail = valueAt(grid.rows[maxRowIdx-1], colIdx)
	}
	if detail != "" {
		if main != "" && normalizeHeader(main) != normalizeHeader(detail) {
			return main + " / " + detail
		}
		return detail
	}
	return main
}

func activityTotalCard(items []activityItem, details []DetailResult) CardResult {
	for _, item := range items {
		if normalizeHeader(item.Subtopic) == "total" {
			return makeCard("nota", "Nota", activityScore(item.NotaAlcancada, item.NotaMaxima), item.Comentario, item.ComentarioAutor, details)
		}
	}
	total := 0.0
	maximum := 0.0
	hasAny := false
	for _, item := range items {
		if value, ok := parseNumber(item.NotaAlcancada); ok {
			total += value
			hasAny = true
		}
		if value, ok := parseNumber(item.NotaMaxima); ok {
			maximum += value
		}
	}
	if !hasAny {
		return makeCard("nota", "Nota", "", "", "", details)
	}
	if maximum > 0 {
		return makeCard("nota", "Nota", formatNumber(total/maximum), "", "", details)
	}
	return makeCard("nota", "Nota", formatNumber(total), "", "", details)
}

func activityScore(value string, maximum string) string {
	score, ok := parseNumber(value)
	if !ok {
		return value
	}
	maxScore, ok := parseNumber(maximum)
	if !ok || maxScore == 0 {
		return value
	}
	if maxScore == 10 {
		return formatNumber(score / 10)
	}
	return formatNumber(score / maxScore)
}
