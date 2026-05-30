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
	Kind      string         `json:"kind"`
	Complete  bool           `json:"complete"`
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
	headers  []string
	notes    []string
	rows     [][]string
	rowNotes [][]string
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
	if table.Kind == "activity" {
		result := activityTableFor(grid, table, user)
		if result.Columns != nil && strings.EqualFold(table.SheetName, table.Label) && strings.HasPrefix(table.Key, "at") {
			c.enrichActivityScore(ctx, &result, table, user)
		}
		return result, result.Columns != nil, nil
	}
	nameIdx := nameColumn(grid.headers)
	matriculaIdx := matriculaColumn(grid.headers)
	if nameIdx < 0 && matriculaIdx < 0 {
		return TableResult{}, false, NewHTTPError(500, "coluna de nome ou matricula nao encontrada na aba "+sheetName)
	}
	for rowIdx, row := range grid.rows {
		if !matchesUser(row, nameIdx, matriculaIdx, user) {
			continue
		}
		columns := make([]ColumnResult, 0, len(grid.headers))
		for colIdx, header := range grid.headers {
			if strings.TrimSpace(header) == "" {
				continue
			}
			if table.Kind == "summary" && !summaryColumn(header) {
				continue
			}
			value := ""
			if colIdx < len(row) {
				value = row[colIdx]
			}
			comment := ""
			if rowIdx < len(grid.rowNotes) {
				comment = noteAt(grid.rowNotes[rowIdx], colIdx)
			}
			if comment == "" {
				comment = noteAt(grid.notes, colIdx)
			}
			columns = append(columns, ColumnResult{
				Key:     fmt.Sprintf("c%d", colIdx),
				Label:   header,
				Value:   value,
				Comment: comment,
			})
		}
		return TableResult{Key: table.Key, Label: table.Label, SheetName: sheetName, Kind: table.Kind, Complete: tableComplete(grid, table), Columns: columns}, true, nil
	}
	return TableResult{}, false, nil
}

func (c *SheetsClient) enrichActivityScore(ctx context.Context, result *TableResult, table TableConfig, user SessionUser) {
	if table.Key != "at1" && table.Key != "at2" && table.Key != "at3" {
		return
	}
	summarySheet := c.cfg.AB1Tables[len(c.cfg.AB1Tables)-1].SheetName
	grid, err := c.loadSheet(ctx, summarySheet)
	if err != nil {
		return
	}
	nameIdx := nameColumn(grid.headers)
	matriculaIdx := matriculaColumn(grid.headers)
	scoreIdx := activityScoreColumn(grid.headers, table.Key)
	if scoreIdx < 0 {
		return
	}
	for rowIdx, row := range grid.rows {
		if !matchesUser(row, nameIdx, matriculaIdx, user) {
			continue
		}
		comment := ""
		if rowIdx < len(grid.rowNotes) {
			comment = noteAt(grid.rowNotes[rowIdx], scoreIdx)
		}
		if comment == "" {
			comment = noteAt(grid.notes, scoreIdx)
		}
		result.Columns = append(result.Columns, ColumnResult{
			Key:     "nota",
			Label:   "Nota",
			Value:   valueAt(row, scoreIdx),
			Comment: comment,
		})
		return
	}
}

func activityTableFor(grid *sheetGrid, table TableConfig, user SessionUser) TableResult {
	for rowIdx, row := range grid.rows {
		for colIdx, value := range row {
			if normalizePerson(value) != normalizePerson(user.Name) {
				continue
			}
			columns := []ColumnResult{
				{Key: "nome", Label: "Nome do Aluno(a)", Value: value},
			}
			if rowIdx < len(grid.rowNotes) {
				if comment := noteAt(grid.rowNotes[rowIdx], colIdx); comment != "" {
					columns[0].Comment = comment
				}
			}
			return TableResult{Key: table.Key, Label: table.Label, SheetName: table.SheetName, Kind: table.Kind, Complete: true, Columns: columns}
		}
	}
	return TableResult{}
}

