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
)

type v2ActivityConfig struct {
	Key           string
	Label         string
	AB            string
	SheetName     string
	Weight        float64
	HasWeight     bool
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
	abs, err := v2ABs(abGrid)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(abs))
	for _, ab := range abs {
		if !ab.Active {
			continue
		}
		keys = append(keys, ab.Key)
	}
	return keys, nil
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
	if ab, found, err := v2ResolveAB(abGrid, exam); err == nil && found {
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
	ab, found, err := v2ResolveAB(abGrid, exam)
	if err != nil {
		return GradeResult{}, err
	}
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
		if isNotFound(err) || isTolerableReadError(err) {
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

	activities, err := v2ActivitiesForAB(activitiesGrid, exam)
	if err != nil {
		return GradeResult{}, err
	}
	if len(activities) == 0 {
		return emptyActiveResult(), nil
	}
	v2BindSummaryColumns(summaryGrid.headers, activities)

	var activitySheets []string
	for _, activity := range activities {
		activitySheets = append(activitySheets, activity.SheetName)
	}
	if len(activitySheets) > 0 {
		if err := c.loadSheets(ctx, activitySheets); err != nil {
			if !isTolerableReadError(err) {
				return GradeResult{}, err
			}
		}
	}

	result := emptyGradeResult(exam, user)
	result.Exam = abLabel
	result.Active = v2GradeActive(true)
	result.SchemaStatus = schemaStatusV2
	result.SpreadsheetID = mergeSourceValue(mergeSourceValue(abGrid.spreadsheetID, activitiesGrid.spreadsheetID), summaryGrid.spreadsheetID)

	for _, activity := range activities {
		table, found, err := c.v2ActivityTable(ctx, activity, summaryGrid, summaryRowIdx, groupValue, user)
		if err != nil {
			if isTolerableReadError(err) {
				continue
			}
			return GradeResult{}, err
		}
		if !found {
			continue
		}
		result.Tables = append(result.Tables, table)
		result.SpreadsheetID = mergeSourceValue(result.SpreadsheetID, table.SpreadsheetID)
	}

	if v2ActivitiesComplete(activities, result.Tables) {
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

// V2ConfiguredSheetNames descobre, a partir das abas de controle (abs e
// atividades), quais abas o modelo v2 usa hoje - util para ferramentas de
// diagnostico (ex. cmd/comments) que precisavam antes de uma lista fixa
// vinda de AB1Tables/AB2Tables, que nao existe mais.
func (c *SheetsClient) V2ConfiguredSheetNames(ctx context.Context) ([]string, error) {
	if err := c.loadSheets(ctx, []string{v2ABsSheet, v2ActivitiesSheet}); err != nil {
		return nil, err
	}
	abGrid, err := c.loadSheet(ctx, v2ABsSheet)
	if err != nil {
		return nil, err
	}
	abs, err := v2ABs(abGrid)
	if err != nil {
		return nil, err
	}
	activitiesGrid, err := c.loadSheet(ctx, v2ActivitiesSheet)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{v2ABsSheet: true, v2ActivitiesSheet: true}
	names := []string{v2ABsSheet, v2ActivitiesSheet}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, ab := range abs {
		if !ab.Active {
			continue
		}
		add(v2SummarySheetName(ab.Key))
		activities, err := v2ActivitiesForAB(activitiesGrid, ab.Key)
		if err != nil {
			continue
		}
		for _, activity := range activities {
			add(activity.SheetName)
		}
	}
	return names, nil
}

func v2SummarySheetName(exam string) string {
	return "nota " + normalizeABKey(exam)
}

func v2ABState(grid *sheetGrid, exam string) (string, bool) {
	ab, found, err := v2ResolveAB(grid, exam)
	if err != nil || !found {
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

func v2ResolveAB(grid *sheetGrid, exam string) (v2ABConfig, bool, error) {
	abs, err := v2ABs(grid)
	if err != nil {
		return v2ABConfig{}, false, err
	}
	if len(abs) == 0 {
		return v2ABConfig{}, false, nil
	}
	candidates := v2ABRouteCandidates(exam)
	if len(candidates) == 0 {
		return abs[0], true, nil
	}
	for _, candidate := range candidates {
		for _, ab := range abs {
			if ab.Key == candidate {
				return ab, true, nil
			}
		}
	}
	return v2ABConfig{}, false, nil
}

// v2ABs le a aba "abs". As colunas "ab"/"avaliacao" e "ativo"/"status" sao
// obrigatorias: sem elas nao ha como saber quais avaliacoes existem ou se
// estao liberadas, entao um cabecalho ausente vira erro claro em vez de
// adivinhar (ex. usar a primeira coluna, ou varrer linhas atras de 0/1).
func v2ABs(grid *sheetGrid) ([]v2ABConfig, error) {
	abIdx, ok := requiredHeaderIndex(grid.headers, "ab", "avaliacao", "avaliacao bimestral")
	if !ok {
		return nil, MissingHeaderError(v2ABsSheet, "ab", "avaliacao")
	}
	activeIdx, ok := requiredHeaderIndex(grid.headers, "ativo", "ativa", "status", "liberado")
	if !ok {
		return nil, MissingHeaderError(v2ABsSheet, "ativo", "status")
	}
	labelIdx := firstHeaderIndex(grid.headers, "nome", "label", "rotulo", "titulo")

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
	return abs, nil
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

// v2ActivitiesForAB le a aba "atividades". As colunas "ab"/"avaliacao" e
// "atividade"/"nome" sao obrigatorias - sem a primeira uma atividade
// vazaria para todas as avaliacoes ao mesmo tempo, sem a segunda nao ha como
// identificar a atividade. A coluna "aba"/"sheet"/"planilha" (onde ficam as
// notas) e a coluna "ativo"/"status" (se a atividade ja foi lancada)
// continuam opcionais de proposito: a primeira tem uma convencao valida de
// atalho (nome da atividade = nome da aba) e a segunda e um recurso
// intencionalmente opcional, nao uma adivinhacao de cabecalho.
func v2ActivitiesForAB(grid *sheetGrid, exam string) ([]v2ActivityConfig, error) {
	abIdx, ok := requiredHeaderIndex(grid.headers, "ab", "avaliacao", "avaliacao bimestral")
	if !ok {
		return nil, MissingHeaderError(v2ActivitiesSheet, "ab", "avaliacao")
	}
	nameIdx, ok := requiredHeaderIndex(grid.headers, "atividade", "nome", "nome da atividade", "titulo")
	if !ok {
		return nil, MissingHeaderError(v2ActivitiesSheet, "atividade", "nome")
	}
	sheetIdx := firstHeaderIndex(grid.headers, "aba", "sheet", "planilha")
	weightIdx := firstHeaderIndex(grid.headers, "pesomaximo", "peso maximo", "peso máximo", "peso", "nota maxima")
	activeIdx := firstHeaderIndex(grid.headers, "ativo", "ativa", "status", "lancada", "lançada")

	var activities []v2ActivityConfig
	for rowIdx, row := range grid.rows {
		if normalizeABKey(valueAt(row, abIdx)) != normalizeABKey(exam) {
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
		hasWeight := ok && weight > 0
		if !hasWeight {
			weight = 0
		}
		activities = append(activities, v2ActivityConfig{
			Key:           fmt.Sprintf("v2-at-%d", rowIdx+1),
			Label:         humanizeLabel(label),
			AB:            strings.ToUpper(exam),
			SheetName:     sheetName,
			Weight:        weight,
			HasWeight:     hasWeight,
			Order:         rowIdx,
			SchemaStatus:  grid.schemaStatus,
			SpreadsheetID: grid.spreadsheetID,
		})
	}
	sort.SliceStable(activities, func(i, j int) bool {
		return activities[i].Order < activities[j].Order
	})
	return activities, nil
}

// v2BindSummaryColumns liga cada atividade com peso a coluna correspondente
// na aba de notas ("nota <ab>"). Uma atividade sem coluna correspondente
// ainda (ex. registrada em "atividades" mas sem nota lancada na planilha
// resumo) fica sem SummaryCol e e renderizada como pendente - isso e um
// estado valido do dia a dia, nao um erro de configuracao, entao
// permanece tolerante (nao faz parte do escopo de "cabecalho obrigatorio":
// aqui estamos comparando o nome de uma atividade com colunas de texto
// livre, nao validando um cabecalho fixo esperado).
func v2BindSummaryColumns(headers []string, activities []v2ActivityConfig) {
	allowFinalGradeFallback := len(activities) == 1
	for idx := range activities {
		activities[idx].SummaryCol = -1
		if !activities[idx].HasWeight {
			continue
		}
		activities[idx].SummaryCol = matchingHeaderIndex(headers, activities[idx].Label, activities[idx].SheetName)
		if activities[idx].SummaryCol < 0 && allowFinalGradeFallback {
			activities[idx].SummaryCol = v2FinalGradeColumn(headers)
		}
	}
}

func v2FinalGradeColumn(headers []string) int {
	return firstHeaderIndex(headers, "nota final", "nota", "total")
}

func (c *SheetsClient) v2ActivityTable(ctx context.Context, activity v2ActivityConfig, summaryGrid *sheetGrid, summaryRowIdx int, groupValue string, user SessionUser) (TableResult, bool, error) {
	summaryRow := summaryGrid.rows[summaryRowIdx]
	grid, err := c.loadSheet(ctx, activity.SheetName)
	if err != nil {
		if isTolerableReadError(err) {
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
	if !activity.HasWeight {
		return TableResult{
			Key:           activity.Key,
			Label:         activity.Label,
			SheetName:     activity.SheetName,
			Kind:          "activity",
			Complete:      false,
			Scoreless:     true,
			Status:        "Não pontua",
			SchemaStatus:  schemaStatusV2,
			SpreadsheetID: mergeSourceValue(activity.SpreadsheetID, grid.spreadsheetID),
			Cards: []CardResult{{
				Key:     "criterios",
				Label:   "Critérios avaliados",
				Details: percentageActivityDetails(items),
			}},
		}, true, nil
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
		SchemaStatus:  schemaStatusV2,
		SpreadsheetID: mergeSourceValue(activity.SpreadsheetID, grid.spreadsheetID),
		Cards:         []CardResult{card},
	}, true, nil
}

func v2SummaryActivityComment(summaryGrid *sheetGrid, summaryRowIdx int, activity v2ActivityConfig) (string, string) {
	return commentAt(rowNotesAt(summaryGrid, summaryRowIdx), rowNoteAuthorsAt(summaryGrid, summaryRowIdx), activity.SummaryCol)
}

func v2ActivitySummaryTable(activity v2ActivityConfig, summaryGrid *sheetGrid, summaryRowIdx int) TableResult {
	if !activity.HasWeight {
		return TableResult{
			Key:           activity.Key,
			Label:         activity.Label,
			SheetName:     activity.SheetName,
			Kind:          "activity",
			Complete:      false,
			Scoreless:     true,
			Status:        "Não pontua",
			SchemaStatus:  activity.SchemaStatus,
			SpreadsheetID: activity.SpreadsheetID,
			Cards: []CardResult{{
				Key:   "criterios",
				Label: "Critérios avaliados",
			}},
		}
	}
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
		Complete:      status == "Encerrado",
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
	// A ordem das colunas na planilha nem sempre segue a ordem numerica das
	// questoes/critérios - reordena para exibir sempre Questão 1, 2, 3...
	// em sequência, independente de como a aba foi organizada.
	sort.SliceStable(items, func(i, j int) bool {
		return compareDetailLabels(items[i].Subtopic, items[j].Subtopic) < 0
	})
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

// firstHeaderIndex faz correspondencia exata e, se nao achar, aproximada por
// substring - reservado para cabecalhos genuinamente opcionais (onde nao
// existir e um estado valido, nao um erro de configuracao).
func firstHeaderIndex(headers []string, candidates ...string) int {
	return headerIndex(headers, false, candidates...)
}

// matchingHeaderIndex compara dois textos livres (rotulo da atividade x
// cabecalho da planilha de notas) e nao um cabecalho contra uma lista fixa de
// nomes aceitos - por isso usa aproximacao nos dois sentidos.
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

// requiredHeaderIndex so aceita correspondencia exata com um dos nomes
// aceitos - usado para os cabecalhos que o app agora exige (ver v2ABs /
// v2ActivitiesForAB). Sem aproximacao por substring: um cabecalho "parecido"
// mas escrito diferente deve falhar de forma clara, nao ser adivinhado.
func requiredHeaderIndex(headers []string, candidates ...string) (int, bool) {
	for _, label := range candidates {
		wanted := normalizeHeader(label)
		for idx, header := range headers {
			if normalizeHeader(header) == wanted {
				return idx, true
			}
		}
	}
	return -1, false
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

func v2ActivitiesComplete(activities []v2ActivityConfig, tables []TableResult) bool {
	if len(activities) == 0 {
		return false
	}
	tablesByKey := make(map[string]TableResult, len(tables))
	for _, table := range tables {
		tablesByKey[table.Key] = table
	}
	hasWeightedActivity := false
	for _, activity := range activities {
		if !activity.HasWeight {
			continue
		}
		hasWeightedActivity = true
		table, found := tablesByKey[activity.Key]
		if !found || !activityTableComplete(table) {
			return false
		}
	}
	return hasWeightedActivity
}

func activityTableComplete(table TableResult) bool {
	if status := normalizeHeader(table.Status); status != "" {
		return status == "encerrado"
	}
	return table.Complete
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

// findMaxRow, isMaxRowLabel, findStudentRow, rubricLabel e activityStatus
// vieram do antigo activity_parser.go (parser legado): o parser em si foi
// removido junto com o modelo de planilha legado, mas essas cinco funcoes
// continuam em uso pelo modelo v2 (rubricas de atividade e localizacao da
// linha do aluno).

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

func activityStatus(items []activityItem) string {
	for _, item := range items {
		if strings.TrimSpace(item.NotaAlcancada) == "" {
			return "Não encerrado"
		}
	}
	return "Encerrado"
}
