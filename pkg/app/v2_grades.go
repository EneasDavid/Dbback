package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	v2ABsSheet        = "abs"
	v2ActivitiesSheet = "atividades"
	v2NotLaunchedText = "essa atividade nao foi lancada"
)

type v2ActivityConfig struct {
	Key           string
	Label         string
	AB            string
	SheetName     string
	Weight        float64
	SummaryCol    int
	Order         int
	SchemaStatus  string
	SpreadsheetID string
}

func (c *SheetsClient) gradesForV2(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	exams = normalizedExams(exams)
	results := make(GradeResults, len(exams))
	for _, exam := range exams {
		result, err := c.gradeForV2(ctx, exam, user)
		if isNotFound(err) {
			results[exam] = emptyGradeResult(exam, user)
			continue
		}
		if err != nil {
			return nil, err
		}
		results[exam] = result
	}
	return results, nil
}

func (c *SheetsClient) gradeForV2(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	exam = strings.ToLower(strings.TrimSpace(exam))
	if exam != "ab1" && exam != "ab2" {
		return GradeResult{}, NewHTTPError(400, "avaliacao invalida")
	}

	if err := c.loadSheets(ctx, []string{v2ABsSheet, v2ActivitiesSheet, v2SummarySheetName(exam)}); err != nil {
		return GradeResult{}, err
	}

	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	if err != nil {
		return GradeResult{}, err
	}
	abLabel, active := v2ABState(abGrid, exam)
	if !active {
		result := emptyGradeResult(exam, user)
		result.SchemaStatus = abGrid.schemaStatus
		result.SpreadsheetID = abGrid.spreadsheetID
		return result, nil
	}
	if abLabel == "" {
		abLabel = strings.ToUpper(exam)
	}

	activitiesGrid, err := c.loadSheet(ctx, v2ActivitiesSheet)
	if err != nil {
		return GradeResult{}, err
	}
	summaryGrid, err := c.loadSheet(ctx, v2SummarySheetName(exam))
	if err != nil {
		return GradeResult{}, err
	}

	summaryRowIdx := findStudentRow(summaryGrid, 0, user)
	if summaryRowIdx < 0 {
		return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+strings.ToUpper(exam))
	}
	summaryRow := summaryGrid.rows[summaryRowIdx]
	groupValue := valueAt(summaryRow, groupColumn(summaryGrid.headers))

	activities := v2ActivitiesForAB(activitiesGrid, exam)
	v2BindSummaryColumns(summaryGrid.headers, activities)
	if len(activities) == 0 {
		return GradeResult{}, NewHTTPError(404, "atividades nao encontradas para "+strings.ToUpper(exam))
	}

	var activitySheets []string
	for _, activity := range activities {
		if v2ActivityLaunched(summaryRow, activity) {
			activitySheets = append(activitySheets, activity.SheetName)
		}
	}
	if len(activitySheets) > 0 {
		if err := c.loadSheets(ctx, activitySheets); err != nil {
			return GradeResult{}, err
		}
	}

	result := emptyGradeResult(exam, user)
	result.SchemaStatus = mergeSchemaStatus(mergeSchemaStatus(abGrid.schemaStatus, activitiesGrid.schemaStatus), summaryGrid.schemaStatus)
	result.SpreadsheetID = mergeSourceValue(mergeSourceValue(abGrid.spreadsheetID, activitiesGrid.spreadsheetID), summaryGrid.spreadsheetID)

	for _, activity := range activities {
		if !v2ActivityLaunched(summaryRow, activity) {
			continue
		}
		table, found, err := c.v2ActivityTable(ctx, activity, summaryRow, groupValue, user)
		if err != nil {
			return GradeResult{}, err
		}
		if !found {
			continue
		}
		result.Tables = append(result.Tables, table)
		result.SchemaStatus = mergeSchemaStatus(result.SchemaStatus, table.SchemaStatus)
		result.SpreadsheetID = mergeSourceValue(result.SpreadsheetID, table.SpreadsheetID)
	}

	if average := v2AverageCard(summaryGrid, summaryRow); average != nil {
		result.Tables = append(result.Tables, TableResult{
			Key:           "media-" + exam,
			Label:         "Média " + abLabel,
			SheetName:     v2SummarySheetName(exam),
			Kind:          exam + "summary",
			Complete:      true,
			SchemaStatus:  summaryGrid.schemaStatus,
			SpreadsheetID: summaryGrid.spreadsheetID,
			Cards:         []CardResult{*average},
		})
	}
	if len(result.Tables) == 0 {
		return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+strings.ToUpper(exam))
	}
	return result, nil
}

func v2SummarySheetName(exam string) string {
	return "nota " + strings.ToLower(strings.TrimSpace(exam))
}

