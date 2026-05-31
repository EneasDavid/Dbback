package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (c *SheetsClient) LoadDriveCommentDebug(ctx context.Context) ([]DriveCommentDebug, error) {
	comments, err := c.driveCommentsForSpreadsheet(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]DriveCommentDebug, 0, len(comments))
	for _, comment := range comments {
		result = append(result, DriveCommentDebug{
			Text:        comment.Text,
			Author:      comment.Author,
			QuotedText:  comment.QuotedText,
			Anchor:      comment.Anchor,
			SheetID:     comment.SheetID,
			HasSheetID:  comment.HasSheetID,
			RowIndex:    comment.RowIndex,
			ColumnIndex: comment.ColumnIndex,
			HasCell:     comment.HasCell,
		})
	}
	return result, nil
}

type driveCommentsPayload struct {
	nextPageToken string
	comments      []driveCellComment
}

type driveCommentRecord struct {
	text    string
	author  string
	quoted  string
	anchor  string
	deleted bool
}

func decodeDriveComments(body []byte) (driveCommentsPayload, error) {
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Comments      []struct {
			Content string `json:"content"`
			Anchor  string `json:"anchor"`
			Deleted bool   `json:"deleted"`
			Author  struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
			QuotedFileContent struct {
				Value string `json:"value"`
			} `json:"quotedFileContent"`
			Replies []struct {
				Content string `json:"content"`
				Deleted bool   `json:"deleted"`
				Author  struct {
					DisplayName string `json:"displayName"`
				} `json:"author"`
			} `json:"replies"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return driveCommentsPayload{}, err
	}

	result := driveCommentsPayload{nextPageToken: payload.NextPageToken}
	for _, comment := range payload.Comments {
		result.appendDriveComment(driveCommentRecord{
			text:    comment.Content,
			author:  comment.Author.DisplayName,
			quoted:  comment.QuotedFileContent.Value,
			anchor:  comment.Anchor,
			deleted: comment.Deleted,
		})
		for _, reply := range comment.Replies {
			result.appendDriveComment(driveCommentRecord{
				text:    reply.Content,
				author:  fallbackText(reply.Author.DisplayName, comment.Author.DisplayName),
				quoted:  comment.QuotedFileContent.Value,
				anchor:  comment.Anchor,
				deleted: comment.Deleted || reply.Deleted,
			})
		}
	}
	return result, nil
}

func decodeDriveV2Comments(body []byte) (driveCommentsPayload, error) {
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			Content string `json:"content"`
			Anchor  string `json:"anchor"`
			Deleted bool   `json:"deleted"`
			Status  string `json:"status"`
			Author  struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
			Context struct {
				Value string `json:"value"`
			} `json:"context"`
			Replies []struct {
				Content string `json:"content"`
				Deleted bool   `json:"deleted"`
				Author  struct {
					DisplayName string `json:"displayName"`
				} `json:"author"`
			} `json:"replies"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return driveCommentsPayload{}, err
	}

	result := driveCommentsPayload{nextPageToken: payload.NextPageToken}
	for _, comment := range payload.Items {
		result.appendDriveComment(driveCommentRecord{
			text:    comment.Content,
			author:  comment.Author.DisplayName,
			quoted:  comment.Context.Value,
			anchor:  comment.Anchor,
			deleted: comment.Deleted,
		})
		for _, reply := range comment.Replies {
			result.appendDriveComment(driveCommentRecord{
				text:    reply.Content,
				author:  fallbackText(reply.Author.DisplayName, comment.Author.DisplayName),
				quoted:  comment.Context.Value,
				anchor:  comment.Anchor,
				deleted: comment.Deleted || reply.Deleted,
			})
		}
	}
	return result, nil
}

func (p *driveCommentsPayload) appendDriveComment(comment driveCommentRecord) {
	text := strings.TrimSpace(comment.text)
	if comment.deleted || text == "" {
		return
	}
	anchor := strings.TrimSpace(comment.anchor)
	sheetID, hasSheetID := driveCommentSheetID(anchor)
	rowIdx, colIdx, hasCell := driveCommentCell(anchor)
	p.comments = append(p.comments, driveCellComment{
		Text:        text,
		Author:      authorDisplayName(comment.author),
		QuotedText:  strings.TrimSpace(comment.quoted),
		Anchor:      anchor,
		SheetID:     sheetID,
		HasSheetID:  hasSheetID,
		RowIndex:    rowIdx,
		ColumnIndex: colIdx,
		HasCell:     hasCell,
	})
}

func driveCommentSheetID(anchor string) (int64, bool) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return 0, false
	}

	var payload struct {
		UID json.RawMessage `json:"uid"`
	}
	if err := json.Unmarshal([]byte(anchor), &payload); err != nil || len(payload.UID) == 0 {
		return 0, false
	}

	var number int64
	if err := json.Unmarshal(payload.UID, &number); err == nil {
		return number, true
	}

	var text string
	if err := json.Unmarshal(payload.UID, &text); err == nil {
		var parsed int64
		if _, err := fmt.Sscan(strings.TrimSpace(text), &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func driveCommentCell(anchor string) (int, int, bool) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return 0, 0, false
	}

	var payload any
	if err := json.Unmarshal([]byte(anchor), &payload); err != nil {
		return 0, 0, false
	}
	return findDriveCommentCell(payload)
}

func findDriveCommentCell(value any) (int, int, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if rowIdx, ok := driveCommentNumberField(typed, "r", "row", "rowIndex", "startRowIndex"); ok {
			if colIdx, ok := driveCommentNumberField(typed, "c", "col", "column", "columnIndex", "startColumnIndex"); ok {
				return rowIdx, colIdx, true
			}
		}
		for _, child := range typed {
			if rowIdx, colIdx, ok := findDriveCommentCell(child); ok {
				return rowIdx, colIdx, true
			}
		}
	case []any:
		for _, child := range typed {
			if rowIdx, colIdx, ok := findDriveCommentCell(child); ok {
				return rowIdx, colIdx, true
			}
		}
	}
	return 0, 0, false
}

func driveCommentNumberField(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if idx, ok := driveCommentNumber(value); ok {
			return idx, true
		}
	}
	return 0, false
}

func driveCommentNumber(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		idx := int(typed)
		if typed >= 0 && float64(idx) == typed {
			return idx, true
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed >= 0 {
			return parsed, true
		}
	}
	return 0, false
}

func fallbackText(preferred string, fallback string) string {
	if strings.TrimSpace(preferred) != "" {
		return preferred
	}
	return fallback
}
