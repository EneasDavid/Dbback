package app

import (
	"context"
	"encoding/json"
	"fmt"
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
			Text:       comment.Text,
			Author:     comment.Author,
			QuotedText: comment.QuotedText,
			Anchor:     comment.Anchor,
			SheetID:    comment.SheetID,
			HasSheetID: comment.HasSheetID,
		})
	}
	return result, nil
}

type driveCommentsPayload struct {
	nextPageToken string
	comments      []driveCellComment
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
		} `json:"comments"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return driveCommentsPayload{}, err
	}

	result := driveCommentsPayload{nextPageToken: payload.NextPageToken}
	for _, comment := range payload.Comments {
		text := strings.TrimSpace(comment.Content)
		quoted := strings.TrimSpace(comment.QuotedFileContent.Value)
		if comment.Deleted || text == "" {
			continue
		}
		sheetID, hasSheetID := driveCommentSheetID(comment.Anchor)
		result.comments = append(result.comments, driveCellComment{
			Text:       text,
			Author:     authorDisplayName(comment.Author.DisplayName),
			QuotedText: quoted,
			Anchor:     strings.TrimSpace(comment.Anchor),
			SheetID:    sheetID,
			HasSheetID: hasSheetID,
		})
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
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return driveCommentsPayload{}, err
	}

	result := driveCommentsPayload{nextPageToken: payload.NextPageToken}
	for _, comment := range payload.Items {
		text := strings.TrimSpace(comment.Content)
		quoted := strings.TrimSpace(comment.Context.Value)
		if comment.Deleted || text == "" {
			continue
		}
		sheetID, hasSheetID := driveCommentSheetID(comment.Anchor)
		result.comments = append(result.comments, driveCellComment{
			Text:       text,
			Author:     authorDisplayName(comment.Author.DisplayName),
			QuotedText: quoted,
			Anchor:     strings.TrimSpace(comment.Anchor),
			SheetID:    sheetID,
			HasSheetID: hasSheetID,
		})
	}
	return result, nil
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
