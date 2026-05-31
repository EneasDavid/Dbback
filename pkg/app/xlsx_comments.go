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
	"path"
	"strconv"
	"strings"
	"time"
)

const xlsxMimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type xlsxWorkbookXML struct {
	Sheets []xlsxWorkbookSheetXML `xml:"sheets>sheet"`
}

type xlsxWorkbookSheetXML struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type xlsxRelationshipsXML struct {
	Relationships []xlsxRelationshipXML `xml:"Relationship"`
}

type xlsxRelationshipXML struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

type xlsxCommentsXML struct {
	Authors  []string         `xml:"authors>author"`
	Comments []xlsxCommentXML `xml:"commentList>comment"`
}

type xlsxCommentXML struct {
	Ref      string       `xml:"ref,attr"`
	AuthorID int          `xml:"authorId,attr"`
	Text     xlsxInnerXML `xml:"text"`
}

type xlsxThreadedCommentsXML struct {
	Comments []xlsxThreadedCommentXML `xml:"threadedComment"`
}

type xlsxThreadedCommentXML struct {
	Ref      string       `xml:"ref,attr"`
	PersonID string       `xml:"personId,attr"`
	Text     xlsxInnerXML `xml:"text"`
}

type xlsxPersonsXML struct {
	Persons []xlsxPersonXML `xml:"person"`
}

