package app

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsClient struct {
	cfg      Config
	service  *sheets.Service
	http     *http.Client
	mu       sync.Mutex
	cache    map[string]cachedGrid
	comments cachedComments
	group    singleflight.Group
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
		"https://www.googleapis.com/auth/drive.readonly",
	)
	if err != nil {
		return nil, err
	}
	httpClient := oauth2.NewClient(ctx, credentials.TokenSource)
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &SheetsClient{cfg: cfg, service: svc, http: httpClient, cache: map[string]cachedGrid{}}, nil
}

func (c *SheetsClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = map[string]cachedGrid{}
	c.comments = cachedComments{}
}

func (c *SheetsClient) loadSheet(ctx context.Context, sheetName string) (*sheetGrid, error) {
	c.mu.Lock()
	if cached, ok := c.cache[sheetName]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.grid, nil
	}
	c.mu.Unlock()

	v, err, _ := c.group.Do(sheetName, func() (interface{}, error) {
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
		grid := parseGrid(resp.Sheets[0].Data[0].RowData, resp.Sheets[0].Merges)
		
		// Try to load comments, but don't fail if they're unavailable
		comments, err := c.commentsForSheet(ctx, sheetName)
		if err == nil && comments != nil {
			grid.applyComments(comments)
			grid.applyCommentMerges(resp.Sheets[0].Merges)
		}
		// Note: if comments fail to load, we continue anyway - comments are optional
		
		c.mu.Lock()
		c.cache[sheetName] = cachedGrid{expires: time.Now().Add(c.cfg.CacheTTL), grid: grid}
		c.mu.Unlock()
		return grid, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*sheetGrid), nil
}
