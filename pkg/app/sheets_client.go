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
	cfg        Config
	service    *sheets.Service
	httpClient *http.Client
	mu         sync.Mutex
	cache      map[string]cachedGrid
	comments   cachedComments
	group      singleflight.Group
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
	c.comments = cachedComments{}
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

		comments := map[string]map[string]cellComment{}
		if c.requiresXLSXComments(missing) {
			var commentsErr error
			comments, commentsErr = c.commentsForSheets(ctx, missing)
			if commentsErr != nil {
				return nil, NewHTTPError(503, "não conseguiu exportar comentários do Google Sheets: "+commentsErr.Error())
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
			if sheetComments := comments[name]; len(sheetComments) > 0 {
				grid.applyComments(sheetComments)
				grid.applyCommentMerges(sheet.Merges)
			}
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

func (c *SheetsClient) requiresXLSXComments(sheetNames []string) bool {
	for _, sheetName := range sheetNames {
		if strings.TrimSpace(sheetName) != "" && sheetName != c.cfg.LoginSheet {
			return true
		}
	}
	return false
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
	"sheets(properties(title),merges(startRowIndex,endRowIndex,startColumnIndex,endColumnIndex),data(startRow,startColumn,rowData(values(formattedValue,note,userEnteredValue))))",
)

func exportMimeTypeXLSX() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

func (c *SheetsClient) exportSpreadsheetXLSX(ctx context.Context) ([]byte, error) {
	exportURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=xlsx", url.PathEscape(c.cfg.SpreadsheetID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("export retornou HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

const driveReadonlyScope = "https://www.googleapis.com/auth/drive.readonly"
