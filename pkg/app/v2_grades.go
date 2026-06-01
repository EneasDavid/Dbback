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
	if len(exams) == 0 {
		resolved, err := c.v2ActiveExamKeys(ctx)
		if err != nil {
			return nil, err
		}
		exams = resolved
	}
	results := make(GradeResults, len(exams))
	for _, exam := range exams {
		result, err := c.gradeForV2(ctx, exam, user)
		if isNotFound(err) {
			results[exam] = c.emptyGradeResultForV2(ctx, exam, user)
			continue
		}
		if err != nil {
			return nil, err
		}
		results[exam] = result
	}
	return results, nil
}

func (c *SheetsClient) v2ActiveExamKeys(ctx context.Context) ([]string, error) {
	if err := c.loadSheets(ctx, []string{v2ABsSheet}); err != nil {
		return nil, err
	}
	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	if err != nil {
		return nil, err
	}
	abs := v2ABs(abGrid)
	keys := make([]string, 0, len(abs))
	for _, ab := range abs {
		if ab.Active {
			keys = append(keys, ab.Key)
		}
	}
	return keys, nil
}

func (c *SheetsClient) hasV2Schema(ctx context.Context) bool {
	if err := c.loadSheets(ctx, []string{v2ABsSheet}); err != nil {
		return false
	}
	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	return err == nil && len(v2ABs(abGrid)) > 0
}

func (c *SheetsClient) emptyGradeResultForV2(ctx context.Context, exam string, user SessionUser) GradeResult {
	result := emptyGradeResult(exam, user)
	if err := c.loadSheets(ctx, []string{v2ABsSheet}); err != nil {
		return result
	}
	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	if err != nil {
		return result
	}
	if ab, found := v2ResolveAB(abGrid, exam); found {
		result.Exam = ab.Label
		result.SchemaStatus = abGrid.schemaStatus
		result.SpreadsheetID = abGrid.spreadsheetID
	}
	return result
}

