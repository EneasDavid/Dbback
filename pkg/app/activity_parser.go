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
	studentRowIdx := findStudentRow(grid, maxRowIdx+1, user)
	if studentRowIdx < 0 {
		return TableResult{}, false, nil
	}
	rowComment, rowCommentAuthor := rowIdentityComment(grid, studentRowIdx)
	if excludesStudentFromGrades(rowComment) {
		return TableResult{}, false, nil
	}

	items := make([]activityItem, 0, len(grid.headers))
	useOfficialWeights := activityUsesOfficialQuestionWeights(grid, maxRowIdx)
	for colIdx := firstSubtopicColumn(grid.headers); colIdx < len(grid.headers); colIdx++ {
		subtopic := rubricLabel(grid, maxRowIdx, colIdx)
		if subtopic == "" {
			continue
		}
		maximumText := valueAt(grid.rows[maxRowIdx], colIdx)
		if maximumText == "" {
			value := valueAt(grid.rows[studentRowIdx], colIdx)
			if value == "" && noteAt(grid.notes, colIdx) == "" {
				continue
			}
			return TableResult{}, false, NewHTTPError(500, "erro de execução: nota máxima ausente em "+table.SheetName+" / "+subtopic)
		}
		sourceMaximum, ok := parseNumber(maximumText)
		if !ok {
			continue
		}
		value := valueAt(grid.rows[studentRowIdx], colIdx)
		maximum := sourceMaximum
		if table.ScoreDivisor > 1 && useOfficialWeights {
			maximum = canonicalCriterionMaximum(subtopic, sourceMaximum)
			value = normalizedScore(value, sourceMaximum, maximum)
		}
		comment, author := activityItemComment(grid, maxRowIdx, studentRowIdx, colIdx)
		items = append(items, activityItem{
			Key:           fmt.Sprintf("i%d", colIdx),
			Subtopic:      subtopic,
			NotaMaxima:    formatNumber(maximum),
			NotaAlcancada: value,
			Comment:       comment,
			CommentAuthor: author,
		})
	}
	if len(items) == 0 {
		return TableResult{}, false, NewHTTPError(500, "erro de execução: lista de sub tópicos vazia na aba "+table.SheetName)
	}

	if useOfficialWeights && table.ScoreDivisor > 1 {
		items = activityItemsForDivisor(items, table.ScoreDivisor)
	} else {
		items = activityItemsForWeight(items, 1)
	}
	details := activityDetails(items)
	card := activityTotalCard(items, details)
	if card.Comment == "" && rowComment != "" {
		card.Comment = rowComment
		card.CommentAuthor = rowCommentAuthor
	}
	status := activityStatus(items)
	return TableResult{
		Key:       table.Key,
		Label:     table.Label,
		SheetName: table.SheetName,
		Kind:      table.Kind,
		Complete:  status == "Encerrado",
		Status:    status,
		Cards:     []CardResult{card},
	}, true, nil
}

func activityUsesOfficialQuestionWeights(grid *sheetGrid, maxRowIdx int) bool {
	labels := make([]string, 0, len(grid.headers))
	for colIdx := firstSubtopicColumn(grid.headers); colIdx < len(grid.headers); colIdx++ {
		labels = append(labels, rubricLabel(grid, maxRowIdx, colIdx))
	}
	return usesOfficialQuestionWeights(labels)
}

func activityStatus(items []activityItem) string {
	for _, item := range items {
		if strings.TrimSpace(item.NotaAlcancada) == "" {
			return "Não encerrado"
		}
	}
	return "Encerrado"
}

func findMaxRow(rows [][]string) int {
	for rowIdx, row := range rows {
		if isMaxRowLabel(valueAt(row, 0)) {
			return rowIdx
		}
	}
	return -1
}

func isMaxRowLabel(value string) bool {
	label := normalizeHeader(value)
	return strings.Contains(label, "nota maxima") ||
		strings.Contains(label, "exemplo nota maxima") ||
		strings.Contains(label, "maximo possivel") ||
		strings.Contains(label, "pontuacao maxima") ||
		strings.Contains(label, "pontuacao possivel")
}

func findStudentRow(grid *sheetGrid, start int, user SessionUser) int {
	for rowIdx := start; rowIdx < len(grid.rows); rowIdx++ {
		for _, colIdx := range identityCommentColumns(grid.headers) {
			value := valueAt(grid.rows[rowIdx], colIdx)
			if sameLookupValue(value, user.Name, true) || sameLookupValue(value, user.Matricula, false) {
				return rowIdx
			}
		}
	}
	return -1
}

func firstSubtopicColumn(headers []string) int {
	startIdx := 0
	for _, colIdx := range identityCommentColumns(headers) {
		if colIdx > startIdx {
			startIdx = colIdx
		}
	}
	return startIdx + 1
}

func activityItemComment(grid *sheetGrid, maxRowIdx int, studentRowIdx int, colIdx int) (string, string) {
	candidates := []struct {
		notes   []string
		authors []string
	}{
		{rowNotesAt(grid, studentRowIdx), rowNoteAuthorsAt(grid, studentRowIdx)},
	}
	if maxRowIdx > 0 {
		candidates = append(candidates, struct {
			notes   []string
			authors []string
		}{rowNotesAt(grid, maxRowIdx-1), rowNoteAuthorsAt(grid, maxRowIdx-1)})
	}
	candidates = append(candidates,
		struct {
			notes   []string
			authors []string
		}{grid.notes, grid.noteAuthors},
		struct {
			notes   []string
			authors []string
		}{rowNotesAt(grid, maxRowIdx), rowNoteAuthorsAt(grid, maxRowIdx)},
	)
	for _, candidate := range candidates {
		if comment, author := commentAt(candidate.notes, candidate.authors, colIdx); comment != "" {
			return comment, author
		}
	}
	return "", ""
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
			return makeActivityScoreCard(activityScore(item.NotaAlcancada, item.NotaMaxima), item.Comment, item.CommentAuthor, details)
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
		return makeActivityScoreCard(normalizedScore(formatNumber(total), maximum, 1), "", "", details)
	}
	return makeActivityScoreCard(formatNumber(total), "", "", details)
}

func makeActivityScoreCard(value string, comment string, commentAuthor string, details []DetailResult) CardResult {
	card := makeCard("nota", "Nota", value, comment, commentAuthor, details)
	if score, ok := parseScore(value); ok {
		card.DisplayValue = scoreComparisonDisplay(score, 1)
	}
	return card
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
	return normalizedScore(formatNumber(score), maxScore, 1)
}