func tableComplete(grid *sheetGrid, table TableConfig) bool {
	var idx int
	switch table.Kind {
	case "summary":
		idx = totalABColumn(grid.headers)
	case "project":
		idx = totalColumn(grid.headers)
	default:
		return true
	}
	if idx < 0 {
		return false
	}
	nameIdx := nameColumn(grid.headers)
	matriculaIdx := matriculaColumn(grid.headers)
	for _, row := range grid.rows {
		if !studentRow(row, nameIdx, matriculaIdx) {
			continue
		}
		if strings.TrimSpace(valueAt(row, idx)) == "" {
			return false
		}
	}
	return true
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
	allValues := make([][]string, 0, len(rows))
	allNotes := make([][]string, 0, len(rows))
	for _, row := range rows {
		values := make([]string, len(row.Values))
		notes := make([]string, len(row.Values))
		for idx, cell := range row.Values {
			values[idx] = cellText(cell)
			notes[idx] = strings.TrimSpace(cell.Note)
		}
		allValues = append(allValues, values)
		allNotes = append(allNotes, notes)
	}

	headerIdx := -1
	bestScore := 0
	for idx, values := range allValues {
		score := headerScore(values)
		if score > bestScore {
			bestScore = score
			headerIdx = idx
		}
	}
	if headerIdx < 0 {
		for idx, values := range allValues {
			if hasAny(values) {
				headerIdx = idx
				break
			}
		}
	}
	if headerIdx < 0 {
		return &sheetGrid{}
	}

	grid := &sheetGrid{headers: allValues[headerIdx], notes: allNotes[headerIdx]}
	for idx, values := range allValues[headerIdx+1:] {
		if hasAny(values) {
			grid.rows = append(grid.rows, values)
			grid.rowNotes = append(grid.rowNotes, allNotes[headerIdx+1+idx])
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

func exactNameColumn(headers []string) int {
	candidates := []string{"nome", "aluno", "estudante", "discente", "nome completo", "nome do aluno", "nome do aluno(a)"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	return -1
}

func headerScore(headers []string) int {
	score := 0
	if matriculaColumn(headers) >= 0 {
		score += 3
	}
	if exactNameColumn(headers) >= 0 {
		score += 4
	} else if nameColumn(headers) >= 0 {
		score++
	}
	for _, header := range headers {
		if summaryColumn(header) {
			score += 2
		}
	}
	return score
}

func summaryColumn(header string) bool {
	normalized := normalizeHeader(header)
	if strings.Contains(normalized, "nota") && strings.Contains(normalized, "ab") {
		return true
	}
	return strings.Contains(normalized, "prova") && strings.Contains(normalized, "ab")
}

func totalABColumn(headers []string) int {
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if strings.Contains(normalized, "nota") && strings.Contains(normalized, "ab") {
			return idx
		}
	}
	return -1
}

func totalColumn(headers []string) int {
	for idx, header := range headers {
		if normalizeHeader(header) == "total" {
			return idx
		}
	}
	return -1
}

func activityScoreColumn(headers []string, key string) int {
	wanted := map[string][]string{
		"at1": {"pesquisa"},
		"at2": {"artigo"},
		"at3": {"lista"},
	}[key]
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, item := range wanted {
			if normalized == item || strings.Contains(normalized, item) {
				return idx
			}
		}
	}
	return -1
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return replacer.Replace(value)
}

func noteAt(notes []string, idx int) string {
	if idx < 0 || idx >= len(notes) {
		return ""
	}
	return strings.TrimSpace(notes[idx])
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

func matchesUser(row []string, nameIdx int, matriculaIdx int, user SessionUser) bool {
	if nameIdx >= 0 && nameIdx < len(row) && sameLookupValue(row[nameIdx], user.Name, true) {
		return true
	}
	if matriculaIdx >= 0 && matriculaIdx < len(row) && sameLookupValue(row[matriculaIdx], user.Matricula, false) {
		return true
	}
	return false
}

func studentRow(row []string, nameIdx int, matriculaIdx int) bool {
	if nameIdx >= 0 && strings.TrimSpace(valueAt(row, nameIdx)) != "" {
		return true
	}
	if matriculaIdx >= 0 && strings.TrimSpace(valueAt(row, matriculaIdx)) != "" {
		return true
	}
	return false
}
