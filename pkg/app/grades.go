package app

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

func (c *SheetsClient) GradeFor(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	tables, err := c.tablesForExam(exam)
	if err != nil {
		return GradeResult{}, err
	}
	result := GradeResult{Exam: strings.ToUpper(strings.TrimSpace(exam)), Matricula: user.Matricula, Name: user.Name}

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
		}
	}
	if len(result.Tables) == 0 {
		return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+strings.ToUpper(strings.TrimSpace(exam)))
	}
	return result, nil
}

func (c *SheetsClient) gradeTableFor(ctx context.Context, table TableConfig, user SessionUser) (TableResult, bool, error) {
	grid, err := c.loadSheet(ctx, table.SheetName)
	if err != nil {
		return TableResult{}, false, err
	}

	switch table.Kind {
	case "activity":
		return parseActivityRubric(grid, table, user)
	case "summary", "ab2summary", "project":
		return parseStudentTable(grid, table, user)
	default:
		return TableResult{}, false, NewHTTPError(500, "tipo de tabela desconhecido: "+table.Kind)
	}
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
		columns := make([]ColumnResult, 0, len(grid.headers))
		for colIdx, header := range grid.headers {
			if strings.TrimSpace(header) == "" || !includeColumn(table.Kind, header) {
				continue
			}
			comment := noteAt(grid.notes, colIdx)
			if rowIdx < len(grid.rowNotes) && noteAt(grid.rowNotes[rowIdx], colIdx) != "" {
				comment = noteAt(grid.rowNotes[rowIdx], colIdx)
			}
			columns = append(columns, ColumnResult{
				Key:     fmt.Sprintf("c%d", colIdx),
				Label:   header,
				Value:   valueAt(row, colIdx),
				Comment: comment,
			})
		}
		return TableResult{
			Key:       table.Key,
			Label:     table.Label,
			SheetName: table.SheetName,
			Kind:      table.Kind,
			Complete:  tableComplete(grid, table),
			Columns:   columns,
		}, true, nil
	}
	return TableResult{}, false, nil
}

func (c *SheetsClient) tablesForExam(exam string) ([]TableConfig, error) {
	switch strings.ToLower(strings.TrimSpace(exam)) {
	case "ab1":
		return c.cfg.AB1Tables, nil
	case "ab2":
		return c.cfg.AB2Tables, nil
	default:
		return nil, NewHTTPError(400, "avaliacao invalida")
	}
}
