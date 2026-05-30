package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type cachedComments struct {
	expires time.Time
	bySheet map[string]map[string]string
}

func (c *SheetsClient) commentsForSheet(ctx context.Context, sheetName string) (map[string]string, error) {
	c.mu.Lock()
	if c.comments.bySheet != nil && time.Now().Before(c.comments.expires) {
		comments := c.comments.bySheet[sheetName]
		c.mu.Unlock()
		return comments, nil
	}
	c.mu.Unlock()

	value, err, _ := c.group.Do("xlsx-comments", func() (interface{}, error) {
		data, err := c.loadXLSX(ctx)
		if err != nil {
			return nil, err
		}
		comments, err := parseXLSXComments(data)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.comments = cachedComments{expires: time.Now().Add(c.cfg.CacheTTL), bySheet: comments}
		c.mu.Unlock()
		return comments, nil
	})
	if err != nil {
		return nil, err
	}
	comments := value.(map[string]map[string]string)
	return comments[sheetName], nil
}

func (c *SheetsClient) loadXLSX(ctx context.Context) ([]byte, error) {
	if strings.TrimSpace(c.cfg.XLSXFile) != "" {
		return os.ReadFile(c.cfg.XLSXFile)
	}
	return c.exportXLSX(ctx)
}

func (c *SheetsClient) exportXLSX(ctx context.Context) ([]byte, error) {
	exportURL := fmt.Sprintf(
		"https://www.googleapis.com/drive/v3/files/%s/export?mimeType=%s",
		url.PathEscape(c.cfg.SpreadsheetID),
		url.QueryEscape("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewHTTPError(resp.StatusCode, "nao foi possivel exportar comentarios da planilha")
	}
	return io.ReadAll(resp.Body)
}

func parseXLSXComments(data []byte) (map[string]map[string]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}

	sheets, err := workbookSheets(files)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]string{}
	for _, sheet := range sheets {
		comments := commentsForWorksheet(files, sheet.Path)
		if len(comments) > 0 {
			result[sheet.Name] = comments
		}
	}
	return result, nil
}

type workbookSheet struct {
	Name string
	Path string
}

func workbookSheets(files map[string]*zip.File) ([]workbookSheet, error) {
	type rel struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	type relsXML struct {
		Relationships []rel `xml:"Relationship"`
	}
	type sheetXML struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	}
	type workbookXML struct {
		Sheets []sheetXML `xml:"sheets>sheet"`
	}

	var rels relsXML
	if err := readXML(files, "xl/_rels/workbook.xml.rels", &rels); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, relationship := range rels.Relationships {
		targets[relationship.ID] = normalizeXLSXPath("xl", relationship.Target)
	}

	var workbook workbookXML
	if err := readXML(files, "xl/workbook.xml", &workbook); err != nil {
		return nil, err
	}
	sheets := make([]workbookSheet, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		if target := targets[sheet.RID]; target != "" {
			sheets = append(sheets, workbookSheet{Name: sheet.Name, Path: target})
		}
	}
	return sheets, nil
}

func commentsForWorksheet(files map[string]*zip.File, worksheetPath string) map[string]string {
	type rel struct {
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	}
	type relsXML struct {
		Relationships []rel `xml:"Relationship"`
	}

	relPath := path.Join(path.Dir(worksheetPath), "_rels", path.Base(worksheetPath)+".rels")
	var rels relsXML
	if err := readXML(files, relPath, &rels); err != nil {
		return nil
	}
	comments := map[string]string{}
	for _, relationship := range rels.Relationships {
		target := normalizeXLSXPath(path.Dir(worksheetPath), relationship.Target)
		switch {
		case strings.Contains(relationship.Type, "threadedComment"):
			mergeComments(comments, parseThreadedCommentFile(files, target))
		case strings.Contains(relationship.Type, "/comments"):
			mergeComments(comments, parseClassicCommentFile(files, target))
		}
	}
	return comments
}

func parseThreadedCommentFile(files map[string]*zip.File, filename string) map[string]string {
	type threadedComment struct {
		Ref  string `xml:"ref,attr"`
		Text string `xml:"text"`
	}
	type threadedComments struct {
		Comments []threadedComment `xml:"threadedComment"`
	}
	var parsed threadedComments
	if err := readXML(files, filename, &parsed); err != nil {
		return nil
	}
	comments := map[string]string{}
	for _, comment := range parsed.Comments {
		if text := strings.TrimSpace(comment.Text); text != "" {
			comments[comment.Ref] = text
		}
	}
	return comments
}

func parseClassicCommentFile(files map[string]*zip.File, filename string) map[string]string {
	type commentXML struct {
		Ref  string            `xml:"ref,attr"`
		Text []commentTextNode `xml:"text>r>t"`
		Flat []commentTextNode `xml:"text>t"`
	}
	type commentsXML struct {
		Comments []commentXML `xml:"commentList>comment"`
	}
	var parsed commentsXML
	if err := readXML(files, filename, &parsed); err != nil {
		return nil
	}
	comments := map[string]string{}
	for _, comment := range parsed.Comments {
		text := commentText(comment.Text, comment.Flat)
		if text != "" {
			comments[comment.Ref] = text
		}
	}
	return comments
}

type commentTextNode struct {
	Text string `xml:",chardata"`
}

func commentText(rich []commentTextNode, flat []commentTextNode) string {
	parts := make([]string, 0, len(rich)+len(flat))
	for _, node := range rich {
		parts = append(parts, node.Text)
	}
	for _, node := range flat {
		parts = append(parts, node.Text)
	}
	text := strings.TrimSpace(strings.Join(parts, ""))
	if idx := strings.LastIndex(text, "Comment:"); idx >= 0 {
		text = strings.TrimSpace(text[idx+len("Comment:"):])
	}
	return strings.TrimSpace(text)
}

func mergeComments(dst map[string]string, src map[string]string) {
	for cell, comment := range src {
		if strings.TrimSpace(comment) != "" {
			dst[cell] = comment
		}
	}
}

func readXML(files map[string]*zip.File, filename string, out any) error {
	file := files[filename]
	if file == nil {
		return fmt.Errorf("arquivo ausente no xlsx: %s", filename)
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return xml.NewDecoder(reader).Decode(out)
}

func normalizeXLSXPath(baseDir string, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(baseDir, target))
}

var cellRefPattern = regexp.MustCompile(`^([A-Za-z]+)([0-9]+)$`)

func parseCellRef(ref string) (int, int, bool) {
	matches := cellRefPattern.FindStringSubmatch(strings.TrimSpace(ref))
	if len(matches) != 3 {
		return 0, 0, false
	}
	col := 0
	for _, char := range strings.ToUpper(matches[1]) {
		col = col*26 + int(char-'A'+1)
	}
	row, err := strconv.Atoi(matches[2])
	if err != nil || row < 1 || col < 1 {
		return 0, 0, false
	}
	return row - 1, col - 1, true
}
