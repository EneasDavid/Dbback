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
		comment, author := activityItemComment(grid, maxRowIdx, studentRowIdx, colIdx)
		items = append(items, activityItem{
			Key:           fmt.Sprintf("i%d", colIdx),
			Subtopic:      subtopic,
			NotaMaxima:    maximum,
			NotaAlcancada: valueAt(grid.rows[studentRowIdx], colIdx),
			Comment:       comment,
			CommentAuthor: author,
		})
	}
	if len(items) == 0 {
		return TableResult{}, false, NewHTTPError(500, "erro de execução: lista de sub tópicos vazia na aba "+table.SheetName)
	}

	details := activityDetails(items, table.ScoreDivisor)
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
		Status:    activityStatus(items),
		Cards:     []CardResult{card},
	}, true, nil
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
		if strings.Contains(normalizeHeader(valueAt(row, 0)), "nota maxima") || strings.Contains(normalizeHeader(valueAt(row, 0)), "exemplo nota maxima") {
			return rowIdx
		}
	}
	return -1
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
		return makeActivityScoreCard(formatNumber(total/maximum), "", "", details)
	}
	return makeActivityScoreCard(formatNumber(total), "", "", details)
}

func makeActivityScoreCard(value string, comment string, commentAuthor string, details []DetailResult) CardResult {
	card := makeCard("nota", "Nota", value, comment, commentAuthor, details)
	if score, ok := parseScore(value); ok {
		card.DisplayValue = formatNumber(score) + "/" + formatNumberFixed(1, 2)
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
	if maxScore == 10 {
		return formatNumber(score / 10)
	}
	return formatNumber(score / maxScore)
}
