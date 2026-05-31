package app

import (
	"context"
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

type SheetsClient struct {
	cfg           Config
	service       *sheets.Service
	httpClient    *http.Client
	mu            sync.Mutex
	cache         map[string]cachedGrid
	driveComments cachedDriveComments
	group         singleflight.Group
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
	httpClient := oauth2.NewClient(ctx, credentials.TokenSource)
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &SheetsClient{cfg: cfg, service: svc, httpClient: httpClient, cache: map[string]cachedGrid{}}, nil
}

func (c *SheetsClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = map[string]cachedGrid{}
	c.driveComments = cachedDriveComments{}
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

	key := "sheets:" + strings.Join(missing, "\x00")
	_, err, _ := c.group.Do(key, func() (interface{}, error) {
		missing := c.missingSheets(missing)
		if len(missing) == 0 {
			return nil, nil
		}

		driveComments := []driveCellComment{}
		if requiresDriveComments(missing, c.cfg.LoginSheet) {
			var commentsErr error
			driveComments, commentsErr = c.driveCommentsForSpreadsheet(ctx)
			if commentsErr != nil {
				return nil, NewHTTPError(503, "não conseguiu ler comentários do Google Drive: "+commentsErr.Error())
			}
		}

		ranges := make([]string, 0, len(missing))
		for _, sheetName := range missing {
			ranges = append(ranges, quoteSheetName(sheetName))
		}
		resp, err := c.service.Spreadsheets.Get(c.cfg.SpreadsheetID).
			Ranges(ranges...).
			Fields(sheetsGridFields).
			Context(ctx).
			Do()
		if err != nil {
			return nil, err
		}

		found := map[string]bool{}
		now := time.Now()
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, sheet := range resp.Sheets {
			if sheet == nil || sheet.Properties == nil {
				continue
			}
			name := sheet.Properties.Title
			if !containsString(missing, name) || len(sheet.Data) == 0 {
				continue
			}
			grid := parseGrid(sheet.Data[0].RowData, sheet.Merges)
			grid.applyDriveComments(driveComments, sheet.Properties.SheetId, sheet.Merges)
			grid.applyCommentMerges(sheet.Merges)
			c.cache[name] = cachedGrid{expires: now.Add(c.cfg.CacheTTL), grid: grid}
			found[name] = true
		}
		for _, sheetName := range missing {
			if !found[sheetName] {
				return nil, NewHTTPError(404, "aba nao encontrada: "+sheetName)
			}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *SheetsClient) cachedSheet(sheetName string) (cachedGrid, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.cache[sheetName]
	if !ok || time.Now().After(cached.expires) {
		return cachedGrid{}, false
	}
	return cached, true
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

var sheetsGridFields = googleapi.Field(
	"sheets(properties(title,sheetId),merges(startRowIndex,endRowIndex,startColumnIndex,endColumnIndex),data(startRow,startColumn,rowData(values(formattedValue,note,userEnteredValue))))",
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
		fields:        "nextPageToken,comments(content,anchor,author(displayName),quotedFileContent(value),deleted)",
		errorLabel:    "comments",
		decode:        decodeDriveComments,
	}
	driveV2CommentEndpoint = driveCommentEndpoint{
		version:       "v2",
		pageSizeParam: "maxResults",
		pageSize:      "100",
		fields:        "nextPageToken,items(content,anchor,author(displayName),context(value),deleted,status)",
		errorLabel:    "comments v2",
		decode:        decodeDriveV2Comments,
	}
)

func requiresDriveComments(sheetNames []string, loginSheet string) bool {
	for _, sheetName := range sheetNames {
		sheetName = strings.TrimSpace(sheetName)
		if sheetName != "" && sheetName != loginSheet {
			return true
		}
	}
	return false
}

func (c *SheetsClient) driveCommentsForSpreadsheet(ctx context.Context) ([]driveCellComment, error) {
	c.mu.Lock()
	if c.driveComments.comments != nil && time.Now().Before(c.driveComments.expires) {
		comments := append([]driveCellComment(nil), c.driveComments.comments...)
		c.mu.Unlock()
		return comments, nil
	}
	c.mu.Unlock()

	value, err, _ := c.group.Do("drive-comments:"+c.cfg.SpreadsheetID, func() (interface{}, error) {
		comments, err := c.fetchDriveComments(ctx)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.driveComments = cachedDriveComments{expires: time.Now().Add(c.cfg.CacheTTL), comments: comments}
		c.mu.Unlock()
		return comments, nil
	})
	if err != nil {
		return nil, err
	}
	return append([]driveCellComment(nil), value.([]driveCellComment)...), nil
}

func (c *SheetsClient) fetchDriveComments(ctx context.Context) ([]driveCellComment, error) {
	comments, err := c.fetchDriveCommentPages(ctx, driveV3CommentEndpoint)
	if err != nil {
		return nil, err
	}
	if len(comments) > 0 {
		return comments, nil
	}
	return c.fetchDriveCommentPages(ctx, driveV2CommentEndpoint)
}

func (c *SheetsClient) fetchDriveCommentPages(ctx context.Context, spec driveCommentEndpoint) ([]driveCellComment, error) {
	var all []driveCellComment
	pageToken := ""
	for {
		endpoint, err := url.Parse(fmt.Sprintf("https://www.googleapis.com/drive/%s/files/%s/comments", spec.version, url.PathEscape(c.cfg.SpreadsheetID)))
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
