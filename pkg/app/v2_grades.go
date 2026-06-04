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
		resolved, err := c.v2ExamKeys(ctx)
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

func (c *SheetsClient) v2ExamKeys(ctx context.Context) ([]string, error) {
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
		if !ab.Active {
			continue
		}
		keys = append(keys, ab.Key)
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
		result.Active = v2GradeActive(ab.Active)
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
		result.Active = v2GradeActive(false)
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
		result.Active = v2GradeActive(true)
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
	result.Active = v2GradeActive(true)
	result.SchemaStatus = mergeSchemaStatus(mergeSchemaStatus(abGrid.schemaStatus, activitiesGrid.schemaStatus), summaryGrid.schemaStatus)
	result.SpreadsheetID = mergeSourceValue(mergeSourceValue(abGrid.spreadsheetID, activitiesGrid.spreadsheetID), summaryGrid.spreadsheetID)

	for _, activity := range activities {
		if !v2ActivityLaunched(summaryRow, activity) {
			continue
		}
		table, found, err := c.v2ActivityTable(ctx, activity, summaryGrid, summaryRowIdx, groupValue, user)
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

	if v2ActivitiesComplete(activities, summaryRow, result.Tables) {
		if average := v2AverageCard(summaryGrid, summaryRow); average != nil {
			result.Tables = append(result.Tables, TableResult{
				Key:           "media-" + exam,
				Label:         "Média",
				SheetName:     v2SummarySheetName(exam),
				Kind:          exam + "summary",
				Complete:      true,
				SchemaStatus:  summaryGrid.schemaStatus,
				SpreadsheetID: summaryGrid.spreadsheetID,
				Cards:         []CardResult{*average},
			})
		}
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

func v2GradeActive(active bool) *bool {
	return &active
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
	return normalized == "0" || normalized == "1"
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
	return v2ABConfig{Key: key, Label: label, Active: activeIdx >= 0 && activeABStatusValue(valueAt(row, activeIdx))}
}

func activeABStatusValue(value string) bool {
	return normalizeHeader(value) == "1"
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
		if activeIdx >= 0 && !activeSpreadsheetValue(valueAt(row, activeIdx), false) {
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
	allowFinalGradeFallback := len(activities) == 1
	for idx := range activities {
		activities[idx].SummaryCol = matchingHeaderIndex(headers, activities[idx].Label, activities[idx].SheetName)
		if activities[idx].SummaryCol < 0 && allowFinalGradeFallback {
			activities[idx].SummaryCol = v2FinalGradeColumn(headers)
		}
	}
}

func v2ActivityLaunched(summaryRow []string, activity v2ActivityConfig) bool {
	if activity.SummaryCol < 0 {
		return false
	}
	value := valueAt(summaryRow, activity.SummaryCol)
	return normalizeHeader(value) != "" && !strings.Contains(normalizeHeader(value), v2NotLaunchedText)
}

func (c *SheetsClient) v2ActivityTable(ctx context.Context, activity v2ActivityConfig, summaryGrid *sheetGrid, summaryRowIdx int, groupValue string, user SessionUser) (TableResult, bool, error) {
	summaryRow := summaryGrid.rows[summaryRowIdx]
	grid, err := c.loadSheet(ctx, activity.SheetName)
	if err != nil {
		if canFallbackToLegacy(err) {
			return v2ActivitySummaryTable(activity, summaryGrid, summaryRowIdx), true, nil
		}
		return TableResult{}, false, err
	}
	rowIdx := v2ActivityRow(grid, groupValue, user)

	maxRowIdx := findMaxRow(grid.rows)
	items := v2ActivityItems(grid, maxRowIdx, rowIdx, activity.Weight)
	if len(items) == 0 {
		return v2ActivitySummaryTable(activity, summaryGrid, summaryRowIdx), true, nil
	}
	details := activityDetails(items)
	score := valueAt(summaryRow, activity.SummaryCol)
	comment, author := v2SummaryActivityComment(summaryGrid, summaryRowIdx, activity)
	card := makeCard("nota", "Nota", score, comment, author, details)
	card.DisplayValue = formatScoreForWeight(score, activity.Weight)
	card.Tone = scoreToneForMaximum(score, activity.Weight)
	status := v2ActivityStatus(items, score)
	card.Tone = activityCardTone(status, card.Tone)
	return TableResult{
		Key:           activity.Key,
		Label:         activity.Label,
		SheetName:     activity.SheetName,
		Kind:          "activity",
		Complete:      status == "Encerrado",
		Status:        status,
		SchemaStatus:  mergeSchemaStatus(activity.SchemaStatus, grid.schemaStatus),
		SpreadsheetID: mergeSourceValue(activity.SpreadsheetID, grid.spreadsheetID),
		Cards:         []CardResult{card},
	}, true, nil
}

func v2SummaryActivityComment(summaryGrid *sheetGrid, summaryRowIdx int, activity v2ActivityConfig) (string, string) {
	return commentAt(rowNotesAt(summaryGrid, summaryRowIdx), rowNoteAuthorsAt(summaryGrid, summaryRowIdx), activity.SummaryCol)
}

func v2ActivitySummaryTable(activity v2ActivityConfig, summaryGrid *sheetGrid, summaryRowIdx int) TableResult {
	summaryRow := summaryGrid.rows[summaryRowIdx]
	score := valueAt(summaryRow, activity.SummaryCol)
	comment, author := v2SummaryActivityComment(summaryGrid, summaryRowIdx, activity)
	card := makeCard("nota", "Nota", score, comment, author, nil)
	card.DisplayValue = formatScoreForWeight(score, activity.Weight)
	status := v2ActivityStatusFromScore(score)
	card.Tone = activityCardTone(status, scoreToneForMaximum(score, activity.Weight))
	return TableResult{
		Key:           activity.Key,
		Label:         activity.Label,
		SheetName:     activity.SheetName,
		Kind:          "activity",
		Complete:      !isPendingValue(score),
		Status:        status,
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
	useOfficialWeights := v2UsesOfficialQuestionWeights(grid, maxRowIdx, criterionColumns)
	sourceMaxima := make(map[int]float64, len(criterionColumns))
	maxima := make(map[int]float64, len(criterionColumns))
	totalMaximum := 0.0
	for _, colIdx := range criterionColumns {
		sourceMaximum := v2CriterionSourceMaximum(grid, maxRowIdx, colIdx)
		maximum := sourceMaximum
		if useOfficialWeights {
			maximum = v2OfficialCriterionMaximum(grid, maxRowIdx, colIdx, maximum)
		}
		sourceMaxima[colIdx] = sourceMaximum
		maxima[colIdx] = maximum
		totalMaximum += maximum
	}
	if useOfficialWeights {
		totalMaximum = officialQuestionRubricMaximum
	}
	for _, colIdx := range criterionColumns {
		sourceMaximum := sourceMaxima[colIdx]
		maximum := maxima[colIdx]
		value := ""
		if studentRowIdx >= 0 && studentRowIdx < len(grid.rows) {
			value = valueAt(grid.rows[studentRowIdx], colIdx)
		}
		if sourceMaximum > 0 && maximum > 0 {
			value = normalizedScore(value, sourceMaximum, maximum)
		}
		if totalMaximum > 0 && weight > 0 {
			value = normalizedScore(value, totalMaximum, weight)
			maximum = normalizedMaximum(maximum, totalMaximum, weight)
		}
		comment, author := v2ActivityItemComment(grid, maxRowIdx, studentRowIdx, colIdx)
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

func v2UsesOfficialQuestionWeights(grid *sheetGrid, maxRowIdx int, columns []int) bool {
	labels := make([]string, 0, len(columns)*2)
	for _, colIdx := range columns {
		labels = append(labels, v2CriterionWeightLabels(grid, maxRowIdx, colIdx)...)
	}
	return usesOfficialQuestionWeights(labels)
}

func v2OfficialCriterionMaximum(grid *sheetGrid, maxRowIdx int, colIdx int, fallback float64) float64 {
	for _, label := range v2CriterionWeightLabels(grid, maxRowIdx, colIdx) {
		if maximum := inferMaxForLabel(label); maximum > 0 {
			return maximum
		}
	}
	return fallback
}

func v2CriterionWeightLabels(grid *sheetGrid, maxRowIdx int, colIdx int) []string {
	labels := []string{valueAt(grid.headers, colIdx)}
	if maxRowIdx > 0 {
		labels = append(labels, valueAt(grid.rows[maxRowIdx-1], colIdx))
	}
	return labels
}

func v2CriterionColumns(grid *sheetGrid, maxRowIdx int, studentRowIdx int) []int {
	var columns []int
	for colIdx := 0; colIdx < len(grid.headers); colIdx++ {
		if !shouldShowV2Criterion(valueAt(grid.headers, colIdx)) {
			continue
		}
		value := ""
		if studentRowIdx >= 0 && studentRowIdx < len(grid.rows) {
			value = valueAt(grid.rows[studentRowIdx], colIdx)
		}
		comment, _ := v2ActivityItemComment(grid, maxRowIdx, studentRowIdx, colIdx)
		if v2CriterionSourceMaximum(grid, maxRowIdx, colIdx) <= 0 && value == "" && comment == "" {
			continue
		}
		columns = append(columns, colIdx)
	}
	return columns
}

func v2FinalGradeColumn(headers []string) int {
	return firstHeaderIndex(headers, "nota final", "nota", "total")
}

func v2CriterionSourceMaximum(grid *sheetGrid, maxRowIdx int, colIdx int) float64 {
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
		normalized != "nota final" &&
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
	value := valueAt(row, idx)
	if isPendingValue(value) {
		return nil
	}
	score, ok := parseScore(value)
	if !ok {
		return nil
	}
	if score > 10 {
		value = formatScore(10)
	}
	card := makeCard("media", "Média", value, comment, author, nil)
	card.Tone = scoreToneForMaximum(value, 10)
	return &card
}

func firstHeaderIndex(headers []string, candidates ...string) int {
	return headerIndex(headers, false, candidates...)
}

func matchingHeaderIndex(headers []string, labels ...string) int {
	return headerIndex(headers, true, labels...)
}

func headerIndex(headers []string, bidirectionalContains bool, labels ...string) int {
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
		if len([]rune(wanted)) <= 2 {
			continue
		}
		for idx, header := range headers {
			normalized := normalizeHeader(header)
			if strings.Contains(normalized, wanted) || (bidirectionalContains && strings.Contains(wanted, normalized)) {
				return idx
			}
		}
	}
	return -1
}

func activeSpreadsheetValue(value string, blankAllowed bool) bool {
	normalized := normalizeHeader(value)
	if normalized == "" {
		return blankAllowed
	}
	return normalized == "1"
}

func v2ActivityItemComment(grid *sheetGrid, maxRowIdx int, studentRowIdx int, colIdx int) (string, string) {
	return commentAt(rowNotesAt(grid, studentRowIdx), rowNoteAuthorsAt(grid, studentRowIdx), colIdx)
}

func v2ActivityStatusFromScore(value string) string {
	if isPendingValue(value) {
		return "Não encerrado"
	}
	return "Encerrado"
}

func v2ActivityStatus(items []activityItem, score string) string {
	if isPendingValue(score) || activityStatus(items) != "Encerrado" {
		return "Não encerrado"
	}
	return "Encerrado"
}

func v2ActivitiesComplete(activities []v2ActivityConfig, summaryRow []string, tables []TableResult) bool {
	if len(activities) == 0 {
		return false
	}
	tablesByKey := make(map[string]TableResult, len(tables))
	for _, table := range tables {
		tablesByKey[table.Key] = table
	}
	for _, activity := range activities {
		table, found := tablesByKey[activity.Key]
		if !v2ActivityLaunched(summaryRow, activity) || !found || !activityTableComplete(table) {
			return false
		}
	}
	return true
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
		return scoreComparisonDisplay(parsed, weight)
	}
	return displayValue("Nota", value)
}

func scoreToneForMaximum(value string, maximum float64) string {
	if isPendingValue(value) {
		return "score-pending"
	}
	score, ok := parseScore(value)
	if !ok || maximum <= 0 {
		return scoreTone("Nota", value)
	}
	return scoreToneFromRatio((score/maximum)*100, false)
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
