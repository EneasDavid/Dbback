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
	Exam      string         `json:"exam"`
	SheetName string         `json:"sheetName"`
	Matricula string         `json:"matricula"`
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

func NewSheetsClient(ctx context.Context, cfg Config) (*SheetsClient, error) {
	svc, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(cfg.ServiceJSON)), option.WithScopes(sheets.SpreadsheetsReadonlyScope))
	if err != nil {
		return nil, err
	}
	return &SheetsClient{cfg: cfg, service: svc, cache: map[string]cachedGrid{}}, nil
}

func (c *SheetsClient) MatriculaExists(ctx context.Context, matricula string) (bool, error) {
	grid, err := c.loadSheet(ctx, c.cfg.LoginSheet)
	if err != nil {
		return false, err
	}
	idx := matriculaColumn(grid.headers)
	if idx < 0 {
		return false, NewHTTPError(500, "coluna de matricula nao encontrada na base de dados")
	}
	for _, row := range grid.rows {
		if idx < len(row) && normalizeID(row[idx]) == normalizeID(matricula) {
			return true, nil
		}
	}
	return false, nil
}

func (c *SheetsClient) GradeFor(ctx context.Context, exam string, matricula string) (GradeResult, error) {
	sheetName, err := c.sheetForExam(exam)
	if err != nil {
		return GradeResult{}, err
	}
	grid, err := c.loadSheet(ctx, sheetName)
	if err != nil {
		return GradeResult{}, err
	}
	idx := matriculaColumn(grid.headers)
	if idx < 0 {
		return GradeResult{}, NewHTTPError(500, "coluna de matricula nao encontrada na aba "+sheetName)
	}
	for _, row := range grid.rows {
		if idx >= len(row) || normalizeID(row[idx]) != normalizeID(matricula) {
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
		return GradeResult{Exam: strings.ToUpper(exam), SheetName: sheetName, Matricula: matricula, Columns: columns}, nil
	}
	return GradeResult{}, NewHTTPError(404, "matricula nao encontrada em "+sheetName)
}

func (c *SheetsClient) sheetForExam(exam string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(exam)) {
	case "ab1":
		return c.cfg.AB1Sheet, nil
	case "ab2":
		return c.cfg.AB2Sheet, nil
	default:
		return "", NewHTTPError(400, "avaliacao invalida")
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

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return replacer.Replace(value)
}

func normalizeID(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}