func (c *SheetsClient) gradeForV2(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	if err := c.loadSheets(ctx, []string{v2ABsSheet}); err != nil {
		return GradeResult{}, err
	}

	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	if err != nil {
		return GradeResult{}, err
	}
	ab, found := v2ResolveAB(abGrid, exam)
	if !found {
		return GradeResult{}, NewHTTPError(400, "avaliacao invalida")
	}
	exam = ab.Key
	abLabel, active := ab.Label, ab.Active
	if !active {
		result := emptyGradeResult(exam, user)
		result.Exam = abLabel
		result.SchemaStatus = abGrid.schemaStatus
		result.SpreadsheetID = abGrid.spreadsheetID
		return result, nil
	}
	if abLabel == "" {
		abLabel = strings.ToUpper(exam)
	}
	emptyActiveResult := func() GradeResult {
		result := emptyGradeResult(exam, user)
		result.Exam = abLabel
		result.SchemaStatus = abGrid.schemaStatus
		result.SpreadsheetID = abGrid.spreadsheetID
		return result
	}

	if err := c.loadSheets(ctx, []string{v2ActivitiesSheet, v2SummarySheetName(exam)}); err != nil {
		if isNotFound(err) || canFallbackToLegacy(err) {
			return emptyActiveResult(), nil
		}
		return GradeResult{}, err
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
		return emptyActiveResult(), nil
	}
	summaryRow := summaryGrid.rows[summaryRowIdx]
	groupValue := valueAt(summaryRow, groupColumn(summaryGrid.headers))

	activities := v2ActivitiesForAB(activitiesGrid, exam)
	v2BindSummaryColumns(summaryGrid.headers, activities)
	if len(activities) == 0 {
		return emptyActiveResult(), nil
	}

	var activitySheets []string
	for _, activity := range activities {
		if v2ActivityLaunched(summaryRow, activity) {
			activitySheets = append(activitySheets, activity.SheetName)
		}
	}
	if len(activitySheets) > 0 {
		if err := c.loadSheets(ctx, activitySheets); err != nil {
			if !canFallbackToLegacy(err) {
				return GradeResult{}, err
			}
		}
	}

	result := emptyGradeResult(exam, user)
	result.Exam = abLabel
	result.SchemaStatus = mergeSchemaStatus(mergeSchemaStatus(abGrid.schemaStatus, activitiesGrid.schemaStatus), summaryGrid.schemaStatus)
	result.SpreadsheetID = mergeSourceValue(mergeSourceValue(abGrid.spreadsheetID, activitiesGrid.spreadsheetID), summaryGrid.spreadsheetID)

	for _, activity := range activities {
		if !v2ActivityLaunched(summaryRow, activity) {
			continue
		}
		table, found, err := c.v2ActivityTable(ctx, activity, summaryRow, groupValue, user)
		if err != nil {
			if canFallbackToLegacy(err) {
				continue
			}
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
		return result, nil
	}
	return result, nil
}

func v2SummarySheetName(exam string) string {
	return "nota " + normalizeABKey(exam)
}

func v2ABState(grid *sheetGrid, exam string) (string, bool) {
	ab, found := v2ResolveAB(grid, exam)
	if !found {
		return strings.ToUpper(strings.TrimSpace(exam)), false
	}
	return ab.Label, ab.Active
}

type v2ABConfig struct {
	Key    string
	Label  string
	Active bool
}

func v2ResolveAB(grid *sheetGrid, exam string) (v2ABConfig, bool) {
	abs := v2ABs(grid)
	if len(abs) == 0 {
		return v2ABConfig{}, false
	}
	candidates := v2ABRouteCandidates(exam)
	if len(candidates) == 0 {
		return abs[0], true
	}
	for _, candidate := range candidates {
		for _, ab := range abs {
			if ab.Key == candidate {
				return ab, true
			}
		}
	}
	return v2ABConfig{}, false
}

func v2ABs(grid *sheetGrid) []v2ABConfig {
	abIdx := firstHeaderIndex(grid.headers, "ab", "avaliacao", "avaliacao bimestral")
	activeIdx := firstHeaderIndex(grid.headers, "ativo", "ativa", "status", "liberado")
	labelIdx := firstHeaderIndex(grid.headers, "nome", "label", "rotulo", "titulo")
	if abIdx < 0 {
		abIdx = 0
	}
	if activeIdx < 0 {
		activeIdx = inferABStatusColumn(grid, abIdx)
	}
	abs := make([]v2ABConfig, 0, len(grid.rows))
	seen := map[string]bool{}
	for _, row := range grid.rows {
		ab := v2ABFromRow(row, abIdx, labelIdx, activeIdx)
		if ab.Key == "" || seen[ab.Key] {
			continue
		}
		seen[ab.Key] = true
		abs = append(abs, ab)
	}
	return abs
}

func inferABStatusColumn(grid *sheetGrid, abIdx int) int {
	limit := len(grid.headers)
	for _, row := range grid.rows {
		if len(row) > limit {
			limit = len(row)
		}
	}
	for colIdx := 0; colIdx < limit; colIdx++ {
		if colIdx == abIdx {
			continue
		}
		hasStatusValue := false
		allStatusValues := true
		for _, row := range grid.rows {
			value := valueAt(row, colIdx)
			if value == "" {
				continue
			}
			if !abStatusLikeValue(value) {
				allStatusValues = false
				break
			}
			hasStatusValue = true
		}
		if hasStatusValue && allStatusValues {
			return colIdx
		}
	}
	return -1
}

func abStatusLikeValue(value string) bool {
	normalized := normalizeHeader(value)
	return normalized == "0" ||
		normalized == "1" ||
		normalized == "sim" ||
		normalized == "s" ||
		normalized == "nao" ||
		normalized == "não" ||
		normalized == "n" ||
		normalized == "true" ||
		normalized == "false" ||
		normalized == "ativo" ||
		normalized == "ativa" ||
		normalized == "inativo" ||
		normalized == "inativa" ||
		normalized == "lancada" ||
		normalized == "lançada"
}

func v2ABFromRow(row []string, abIdx int, labelIdx int, activeIdx int) v2ABConfig {
	key := normalizeABKey(valueAt(row, abIdx))
	if key == "" {
		return v2ABConfig{}
	}
	label := valueAt(row, labelIdx)
	if label == "" {
		label = valueAt(row, abIdx)
	}
	if label == "" {
		label = strings.ToUpper(key)
	}
	return v2ABConfig{Key: key, Label: label, Active: activeIdx < 0 || activeABValue(valueAt(row, activeIdx))}
}

func v2ABRouteCandidates(exam string) []string {
	seen := map[string]bool{}
	var candidates []string
	add := func(value string) {
		key := normalizeABKey(value)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, key)
	}
	add(exam)
	for _, value := range strings.FieldsFunc(exam, func(r rune) bool {
		return r == '|' || r == ',' || r == ';' || r == '/' || r == '\\'
	}) {
		add(value)
	}
	return candidates
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
		if abIdx >= 0 && normalizeABKey(valueAt(row, abIdx)) != normalizeABKey(exam) {
			continue
		}
		if activeIdx >= 0 && !activeActivityValue(valueAt(row, activeIdx)) {
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
		if canFallbackToLegacy(err) {
			return v2ActivitySummaryTable(activity, summaryRow), true, nil
		}
		return TableResult{}, false, err
	}
	rowIdx := v2ActivityRow(grid, groupValue, user)
	if rowIdx < 0 {
		return v2ActivitySummaryTable(activity, summaryRow), true, nil
	}

	maxRowIdx := findMaxRow(grid.rows)
	items := v2ActivityItems(grid, maxRowIdx, rowIdx, activity.Weight)
	if len(items) == 0 {
		return v2ActivitySummaryTable(activity, summaryRow), true, nil
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

func v2ActivitySummaryTable(activity v2ActivityConfig, summaryRow []string) TableResult {
	score := valueAt(summaryRow, activity.SummaryCol)
	card := makeCard("nota", "Nota", score, "", "", nil)
	card.DisplayValue = formatScoreForWeight(score, activity.Weight)
	return TableResult{
		Key:           activity.Key,
		Label:         activity.Label,
		SheetName:     activity.SheetName,
		Kind:          "activity",
		Complete:      !isPendingValue(score),
		Status:        v2ActivityStatusFromScore(score),
		SchemaStatus:  activity.SchemaStatus,
		SpreadsheetID: activity.SpreadsheetID,
		Cards:         []CardResult{card},
	}
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
	criterionColumns := v2CriterionColumns(grid, maxRowIdx, studentRowIdx)
	totalMax := v2TotalMaximum(grid, maxRowIdx, criterionColumns)
	for _, colIdx := range criterionColumns {
		maximum := v2CriterionMaximum(grid, maxRowIdx, colIdx)
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

func v2CriterionColumns(grid *sheetGrid, maxRowIdx int, studentRowIdx int) []int {
	var columns []int
	for colIdx := 0; colIdx < len(grid.headers); colIdx++ {
		if !shouldShowV2Criterion(valueAt(grid.headers, colIdx)) {
			continue
		}
		value := valueAt(grid.rows[studentRowIdx], colIdx)
		comment := noteAt(rowNotesAt(grid, studentRowIdx), colIdx)
		if v2CriterionMaximum(grid, maxRowIdx, colIdx) <= 0 && value == "" && comment == "" {
			continue
		}
		columns = append(columns, colIdx)
	}
	return columns
}

func v2TotalMaximum(grid *sheetGrid, maxRowIdx int, columns []int) float64 {
	total := 0.0
	for _, colIdx := range columns {
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
	if maximum := inferMaxForLabel(valueAt(grid.headers, colIdx)); maximum > 0 {
		return maximum
	}
	return 1
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
		normalized == "0" ||
		normalized == "1" ||
		normalized == "sim" ||
		normalized == "s" ||
		normalized == "true" ||
		normalized == "ativo" ||
		normalized == "ativa" ||
		normalized == "lancada" ||
		normalized == "lançada"
}

func activeActivityValue(value string) bool {
	normalized := normalizeHeader(value)
	if normalized == "" {
		return true
	}
	return normalized == "1" ||
		normalized == "sim" ||
		normalized == "s" ||
		normalized == "true" ||
		normalized == "ativo" ||
		normalized == "ativa" ||
		normalized == "lancada" ||
		normalized == "lançada"
}

func activeABValue(value string) bool {
	normalized := normalizeHeader(value)
	return normalized == "1" ||
		normalized == "sim" ||
		normalized == "s" ||
		normalized == "true" ||
		normalized == "ativo" ||
		normalized == "ativa" ||
		normalized == "lancada" ||
		normalized == "lançada"
}

func v2ActivityStatusFromScore(value string) string {
	if isPendingValue(value) {
		return "Não encerrado"
	}
	return "Encerrado"
}

func normalizeABKey(value string) string {
	normalized := normalizeHeader(value)
	var builder strings.Builder
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
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
