package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type ColumnResult struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"`
}

type GradeResult struct {
	Exam      string        `json:"exam"`
	Matricula string        `json:"matricula"`
	Name      string        `json:"name"`
	Tables    []TableResult `json:"tables"`
}

type TableResult struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	SheetName string         `json:"sheetName"`
	Columns   []ColumnResult `json:"columns"`
}

type SheetsClient struct {
	cfg     Config
	service *sheets.Service
	mu      sync.Mutex
	cache   map[string]cachedGrid
}

type cachedGrid struct {
	expires time.Time
	grid    *sheetGrid
}

type sheetGrid struct {
	headers []string
	notes   []string
	rows    [][]string
}

type LoginIdentity struct {
	Matricula string `json:"matricula"`
	Name      string `json:"name"`
}

func NewSheetsClient(ctx context.Context, cfg Config) (*SheetsClient, error) {
	svc, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(cfg.ServiceJSON)), option.WithScopes(sheets.SpreadsheetsReadonlyScope))
	if err != nil {
		return nil, err
	}
	return &SheetsClient{cfg: cfg, service: svc, cache: map[string]cachedGrid{}}, nil
}

func (c *SheetsClient) MatriculaExists(ctx context.Context, matricula string) (bool, error) {
	_, err := c.LoginIdentity(ctx, matricula)
	if err != nil {
		if httpErr, ok := err.(HTTPError); ok && httpErr.Status == 401 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *SheetsClient) LoginIdentity(ctx context.Context, matricula string) (LoginIdentity, error) {
	grid, err := c.loadSheet(ctx, c.cfg.LoginSheet)
	if err != nil {
		return LoginIdentity{}, err
	}
	matriculaIdx := matriculaColumn(grid.headers)
	nameIdx := nameColumn(grid.headers)
	if matriculaIdx < 0 {
		return LoginIdentity{}, NewHTTPError(500, "coluna de matricula nao encontrada na base de dados")
	}
	if nameIdx < 0 {
		return LoginIdentity{}, NewHTTPError(500, "coluna de nome nao encontrada na base de dados")
	}
	for _, row := range grid.rows {
		if matriculaIdx < len(row) && normalizeID(row[matriculaIdx]) == normalizeID(matricula) {
			name := valueAt(row, nameIdx)
			if strings.TrimSpace(name) == "" {
				return LoginIdentity{}, NewHTTPError(401, "matricula sem nome vinculado")
			}
			return LoginIdentity{Matricula: valueAt(row, matriculaIdx), Name: name}, nil
		}
	}
	return LoginIdentity{}, NewHTTPError(401, "matricula nao autorizada")
}

func (c *SheetsClient) GradeFor(ctx context.Context, exam string, user SessionUser) (GradeResult, error) {
	tables, err := c.tablesForExam(exam)
	if err != nil {
		return GradeResult{}, err
	}
	result := GradeResult{Exam: strings.ToUpper(strings.TrimSpace(exam)), Matricula: user.Matricula, Name: user.Name}
	for _, table := range tables {
		if strings.TrimSpace(table.SheetName) == "" {
			continue
		}
		tableResult, found, err := c.gradeTableFor(ctx, table, user)
		if err != nil {
			return GradeResult{}, err
		}
		if found {
			result.Tables = append(result.Tables, tableResult)
		}
	}
	if len(result.Tables) == 0 {
		return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+strings.ToUpper(strings.TrimSpace(exam)))
	}
	return result, nil
}

