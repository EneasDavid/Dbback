package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	v2GradeSheetCacheTTL = 30 * time.Second
	v2CommentsCacheTTL   = time.Minute
)

type SheetsClient struct {
	cfg              Config
	service          *sheets.Service
	httpClient       *http.Client
	mu               sync.Mutex
	cache            map[string]cachedGrid
	driveComments    map[string]cachedDriveComments
	workbookComments map[string]cachedWorkbookComments
	group            singleflight.Group
	cacheOwner       *SheetsClient
}

type cachedGrid struct {
	expires time.Time
	grid    *sheetGrid
}

func NewSheetsClient(ctx context.Context, cfg Config) (*SheetsClient, error) {
	credentials, err := google.CredentialsFromJSON(
		ctx,
		[]byte(cfg.ServiceJSON),
		sheets.SpreadsheetsReadonlyScope,
		driveReadonlyScope,
	)
	if err != nil {
		return nil, err
	}
	httpClient := googleHTTPClient(credentials.TokenSource)
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &SheetsClient{
		cfg:              cfg,
		service:          svc,
		httpClient:       httpClient,
		cache:            map[string]cachedGrid{},
		driveComments:    map[string]cachedDriveComments{},
		workbookComments: map[string]cachedWorkbookComments{},
	}, nil
}

func googleHTTPClient(source oauth2.TokenSource) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.ExpectContinueTimeout = time.Second
	return &http.Client{
		Transport: &oauth2.Transport{
			Source: source,
			Base:   transport,
		},
		Timeout: 15 * time.Second,
	}
}

func (c *SheetsClient) ClearCache() {
	owner := c.cacheRuntime()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.cache = map[string]cachedGrid{}
	owner.driveComments = map[string]cachedDriveComments{}
	owner.workbookComments = map[string]cachedWorkbookComments{}
}

func (c *SheetsClient) loadSheet(ctx context.Context, sheetName string) (*sheetGrid, error) {
	if cached, ok := c.cachedSheet(sheetName); ok {
		return cached.grid, nil
	}

	if err := c.loadSheets(ctx, []string{sheetName}); err != nil {
		return nil, err
	}
	if cached, ok := c.cachedSheet(sheetName); ok {
		return cached.grid, nil
	}
	return nil, NewHTTPError(404, "aba nao encontrada: "+sheetName)
}

