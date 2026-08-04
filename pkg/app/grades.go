package app

import (
	"context"
	"errors"
	"strings"
	"sync"
)

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
		result, err := scoped.gradeForV2(ctx, exam, candidateUser)
		if err == nil {
			if result.Active != nil || hasTables(result) {
				return result, nil
			}
			emptyResult = &result
			lastErr = nil
			continue
		}
		lastErr = err
		if !canFallbackToNextBase(err) {
			return GradeResult{}, err
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
		results, err := scoped.gradesForRuntimeV2(ctx, exams, candidateUser)
		if err == nil {
			if hasV2GradeState(results) || hasAnyGradeTables(results) {
				return results, nil
			}
			emptyResults = results
			lastErr = nil
			continue
		}
		lastErr = err
		if !canFallbackToNextBase(err) {
			return nil, err
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

// gradesForRuntimeV2 busca cada prova em paralelo e nunca descarta o resultado
// de uma prova por causa de erro em outra: falhas pontuais (ex. limite de
// requisicoes do Google) ficam registradas em GradeResult.Error daquela prova.
func (c *SheetsClient) gradesForRuntimeV2(ctx context.Context, exams []string, user SessionUser) (GradeResults, error) {
	exams = normalizedExams(exams)
	if len(exams) == 0 || isDefaultExamPair(exams) {
		resolved, err := c.v2ExamKeys(ctx)
		if err != nil {
			return nil, err
		}
		exams = resolved
	}

	type examOutcome struct {
		exam   string
		result GradeResult
	}

	outcomes := make(chan examOutcome, len(exams))
	var wg sync.WaitGroup
	for _, exam := range exams {
		wg.Add(1)
		go func(exam string) {
			defer wg.Done()
			outcomes <- examOutcome{exam: exam, result: c.gradeOrErrorForV2(ctx, exam, user)}
		}(exam)
	}
	wg.Wait()
	close(outcomes)

	results := make(GradeResults, len(exams))
	for outcome := range outcomes {
		results[outcome.exam] = outcome.result
	}
	return results, nil
}

// gradeOrErrorForV2 nunca retorna erro: uma falha de leitura vira um
// GradeResult vazio com Error preenchido, para nao derrubar as outras provas
// buscadas em paralelo por gradesForRuntimeV2.
func (c *SheetsClient) gradeOrErrorForV2(ctx context.Context, exam string, user SessionUser) GradeResult {
	result, err := c.gradeForV2(ctx, exam, user)
	if err == nil {
		return result
	}
	if isNotFound(err) {
		return c.emptyGradeResultForV2(ctx, exam, user)
	}
	result = emptyGradeResult(exam, user)
	result.Error = err.Error()
	return result
}

func isDefaultExamPair(exams []string) bool {
	return len(exams) == 2 && canonicalExamKey(exams[0]) == "ab1" && canonicalExamKey(exams[1]) == "ab2"
}

func canonicalExamKey(exam string) string {
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

func hasV2GradeState(results GradeResults) bool {
	for _, result := range results {
		if result.Active != nil {
			return true
		}
	}
	return false
}

// isTolerableReadError identifica respostas do Google Sheets que o v2 trata
// como "sem dado disponivel" (ex. aba/avaliacao ainda nao existe) em vez de
// como falha grave.
func isTolerableReadError(err error) bool {
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

func isServiceUnavailable(err error) bool {
	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.Status == 503
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
	addAll(c.cfg.SpreadsheetIDs)
	return ids
}

func (c *SheetsClient) userForSpreadsheet(ctx context.Context, user SessionUser) (SessionUser, bool, error) {
	if c.trustsSessionSpreadsheet(user) {
		return user, true, nil
	}
	identity, err := c.LoginIdentity(ctx, user.Matricula)
	if err != nil {
		if user.hasResolvedIdentity() && isServiceUnavailable(err) {
			return user, true, nil
		}
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

func (user SessionUser) hasResolvedIdentity() bool {
	return strings.TrimSpace(user.Matricula) != "" && strings.TrimSpace(user.Name) != ""
}

func (c *SheetsClient) scopedToSpreadsheet(spreadsheetID string) *SheetsClient {
	spreadsheetID = strings.TrimSpace(spreadsheetID)
	if spreadsheetID == "" {
		return c
	}
	cfg := c.cfg
	cfg.SpreadsheetID = spreadsheetID
	cfg.SpreadsheetIDs = []string{spreadsheetID}
	return &SheetsClient{
		cfg:        cfg,
		service:    c.service,
		httpClient: c.httpClient,
		cacheOwner: c.cacheRuntime(),
	}
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