func (c *SheetsClient) gradeTableFor(ctx context.Context, table TableConfig, user SessionUser) (TableResult, bool, error) {
	sheetName := table.SheetName
	grid, err := c.loadSheet(ctx, sheetName)
	if err != nil {
		return TableResult{}, false, err
	}
	idx := nameColumn(grid.headers)
	searchValue := user.Name
	matchByName := true
	if idx < 0 {
		idx = matriculaColumn(grid.headers)
		searchValue = user.Matricula
		matchByName = false
	}
	if idx < 0 {
		return TableResult{}, false, NewHTTPError(500, "coluna de nome ou matricula nao encontrada na aba "+sheetName)
	}
	for _, row := range grid.rows {
		if idx >= len(row) || !sameLookupValue(row[idx], searchValue, matchByName) {
			continue
		}
		columns := make([]ColumnResult, 0, len(grid.headers))
		for colIdx, header := range grid.headers {
			if strings.TrimSpace(header) == "" {
				continue
			}
			value := ""
			if colIdx < len(row) {
				value = row[colIdx]
			}
			comment := ""
			if colIdx < len(grid.notes) {
				comment = grid.notes[colIdx]
			}
			columns = append(columns, ColumnResult{
				Key:     fmt.Sprintf("c%d", colIdx),
				Label:   header,
				Value:   value,
				Comment: comment,
			})
		}
		return TableResult{Key: table.Key, Label: table.Label, SheetName: sheetName, Columns: columns}, true, nil
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

func (c *SheetsClient) loadSheet(ctx context.Context, sheetName string) (*sheetGrid, error) {
	c.mu.Lock()
	if cached, ok := c.cache[sheetName]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.grid, nil
	}
	c.mu.Unlock()

	resp, err := c.service.Spreadsheets.Get(c.cfg.SpreadsheetID).
		IncludeGridData(true).
		Ranges("'" + strings.ReplaceAll(sheetName, "'", "''") + "'").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	if len(resp.Sheets) == 0 || len(resp.Sheets[0].Data) == 0 {
		return nil, NewHTTPError(404, "aba nao encontrada: "+sheetName)
	}
	grid := parseGrid(resp.Sheets[0].Data[0].RowData)
	c.mu.Lock()
	c.cache[sheetName] = cachedGrid{expires: time.Now().Add(c.cfg.CacheTTL), grid: grid}
	c.mu.Unlock()
	return grid, nil
}

func parseGrid(rows []*sheets.RowData) *sheetGrid {
	grid := &sheetGrid{}
	for _, row := range rows {
		values := make([]string, len(row.Values))
		notes := make([]string, len(row.Values))
		for idx, cell := range row.Values {
			values[idx] = cellText(cell)
			notes[idx] = strings.TrimSpace(cell.Note)
		}
		if grid.headers == nil && hasAny(values) {
			grid.headers = values
			grid.notes = notes
			continue
		}
		if grid.headers != nil && hasAny(values) {
			grid.rows = append(grid.rows, values)
		}
	}
	return grid
}

func cellText(cell *sheets.CellData) string {
	if cell == nil {
		return ""
	}
	if cell.FormattedValue != "" {
		return strings.TrimSpace(cell.FormattedValue)
	}
	if cell.UserEnteredValue == nil {
		return ""
	}
	if cell.UserEnteredValue.StringValue != nil {
		return strings.TrimSpace(*cell.UserEnteredValue.StringValue)
	}
	if cell.UserEnteredValue.NumberValue != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", *cell.UserEnteredValue.NumberValue))
	}
	if cell.UserEnteredValue.BoolValue != nil {
		if *cell.UserEnteredValue.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}

func hasAny(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func matriculaColumn(headers []string) int {
	candidates := []string{"matricula", "matrícula", "mat", "registro", "ra"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	for idx, header := range headers {
		if strings.Contains(normalizeHeader(header), "matric") {
			return idx
		}
	}
	return -1
}

func nameColumn(headers []string) int {
	candidates := []string{"nome", "aluno", "estudante", "discente", "nome completo", "nome do aluno"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if strings.Contains(normalized, "nome") || strings.Contains(normalized, "aluno") {
			return idx
		}
	}
	return -1
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return replacer.Replace(value)
}

func valueAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func normalizeID(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func normalizePerson(value string) string {
	fields := strings.Fields(normalizeHeader(value))
	return strings.Join(fields, " ")
}

func sameLookupValue(left string, right string, person bool) bool {
	if person {
		return normalizePerson(left) == normalizePerson(right)
	}
	return normalizeID(left) == normalizeID(right)
}