type xlsxPersonXML struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"displayName,attr"`
}

type xlsxInnerXML struct {
	InnerXML string `xml:",innerxml"`
	CharData string `xml:",chardata"`
}

func (c *SheetsClient) workbookCommentsForSpreadsheet(ctx context.Context) (map[string][]workbookCellComment, error) {
	c.mu.Lock()
	if c.workbookComments.comments != nil && time.Now().Before(c.workbookComments.expires) {
		comments := cloneWorkbookComments(c.workbookComments.comments)
		c.mu.Unlock()
		return comments, nil
	}
	c.mu.Unlock()

	value, err, _ := c.group.Do("workbook-comments:"+c.cfg.SpreadsheetID, func() (interface{}, error) {
		comments, err := c.fetchWorkbookComments(ctx)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.workbookComments = cachedWorkbookComments{expires: time.Now().Add(c.cfg.CacheTTL), comments: comments}
		c.mu.Unlock()
		return comments, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneWorkbookComments(value.(map[string][]workbookCellComment)), nil
}

func (c *SheetsClient) fetchWorkbookComments(ctx context.Context) (map[string][]workbookCellComment, error) {
	endpoint, err := url.Parse(fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s/export", url.PathEscape(c.cfg.SpreadsheetID)))
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("mimeType", xlsxMimeType)
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
		return nil, fmt.Errorf("exportacao XLSX retornou HTTP %d: %s", resp.StatusCode, driveErrorMessage(body))
	}
	return decodeXLSXComments(body)
}

func decodeXLSXComments(body []byte) (map[string][]workbookCellComment, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[cleanXLSXPath(file.Name)] = file
	}

	var workbook xlsxWorkbookXML
	if err := readZipXML(files["xl/workbook.xml"], &workbook); err != nil {
		return nil, err
	}
	workbookRels, err := readXLSXRelationships(files["xl/_rels/workbook.xml.rels"])
	if err != nil {
		return nil, err
	}
	relationshipByID := map[string]xlsxRelationshipXML{}
	for _, relationship := range workbookRels.Relationships {
		relationshipByID[relationship.ID] = relationship
	}

	persons := readXLSXPersons(files)
	result := map[string][]workbookCellComment{}
	for _, sheet := range workbook.Sheets {
		relationship, ok := relationshipByID[sheet.RID]
		if !ok || strings.TrimSpace(sheet.Name) == "" {
			continue
		}
		sheetPath := resolveXLSXPath("xl/workbook.xml", relationship.Target)
		sheetRels, err := readXLSXRelationships(files[relsPath(sheetPath)])
		if err != nil {
			return nil, err
		}
		for _, sheetRel := range sheetRels.Relationships {
			targetPath := resolveXLSXPath(sheetPath, sheetRel.Target)
			switch {
			case isXLSXCommentRelationship(sheetRel.Type):
				comments, err := parseXLSXComments(files[targetPath], sheet.Name)
				if err != nil {
					return nil, err
				}
				appendWorkbookComments(result, comments)
			case isXLSXThreadedCommentRelationship(sheetRel.Type):
				comments, err := parseXLSXThreadedComments(files[targetPath], sheet.Name, persons)
				if err != nil {
					return nil, err
				}
				appendWorkbookComments(result, comments)
			}
		}
	}
	return result, nil
}

func readXLSXRelationships(file *zip.File) (xlsxRelationshipsXML, error) {
	if file == nil {
		return xlsxRelationshipsXML{}, nil
	}
	var relationships xlsxRelationshipsXML
	if err := readZipXML(file, &relationships); err != nil {
		return xlsxRelationshipsXML{}, err
	}
	return relationships, nil
}

func readXLSXPersons(files map[string]*zip.File) map[string]string {
	persons := map[string]string{}
	for name, file := range files {
		if !strings.HasPrefix(name, "xl/persons/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		var payload xlsxPersonsXML
		if err := readZipXML(file, &payload); err != nil {
			continue
		}
		for _, person := range payload.Persons {
			if strings.TrimSpace(person.ID) != "" && strings.TrimSpace(person.DisplayName) != "" {
				persons[person.ID] = person.DisplayName
			}
		}
	}
	return persons
}

func parseXLSXComments(file *zip.File, sheetName string) ([]workbookCellComment, error) {
	if file == nil {
		return nil, nil
	}
	var payload xlsxCommentsXML
	if err := readZipXML(file, &payload); err != nil {
		return nil, err
	}
	comments := make([]workbookCellComment, 0, len(payload.Comments))
	for _, comment := range payload.Comments {
		rowIdx, colIdx, ok := a1CellRef(comment.Ref)
		if !ok {
			continue
		}
		comments = append(comments, workbookCellComment{
			SheetName:   sheetName,
			RowIndex:    rowIdx,
			ColumnIndex: colIdx,
			Text:        xlsxText(comment.Text),
			Author:      xlsxAuthor(payload.Authors, comment.AuthorID),
		})
	}
	return comments, nil
}

func parseXLSXThreadedComments(file *zip.File, sheetName string, persons map[string]string) ([]workbookCellComment, error) {
	if file == nil {
		return nil, nil
	}
	var payload xlsxThreadedCommentsXML
	if err := readZipXML(file, &payload); err != nil {
		return nil, err
	}
	comments := make([]workbookCellComment, 0, len(payload.Comments))
	for _, comment := range payload.Comments {
		rowIdx, colIdx, ok := a1CellRef(comment.Ref)
		if !ok {
			continue
		}
		comments = append(comments, workbookCellComment{
			SheetName:   sheetName,
			RowIndex:    rowIdx,
			ColumnIndex: colIdx,
			Text:        xlsxText(comment.Text),
			Author:      cleanXLSXCommentAuthor(persons[comment.PersonID]),
		})
	}
	return comments, nil
}

func appendWorkbookComments(target map[string][]workbookCellComment, comments []workbookCellComment) {
	for _, comment := range comments {
		comment.Text = visibleFeedbackComment(comment.Text)
		if strings.TrimSpace(comment.Text) == "" || strings.TrimSpace(comment.SheetName) == "" {
			continue
		}
		comment.Author = cleanXLSXCommentAuthor(comment.Author)
		existing := target[comment.SheetName]
		merged := false
		for idx := range existing {
			if existing[idx].RowIndex != comment.RowIndex || existing[idx].ColumnIndex != comment.ColumnIndex {
				continue
			}
			existing[idx].Text = joinCommentText(existing[idx].Text, comment.Text)
			existing[idx].Author = joinCommentAuthor(existing[idx].Author, comment.Author)
			merged = true
			break
		}
		if merged {
			target[comment.SheetName] = existing
			continue
		}
		target[comment.SheetName] = append(existing, comment)
	}
}

func cloneWorkbookComments(input map[string][]workbookCellComment) map[string][]workbookCellComment {
	if input == nil {
		return nil
	}
	output := make(map[string][]workbookCellComment, len(input))
	for sheetName, comments := range input {
		output[sheetName] = append([]workbookCellComment(nil), comments...)
	}
	return output
}

func readZipXML(file *zip.File, target any) error {
	if file == nil {
		return fmt.Errorf("arquivo XML ausente no XLSX")
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return xml.NewDecoder(reader).Decode(target)
}

func cleanXLSXPath(filePath string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	filePath = path.Clean(filePath)
	return strings.TrimPrefix(filePath, "/")
}

func resolveXLSXPath(baseFile string, target string) string {
	target = strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	if strings.HasPrefix(target, "/") {
		return cleanXLSXPath(target)
	}
	return cleanXLSXPath(path.Join(path.Dir(baseFile), target))
}

func relsPath(filePath string) string {
	dir, file := path.Split(filePath)
	return path.Join(dir, "_rels", file+".rels")
}

func isXLSXCommentRelationship(value string) bool {
	return strings.HasSuffix(strings.ToLower(value), "/comments")
}

func isXLSXThreadedCommentRelationship(value string) bool {
	return strings.Contains(strings.ToLower(value), "threadedcomment")
}

func xlsxAuthor(authors []string, authorID int) string {
	if authorID < 0 || authorID >= len(authors) {
		return ""
	}
	return cleanXLSXCommentAuthor(authors[authorID])
}

func xlsxText(value xlsxInnerXML) string {
	text := ""
	if strings.TrimSpace(value.InnerXML) == "" {
		text = strings.TrimSpace(value.CharData)
	} else {
		text = xmlText(value.InnerXML)
	}
	return cleanXLSXCommentText(text)
}

func xmlText(innerXML string) string {
	decoder := xml.NewDecoder(strings.NewReader("<root>" + innerXML + "</root>"))
	var builder strings.Builder
	inTextNode := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return strings.TrimSpace(innerXML)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "t" {
				inTextNode = true
			}
		case xml.EndElement:
			if typed.Name.Local == "t" {
				inTextNode = false
			}
		case xml.CharData:
			text := string(typed)
			if inTextNode || strings.TrimSpace(text) != "" {
				builder.WriteString(text)
			}
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(builder.String(), "\r\n", "\n"))
}

func a1CellRef(ref string) (int, int, bool) {
	ref = strings.TrimSpace(ref)
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		ref = ref[bang+1:]
	}
	if colon := strings.Index(ref, ":"); colon >= 0 {
		ref = ref[:colon]
	}
	ref = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""))
	if ref == "" {
		return 0, 0, false
	}

	lettersEnd := 0
	for lettersEnd < len(ref) && ref[lettersEnd] >= 'A' && ref[lettersEnd] <= 'Z' {
		lettersEnd++
	}
	if lettersEnd == 0 || lettersEnd == len(ref) {
		return 0, 0, false
	}

	colIdx := 0
	for _, letter := range ref[:lettersEnd] {
		colIdx = colIdx*26 + int(letter-'A'+1)
	}
	rowNumber, err := strconv.Atoi(ref[lettersEnd:])
	if err != nil || rowNumber <= 0 || colIdx <= 0 {
		return 0, 0, false
	}
	return rowNumber - 1, colIdx - 1, true
}

func joinCommentText(current string, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" || current == next || strings.Contains(current, next) {
		return current
	}
	return current + "\n\n" + next
}

func joinCommentAuthor(current string, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" || current == next {
		return fallbackText(next, current)
	}
	if next == "" || strings.Contains(current, next) {
		return current
	}
	return current + ", " + next
}

func cleanXLSXCommentAuthor(author string) string {
	author = authorDisplayName(author)
	if strings.HasPrefix(author, "tc={") && strings.HasSuffix(author, "}") {
		return ""
	}
	return author
}

func cleanXLSXCommentText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if strings.HasPrefix(text, "[Threaded comment]") {
		if idx := strings.LastIndex(text, "Comment:"); idx >= 0 {
			text = text[idx+len("Comment:"):]
		}
	}
	return trimCommentLines(text)
}

func trimCommentLines(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for idx := range lines {
		lines[idx] = strings.TrimSpace(lines[idx])
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