func (c *SheetsClient) loadSheets(ctx context.Context, sheetNames []string) error {
	missing := c.missingSheets(sheetNames)
	if len(missing) == 0 {
		return nil
	}

	key := "sheets:" + strings.Join(c.cfg.SpreadsheetIDs, "\x00") + "\x00" + strings.Join(missing, "\x00")
	owner := c.cacheRuntime()
	_, err, _ := owner.group.Do(key, func() (interface{}, error) {
		missing := c.missingSheets(missing)
		if len(missing) == 0 {
			return nil, nil
		}

		ranges := make([]string, 0, len(missing))
		for _, sheetName := range missing {
			ranges = append(ranges, quoteSheetName(sheetName))
		}
		grids := map[string]*sheetGrid{}
		now := time.Now()
		var lastReadErr error

		for _, spreadsheetID := range c.cfg.SpreadsheetIDs {
			spreadsheetID = strings.TrimSpace(spreadsheetID)
			if spreadsheetID == "" {
				continue
			}
			driveCommentsCh := c.optionalDriveCommentsAsync(ctx, spreadsheetID, missing)
			workbookCommentsCh := c.optionalWorkbookCommentsAsync(ctx, spreadsheetID, missing)

			responses, err := c.spreadsheetResponses(ctx, spreadsheetID, ranges)
			if err != nil {
				lastReadErr = err
				if len(c.cfg.SpreadsheetIDs) > 1 && skippableSpreadsheetReadError(err) {
					continue
				}
				return nil, sheetReadError(err)
			}

			driveComments := <-driveCommentsCh
			workbookComments := <-workbookCommentsCh
			for _, resp := range responses {
				schemaStatus := c.schemaStatusForSpreadsheet(resp.DeveloperMetadata)
				for _, sheet := range resp.Sheets {
					if sheet == nil || sheet.Properties == nil {
						continue
					}
					name := sheet.Properties.Title
					cacheName, ok := matchingSheetName(missing, name)
					if !ok || len(sheet.Data) == 0 {
						continue
					}
					grid := parseGrid(sheet.Data[0].RowData, sheet.Merges)
					grid.spreadsheetID = spreadsheetID
					grid.schemaStatus = schemaStatus
					grid.setRowSource(spreadsheetID)
					grid.applyWorkbookComments(workbookComments[name], sheet.Merges)
					grid.applyDriveComments(driveComments, sheet.Properties.SheetId, sheet.Merges)
					grid.applyCommentMerges(sheet.Merges)
					grids[cacheName] = mergeSheetGrid(grids[cacheName], grid)
				}
			}
		}

		owner.mu.Lock()
		defer owner.mu.Unlock()
		for _, sheetName := range missing {
			grid := grids[sheetName]
			if grid == nil {
				if lastReadErr != nil && len(grids) == 0 {
					return nil, sheetReadError(lastReadErr)
				}
				return nil, NewHTTPError(404, "aba nao encontrada: "+sheetName)
			}
			owner.cache[c.sheetCacheKey(sheetName)] = cachedGrid{expires: now.Add(c.sheetCacheTTL(sheetName)), grid: grid}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *SheetsClient) sheetCacheTTL(sheetName string) time.Duration {
	ttl := c.cfg.CacheTTL
	normalized := normalizeHeader(sheetName)
	isControlSheet := normalized == v2ABsSheet ||
		normalized == v2ActivitiesSheet ||
		strings.HasPrefix(normalized, "nota ")
	isV2GradeSheet := c.isV2Runtime() && normalized != normalizeHeader(c.cfg.LoginSheet)
	if isControlSheet || isV2GradeSheet {
		return cappedCacheTTL(ttl, v2GradeSheetCacheTTL)
	}
	return ttl
}

func (c *SheetsClient) commentsCacheTTL() time.Duration {
	if c.isV2Runtime() {
		return cappedCacheTTL(c.cfg.CacheTTL, v2CommentsCacheTTL)
	}
	return c.cfg.CacheTTL
}

func (c *SheetsClient) isV2Runtime() bool {
	return strings.EqualFold(strings.TrimSpace(c.cfg.RuntimeVersion), "v2")
}

func cappedCacheTTL(ttl time.Duration, maximum time.Duration) time.Duration {
	if ttl <= 0 || ttl > maximum {
		return maximum
	}
	return ttl
}

func (c *SheetsClient) optionalDriveComments(ctx context.Context, spreadsheetID string, sheetNames []string) []driveCellComment {
	if !requiresDriveComments(sheetNames, c.cfg.LoginSheet) {
		return nil
	}
	comments, err := c.driveCommentsForSpreadsheet(ctx, spreadsheetID)
	if err != nil {
		return nil
	}
	return comments
}

func (c *SheetsClient) optionalDriveCommentsAsync(ctx context.Context, spreadsheetID string, sheetNames []string) <-chan []driveCellComment {
	ch := make(chan []driveCellComment, 1)
	go func() {
		ch <- c.optionalDriveComments(ctx, spreadsheetID, sheetNames)
	}()
	return ch
}

func (c *SheetsClient) optionalWorkbookComments(ctx context.Context, spreadsheetID string, sheetNames []string) map[string][]workbookCellComment {
	if !requiresDriveComments(sheetNames, c.cfg.LoginSheet) {
		return nil
	}
	comments, err := c.workbookCommentsForSpreadsheet(ctx, spreadsheetID)
	if err != nil {
		return nil
	}
	return filterWorkbookComments(comments, sheetNameSet(sheetNames))
}

func (c *SheetsClient) optionalWorkbookCommentsAsync(ctx context.Context, spreadsheetID string, sheetNames []string) <-chan map[string][]workbookCellComment {
	ch := make(chan map[string][]workbookCellComment, 1)
	go func() {
		ch <- c.optionalWorkbookComments(ctx, spreadsheetID, sheetNames)
	}()
	return ch
}

func (c *SheetsClient) cachedSheet(sheetName string) (cachedGrid, bool) {
	owner := c.cacheRuntime()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	cached, ok := owner.cache[c.sheetCacheKey(sheetName)]
	if !ok {
		cached, ok = owner.cache[sheetName]
	}
	if !ok || time.Now().After(cached.expires) {
		return cachedGrid{}, false
	}
	return cached, true
}

func (c *SheetsClient) cacheRuntime() *SheetsClient {
	if c.cacheOwner != nil {
		return c.cacheOwner.cacheRuntime()
	}
	if c.cache == nil {
		c.cache = map[string]cachedGrid{}
	}
	if c.driveComments == nil {
		c.driveComments = map[string]cachedDriveComments{}
	}
	if c.workbookComments == nil {
		c.workbookComments = map[string]cachedWorkbookComments{}
	}
	return c
}

func (c *SheetsClient) sheetCacheKey(sheetName string) string {
	scope := strings.Join(c.cfg.SpreadsheetIDs, "\x00")
	if strings.TrimSpace(scope) == "" {
		scope = strings.TrimSpace(c.cfg.SpreadsheetID)
	}
	if strings.TrimSpace(scope) == "" {
		return sheetName
	}
	return scope + "\x00" + sheetName
}

func (c *SheetsClient) missingSheets(sheetNames []string) []string {
	seen := map[string]bool{}
	missing := make([]string, 0, len(sheetNames))
	for _, sheetName := range sheetNames {
		sheetName = strings.TrimSpace(sheetName)
		if sheetName == "" || seen[sheetName] {
			continue
		}
		seen[sheetName] = true
		if _, ok := c.cachedSheet(sheetName); !ok {
			missing = append(missing, sheetName)
		}
	}
	sort.Strings(missing)
	return missing
}

func quoteSheetName(sheetName string) string {
	return "'" + strings.ReplaceAll(sheetName, "'", "''") + "'"
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func matchingSheetName(values []string, target string) (string, bool) {
	for _, value := range values {
		if value == target {
			return value, true
		}
	}
	normalizedTarget := normalizeHeader(target)
	for _, value := range values {
		if normalizeHeader(value) == normalizedTarget {
			return value, true
		}
	}
	return "", false
}

func (c *SheetsClient) spreadsheetResponses(ctx context.Context, spreadsheetID string, ranges []string) ([]*sheets.Spreadsheet, error) {
	resp, err := c.service.Spreadsheets.Get(spreadsheetID).
		Ranges(ranges...).
		Fields(sheetsGridFields).
		Context(ctx).
		Do()
	if err == nil {
		return []*sheets.Spreadsheet{resp}, nil
	}
	if !isGoogleBadRequest(err) {
		return nil, err
	}

	var responses []*sheets.Spreadsheet
	for _, rangeName := range ranges {
		resp, err := c.service.Spreadsheets.Get(spreadsheetID).
			Ranges(rangeName).
			Fields(sheetsGridFields).
			Context(ctx).
			Do()
		if err != nil {
			if isGoogleBadRequest(err) {
				resp, looseErr := c.spreadsheetByLooseRange(ctx, spreadsheetID, rangeName)
				if looseErr == nil {
					responses = append(responses, resp)
				}
				continue
			}
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

func (c *SheetsClient) spreadsheetByLooseRange(ctx context.Context, spreadsheetID string, rangeName string) (*sheets.Spreadsheet, error) {
	target := normalizeHeader(unquoteSheetRange(rangeName))
	if target == "" {
		return nil, NewHTTPError(404, "aba nao encontrada")
	}
	meta, err := c.service.Spreadsheets.Get(spreadsheetID).
		Fields("sheets(properties(title))").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	for _, sheet := range meta.Sheets {
		if sheet == nil || sheet.Properties == nil {
			continue
		}
		title := strings.TrimSpace(sheet.Properties.Title)
		if normalizeHeader(title) != target {
			continue
		}
		return c.service.Spreadsheets.Get(spreadsheetID).
			Ranges(quoteSheetName(title)).
			Fields(sheetsGridFields).
			Context(ctx).
			Do()
	}
	return nil, NewHTTPError(404, "aba nao encontrada")
}

func unquoteSheetRange(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}

func isGoogleBadRequest(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusBadRequest
}

func skippableSpreadsheetReadError(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == http.StatusForbidden ||
		apiErr.Code == http.StatusNotFound ||
		apiErr.Code == http.StatusBadRequest
}

func (c *SheetsClient) schemaStatusForSpreadsheet(metadata []*sheets.DeveloperMetadata) string {
	if strings.EqualFold(strings.TrimSpace(c.cfg.RuntimeVersion), "legacy") || strings.EqualFold(strings.TrimSpace(c.cfg.RuntimeVersion), "v1") {
		return "legacy"
	}
	if !c.detectsSpreadsheetSchema() {
		return ""
	}
	runtimeVersion := strings.ToLower(strings.TrimSpace(c.cfg.RuntimeVersion))
	expectedKey := strings.TrimSpace(c.cfg.MetadataKey)
	expectedValue := strings.TrimSpace(c.cfg.MetadataValue)
	if expectedKey == "" || expectedValue == "" {
		if runtimeVersion == "auto" {
			return ""
		}
		return "legacy"
	}
	foundMetadataKey := false
	for _, item := range metadata {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.MetadataKey) == expectedKey {
			foundMetadataKey = true
			if strings.TrimSpace(item.MetadataValue) == expectedValue {
				return "v2"
			}
		}
	}
	if foundMetadataKey {
		return "legacy"
	}
	if runtimeVersion == "auto" {
		return ""
	}
	return ""
}

func (c *SheetsClient) detectsSpreadsheetSchema() bool {
	switch strings.ToLower(strings.TrimSpace(c.cfg.RuntimeVersion)) {
	case "v2", "auto":
		return true
	default:
		return false
	}
}

func mergeSheetGrid(base *sheetGrid, next *sheetGrid) *sheetGrid {
	if next == nil {
		return base
	}
	if base == nil || len(base.headers) == 0 {
		return next
	}
	if len(next.headers) == 0 {
		return base
	}

	base.rows = append(base.rows, next.rows...)
	base.rowNotes = append(base.rowNotes, next.rowNotes...)
	base.rowNoteAuthors = append(base.rowNoteAuthors, next.rowNoteAuthors...)
	base.rowIndices = append(base.rowIndices, next.rowIndices...)
	base.rowSources = append(base.rowSources, next.rowSources...)
	base.rowSchemas = append(base.rowSchemas, next.rowSchemas...)
	base.spreadsheetID = mergeSourceValue(base.spreadsheetID, next.spreadsheetID)
	base.schemaStatus = mergeSchemaStatus(base.schemaStatus, next.schemaStatus)
	return base
}

func (g *sheetGrid) setRowSource(spreadsheetID string) {
	g.rowSources = make([]string, len(g.rows))
	g.rowSchemas = make([]string, len(g.rows))
	for idx := range g.rowSources {
		g.rowSources[idx] = spreadsheetID
		g.rowSchemas[idx] = g.schemaStatus
	}
}

func (g *sheetGrid) rowSource(rowIdx int) string {
	if g == nil || rowIdx < 0 || rowIdx >= len(g.rowSources) {
		return ""
	}
	return g.rowSources[rowIdx]
}

func (g *sheetGrid) rowSchema(rowIdx int) string {
	if g == nil || rowIdx < 0 || rowIdx >= len(g.rowSchemas) {
		return ""
	}
	return g.rowSchemas[rowIdx]
}

func mergeSourceValue(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" || left == right || containsString(strings.Split(left, ","), right) {
		return left
	}
	return left + "," + right
}

func mergeSchemaStatus(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "legacy" || right == "legacy" {
		return "legacy"
	}
	if left == "" {
		return right
	}
	return left
}

func sheetReadError(err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusUnauthorized:
			return NewHTTPError(http.StatusServiceUnavailable, "credencial Google invalida ou expirada")
		case http.StatusForbidden:
			return NewHTTPError(http.StatusServiceUnavailable, "service account sem acesso a planilha; compartilhe a planilha com o client_email da credencial")
		case http.StatusNotFound:
			return NewHTTPError(http.StatusNotFound, "planilha nao encontrada; confira GOOGLE_SHEET_ID ou GOOGLE_SHEET_IDS")
		case http.StatusBadRequest:
			return NewHTTPError(http.StatusBadRequest, "nao conseguiu ler as abas configuradas; confira os nomes das abas no ambiente")
		default:
			return NewHTTPError(http.StatusServiceUnavailable, fmt.Sprintf("nao conseguiu ler dados da planilha: Google API HTTP %d", apiErr.Code))
		}
	}
	return err
}

var sheetsGridFields = googleapi.Field(
	"developerMetadata(metadataKey,metadataValue),sheets(properties(title,sheetId),merges(startRowIndex,endRowIndex,startColumnIndex,endColumnIndex),data(startRow,startColumn,rowData(values(formattedValue,note,userEnteredValue))))",
)

const driveReadonlyScope = "https://www.googleapis.com/auth/drive.readonly"

type driveCommentEndpoint struct {
	version       string
	pageSizeParam string
	pageSize      string
	fields        string
	errorLabel    string
	decode        func([]byte) (driveCommentsPayload, error)
}

var (
	driveV3CommentEndpoint = driveCommentEndpoint{
		version:       "v3",
		pageSizeParam: "pageSize",
		pageSize:      "100",
		fields:        "nextPageToken,comments(content,anchor,author(displayName),quotedFileContent(value),deleted,replies(content,author(displayName),deleted))",
		errorLabel:    "comments",
		decode:        decodeDriveComments,
	}
	driveV2CommentEndpoint = driveCommentEndpoint{
		version:       "v2",
		pageSizeParam: "maxResults",
		pageSize:      "100",
		fields:        "nextPageToken,items(content,anchor,author(displayName),context(value),deleted,status,replies(content,author(displayName),deleted))",
		errorLabel:    "comments v2",
		decode:        decodeDriveV2Comments,
	}
)

func requiresDriveComments(sheetNames []string, loginSheet string) bool {
	for _, sheetName := range sheetNames {
		if requiresSheetComments(sheetName, loginSheet) {
			return true
		}
	}
	return false
}

func requiresSheetComments(sheetName string, loginSheet string) bool {
	normalized := normalizeHeader(sheetName)
	if normalized == "" {
		return false
	}
	return normalized != normalizeHeader(loginSheet) &&
		normalized != normalizeHeader(v2ABsSheet) &&
		normalized != normalizeHeader(v2ActivitiesSheet)
}

func (c *SheetsClient) driveCommentsForSpreadsheet(ctx context.Context, spreadsheetID string) ([]driveCellComment, error) {
	owner := c.cacheRuntime()
	owner.mu.Lock()
	if owner.driveComments == nil {
		owner.driveComments = map[string]cachedDriveComments{}
	}
	cached := owner.driveComments[spreadsheetID]
	if cached.comments != nil && time.Now().Before(cached.expires) {
		comments := append([]driveCellComment(nil), cached.comments...)
		owner.mu.Unlock()
		return comments, nil
	}
	owner.mu.Unlock()

	value, err, _ := owner.group.Do("drive-comments:"+spreadsheetID, func() (interface{}, error) {
		comments, err := c.fetchDriveComments(ctx, spreadsheetID)
		if err != nil {
			return nil, err
		}
		owner.mu.Lock()
		if owner.driveComments == nil {
			owner.driveComments = map[string]cachedDriveComments{}
		}
		owner.driveComments[spreadsheetID] = cachedDriveComments{expires: time.Now().Add(c.commentsCacheTTL()), comments: comments}
		owner.mu.Unlock()
		return comments, nil
	})
	if err != nil {
		return nil, err
	}
	return append([]driveCellComment(nil), value.([]driveCellComment)...), nil
}

func (c *SheetsClient) fetchDriveComments(ctx context.Context, spreadsheetID string) ([]driveCellComment, error) {
	comments, err := c.fetchDriveCommentPages(ctx, spreadsheetID, driveV3CommentEndpoint)
	if err != nil {
		return nil, err
	}
	if len(comments) > 0 {
		return comments, nil
	}
	return c.fetchDriveCommentPages(ctx, spreadsheetID, driveV2CommentEndpoint)
}

func (c *SheetsClient) fetchDriveCommentPages(ctx context.Context, spreadsheetID string, spec driveCommentEndpoint) ([]driveCellComment, error) {
	var all []driveCellComment
	pageToken := ""
	for {
		endpoint, err := url.Parse(fmt.Sprintf("https://www.googleapis.com/drive/%s/files/%s/comments", spec.version, url.PathEscape(spreadsheetID)))
		if err != nil {
			return nil, err
		}
		query := endpoint.Query()
		query.Set(spec.pageSizeParam, spec.pageSize)
		query.Set("includeDeleted", "false")
		query.Set("fields", spec.fields)
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		endpoint.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s retornou HTTP %d: %s", spec.errorLabel, resp.StatusCode, driveErrorMessage(body))
		}

		payload, err := spec.decode(body)
		if err != nil {
			return nil, err
		}
		all = append(all, payload.comments...)
		if strings.TrimSpace(payload.nextPageToken) == "" {
			return all, nil
		}
		pageToken = payload.nextPageToken
	}
}

func driveErrorMessage(body []byte) string {
	text := strings.TrimSpace(string(body))
	if strings.Contains(text, "SERVICE_DISABLED") || strings.Contains(text, "accessNotConfigured") {
		return "Google Drive API desativada no projeto da service account; habilite drive.googleapis.com no Google Cloud e tente novamente"
	}
	if strings.Contains(text, "insufficientFilePermissions") || strings.Contains(text, "The user does not have sufficient permissions") {
		return "a service account não tem permissão suficiente no arquivo; compartilhe a planilha com o e-mail da service account"
	}
	if text == "" {
		return "resposta vazia"
	}
	return text
}
