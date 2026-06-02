package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type tableParser func(*sheetGrid, TableConfig, SessionUser) (TableResult, bool, error)

var tableParsers = map[string]tableParser{
	"activity":   parseActivityRubric,
	"summary":    parseStudentTable,
	"ab2summary": parseStudentTable,
	"project":    parseStudentTable,
}

func (c *SheetsClient) GradeFor(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	var lastErr error
	var emptyResult *GradeResult
	for _, spreadsheetID := range c.candidateSpreadsheetIDs(user) {
		scoped := c.scopedToSpreadsheet(spreadsheetID)
		candidateUser, ok, err := scoped.userForSpreadsheet(ctx, user)
		if err != nil {
			lastErr = err
			continue
		}
		if !ok {
			continue
		}
		result, err := scoped.gradeForConfiguredRuntime(ctx, exam, candidateUser)
		if err == nil {
			if hasTables(result) {
				return result, nil
			}
			emptyResult = &result
			lastErr = nil
			continue
		}
		if err != nil {
			lastErr = err
			if !canFallbackToNextBase(err) {
				return GradeResult{}, err
			}
		}
	}
	if lastErr != nil {
		return GradeResult{}, lastErr
	}
	if emptyResult != nil {
		return *emptyResult, nil
	}
	return GradeResult{}, NewHTTPError(404, "matricula nao encontrada")
}

func (c *SheetsClient) gradeForConfiguredRuntime(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	switch runtimeForUser(c.cfg, user) {
	case "v2":
		result, err := c.gradeForV2(ctx, exam, user)
		if err == nil && hasTables(result) {
			return result, nil
		}
		if err != nil && !canFallbackToLegacy(err) {
			return GradeResult{}, err
		}
		legacy, legacyErr := c.gradeForLegacy(ctx, exam, user)
		if legacyErr == nil {
			return legacy, nil
		}
		if err != nil {
			return GradeResult{}, err
		}
		return result, nil
	case "legacy":
		if c.detectsSpreadsheetSchema() && c.hasV2Schema(ctx) {
			return c.gradeForV2(ctx, exam, user)
		}
		return c.gradeForLegacy(ctx, exam, user)
	}
	if c.detectsSpreadsheetSchema() {
		result, err := c.gradeForV2(ctx, exam, user)
		if err == nil && hasTables(result) {
			return result, nil
		}
		if err == nil && c.hasV2Schema(ctx) {
			return result, nil
		}
		if err != nil && c.hasV2Schema(ctx) {
			if isNotFound(err) {
				return c.emptyGradeResultForV2(ctx, exam, user), nil
			}
			return GradeResult{}, err
		}
		if err != nil && !canFallbackToLegacy(err) {
			return GradeResult{}, err
		}
	}
	return c.gradeForLegacy(ctx, exam, user)
}

func (c *SheetsClient) gradeForLegacy(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	tables, err := c.tablesForExam(exam)
	if err != nil {
		return GradeResult{}, err
	}
	if err := c.loadSheets(ctx, sheetNamesForTables(tables)); err != nil {
		return GradeResult{}, err
	}
	return c.gradeForTables(ctx, exam, tables, user)
}

