package app

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func (c *SheetsClient) LoadSheetComments(ctx context.Context, sheetNames []string) (map[string][]SheetComment, error) {
	byCell, err := c.commentsForSheets(ctx, sheetNames)
	if err != nil {
		return nil, err
	}
	result := map[string][]SheetComment{}
	for sheetName, comments := range byCell {
		for cell, comment := range comments {
			result[sheetName] = append(result[sheetName], SheetComment{
				Cell:   cell,
				Text:   comment.Text,
				Author: comment.Author,
			})
		}
		sort.Slice(result[sheetName], func(i, j int) bool {
			return result[sheetName][i].Cell < result[sheetName][j].Cell
		})
	}
	return result, nil
}

func (c *SheetsClient) LoadSheetFeedbacks(ctx context.Context, sheetNames []string) (map[string][]SheetComment, error) {
	result := map[string][]SheetComment{}
	for _, sheetName := range sheetNames {
		grid, err := c.loadSheet(ctx, sheetName)
		if err != nil {
			return nil, err
		}
		result[sheetName] = grid.feedbacks()
	}
	return result, nil
}

func (g *sheetGrid) feedbacks() []SheetComment {
	var comments []SheetComment
	appendRow := func(rowIdx int, notes []string, authors []string) {
		for colIdx, note := range notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				continue
			}
			comments = append(comments, SheetComment{
				Cell:   cell,
				Text:   note,
				Author: noteAt(authors, colIdx),
			})
		}
	}

	appendRow(g.headerRow, g.notes, g.noteAuthors)
	for idx, rowIdx := range g.rowIndices {
		appendRow(rowIdx, g.rowNotes[idx], g.rowNoteAuthors[idx])
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].Cell < comments[j].Cell
	})
	return comments
}

func (c *SheetsClient) commentsForSheets(ctx context.Context, sheetNames []string) (map[string]map[string]cellComment, error) {
	wanted := normalizeSheetSet(sheetNames)
	if len(wanted) == 0 {
		return map[string]map[string]cellComment{}, nil
	}

	c.mu.Lock()
	if c.comments.bySheet != nil && time.Now().Before(c.comments.expires) {
		cached := filterComments(c.comments.bySheet, wanted)
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	value, err, _ := c.group.Do("xlsx-comments:"+c.cfg.SpreadsheetID, func() (interface{}, error) {
		data, err := c.exportSpreadsheetXLSX(ctx)
		if err != nil {
			return nil, err
		}
		comments, err := parseXLSXComments(data, nil)
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
	return filterComments(value.(map[string]map[string]cellComment), wanted), nil
}

func parseXLSXComments(data []byte, sheetNames []string) (map[string]map[string]cellComment, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	wanted := normalizeSheetSet(sheetNames)
	result := map[string]map[string]cellComment{}
	for _, sheetName := range file.GetSheetList() {
		if len(wanted) > 0 && !wanted[sheetName] {
			continue
		}
		comments, err := file.GetComments(sheetName)
		if err != nil {
			return nil, err
		}
		for _, comment := range comments {
			text := strings.TrimSpace(commentText(comment))
			if text == "" {
				continue
			}
			if result[sheetName] == nil {
				result[sheetName] = map[string]cellComment{}
			}
			result[sheetName][comment.Cell] = cellComment{Text: text, Author: strings.TrimSpace(comment.Author)}
		}
	}
	return result, nil
}

func commentText(comment excelize.Comment) string {
	if strings.TrimSpace(comment.Text) != "" {
		return comment.Text
	}
	var builder strings.Builder
	for _, paragraph := range comment.Paragraph {
		builder.WriteString(paragraph.Text)
	}
	return builder.String()
}

func normalizeSheetSet(sheetNames []string) map[string]bool {
	set := map[string]bool{}
	for _, sheetName := range sheetNames {
		sheetName = strings.TrimSpace(sheetName)
		if sheetName != "" {
			set[sheetName] = true
		}
	}
	return set
}

func filterComments(comments map[string]map[string]cellComment, wanted map[string]bool) map[string]map[string]cellComment {
	filtered := map[string]map[string]cellComment{}
	for sheetName := range wanted {
		if byCell := comments[sheetName]; len(byCell) > 0 {
			filtered[sheetName] = byCell
		}
	}
	return filtered
}