func v2ABState(grid *sheetGrid, exam string) (string, bool) {
	abIdx := firstHeaderIndex(grid.headers, "ab", "avaliacao", "avaliacao bimestral")
	activeIdx := firstHeaderIndex(grid.headers, "ativo", "ativa", "status", "liberado")
	labelIdx := firstHeaderIndex(grid.headers, "nome", "label", "rotulo", "titulo")
	if abIdx < 0 {
		abIdx = 0
	}
	for _, row := range grid.rows {
		if normalizeHeader(valueAt(row, abIdx)) != normalizeHeader(exam) {
			continue
		}
		label := valueAt(row, labelIdx)
		if label == "" {
			label = strings.ToUpper(exam)
		}
		return label, activeIdx < 0 || truthySpreadsheetValue(valueAt(row, activeIdx))
	}
	return strings.ToUpper(exam), false
}

func v2ActivitiesForAB(grid *sheetGrid, exam string) []v2ActivityConfig {
	abIdx := firstHeaderIndex(grid.headers, "ab", "avaliacao", "avaliacao bimestral")
	nameIdx := firstHeaderIndex(grid.headers, "atividade", "nome", "nome da atividade", "titulo")
	sheetIdx := firstHeaderIndex(grid.headers, "aba", "sheet", "planilha")
	weightIdx := firstHeaderIndex(grid.headers, "pesomaximo", "peso maximo", "peso máximo", "peso", "nota maxima")
	activeIdx := firstHeaderIndex(grid.headers, "ativo", "ativa", "status", "lancada", "lançada")
	if nameIdx < 0 {
		nameIdx = 0
	}
	var activities []v2ActivityConfig
	for rowIdx, row := range grid.rows {
		if abIdx >= 0 && normalizeHeader(valueAt(row, abIdx)) != normalizeHeader(exam) {
			continue
		}
		if activeIdx >= 0 && !truthySpreadsheetValue(valueAt(row, activeIdx)) {
			continue
		}
		label := valueAt(row, nameIdx)
		if label == "" {
			continue
		}
		sheetName := valueAt(row, sheetIdx)
		if sheetName == "" {
			sheetName = label
		}
		weight, ok := parseNumber(valueAt(row, weightIdx))
		if !ok || weight <= 0 {
			weight = 1
		}
		activities = append(activities, v2ActivityConfig{
			Key:           fmt.Sprintf("v2-at-%d", rowIdx+1),
			Label:         humanizeLabel(label),
			AB:            strings.ToUpper(exam),
			SheetName:     sheetName,
			Weight:        weight,
			Order:         rowIdx,
			SchemaStatus:  grid.schemaStatus,
			SpreadsheetID: grid.spreadsheetID,
		})
	}
	sort.SliceStable(activities, func(i, j int) bool {
		return activities[i].Order < activities[j].Order
	})
	return activities
}

func v2BindSummaryColumns(headers []string, activities []v2ActivityConfig) {
	for idx := range activities {
		activities[idx].SummaryCol = matchingHeaderIndex(headers, activities[idx].Label, activities[idx].SheetName)
	}
}

func v2ActivityLaunched(summaryRow []string, activity v2ActivityConfig) bool {
	if activity.SummaryCol < 0 {
		return false
	}
	value := valueAt(summaryRow, activity.SummaryCol)
	return normalizeHeader(value) != "" && !strings.Contains(normalizeHeader(value), v2NotLaunchedText)
}

func (c *SheetsClient) v2ActivityTable(ctx context.Context, activity v2ActivityConfig, summaryRow []string, groupValue string, user SessionUser) (TableResult, bool, error) {
	grid, err := c.loadSheet(ctx, activity.SheetName)
	if err != nil {
		return TableResult{}, false, err
	}
	rowIdx := v2ActivityRow(grid, groupValue, user)
	if rowIdx < 0 {
		return TableResult{}, false, nil
	}

	maxRowIdx := findMaxRow(grid.rows)
	items := v2ActivityItems(grid, maxRowIdx, rowIdx, activity.Weight)
	if len(items) == 0 {
		return TableResult{}, false, nil
	}
	details := activityDetails(items, 1)
	score := valueAt(summaryRow, activity.SummaryCol)
	if parsed, ok := parseNumber(score); ok && parsed > activity.Weight && activity.Weight > 0 {
		score = formatNumber(parsed / activity.Weight)
	}
	card := makeCard("nota", "Nota", score, "", "", details)
	card.DisplayValue = formatScoreForWeight(score, activity.Weight)
	return TableResult{
		Key:           activity.Key,
		Label:         activity.Label,
		SheetName:     activity.SheetName,
		Kind:          "activity",
		Complete:      true,
		Status:        activityStatus(items),
		SchemaStatus:  mergeSchemaStatus(activity.SchemaStatus, grid.schemaStatus),
		SpreadsheetID: mergeSourceValue(activity.SpreadsheetID, grid.spreadsheetID),
		Cards:         []CardResult{card},
	}, true, nil
}

func v2ActivityRow(grid *sheetGrid, groupValue string, user SessionUser) int {
	groupIdx := groupColumn(grid.headers)
	if groupIdx >= 0 && strings.TrimSpace(groupValue) != "" {
		for rowIdx, row := range grid.rows {
			if sameLookupValue(valueAt(row, groupIdx), groupValue, true) {
				return rowIdx
			}
		}
	}
	return findStudentRow(grid, 0, user)
}