func (c *SheetsClient) GradesFor(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	var lastErr error
	var emptyResults GradeResults
	for _, spreadsheetID := range c.candidateSpreadsheetIDs(user) {
		scoped := c.scopedToSpreadsheet(spreadsheetID)
		candidateUser, ok, err := scoped.userForSpreadsheet(ctx, user)
		if err != nil {
			lastErr = err
			continue
		}
		if !ok {
			continue
		}
		results, err := scoped.gradesForConfiguredRuntime(ctx, exams, candidateUser)
		if err == nil {
			if hasAnyGradeTables(results) {
				return results, nil
			}
			emptyResults = results
			lastErr = nil
			continue
		}
		if err != nil {
			lastErr = err
			if !canFallbackToNextBase(err) {
				return nil, err
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if emptyResults != nil {
		return emptyResults, nil
	}
	return nil, NewHTTPError(404, "matricula nao encontrada")
}

func (c *SheetsClient) gradesForConfiguredRuntime(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	switch runtimeForUser(c.cfg, user) {
	case "v2":
		results, err := c.gradesForRuntimeV2(ctx, exams, user)
		if err == nil && hasAnyGradeTables(results) {
			return results, nil
		}
		if err != nil && !canFallbackToLegacy(err) {
			return nil, err
		}
		legacy, legacyErr := c.gradesForLegacy(ctx, exams, user)
		if legacyErr == nil {
			return legacy, nil
		}
		if err != nil {
			return nil, err
		}
		return results, nil
	case "legacy":
		if c.detectsSpreadsheetSchema() && c.hasV2Schema(ctx) {
			return c.gradesForRuntimeV2(ctx, exams, user)
		}
		return c.gradesForLegacy(ctx, exams, user)
	}
	if c.detectsSpreadsheetSchema() {
		return c.gradesForRuntimeV2(ctx, exams, user)
	}
	return c.gradesForLegacy(ctx, exams, user)
}

func (c *SheetsClient) gradesForRuntimeV2(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	strictV2 := runtimeForUser(c.cfg, user) == "v2"
	schemaV2 := strictV2 || c.hasV2Schema(ctx)
	exams = normalizedExams(exams)
	if len(exams) == 0 || defaultLegacyExams(exams) {
		resolved, err := c.v2ExamKeys(ctx)
		if err != nil {
			if schemaV2 || !canFallbackToLegacy(err) {
				return nil, err
			}
		} else {
			exams = resolved
			schemaV2 = true
		}
	}
	results := make(GradeResults, len(exams))
	for _, exam := range exams {
		result, err := c.gradeForV2(ctx, exam, user)
		if err == nil && hasTables(result) {
			results[exam] = result
			continue
		}
		if err == nil && schemaV2 {
			results[exam] = result
			continue
		}
		if err != nil && !canFallbackToLegacy(err) {
			return nil, err
		}
		if err != nil && schemaV2 {
			if isNotFound(err) {
				results[exam] = c.emptyGradeResultForV2(ctx, exam, user)
				continue
			}
			return nil, err
		}
		result, err = c.gradeForLegacy(ctx, exam, user)
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

func defaultLegacyExams(exams []string) bool {
	return len(exams) == 2 && legacyExamKey(exams[0]) == "ab1" && legacyExamKey(exams[1]) == "ab2"
}

func runtimeForUser(cfg Config, user SessionUser) string {
	cfgRuntime := strings.ToLower(strings.TrimSpace(cfg.RuntimeVersion))
	switch strings.ToLower(strings.TrimSpace(user.SchemaStatus)) {
	case "v2":
		return "v2"
	case "legacy":
		if cfgRuntime == "auto" {
			return ""
		}
		return "legacy"
	}
	if cfgRuntime == "v1" || cfgRuntime == "legacy" {
		return "legacy"
	}
	if cfgRuntime == "v2" {
		return "v2"
	}
	return ""
}

func (c *SheetsClient) gradesForLegacy(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	exams = normalizedExams(exams)
	tablesByExam := make(map[string][]TableConfig, len(exams))
	var allSheetNames []string
	for _, exam := range exams {
		tables, err := c.tablesForExam(exam)
		if err != nil {
			return nil, err
		}
		tablesByExam[exam] = tables
		allSheetNames = append(allSheetNames, sheetNamesForTables(tables)...)
	}
	if err := c.loadSheets(ctx, allSheetNames); err != nil {
		return nil, err
	}

	results := make(GradeResults, len(exams))
	for _, exam := range exams {
		result, err := c.gradeForTables(ctx, exam, tablesByExam[exam], user)
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

func hasTables(result GradeResult) bool {
	return len(result.Tables) > 0
}

func hasAnyGradeTables(results GradeResults) bool {
	for _, result := range results {
		if hasTables(result) {
			return true
		}
	}
	return false
}

func canFallbackToLegacy(err error) bool {
	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.Status == 404 || httpErr.Status == 400
}

func canFallbackToNextBase(err error) bool {
	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.Status == 401 || httpErr.Status == 404 || httpErr.Status == 400
}

func (c *SheetsClient) candidateSpreadsheetIDs(user SessionUser) []string {
	var ids []string
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		ids = append(ids, value)
	}
	addAll := func(values []string) {
		for _, spreadsheetID := range values {
			add(spreadsheetID)
		}
	}
	add(user.SpreadsheetID)
	addAll(c.cfg.LegacySpreadsheetIDs)
	addAll(c.cfg.V2SpreadsheetIDs)
	addAll(c.cfg.SpreadsheetIDs)
	return ids
}

func (c *SheetsClient) userForSpreadsheet(ctx context.Context, user SessionUser) (SessionUser, bool, error) {
	if c.trustsSessionSpreadsheet(user) {
		return user, true, nil
	}
	identity, err := c.LoginIdentity(ctx, user.Matricula)
	if err != nil {
		if canFallbackToNextBase(err) {
			return SessionUser{}, false, nil
		}
		return SessionUser{}, false, err
	}
	return SessionUser{Matricula: identity.Matricula, Name: identity.Name, SpreadsheetID: identity.SpreadsheetID, SchemaStatus: identity.SchemaStatus}, true, nil
}

func (c *SheetsClient) trustsSessionSpreadsheet(user SessionUser) bool {
	spreadsheetID := strings.TrimSpace(user.SpreadsheetID)
	if spreadsheetID == "" || strings.TrimSpace(user.Matricula) == "" {
		return false
	}
	return len(c.cfg.SpreadsheetIDs) == 1 && c.cfg.SpreadsheetIDs[0] == spreadsheetID
}

func (c *SheetsClient) scopedToSpreadsheet(spreadsheetID string) *SheetsClient {
	spreadsheetID = strings.TrimSpace(spreadsheetID)
	if spreadsheetID == "" {
		return c
	}
	cfg := c.cfg
	cfg.SpreadsheetID = spreadsheetID
	cfg.SpreadsheetIDs = []string{spreadsheetID}
	cfg.RuntimeVersion = c.runtimeForSpreadsheet(spreadsheetID)
	return &SheetsClient{
		cfg:        cfg,
		service:    c.service,
		httpClient: c.httpClient,
		cacheOwner: c.cacheRuntime(),
	}
}

func (c *SheetsClient) runtimeForSpreadsheet(spreadsheetID string) string {
	if containsString(c.cfg.V2SpreadsheetIDs, spreadsheetID) {
		return "v2"
	}
	if containsString(c.cfg.LegacySpreadsheetIDs, spreadsheetID) {
		return "legacy"
	}
	return c.cfg.RuntimeVersion
}

func (c *SheetsClient) gradeForTables(ctx context.Context, exam string, tables []TableConfig, user SessionUser) (GradeResult, error) {
	result := emptyGradeResult(exam, user)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type tableResponse struct {
		idx    int
		result TableResult
		found  bool
	}

	responses := make([]tableResponse, len(tables))
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	maxWorkers := runtime.GOMAXPROCS(0)
	if maxWorkers < 1 {
		maxWorkers = 4
	}
	sem := make(chan struct{}, maxWorkers)

	for idx, table := range tables {
		if strings.TrimSpace(table.SheetName) == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, table TableConfig) {
			defer wg.Done()
			defer func() { <-sem }()
			tableResult, found, err := c.gradeTableFor(ctx, table, user)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
				return
			}
			responses[idx] = tableResponse{idx: idx, result: tableResult, found: found}
		}(idx, table)
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return GradeResult{}, err
	default:
	}

	for _, response := range responses {
		if response.found {
			result.Tables = append(result.Tables, response.result)
			result.SpreadsheetID = mergeSourceValue(result.SpreadsheetID, response.result.SpreadsheetID)
			result.SchemaStatus = mergeSchemaStatus(result.SchemaStatus, response.result.SchemaStatus)
		}
	}
	addScoreAverages(&result)
	if len(result.Tables) == 0 {
		return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+strings.ToUpper(strings.TrimSpace(exam)))
	}
	return result, nil
}

func normalizedExams(exams []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(exams))
	for _, exam := range exams {
		exam = strings.ToLower(strings.TrimSpace(exam))
		if exam == "" || seen[exam] {
			continue
		}
		seen[exam] = true
		result = append(result, exam)
	}
	return result
}

func emptyGradeResult(exam string, user SessionUser) GradeResult {
	return GradeResult{Exam: strings.ToUpper(strings.TrimSpace(exam)), Matricula: user.Matricula, Name: user.Name, Tables: []TableResult{}}
}

func isNotFound(err error) bool {
	var httpErr HTTPError
	return err != nil && errors.As(err, &httpErr) && httpErr.Status == 404
}

func (c *SheetsClient) gradeTableFor(ctx context.Context, table TableConfig, user SessionUser) (TableResult, bool, error) {
	grid, err := c.loadSheet(ctx, table.SheetName)
	if err != nil {
		return TableResult{}, false, err
	}

	parser, ok := tableParsers[table.Kind]
	if !ok {
		return TableResult{}, false, NewHTTPError(500, "tipo de tabela desconhecido: "+table.Kind)
	}
	result, found, err := parser(grid, table, user)
	if err != nil || !found {
		return result, found, err
	}
	result.SpreadsheetID = grid.spreadsheetID
	result.SchemaStatus = grid.schemaStatus
	return result, true, nil
}

func parseStudentTable(grid *sheetGrid, table TableConfig, user SessionUser) (TableResult, bool, error) {
	nameIdx := nameColumn(grid.headers)
	matriculaIdx := matriculaColumn(grid.headers)
	if nameIdx < 0 && matriculaIdx < 0 {
		return TableResult{}, false, NewHTTPError(500, "coluna de nome ou matricula nao encontrada na aba "+table.SheetName)
	}
	for rowIdx, row := range grid.rows {
		if !matchesUser(row, nameIdx, matriculaIdx, user) {
			continue
		}
		rowComment, rowCommentAuthor := rowIdentityComment(grid, rowIdx)
		if excludesStudentFromGrades(rowComment) {
			return TableResult{}, false, nil
		}
		cells := studentCellsForRow(grid, rowIdx, row, table.Kind)
		cards := cardsForStudentCells(table, cells)
		applyRowCommentToCards(cards, rowComment, rowCommentAuthor)
		return TableResult{
			Key:       table.Key,
			Label:     table.Label,
			SheetName: table.SheetName,
			Kind:      table.Kind,
			Complete:  tableComplete(grid, table),
			Cards:     cards,
		}, true, nil
	}
	return TableResult{}, false, nil
}

func studentCellsForRow(grid *sheetGrid, rowIdx int, row []string, tableKind string) []studentCell {
	cells := make([]studentCell, 0, len(grid.headers))
	for colIdx, header := range grid.headers {
		if strings.TrimSpace(header) == "" || !includeColumn(tableKind, header) || !shouldShowColumn(header) {
			continue
		}
		comment, author := studentCellComment(grid, rowIdx, colIdx, tableKind)
		cells = append(cells, studentCell{
			Key:           fmt.Sprintf("c%d", colIdx),
			Header:        header,
			Label:         cardLabel(header),
			Value:         valueAt(row, colIdx),
			Comment:       comment,
			CommentAuthor: author,
		})
	}
	return cells
}

func studentCellComment(grid *sheetGrid, rowIdx int, colIdx int, tableKind string) (string, string) {
	if comment, author := commentAt(rowNotesAt(grid, rowIdx), rowNoteAuthorsAt(grid, rowIdx), colIdx); comment != "" {
		return comment, author
	}
	if tableKind == "project" {
		return commentAt(grid.notes, grid.noteAuthors, colIdx)
	}
	return "", ""
}

func rowNotesAt(grid *sheetGrid, rowIdx int) []string {
	if grid == nil || rowIdx < 0 || rowIdx >= len(grid.rowNotes) {
		return nil
	}
	return grid.rowNotes[rowIdx]
}

func rowNoteAuthorsAt(grid *sheetGrid, rowIdx int) []string {
	if grid == nil || rowIdx < 0 || rowIdx >= len(grid.rowNoteAuthors) {
		return nil
	}
	return grid.rowNoteAuthors[rowIdx]
}

func applyRowCommentToCards(cards []CardResult, comment string, author string) {
	if strings.TrimSpace(comment) == "" {
		return
	}
	for idx := range cards {
		if strings.TrimSpace(cards[idx].Comment) != "" {
			continue
		}
		cards[idx].Comment = comment
		cards[idx].CommentAuthor = author
	}
}

func sheetNamesForTables(tables []TableConfig) []string {
	names := make([]string, 0, len(tables))
	seen := map[string]bool{}
	for _, table := range tables {
		name := strings.TrimSpace(table.SheetName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func cardsForStudentCells(table TableConfig, cells []studentCell) []CardResult {
	if table.Kind == "summary" || table.Kind == "ab2summary" {
		cards := make([]CardResult, 0, len(cells))
		for _, cell := range cells {
			if !shouldShowSummaryCard(cell) {
				continue
			}
			if isAverageColumn(cell.Header) && isPendingValue(cell.Value) {
				continue
			}
			cards = append(cards, makeCard(cell.Key, summaryCardLabel(cell.Header), cell.Value, cell.Comment, cell.CommentAuthor, nil))
		}
		sort.SliceStable(cards, func(i, j int) bool {
			return summaryCardOrder(cards[i].Label) < summaryCardOrder(cards[j].Label)
		})
		return cards
	}
	if table.Kind == "project" {
		return projectCards(table, cells)
	}

	details := columnDetails(cells)
	cards := make([]CardResult, 0, len(cells))
	for _, cell := range cells {
		if !shouldShowMainCard(cell.Header) {
			continue
		}
		cards = append(cards, makeCard(cell.Key, cell.Label, cell.Value, "", "", details))
	}
	if len(cards) == 0 && len(details) > 0 {
		cards = append(cards, makeCard("detalhes", "Detalhes", "", "", "", details))
	}
	if len(cards) == 0 {
		cards = fallbackCards(cells)
	}
	return cards
}

func projectCards(table TableConfig, cells []studentCell) []CardResult {
	details := projectDetails(cells)
	for _, cell := range cells {
		if projectMainColumn(cell.Header) && hasVisibleCellData(cell) {
			return []CardResult{makeCard(cell.Key, cardLabel(cell.Header), cell.Value, cell.Comment, cell.CommentAuthor, details)}
		}
	}
	if len(details) > 0 {
		return []CardResult{makeCard("projeto", table.Label, "", "", "", details)}
	}
	return nil
}

func shouldShowSummaryCard(cell studentCell) bool {
	return shouldShowColumn(cell.Header) &&
		hasVisibleCellData(cell) &&
		(isProofColumn(cell.Header) || isAverageColumn(cell.Header))
}

func fallbackCards(cells []studentCell) []CardResult {
	cards := make([]CardResult, 0, len(cells))
	for _, cell := range cells {
		if !shouldShowColumn(cell.Header) || !hasVisibleCellData(cell) {
			continue
		}
		cards = append(cards, makeCard(cell.Key, cardLabel(cell.Header), cell.Value, cell.Comment, cell.CommentAuthor, nil))
	}
	return cards
}

func hasVisibleCellData(cell studentCell) bool {
	return strings.TrimSpace(cell.Value) != ""
}

func summaryCardOrder(label string) int {
	normalized := normalizeHeader(label)
	switch {
	case strings.Contains(normalized, "prova"):
		return 0
	case strings.Contains(normalized, "somatorio"):
		return 1
	case strings.Contains(normalized, "media"):
		return 2
	case strings.Contains(normalized, "at."):
		return 3
	default:
		return 4
	}
}

type scoreAverageRule struct {
	exam      string
	key       string
	label     string
	kind      string
	calculate func([]TableResult) (float64, bool)
}

var (
	ab1AverageRule = scoreAverageRule{
		exam:      "AB1",
		key:       "media-ab1",
		label:     "Média AB1",
		kind:      "ab1summary",
		calculate: ab1AverageScore,
	}
	ab2AverageRule = scoreAverageRule{
		exam:      "AB2",
		key:       "media-ab2",
		label:     "Média AB2",
		kind:      "ab2summary",
		calculate: ab2AverageScore,
	}
)

func addScoreAverages(result *GradeResult) {
	addScoreAverage(result, ab1AverageRule)
	addScoreAverage(result, ab2AverageRule)
}

func addAB1ScoreAverage(result *GradeResult) {
	addScoreAverage(result, ab1AverageRule)
}

func addAB2ScoreAverage(result *GradeResult) {
	addScoreAverage(result, ab2AverageRule)
}

func addScoreAverage(result *GradeResult, rule scoreAverageRule) {
	if strings.ToUpper(strings.TrimSpace(result.Exam)) != rule.exam || hasAverageTable(result.Tables, rule) {
		return
	}
	total, ok := rule.calculate(result.Tables)
	if !ok {
		return
	}
	result.Tables = append(result.Tables, averageTable(rule, capScore(total)))
}

func hasAverageTable(tables []TableResult, rule scoreAverageRule) bool {
	for _, table := range tables {
		if table.Key == rule.key || table.Kind == rule.kind {
			return true
		}
	}
	return false
}

func averageTable(rule scoreAverageRule, total float64) TableResult {
	return TableResult{
		Key:       rule.key,
		Label:     rule.label,
		SheetName: rule.label,
		Kind:      rule.kind,
		Complete:  true,
		Cards: []CardResult{
			makeCard(rule.key, "", formatScore(total), "", "", nil),
		},
	}
}

func capScore(score float64) float64 {
	if score > 10 {
		return 10
	}
	return score
}

func ab1AverageScore(tables []TableResult) (float64, bool) {
	if total, ok := firstTableScore(tables, summaryTable, somatorioCard); ok {
		return total, true
	}

	activityTotal, hasActivity := sumTableScores(tables, ab1ActivityTable, ab1MainScoreCard)
	proofScore, hasProof := firstTableScore(tables, summaryTable, proofCard)
	total := activityTotal
	if hasProof {
		total += proofScore
	}
	return total, hasActivity || hasProof
}

func ab1MainScoreCard(card CardResult) bool {
	label := normalizeHeader(card.Label)
	return label == "nota" ||
		label == "total" ||
		label == "somatorio-ab" ||
		strings.Contains(label, "somatório ab") ||
		strings.Contains(label, "prova") ||
		strings.Contains(label, "atividade") ||
		strings.HasPrefix(label, "at.")
}

func ab2AverageScore(tables []TableResult) (float64, bool) {
	return sumTableScores(tables, ab2ScoredTable, ab2MainScoreCard)
}

func ab2MainScoreCard(card CardResult) bool {
	label := normalizeHeader(card.Label)
	return label == "nota" ||
		label == "total" ||
		strings.Contains(label, "projeto") ||
		strings.Contains(label, "atividade") ||
		strings.HasPrefix(label, "at.")
}

func firstTableScore(tables []TableResult, includeTable func(TableResult) bool, includeCard func(CardResult) bool) (float64, bool) {
	for _, table := range tables {
		if !includeTable(table) {
			continue
		}
		if score, ok := firstCardScore(table.Cards, includeCard); ok {
			return score, true
		}
	}
	return 0, false
}

func sumTableScores(tables []TableResult, includeTable func(TableResult) bool, includeCard func(CardResult) bool) (float64, bool) {
	total := 0.0
	hasAny := false
	for _, table := range tables {
		if !includeTable(table) {
			continue
		}
		score, ok := firstCardScore(table.Cards, includeCard)
		if !ok {
			continue
		}
		total += score
		hasAny = true
	}
	return total, hasAny
}

func firstCardScore(cards []CardResult, includeCard func(CardResult) bool) (float64, bool) {
	for _, card := range cards {
		if !includeCard(card) {
			continue
		}
		score, ok := parseScore(card.Value)
		if ok {
			return score, true
		}
	}
	return 0, false
}

func summaryTable(table TableResult) bool {
	return table.Kind == "summary"
}

func ab1ActivityTable(table TableResult) bool {
	return table.Kind != "summary" && table.Kind != "ab1summary" && table.Kind != "ab2summary"
}

func ab2ScoredTable(table TableResult) bool {
	return table.Kind != "summary" && table.Kind != "ab2summary"
}

func somatorioCard(card CardResult) bool {
	return strings.Contains(normalizeHeader(card.Label), "somatorio")
}

func proofCard(card CardResult) bool {
	return strings.Contains(normalizeHeader(card.Label), "prova")
}

func (c *SheetsClient) tablesForExam(exam string) ([]TableConfig, error) {
	switch legacyExamKey(exam) {
	case "ab1":
		return c.cfg.AB1Tables, nil
	case "ab2":
		return c.cfg.AB2Tables, nil
	default:
		return nil, NewHTTPError(400, "avaliacao invalida")
	}
}

func legacyExamKey(exam string) string {
	exam = strings.ToLower(strings.TrimSpace(exam))
	if exam == "ab1" || exam == "ab2" {
		return exam
	}
	ab1Idx := strings.Index(exam, "ab1")
	ab2Idx := strings.Index(exam, "ab2")
	switch {
	case ab1Idx >= 0 && (ab2Idx < 0 || ab1Idx < ab2Idx):
		return "ab1"
	case ab2Idx >= 0:
		return "ab2"
	default:
		return exam
	}
}
