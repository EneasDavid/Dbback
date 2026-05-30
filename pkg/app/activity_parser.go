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
	fillActivityCommentsFromDriveSequence(items, grid.driveComments)

	status := "Encerrado"
	if incompleteCount > 0 {
		status = "Não encerrado"
	}

	details := activityDetails(items)
	card := activityTotalCard(items, details)
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

func fillActivityCommentsFromDriveSequence(items []activityItem, comments []driveCellComment) {
	targets := activityCommentTargets(items)
	if len(targets) == 0 || len(comments) < len(targets) {
		return
	}

	for start := 0; start+len(targets) <= len(comments); start++ {
		if applyActivityCommentSequence(items, targets, comments[start:start+len(targets)], false) {
			return
		}
		if applyActivityCommentSequence(items, targets, comments[start:start+len(targets)], true) {
			return
		}
	}
}

func activityCommentTargets(items []activityItem) []int {
	targets := make([]int, 0, len(items))
	for idx, item := range items {
		if normalizeHeader(item.Subtopic) == "total" || strings.TrimSpace(item.NotaAlcancada) == "" {
			continue
		}
		targets = append(targets, idx)
	}
	return targets
}

func applyActivityCommentSequence(items []activityItem, targets []int, comments []driveCellComment, reverse bool) bool {
	if len(targets) != len(comments) {
		return false
	}
	matched := make([]driveCellComment, len(targets))
	for offset, itemIdx := range targets {
		commentIdx := offset
		if reverse {
			commentIdx = len(comments) - 1 - offset
		}
		comment := comments[commentIdx]
		if !sameQuotedCellValue(comment.QuotedText, items[itemIdx].NotaAlcancada) {
			return false
		}
		if visibleFeedbackComment(comment.Text) == "" {
			return false
		}
		matched[offset] = comment
	}
	for offset, itemIdx := range targets {
		comment := matched[offset]
		items[itemIdx].Comentario = visibleFeedbackComment(comment.Text)
		items[itemIdx].ComentarioAutor = authorDisplayName(comment.Author)
	}
	return true
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