func v2ActivityItems(grid *sheetGrid, maxRowIdx int, studentRowIdx int, weight float64) []activityItem {
	items := make([]activityItem, 0, len(grid.headers))
	totalMax := v2TotalMaximum(grid, maxRowIdx, studentRowIdx)
	for colIdx := 0; colIdx < len(grid.headers); colIdx++ {
		header := valueAt(grid.headers, colIdx)
		if !shouldShowV2Criterion(header) {
			continue
		}
		maximum := v2CriterionMaximum(grid, maxRowIdx, colIdx)
		if maximum <= 0 {
			continue
		}
		value := valueAt(grid.rows[studentRowIdx], colIdx)
		if totalMax > 0 && weight > 0 {
			if parsed, ok := parseNumber(value); ok {
				value = formatNumber((parsed / totalMax) * weight)
			}
			maximum = (maximum / totalMax) * weight
		}
		comment, author := activityItemComment(grid, maxRowIdx, studentRowIdx, colIdx)
		items = append(items, activityItem{
			Key:           fmt.Sprintf("i%d", colIdx),
			Subtopic:      rubricLabel(grid, maxRowIdx, colIdx),
			NotaMaxima:    formatNumber(maximum),
			NotaAlcancada: value,
			Comment:       comment,
			CommentAuthor: author,
		})
	}
	return items
}

func v2TotalMaximum(grid *sheetGrid, maxRowIdx int, studentRowIdx int) float64 {
	total := 0.0
	for colIdx := 0; colIdx < len(grid.headers); colIdx++ {
		if !shouldShowV2Criterion(valueAt(grid.headers, colIdx)) {
			continue
		}
		maximum := v2CriterionMaximum(grid, maxRowIdx, colIdx)
		if maximum > 0 {
			total += maximum
		}
	}
	return total
}

func v2CriterionMaximum(grid *sheetGrid, maxRowIdx int, colIdx int) float64 {
	if maxRowIdx >= 0 {
		if maximum, ok := parseNumber(valueAt(grid.rows[maxRowIdx], colIdx)); ok {
			return maximum
		}
	}
	return inferMaxForLabel(valueAt(grid.headers, colIdx))
}

func shouldShowV2Criterion(header string) bool {
	if !shouldShowColumn(header) {
		return false
	}
	normalized := normalizeHeader(header)
	return normalized != "ab" &&
		normalized != "atividade" &&
		normalized != "peso" &&
		normalized != "peso maximo" &&
		normalized != "nota" &&
		normalized != "total" &&
		normalized != "media" &&
		normalized != "ativo" &&
		normalized != "ativa" &&
		normalized != "status"
}

func v2AverageCard(grid *sheetGrid, row []string) *CardResult {
	idx := firstHeaderIndex(grid.headers, "media", "média", "media ab", "média ab", "nota ab")
	if idx < 0 {
		idx = totalABColumn(grid.headers)
	}
	if idx < 0 {
		return nil
	}
	comment, author := commentAt(rowNotesAt(grid, indexOfRow(grid.rows, row)), rowNoteAuthorsAt(grid, indexOfRow(grid.rows, row)), idx)
	card := makeCard("media", "Média AB", valueAt(row, idx), comment, author, nil)
	return &card
}

func firstHeaderIndex(headers []string, candidates ...string) int {
	for _, candidate := range candidates {
		wanted := normalizeHeader(candidate)
		for idx, header := range headers {
			if normalizeHeader(header) == wanted {
				return idx
			}
		}
	}
	for _, candidate := range candidates {
		wanted := normalizeHeader(candidate)
		if len([]rune(wanted)) <= 2 {
			continue
		}
		for idx, header := range headers {
			if strings.Contains(normalizeHeader(header), wanted) {
				return idx
			}
		}
	}
	return -1
}

func matchingHeaderIndex(headers []string, labels ...string) int {
	for _, label := range labels {
		wanted := normalizeHeader(label)
		for idx, header := range headers {
			if normalizeHeader(header) == wanted {
				return idx
			}
		}
	}
	for _, label := range labels {
		wanted := normalizeHeader(label)
		for idx, header := range headers {
			normalized := normalizeHeader(header)
			if wanted != "" && (strings.Contains(normalized, wanted) || strings.Contains(wanted, normalized)) {
				return idx
			}
		}
	}
	return -1
}

func truthySpreadsheetValue(value string) bool {
	normalized := normalizeHeader(value)
	return normalized == "" ||
		normalized == "1" ||
		normalized == "sim" ||
		normalized == "s" ||
		normalized == "true" ||
		normalized == "ativo" ||
		normalized == "ativa" ||
		normalized == "lancada" ||
		normalized == "lançada"
}

func formatScoreForWeight(value string, weight float64) string {
	if parsed, ok := parseNumber(value); ok {
		return formatScore(parsed) + "/" + formatScore(weight)
	}
	return displayValue("Nota", value)
}

func indexOfRow(rows [][]string, target []string) int {
	for idx := range rows {
		if len(rows[idx]) == len(target) {
			match := true
			for colIdx := range target {
				if rows[idx][colIdx] != target[colIdx] {
					match = false
					break
				}
			}
			if match {
				return idx
			}
		}
	}
	return -1
}
